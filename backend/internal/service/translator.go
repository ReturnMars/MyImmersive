package service

import (
	"context"
	"fmt"
	"strings"

	"MyImmersive/backend/config"
	"MyImmersive/backend/internal/cache"

	openai "github.com/sashabaranov/go-openai"
)

// TranslationResult 翻译结果
type TranslationResult struct {
	Translations []string
	Error        error
}

// Translator 翻译服务
type Translator struct {
	client *openai.Client
	cache  cache.Cache
}

// NewTranslator 创建翻译器实例
func NewTranslator(c cache.Cache) *Translator {
	cfg := config.Load()
	oaiConfig := openai.DefaultConfig(cfg.DeepSeekAPIKey)
	oaiConfig.BaseURL = cfg.DeepSeekURL
	return &Translator{
		client: openai.NewClientWithConfig(oaiConfig),
		cache:  c,
	}
}

// Translate 执行翻译 (带缓存)
func (t *Translator) Translate(ctx context.Context, segments []string, pageURL string) TranslationResult {
	if len(segments) == 0 {
		return TranslationResult{Translations: []string{}}
	}

	// 1. 检查缓存，收集未命中的段落
	results := make([]string, len(segments))
	uncached := []string{}
	uncachedIdx := []int{}

	for i, seg := range segments {
		key := cache.HashKey(seg)
		if val, ok := t.cache.Get(key); ok {
			results[i] = val // 命中缓存
		} else {
			uncached = append(uncached, seg)
			uncachedIdx = append(uncachedIdx, i)
		}
	}

	hitCount := len(segments) - len(uncached)
	fmt.Printf("[Cache] Hit %d segments, Miss %d segments\n", hitCount, len(uncached))

	// 2. 如果全部命中，直接返回
	if len(uncached) == 0 {
		return TranslationResult{Translations: results}
	}

	// 3. 调用 AI 翻译未命中的段落
	translated := t.translateWithAI(ctx, uncached, pageURL)
	if translated.Error != nil {
		return translated
	}

	// 4. 合并结果并写入缓存
	for j, trans := range translated.Translations {
		idx := uncachedIdx[j]
		results[idx] = trans
		// 写入缓存
		key := cache.HashKey(uncached[j])
		if err := t.cache.Set(key, trans); err != nil {
			fmt.Printf("[Cache] Failed to store: %v\n", err)
		}
	}

	fmt.Printf("[Cache] Stored %d new translations\n", len(translated.Translations))
	return TranslationResult{Translations: results}
}

// translateWithAI 调用 AI API 翻译
func (t *Translator) translateWithAI(ctx context.Context, segments []string, pageURL string) TranslationResult {
	systemPrompt := `你是一个翻译引擎。请将提供的文本段落翻译为中文。
规则：
1. 必须保留对应的段落数量。
2. 每个翻译段落之间必须使用 "---" 独立成行作为分隔符。
3. 保持专业术语、代码变量、品牌名称原样不翻译。
4. 仅返回翻译内容，不要添加解释。`

	if strings.Contains(pageURL, "github.com") {
		systemPrompt += "\n5. 针对 GitHub：保留 PR, Issue, Commit 等术语。"
	}

	userPrompt := strings.Join(segments, "\n---\n")

	fmt.Printf("[Translator] Sending %d segments to DeepSeek...\n", len(segments))

	resp, err := t.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: "deepseek-chat",
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userPrompt},
			},
		},
	)

	if err != nil {
		fmt.Printf("[Translator] API Error: %v\n", err)
		return TranslationResult{Error: fmt.Errorf("API 请求失败: %w", err)}
	}

	translatedText := resp.Choices[0].Message.Content
	fmt.Printf("[Translator] API returned %d chars\n", len(translatedText))

	return TranslationResult{
		Translations: t.parseTranslation(translatedText, len(segments)),
	}
}

// parseTranslation 解析翻译结果
func (t *Translator) parseTranslation(text string, expectedCount int) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	rawResults := strings.Split(text, "---")

	var finalResults []string
	for _, r := range rawResults {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			finalResults = append(finalResults, trimmed)
		}
	}

	// 数量匹配校验与兜底
	if len(finalResults) != expectedCount {
		fmt.Printf("[Translator] Count mismatch: Got %d, Expected %d. Trying fallback...\n", len(finalResults), expectedCount)
		lines := strings.Split(strings.TrimSpace(text), "\n")
		var validLines []string
		for _, l := range lines {
			lTrim := strings.TrimSpace(l)
			if lTrim != "---" && lTrim != "" {
				validLines = append(validLines, lTrim)
			}
		}
		if len(validLines) == expectedCount {
			finalResults = validLines
		}
	}

	fmt.Printf("[Translator] Final segments returned: %d\n", len(finalResults))
	return finalResults
}
