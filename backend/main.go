package main

import (
	"log"

	"MyImmersive/backend/config"
	"MyImmersive/backend/internal/handler"
	"MyImmersive/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化 Gin
	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// 注册路由
	translateHandler := handler.NewTranslateHandler()
	r.POST("/api/translate", translateHandler.Handle)

	// 启动服务器
	log.Printf("[Server] Starting on port %s...\n", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("[Server] Failed to start: %v", err)
	}
}
