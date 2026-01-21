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

func (t *Translator) translateWithAI(ctx context.Context, segments []string, pageURL string) TranslationResult {
	systemPrompt := `你是一位专业的翻译专家，精通中英互译，擅长技术文档和网页内容的本地化翻译。

翻译规则：
1. **段落对应**：输入 N 个段落，必须输出 N 个对应的翻译段落，使用 "---" 独立成行作为分隔符。
2. **自然流畅**：译文应符合中文表达习惯，避免生硬的直译和"机翻腔"。
3. **保留原样**：
   - 代码、变量名、函数名 (如 useState, onClick)
   - 品牌名 (如 React, Vue, Tailwind, shadcn)
   - 专有名词和缩写 (如 API, CSS, HTML, JSON)
   - 占位符 (如 {{0}}, {{1}} - 这些必须原样保留不翻译)
4. **术语一致**：同一术语在全文保持翻译一致性。
5. **格式保留**：保持原文的格式结构，如列表项、标题层级等。
6. **仅输出译文**：不要添加任何解释、注释或额外内容。

示例：
输入: "The useState hook returns a stateful value."
输出: "useState Hook 返回一个有状态的值。"`

	// 针对特定网站的优化
	if strings.Contains(pageURL, "github.com") {
		systemPrompt += "\n\n额外规则 (GitHub)：保留 Pull Request, Issue, Commit, Branch, Fork, Star 等术语。"
	} else if strings.Contains(pageURL, "developer.") || strings.Contains(pageURL, "docs.") {
		systemPrompt += "\n\n额外规则 (技术文档)：保持技术准确性，优先使用业界通用译法。"
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
