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

func setupMiniProgramTestRouter() (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:         "test-miniprogram-secret-67890",
			ExpireDuration: time.Hour * 2,
			RefreshExpire:  time.Hour * 24 * 30,
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
			public.GET("/quota", middleware.OptionalAuth(&cfg.JWT), userHandler.GetQuotaInfo)
			public.POST("/detect/platform", taskHandler.DetectPlatform)
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
		}
	}

	return router, cfg.JWT.Secret
}

// ==================== 小程序登录流程测试 ====================

func TestMiniProgram_LoginFlow(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()

	t.Run("微信登录", func(t *testing.T) {
		data := `{"code":"mp_login_001"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(data))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("小程序登录失败: %d, %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		result := response["data"].(map[string]interface{})

		if result["token"] == nil || result["token"] == "" {
			t.Error("登录必须返回Token")
		}
	})

	t.Run("登录后查询用户信息", func(t *testing.T) {
		loginData := `{"code":"mp_login_002"}`
		loginW := httptest.NewRecorder()
		loginReq, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(loginData))
		loginReq.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(loginW, loginReq)

		var loginResp map[string]interface{}
		json.Unmarshal(loginW.Body.Bytes(), &loginResp)
		token := loginResp["data"].(map[string]interface{})["token"].(string)

		infoW := httptest.NewRecorder()
		infoReq, _ := http.NewRequest("GET", "/api/v1/user/info", nil)
		infoReq.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(infoW, infoReq)

		if infoW.Code != 200 {
			t.Fatalf("查询用户信息失败: %d, %s", infoW.Code, infoW.Body.String())
		}
	})
}

// ==================== 小程序首页功能测试 ====================

func TestMiniProgram_IndexPage(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()

	t.Run("平台检测功能", func(t *testing.T) {
		platforms := []struct {
			url      string
			expected string
		}{
			{"https://www.douyin.com/video/123456", "douyin"},
			{"https://www.kuaishou.com/short-video/abc", "kuaishou"},
			{"https://www.xiaohongshu.com/discovery/item/xyz", "xiaohongshu"},
			{"https://www.bilibili.com/video/BVtest", "bilibili"},
			{"https://weibo.com/tv/show/1034:456", "weibo"},
		}

		for _, p := range platforms {
			data := fmt.Sprintf(`{"url":"%s"}`, p.url)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/detect/platform", bytes.NewBufferString(data))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != 200 {
				t.Errorf("平台检测失败(%s): %d", p.expected, w.Code)
				continue
			}

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			result := response["data"].(map[string]interface{})

			if result["platform"] != p.expected {
				t.Errorf("平台%s检测错误: 期望%s，得到%v",
					p.url[:15], p.expected, result["platform"])
			}
		}
	})

	t.Run("额度信息展示", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/quota", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("获取额度失败: %d", w.Code)
		}
	})
}

// ==================== 小程序去水印功能测试 ====================

func TestMiniProgram_RemoveWatermark(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()

	t.Run("单个去水印任务", func(t *testing.T) {
		loginData := `{"code":"wm_single_test"}`
		loginW := httptest.NewRecorder()
		loginReq, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(loginData))
		loginReq.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(loginW, loginReq)

		var loginResp map[string]interface{}
		json.Unmarshal(loginW.Body.Bytes(), &loginResp)
		token := loginResp["data"].(map[string]interface{})["token"].(string)

		taskData := `{"url":"https://www.douyin.com/wm_test","quality":"hd"}`
		taskW := httptest.NewRecorder()
		taskReq, _ := http.NewRequest("POST", "/api/v1/tasks", bytes.NewBufferString(taskData))
		taskReq.Header.Set("Content-Type", "application/json")
		taskReq.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(taskW, taskReq)

		if taskW.Code != 200 {
			t.Errorf("创建去水印任务失败: %d, %s", taskW.Code, taskW.Body.String())
		}
	})

	t.Run("批量去水印任务", func(t *testing.T) {
		loginData := `{"code":"wm_batch_test"}`
		loginW := httptest.NewRecorder()
		loginReq, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(loginData))
		loginReq.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(loginW, loginReq)

		var loginResp map[string]interface{}
		json.Unmarshal(loginW.Body.Bytes(), &loginResp)
		token := loginResp["data"].(map[string]interface{})["token"].(string)

		batchData := `{"urls":["https://douyin.com/b1","https://kuaishou.com/b2","https://bilibili.com/b3"]}`
		batchW := httptest.NewRecorder()
		batchReq, _ := http.NewRequest("POST", "/api/v1/tasks/batch", bytes.NewBufferString(batchData))
		batchReq.Header.Set("Content-Type", "application/json")
		batchReq.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(batchW, batchReq)

		if batchW.Code != 200 {
			t.Fatalf("批量去水印失败: %d, %s", batchW.Code, batchW.Body.String())
		}

		var batchResp map[string]interface{}
		json.Unmarshal(batchW.Body.Bytes(), &batchResp)
		data := batchResp["data"].(map[string]interface{})
		tasks := data["tasks"].([]interface{})

		if len(tasks) != 3 {
			t.Errorf("批量任务数量应该是3，得到: %d", len(tasks))
		}
	})
}

// ==================== 小程序我的页面测试 ====================

func TestMiniProgram_MinePage(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()

	t.Run("用户信息展示", func(t *testing.T) {
		loginData := `{"code":"mine_user_test"}`
		loginW := httptest.NewRecorder()
		loginReq, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(loginData))
		loginReq.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(loginW, loginReq)

		var loginResp map[string]interface{}
		json.Unmarshal(loginW.Body.Bytes(), &loginResp)
		token := loginResp["data"].(map[string]interface{})["token"].(string)

		infoW := httptest.NewRecorder()
		infoReq, _ := http.NewRequest("GET", "/api/v1/user/info", nil)
		infoReq.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(infoW, infoReq)

		if infoW.Code != 200 {
			t.Fatalf("获取用户信息失败: %d, %s", infoW.Code, infoW.Body.String())
		}

		var infoResp map[string]interface{}
		json.Unmarshal(infoW.Body.Bytes(), &infoResp)
		userInfo := infoResp["data"].(map[string]interface{})

		requiredFields := []string{"id", "nickname", "vip_level"}
		for _, field := range requiredFields {
			if userInfo[field] == nil {
				t.Errorf("缺少字段: %s", field)
			}
		}
	})

	t.Run("历史任务列表", func(t *testing.T) {
		db := repository.GetTestDB()

		userID := uint(5555)
		user := model.User{OpenID: fmt.Sprintf("history_user_%d", userID)}
		db.Create(&user)

		for i := 0; i < 5; i++ {
			task := model.Task{
				UserID:       user.ID,
				PlatformType: []string{"douyin", "kuaishou", "bilibili"}[i%3],
				Status:       "success",
				Title:        fmt.Sprintf("历史任务%d", i+1),
			}
			db.Create(&task)
		}

		token, err := getTestUserToken("", user.ID)
		if err != nil {
			t.Fatalf("生成Token失败: %v", err)
		}

		listW := httptest.NewRecorder()
		listReq, _ := http.NewRequest("GET", "/api/v1/tasks?page=1&page_size=20", nil)
		listReq.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(listW, listReq)

		if listW.Code != 200 {
			t.Fatalf("获取历史任务失败: %d, %s", listW.Code, listW.Body.String())
		}

		var listResp map[string]interface{}
		json.Unmarshal(listW.Body.Bytes(), &listResp)
		data := listResp["data"].(map[string]interface{})
		list := data["list"].([]interface{})

		if len(list) < 5 {
			t.Errorf("历史任务不足: %d", len(list))
		}
	})
}

// ==================== 小程序VIP功能测试 ====================

func TestMiniProgram_VIP(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()

	t.Run("VIP产品列表", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/payment/products", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("获取VIP产品失败: %d, %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})
		products := data["products"].([]interface{})

		if len(products) < 4 {
			t.Errorf("VIP产品不足: %d", len(products))
		}
	})

	t.Run("购买VIP流程", func(t *testing.T) {
		loginData := `{"code":"vip_purchase_test"}`
		loginW := httptest.NewRecorder()
		loginReq, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(loginData))
		loginReq.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(loginW, loginReq)

		var loginResp map[string]interface{}
		json.Unmarshal(loginW.Body.Bytes(), &loginResp)
		token := loginResp["data"].(map[string]interface{})["token"].(string)

		orderData := `{"product_type":"monthly","pay_method":"wechat"}`
		orderW := httptest.NewRecorder()
		orderReq, _ := http.NewRequest("POST", "/api/v1/payment/create-order", bytes.NewBufferString(orderData))
		orderReq.Header.Set("Content-Type", "application/json")
		orderReq.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(orderW, orderReq)

		if orderW.Code != 200 {
			t.Errorf("创建VIP订单失败: %d, %s", orderW.Code, orderW.Body.String())
		}
	})
}

// ==================== 小程序结果页测试 ====================

func TestMiniProgram_ResultPage(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()
	db := repository.GetTestDB()

	t.Run("成功任务结果", func(t *testing.T) {
		userID := uint(6666)
		user := model.User{OpenID: fmt.Sprintf("result_user_%d", userID)}
		db.Create(&user)

		task := model.Task{
			UserID:    user.ID,
			Status:    "success",
			CleanURL: "https://cdn.test.com/result.mp4",
			FileSize:  15728640,
			Duration:  180,
		}
		db.Create(&task)

		token, _ := getTestUserToken("", user.ID)

		detailW := httptest.NewRecorder()
		detailReq, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/tasks/%d", task.ID), nil)
		detailReq.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(detailW, detailReq)

		if detailW.Code != 200 {
			t.Fatalf("获取任务详情失败: %d, %s", detailW.Code, detailW.Body.String())
		}

		var detailResp map[string]interface{}
		json.Unmarshal(detailW.Body.Bytes(), &detailResp)
		taskDetail := detailResp["data"].(map[string]interface{})

		if taskDetail["result_url"] == nil || taskDetail["result_url"] == "" {
			t.Error("成功任务应该有下载链接")
		}
	})

	t.Run("删除历史记录", func(t *testing.T) {
		userID := uint(7777)
		user := model.User{OpenID: fmt.Sprintf("delete_user_%d", userID)}
		db.Create(&user)

		task := model.Task{UserID: user.ID, Status: "success", Title: "待删除记录"}
		db.Create(&task)

		token, _ := getTestUserToken("", user.ID)

		deleteW := httptest.NewRecorder()
		deleteReq, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/tasks/%d", task.ID), nil)
		deleteReq.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(deleteW, deleteReq)

		if deleteW.Code != 200 {
			t.Errorf("删除记录失败: %d, %s", deleteW.Code, deleteW.Body.String())
		}
	})
}

// ==================== 异常场景测试 ====================

func TestMiniProgram_ErrorScenarios(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()

	t.Run("无效Token访问", func(t *testing.T) {
		endpoints := []string{
			"/api/v1/user/info",
			"/api/v1/tasks",
			"/api/v1/tasks?page=1",
		}

		for _, endpoint := range endpoints {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", endpoint, nil)
			req.Header.Set("Authorization", "Bearer invalid_token")
			router.ServeHTTP(w, req)

			if w.Code != 401 {
				t.Errorf("%s 应该返回401，得到: %d", endpoint, w.Code)
			}
		}
	})

	t.Run("空链接提交", func(t *testing.T) {
		loginData := `{"code":"error_test"}`
		loginW := httptest.NewRecorder()
		loginReq, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(loginData))
		loginReq.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(loginW, loginReq)

		var loginResp map[string]interface{}
		json.Unmarshal(loginW.Body.Bytes(), &loginResp)
		token := loginResp["data"].(map[string]interface{})["token"].(string)

		emptyURLCases := []struct {
			name string
			url  string
		}{
			{"空字符串", ""},
			{"纯空格", "   "},
			{"无协议前缀", "www.douyin.com/test"},
		}

		for _, ec := range emptyURLCases {
			taskData := fmt.Sprintf(`{"url":"%s"}`, ec.url)
			taskW := httptest.NewRecorder()
			taskReq, _ := http.NewRequest("POST", "/api/v1/tasks", bytes.NewBufferString(taskData))
			taskReq.Header.Set("Content-Type", "application/json")
			taskReq.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(taskW, taskReq)

			if taskW.Code != 400 && taskW.Code != 422 {
				t.Errorf("异常链接'%s'应被拒绝，得到: %d", ec.name, taskW.Code)
			}
		}
	})

	t.Run("特殊字符处理", func(t *testing.T) {
		specialInputs := []string{
			`<script>alert('xss')</script>`,
			"' OR '1'='1",
			"中文昵称🎉🚀",
		}

		for _, input := range specialInputs {
			data := fmt.Sprintf(`{"code":"%s"}`, input)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(data))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code == 500 {
				t.Errorf("特殊字符导致服务器错误: %s", input)
			}
		}
	})
}

// ==================== 性能测试 ====================

func TestMiniProgram_Performance(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()

	t.Run("登录接口响应时间", func(t *testing.T) {
		start := time.Now()

		data := `{"code":"perf_login_test"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(data))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		duration := time.Since(start)

		if w.Code != 200 {
			t.Fatalf("性能测试-登录失败: %d", w.Code)
		}

		if duration > 500*time.Millisecond {
			t.Errorf("登录响应过慢: %v (应<500ms)", duration)
		} else {
			t.Logf("✅ 登录响应时间: %v", duration)
		}
	})

	t.Run("平台检测响应时间", func(t *testing.T) {
		start := time.Now()

		data := `{"url":"https://www.douyin.com/perf_test"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/detect/platform", bytes.NewBufferString(data))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		duration := time.Since(start)

		if w.Code != 200 {
			t.Fatalf("性能测试-平台检测失败: %d", w.Code)
		}

		if duration > 300*time.Millisecond {
			t.Errorf("平台检测响应过慢: %v (应<300ms)", duration)
		} else {
			t.Logf("✅ 平台检测响应时间: %v", duration)
		}
	})
}

// ==================== 数据一致性测试 ====================

func TestMiniProgram_DataConsistency(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()

	t.Run("用户数据一致性", func(t *testing.T) {
		loginData := `{"code":"consistency_user"}`
		loginW := httptest.NewRecorder()
		loginReq, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(loginData))
		loginReq.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(loginW, loginReq)

		var loginResp map[string]interface{}
		json.Unmarshal(loginW.Body.Bytes(), &loginResp)
		data := loginResp["data"].(map[string]interface{})
		token := data["token"].(string)
		apiUserID := data["user_id"].(float64)

		infoW := httptest.NewRecorder()
		infoReq, _ := http.NewRequest("GET", "/api/v1/user/info", nil)
		infoReq.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(infoW, infoReq)

		var infoResp map[string]interface{}
		json.Unmarshal(infoW.Body.Bytes(), &infoResp)
		userInfo := infoResp["data"].(map[string]interface{})
		infoUserID := userInfo["id"].(float64)

		if apiUserID != infoUserID {
			t.Errorf("用户ID不一致: 登录=%v, 信息=%v", apiUserID, infoUserID)
		}
	})
}

// ==================== 边界条件测试 ====================

func TestMiniProgram_EdgeCases(t *testing.T) {
	router, _ := setupMiniProgramTestRouter()

	t.Run("超长输入", func(t *testing.T) {
		longURL := "https://www.douyin.com/" + strings.Repeat("a", 5000)
		data := fmt.Sprintf(`{"url":"%s"}`, longURL)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/detect/platform", bytes.NewBufferString(data))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code == 500 {
			t.Errorf("超长URL不应导致服务器错误: %d", w.Code)
		}
	})

	t.Run("并发安全性", func(t *testing.T) {
		loginData := `{"code":"concurrent_test"}`
		loginW := httptest.NewRecorder()
		loginReq, _ := http.NewRequest("POST", "/api/v1/auth/wechat/login", bytes.NewBufferString(loginData))
		loginReq.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(loginW, loginReq)

		var loginResp map[string]interface{}
		json.Unmarshal(loginW.Body.Bytes(), &loginResp)
		token := loginResp["data"].(map[string]interface{})["token"].(string)

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
			t.Errorf("并发请求成功率低: %d/10", successCount)
		}
	})
}
