package tests

import (
	"bytes"
	"cleanmark/internal/handler"
	"cleanmark/internal/middleware"
	"cleanmark/internal/model"
	"cleanmark/internal/repository"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupAdminTestRouter() (*gin.Engine, *handler.AdminHandler) {
	gin.SetMode(gin.TestMode)

	repository.InitTestDB()

	router := gin.New()
	router.Use(gin.Recovery())

	adminHandler := handler.NewAdminHandler()

	public := router.Group("/api/v1/admin")
	{
		public.POST("/login", adminHandler.AdminLogin)
		public.GET("/dashboard", adminHandler.GetDashboardStats)
		public.GET("/system/info", adminHandler.GetSystemInfo)
	}

	protected := router.Group("/api/v1/admin")
	protected.Use(middleware.AdminAuth())
	{
		protected.GET("/users", adminHandler.GetUsers)
		protected.GET("/users/:id", adminHandler.GetUserDetail)
		protected.PUT("/users/:id/vip", adminHandler.UpdateUserVip)
		protected.GET("/tasks", adminHandler.GetTasks)
		protected.GET("/orders", adminHandler.GetOrders)
		protected.GET("/analytics", adminHandler.GetAnalyticsData)
	}

	return router, adminHandler
}

// ==================== 管理员登录测试 ====================

func TestAdminLogin_Success(t *testing.T) {
	router, _ := setupAdminTestRouter()

	loginData := `{"username":"admin","password":"admin123"}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/login", bytes.NewBufferString(loginData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("登录成功应该返回200，得到: %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["code"].(float64) != 200 {
		t.Errorf("响应码应该是200")
	}
}

func TestAdminLogin_WrongPassword(t *testing.T) {
	router, _ := setupAdminTestRouter()

	loginData := `{"username":"admin","password":"wrong"}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/login", bytes.NewBufferString(loginData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("密码错误应该返回401，得到: %d", w.Code)
	}
}

// ==================== 仪表盘测试 ====================

func TestGetDashboardStats(t *testing.T) {
	router, _ := setupAdminTestRouter()
	db := repository.GetTestDB()

	user := model.User{OpenID: "dash_user", Nickname: "测试用户"}
	db.Create(&user)

	task := model.Task{UserID: user.ID, PlatformType: "douyin", Status: "success"}
	db.Create(&task)

	token := "admin-token-test-123"

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取仪表盘失败: %d, %s", w.Code, w.Body.String())
	}
}

// ==================== 用户管理测试 ====================

func TestGetUsers(t *testing.T) {
	router, _ := setupAdminTestRouter()
	db := repository.GetTestDB()

	for i := 0; i < 5; i++ {
		user := model.User{
			OpenID:   fmt.Sprintf("user_%d", i),
			Nickname: fmt.Sprintf("用户%d", i),
			VipLevel: i % 4,
		}
		db.Create(&user)
	}

	token := "admin-token-test-123"

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=1&page_size=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取用户列表失败: %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})
	list := data["list"].([]interface{})

	if len(list) < 5 {
		t.Errorf("应该返回至少5个用户，得到: %d", len(list))
	}
}

func TestUpdateUserVIP(t *testing.T) {
	router, _ := setupAdminTestRouter()
	db := repository.GetTestDB()

	user := model.User{OpenID: "vip_user", Nickname: "VIP测试", VipLevel: 0}
	db.Create(&user)

	token := "admin-token-test-123"

	updateData := `{"vip_level":2,"daily_quota":100}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT",
		fmt.Sprintf("/api/v1/admin/users/%d/vip", user.ID),
		bytes.NewBufferString(updateData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("更新VIP失败: %d, %s", w.Code, w.Body.String())
	}
}

// ==================== 任务监控测试 ====================

func TestGetTasks(t *testing.T) {
	router, _ := setupAdminTestRouter()
	db := repository.GetTestDB()

	user := model.User{OpenID: "task_user"}
	db.Create(&user)

	for i := 0; i < 3; i++ {
		task := model.Task{
			UserID:       user.ID,
			PlatformType: []string{"douyin", "kuaishou", "bilibili"}[i],
			Status:       []string{"success", "failed", "processing"}[i],
		}
		db.Create(&task)
	}

	token := "admin-token-test-123"

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/tasks?page=1&page_size=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取任务列表失败: %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})
	list := data["list"].([]interface{})

	if len(list) < 3 {
		t.Errorf("任务数量不足: %d", len(list))
	}
}

// ==================== 订单管理测试 ====================

func TestGetOrders(t *testing.T) {
	router, _ := setupAdminTestRouter()
	db := repository.GetTestDB()

	user := model.User{OpenID: "order_user"}
	db.Create(&user)

	order := model.Order{
		UserID:     user.ID,
		OrderNo:    "TEST001",
		ProductType: "monthly",
		Amount:     19.9,
		Status:     "paid",
	}
	db.Create(&order)

	token := "admin-token-test-123"

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/orders?page=1&page_size=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取订单列表失败: %d", w.Code)
	}
}

// ==================== 数据分析测试 ====================

func TestGetAnalytics(t *testing.T) {
	router, _ := setupAdminTestRouter()

	token := "admin-token-test-123"

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/analytics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取分析数据失败: %d, %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})

	dailyTasks := data["daily_tasks"].([]interface{})
	hourlyStats := data["hourly_stats"].([]interface{})

	if len(dailyTasks) != 7 {
		t.Errorf("每日任务数据应该是7天，得到: %d", len(dailyTasks))
	}

	if len(hourlyStats) != 24 {
		t.Errorf("每小时数据应该是24小时，得到: %d", len(hourlyStats))
	}
}

// ==================== 系统信息测试 ====================

func TestGetSystemInfo(t *testing.T) {
	router, _ := setupAdminTestRouter()

	token := "admin-token-test-123"

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/system/info", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("获取系统信息失败: %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	data := response["data"].(map[string]interface{})

	if data["version"] == nil || data["uptime"] == nil {
		t.Error("系统信息不完整")
	}
}

// ==================== 认证中间件测试 ====================

func TestAdminAuth_Middleware(t *testing.T) {
	router, _ := setupAdminTestRouter()

	t.Run("无Token访问", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=1", nil)
		router.ServeHTTP(w, req)

		if w.Code != 401 {
			t.Errorf("无Token应该返回401，得到: %d", w.Code)
		}
	})

	t.Run("无效Token访问", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=1", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		router.ServeHTTP(w, req)

		if w.Code != 403 {
			t.Errorf("无效Token应该返回403，得到: %d", w.Code)
		}
	})

	t.Run("有效Token访问", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=1", nil)
		req.Header.Set("Authorization", "Bearer admin-token-test-123")
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("有效Token应该返回200，得到: %d", w.Code)
		}
	})
}

// ==================== 分页功能测试 ====================

func TestPagination(t *testing.T) {
	router, _ := setupAdminTestRouter()
	db := repository.GetTestDB()

	for i := 0; i < 25; i++ {
		user := model.User{OpenID: fmt.Sprintf("page_user_%d", i)}
		db.Create(&user)
	}

	token := "admin-token-test-123"

	t.Run("第一页", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=1&page_size=10", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})
		list := data["list"].([]interface{})

		if len(list) > 10 {
			t.Errorf("第一页最多10条，得到: %d", len(list))
		}
	})

	t.Run("第二页", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/admin/users?page=2&page_size=10", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})
		list := data["list"].([]interface{})

		if len(list) > 10 {
			t.Errorf("第二页最多10条，得到: %d", len(list))
		}
	})
}
