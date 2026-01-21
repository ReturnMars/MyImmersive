package handler

import (
	"context"
	"net/http"

	"MyImmersive/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// TransRequest 翻译请求
type TransRequest struct {
	URL      string   `json:"url"`
	Segments []string `json:"segments"`
}

// TransResponse 翻译响应
type TransResponse struct {
	Translations []string `json:"translations"`
}

// TranslateHandler 翻译 HTTP 处理器
type TranslateHandler struct {
	translator *service.Translator
}

// NewTranslateHandler 创建处理器实例
func NewTranslateHandler() *TranslateHandler {
	return &TranslateHandler{
		translator: service.NewTranslator(),
	}
}

// Handle 处理翻译请求
func (h *TranslateHandler) Handle(c *gin.Context) {
	var req TransRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON 解析失败"})
		return
	}

	if len(req.Segments) == 0 {
		c.JSON(http.StatusOK, TransResponse{Translations: []string{}})
		return
	}

	result := h.translator.Translate(context.Background(), req.Segments, req.URL)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, TransResponse{Translations: result.Translations})
}
