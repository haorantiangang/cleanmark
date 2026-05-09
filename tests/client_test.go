package tests

import (
	"bytes"
	"cleanmark/config"
	"cleanmark/internal/handler"
	"cleanmark/internal/middleware"
	"cleanmark/internal/model"
	"cleanmark/internal/repository"
	"cleanmark/internal/service"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupClientTestRouter() (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:         "test-client-secret-12345",
			ExpireDuration: time.Hour,
			RefreshExpire:  time.Hour * 24 * 7,
		},
		RateLimit: config.RateLimitConfig{
			FreeUserRPM: 10,
			VipUserRPM:  60,
		},
	}

	repository.InitTestDB()

	userService := service.NewUserService(&cfg.JWT)
	taskService := service.NewTaskService(userService)

	userHandler := handler.NewUserHandler(userService)
	taskHandler := handler.NewTaskHandler(taskService)
	paymentHandler := handler.NewPaymentHandler()

	router := gin.New()
	router.Use(gin.Recovery())

	api := router.Group("/api/v1")
	{
		public := api.Group("")
		{
			public.POST("/auth/wechat/login", userHandler.WechatLogin)
			public.POST("/auth/phone/login", userHandler.PhoneLogin)
			public.POST("/auth/refresh", userHandler.RefreshToken)
			public.GET("/quota", middleware.OptionalAuth(&cfg.JWT), userHandler.GetQuotaInfo)
			public.POST("/detect/platform", taskHandler.DetectPlatform)
			public.GET("/stats", taskHandler.GetStats)
			public.GET("/payment/products", paymentHandler.GetProducts)
		}

		protected := api.Group("")
		protected.Use(middleware.JWTAuth(&cfg.JWT))
		{
			protected.GET("/user/info", userHandler.GetUserInfo)
			protected.POST("/tasks", taskHandler.Parse)
			protected.POST("/tasks/batch", taskHandler.BatchParse)
			protected.GET("/tasks", taskHandler.GetTaskList)
			protected.GET("/tasks/:id", taskHandler.GetTask)
			protected.DELETE("/tasks/:id", taskHandler.DeleteTask)
			protected.POST("/payment/create-order", paymentHandler.CreateOrder)
			protected.GET("/payment/orders", paymentHandler.GetOrderList)
		}
	}

	return router, cfg.JWT.Secret
}

func getTestUserToken(secret string, userID uint) (string, error) {
	jwtCfg := &config.JWTConfig{
		Secret:         secret,
		ExpireDuration: time.Hour,
		RefreshExpire:  time.Hour * 24 * 7,
	}
	return middleware.GenerateToken(userID, jwtCfg)
}

// ==================== 用户认证测试 ====================

func TestClient_WechatLogin(t *testing.T) {
	router, _ := setupClientTestRouter()

	t.Run("成功登录", func(t *testing.T) {
		data := `{"code":"test_code_001"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(data))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("微信登录应该返回200，得到: %d", w.Code)
		}
	})

	t.Run("缺少code参数", func(t *testing.T) {
		data := `{}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(data))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Errorf("缺少code应该返回400，得到: %d", w.Code)
		}
	})
}

func TestClient_PhoneLogin(t *testing.T) {
	router, _ := setupClientTestRouter()

	data := `{"phone":"13800138000","code":"123456"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/phone/login", bytes.NewBufferString(data))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("手机号登录应该返回200，得到: %d", w.Code)
	}
}

func TestRefreshToken(t *testing.T) {
	router, secret := setupClientTestRouter()

	token, err := getTestUserToken(secret, 999)
	if err != nil {
		t.Fatalf("生成Token失败: %v", err)
	}

	data := fmt.Sprintf(`{"refresh_token":"%s"}`, token)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBufferString(data))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("刷新Token应该返回200，得到: %d", w.Code)
	}
}

// ==================== 用户信息测试 ====================

func TestClient_GetUserInfo(t *testing.T) {
	router, secret := setupClientTestRouter()

	token, _ := getTestUserToken(secret, 888)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/user/info", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取用户信息失败: %d, %s", w.Code, w.Body.String())
	}
}

func TestGetUserInfo_Unauthorized(t *testing.T) {
	router, _ := setupClientTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/user/info", nil)
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("未认证应该返回401，得到: %d", w.Code)
	}
}

// ==================== 额度查询测试 ====================

func TestGetQuotaInfo(t *testing.T) {
	router, secret := setupClientTestRouter()

	t.Run("带Token查询", func(t *testing.T) {
		token, _ := getTestUserToken(secret, 777)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/quota", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("查询额度应该返回200，得到: %d", w.Code)
		}
	})

	t.Run("不带Token查询", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/quota", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("未认证查询额度应该返回200（默认值），得到: %d", w.Code)
		}
	})
}

// ==================== 平台检测测试 ====================

func TestClient_DetectPlatform(t *testing.T) {
	router, _ := setupClientTestRouter()

	testCases := []struct {
		url      string
		expected string
	}{
		{"https://www.douyin.com/video/123", "douyin"},
		{"https://www.kuaishou.com/short-video/abc", "kuaishou"},
		{"https://www.xiaohongshu.com/discovery/item/xyz", "xiaohongshu"},
		{"https://www.bilibili.com/video/BV1xx411c7mD", "bilibili"},
		{"https://weibo.com/tv/show/1034:456789", "weibo"},
	}

	for _, tc := range testCases {
		t.Run(tc.url[:20], func(t *testing.T) {
			data := fmt.Sprintf(`{"url":"%s"}`, tc.url)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/detect/platform", bytes.NewBufferString(data))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != 200 {
				t.Fatalf("平台检测失败(%s): %d", tc.expected, w.Code)
			}

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			result := response["data"].(map[string]interface{})

			if result["platform"] != tc.expected {
				t.Errorf("期望平台%s，得到%v", tc.expected, result["platform"])
			}
		})
	}
}

// ==================== 任务处理测试 ====================

func TestParseTask(t *testing.T) {
	router, secret := setupClientTestRouter()

	token, _ := getTestUserToken(secret, 555)

	data := `{"url":"https://www.douyin.com/video/test_task","quality":"hd"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/tasks", bytes.NewBufferString(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("创建任务失败: %d, %s", w.Code, w.Body.String())
	}
}

func TestBatchParseTask(t *testing.T) {
	router, secret := setupClientTestRouter()

	token, _ := getTestUserToken(secret, 444)

	data := `{"urls":["https://www.douyin.com/batch1","https://www.kuaishou.com/batch2","https://www.bilibili.com/batch3"]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/tasks/batch", bytes.NewBufferString(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("批量创建任务失败: %d, %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	result := response["data"].(map[string]interface{})
	tasks := result["tasks"].([]interface{})

	if len(tasks) != 3 {
		t.Errorf("批量任务数量应该是3，得到: %d", len(tasks))
	}
}

func TestParseTask_Unauthorized(t *testing.T) {
	router, _ := setupClientTestRouter()

	data := `{"url":"https://test.com/unauth"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/tasks", bytes.NewBufferString(data))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("未认证创建任务应该返回401，得到: %d", w.Code)
	}
}

// ==================== 任务列表和详情测试 ====================

func TestGetTaskList(t *testing.T) {
	router, secret := setupClientTestRouter()
	db := repository.GetTestDB()

	userID := uint(111)
	user := model.User{OpenID: fmt.Sprintf("user_%d", userID)}
	db.Create(&user)

	for i := 0; i < 5; i++ {
		task := model.Task{
			UserID:       user.ID,
			PlatformType: []string{"douyin", "kuaishou", "bilibili"}[i%3],
			Status:       "success",
		}
		db.Create(&task)
	}

	token, _ := getTestUserToken(secret, user.ID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/tasks?page=1&page_size=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取任务列表失败: %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})
	list := data["list"].([]interface{})

	if len(list) < 5 {
		t.Errorf("任务数量不足: %d", len(list))
	}
}

func TestGetTaskDetail(t *testing.T) {
	router, secret := setupClientTestRouter()
	db := repository.GetTestDB()

	userID := uint(100)
	user := model.User{OpenID: fmt.Sprintf("detail_user_%d", userID)}
	db.Create(&user)

	task := model.Task{
		UserID:       user.ID,
		PlatformType: "douyin",
		Status:       "success",
		Title:        "详情测试任务",
		CleanURL:    "https://result.test.com/video.mp4",
	}
	db.Create(&task)

	token, _ := getTestUserToken(secret, user.ID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/tasks/%d", task.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取任务详情失败: %d, %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})

	if data["title"] != "详情测试任务" {
		t.Errorf("任务标题不正确: %v", data["title"])
	}
}

func TestDeleteTask(t *testing.T) {
	router, secret := setupClientTestRouter()
	db := repository.GetTestDB()

	userID := uint(99)
	user := model.User{OpenID: fmt.Sprintf("delete_user_%d", userID)}
	db.Create(&user)

	task := model.Task{UserID: user.ID, PlatformType: "douyin", Status: "success"}
	db.Create(&task)

	token, _ := getTestUserToken(secret, user.ID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/tasks/%d", task.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("删除任务失败: %d, %s", w.Code, w.Body.String())
	}
}

// ==================== 支付功能测试 ====================

func TestGetPaymentProducts(t *testing.T) {
	router, _ := setupClientTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/payment/products", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取支付产品失败: %d, %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	data := response["data"].(map[string]interface{})
	products := data["products"].([]interface{})

	if len(products) < 4 {
		t.Errorf("产品数量不足: %d", len(products))
	}
}

func TestCreateOrder(t *testing.T) {
	router, secret := setupClientTestRouter()

	token, _ := getTestUserToken(secret, 666)

	data := `{"product_type":"monthly","pay_method":"wechat"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/payment/create-order", bytes.NewBufferString(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("创建订单失败: %d, %s", w.Code, w.Body.String())
	}
}

// ==================== 统计数据测试 ====================

func TestGetStats(t *testing.T) {
	router, _ := setupClientTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stats", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取统计失败: %d, %s", w.Code, w.Body.String())
	}
}

// ==================== 错误处理测试 ====================

func TestErrorHandling(t *testing.T) {
	router, _ := setupClientTestRouter()

	t.Run("无效JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", strings.NewReader("invalid"))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != 400 {
			t.Errorf("无效JSON应该返回400，得到: %d", w.Code)
		}
	})

	t.Run("不存在接口", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/nonexistent", nil)
		router.ServeHTTP(w, req)

		if w.Code != 404 {
			t.Errorf("不存在接口应该返回404，得到: %d", w.Code)
		}
	})
}

// ==================== 并发测试 ====================

func TestConcurrentRequests(t *testing.T) {
	router, secret := setupClientTestRouter()

	token, _ := getTestUserToken(secret, 111)

	type result struct {
		code int
	}

	results := make(chan result, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/quota", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(w, req)

			results <- result{code: w.Code}
		}(i)
	}

	successCount := 0
	for i := 0; i < 10; i++ {
		r := <-results
		if r.code == 200 {
			successCount++
		}
	}

	if successCount != 10 {
		t.Errorf("10个并发请求应该全部成功，成功数: %d", successCount)
	}
}
