package main

import (
	"log"

	"MyImmersive/backend/config"
	"MyImmersive/backend/internal/cache"
	"MyImmersive/backend/internal/handler"
	"MyImmersive/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化缓存
	translationCache, err := cache.NewBadgerCache("./data/cache")
	if err != nil {
		log.Fatalf("[Server] Failed to init cache: %v", err)
	}
	defer translationCache.Close()

	// 初始化 Gin
	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// 注册路由 (注入缓存)
	translateHandler := handler.NewTranslateHandler(translationCache)
	r.POST("/api/translate", translateHandler.Handle)

	// 启动服务器
	log.Printf("[Server] Starting on port %s...\n", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("[Server] Failed to start: %v", err)
	}
}
