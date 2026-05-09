package routes

import (
	"cleanmark/config"
	"cleanmark/internal/handler"
	"cleanmark/internal/middleware"
	"cleanmark/internal/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.Config) *gin.Engine {
	userService := service.NewUserService(&cfg.JWT)
	taskService := service.NewTaskService(userService)

	userHandler := handler.NewUserHandler(userService)
	taskHandler := handler.NewTaskHandler(taskService)
	paymentHandler := handler.NewPaymentHandler()
	adminHandler := handler.NewAdminHandler()

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * 3600,
	}))

	r.Static("/static", "./web")
	r.Static("/admin", "./admin")
	r.StaticFile("/", "./web/index.html")

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "cleanmark-api",
			"version": "2.0.0",
		})
	})

	api := r.Group("/api/v1")
	{
		public := api.Group("")
		{
			public.POST("/auth/wechat/login", userHandler.WechatLogin)
			public.POST("/auth/phone/login", userHandler.PhoneLogin)
			public.POST("/auth/refresh", userHandler.RefreshToken)
			public.GET("/quota", middleware.OptionalAuth(&cfg.JWT), userHandler.GetQuotaInfo)
			public.POST("/detect/platform", taskHandler.DetectPlatform)
			public.GET("/download/:taskId", taskHandler.DownloadProxy)
			public.GET("/stats", taskHandler.GetStats)

			payment := public.Group("/payment")
			{
				payment.GET("/products", paymentHandler.GetProducts)
				payment.POST("/wechat/callback", paymentHandler.HandleWechatCallback)
				payment.POST("/alipay/callback", paymentHandler.HandleAlipayCallback)
			}

			admin := public.Group("/admin")
			{
				admin.POST("/login", adminHandler.AdminLogin)
				admin.GET("/dashboard", adminHandler.GetDashboardStats)
				admin.GET("/system/info", adminHandler.GetSystemInfo)
			}
		}

		protected := api.Group("")
		protected.Use(middleware.JWTAuth(&cfg.JWT))
		{
			protected.GET("/user/info", userHandler.GetUserInfo)
			
			tasks := protected.Group("/tasks")
			{
				tasks.POST("", middleware.RateLimit(cfg.RateLimit.VipUserRPM), taskHandler.Parse)
				tasks.POST("/batch", middleware.RateLimit(cfg.RateLimit.VipUserRPM), taskHandler.BatchParse)
				tasks.GET("", taskHandler.GetTaskList)
				tasks.GET("/:id", taskHandler.GetTask)
				tasks.DELETE("/:id", taskHandler.DeleteTask)
			}

			payment := protected.Group("/payment")
			{
				payment.POST("/create-order", paymentHandler.CreateOrder)
				payment.GET("/orders", paymentHandler.GetOrderList)
				payment.GET("/order/status", paymentHandler.CheckOrderStatus)
			}
		}

		adminProtected := api.Group("/admin")
		adminProtected.Use(middleware.AdminAuth())
		{
			adminProtected.GET("/users", adminHandler.GetUsers)
			adminProtected.GET("/users/:id", adminHandler.GetUserDetail)
			adminProtected.PUT("/users/:id/vip", adminHandler.UpdateUserVip)

			adminProtected.GET("/tasks", adminHandler.GetTasks)

			adminProtected.GET("/orders", adminHandler.GetOrders)

			adminProtected.GET("/analytics", adminHandler.GetAnalyticsData)
		}
	}

	return r
}
