package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	openai "github.com/sashabaranov/go-openai"
)

// TransRequest 接收前端发送的翻译请求
type TransRequest struct {
	URL      string   `json:"url"`
	Segments []string `json:"segments"`
}

// TransResponse 返回给前端的翻译结果
type TransResponse struct {
	Translations []string `json:"translations"`
}

const (
	DeepSeekAPIKey = "sk-af37a5e305b945c3b000ed2aefeea092"
	DeepSeekURL    = "https://api.deepseek.com"
)

func main() {
	r := gin.Default()
	r.Use(gin.Recovery())

	// 允许跨域 (CORS)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.POST("/api/translate", handleTranslate)

	r.Run(":8080")
}

func handleTranslate(c *gin.Context) {
	fmt.Printf("[DEBUG] --- New Translation Request ---\n")

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Printf("[DEBUG] Read Body Error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取请求体"})
		return
	}

	var req TransRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		fmt.Printf("[DEBUG] JSON Unmarshal Error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON 解析失败"})
		return
	}

	if len(req.Segments) == 0 {
		c.JSON(http.StatusOK, TransResponse{Translations: []string{}})
		return
	}

	// 初始化 OpenAI 兼容客户端
	config := openai.DefaultConfig(DeepSeekAPIKey)
	config.BaseURL = DeepSeekURL
	client := openai.NewClientWithConfig(config)

	systemPrompt := `你是一个翻译引擎。请将提供的文本段落翻译为中文。
规则：
1. 必须保留对应的段落数量。
2. 每个翻译段落之间必须使用 "---" 独立成行作为分隔符。
3. 保持专业术语和代码变量原样。
4. 仅返回翻译内容。`

	if strings.Contains(req.URL, "github.com") {
		systemPrompt += "\n5. 针对 GitHub：保留 PR, Issue, Commit 等。"
	}

	userPrompt := strings.Join(req.Segments, "\n---\n")

	fmt.Printf("[DEBUG] Sending %d segments to DeepSeek...\n", len(req.Segments))

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: "deepseek-chat",
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userPrompt},
			},
		},
	)

	if err != nil {
		fmt.Printf("[DEBUG] API Error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "API 请求失败"})
		return
	}

	translatedText := resp.Choices[0].Message.Content
	fmt.Printf("[DEBUG] API returned %d chars\n", len(translatedText))

	// 解析结果
	translatedText = strings.ReplaceAll(translatedText, "\r\n", "\n")
	rawResults := strings.Split(translatedText, "---")

	var finalResults []string
	for _, r := range rawResults {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			finalResults = append(finalResults, trimmed)
		}
	}

	// 数量匹配校验与兜底
	if len(finalResults) != len(req.Segments) {
		fmt.Printf("[DEBUG] Count mismatch: Got %d, Expected %d. Trying fallback...\n", len(finalResults), len(req.Segments))
		lines := strings.Split(strings.TrimSpace(translatedText), "\n")
		var validLines []string
		for _, l := range lines {
			lTrim := strings.TrimSpace(l)
			if lTrim != "---" && lTrim != "" {
				validLines = append(validLines, lTrim)
			}
		}
		if len(validLines) == len(req.Segments) {
			finalResults = validLines
		}
	}

	fmt.Printf("[DEBUG] Final segments returned: %d\n", len(finalResults))
	c.JSON(http.StatusOK, TransResponse{Translations: finalResults})
}
