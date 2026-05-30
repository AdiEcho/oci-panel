package main

import (
	"log"

	"github.com/adiecho/oci-panel/internal/config"
	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/middleware"
	"github.com/adiecho/oci-panel/internal/router"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// 必须先初始化数据库：InitJwtSecret 在未配置密钥时会把自动生成的密钥持久化到数据库。
	if err := database.InitDB(cfg.Database.DSN); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	middleware.InitJwtSecret(cfg)

	r := gin.Default()
	services := router.Setup(r, cfg)

	// 启动定时任务服务
	services.Scheduler.Start()
	defer services.Scheduler.Stop()

	// 启动创建实例任务服务
	services.Task.Start()
	defer services.Task.Stop()

	// 启动 Telegram Bot（如果已配置并启用）
	_, _, tgEnabled := services.Telegram.GetConfig()
	if tgEnabled {
		services.Telegram.StartBot()
		defer services.Telegram.StopBot()
	}

	log.Printf("Server starting on port %s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
