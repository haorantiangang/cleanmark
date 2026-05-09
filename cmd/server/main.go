package main

import (
	"cleanmark/config"
	"cleanmark/internal/repository"
	"cleanmark/internal/routes"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	if err := repository.Init(&cfg.Database); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	fmt.Println("✅ 数据库连接成功")

	r := routes.SetupRouter(cfg)

	gin.SetMode(gin.ReleaseMode)

	fmt.Printf("🚀 CleanMark API 服务启动在端口 %s\n", cfg.Server.Port)
	fmt.Println("📚 API文档: http://localhost:8080/api/v1/health")
	
	if err := r.Run(cfg.Server.Port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
