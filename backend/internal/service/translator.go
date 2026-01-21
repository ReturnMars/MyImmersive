package service

import (
	"context"
	"fmt"
	"strings"

	"MyImmersive/backend/config"

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
}

// NewTranslator 创建翻译器实例
func NewTranslator() *Translator {
	cfg := config.Load()
	oaiConfig := openai.DefaultConfig(cfg.DeepSeekAPIKey)
	oaiConfig.BaseURL = cfg.DeepSeekURL
	return &Translator{
		client: openai.NewClientWithConfig(oaiConfig),
	}
}

// Translate 执行翻译
func (t *Translator) Translate(ctx context.Context, segments []string, pageURL string) TranslationResult {
	if len(segments) == 0 {
		return TranslationResult{Translations: []string{}}
	}

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

	// 解析结果
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
