package tests

import (
	"cleanmark/internal/middleware"
	"cleanmark/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	
	jwtCfg := &config.JWTConfig{
		Secret:         "test-secret-key-for-testing",
		ExpireDuration: time.Hour,
		RefreshExpire:  time.Hour * 24,
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Next()
	})

	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})

	protected := router.Group("/protected")
	protected.Use(middleware.JWTAuth(jwtCfg))
	{
		protected.GET("/user", func(c *gin.Context) {
			userID, _ := c.Get("user_id")
			c.JSON(200, gin.H{"user_id": userID})
		})
	}

	return router
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("状态码应该是200，得到: %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), `"message":"ok"`) {
		t.Errorf("响应体不正确: %s", w.Body.String())
	}
}

func TestJWTAuthMiddleware(t *testing.T) {
	jwtCfg := &config.JWTConfig{
		Secret:         "test-secret-key",
		ExpireDuration: time.Hour,
		RefreshExpire:  time.Hour * 24,
	}

	token, err := middleware.GenerateToken(123, jwtCfg)
	if err != nil {
		t.Fatalf("生成Token失败: %v", err)
	}

	if token == "" {
		t.Error("Token不应该为空")
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Next()
	})

	router.GET("/public", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "public"})
	})

	protected := router.Group("/protected")
	protected.Use(middleware.JWTAuth(jwtCfg))
	{
		protected.GET("/data", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "protected"})
		})
	}

	t.Run("公开接口不需要认证", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/public", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("公开接口应该返回200，得到: %d", w.Code)
		}
	})

	t.Run("未携带Token访问受保护接口", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected/data", nil)
		router.ServeHTTP(w, req)

		if w.Code != 401 {
			t.Errorf("应该返回401未授权，得到: %d", w.Code)
		}
	})

	t.Run("携带有效Token访问受保护接口", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected/data", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("应该返回200成功，得到: %d", w.Code)
		}
	})

	t.Run("携带无效Token访问受保护接口", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected/data", nil)
		req.Header.Set("Authorization", "Bearer invalid-token-12345")
		router.ServeHTTP(w, req)

		if w.Code != 401 {
			t.Errorf("应该返回401未授权，得到: %d", w.Code)
		}
	})

	t.Run("Token格式错误", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected/data", nil)
		req.Header.Set("Authorization", "InvalidFormat token123")
		router.ServeHTTP(w, req)

		if w.Code != 401 {
			t.Errorf("应该返回401未授权，得到: %d", w.Code)
		}
	})
}

func TestGenerateAndRefreshToken(t *testing.T) {
	jwtCfg := &config.JWTConfig{
		Secret:         "test-secret-key-refresh",
		ExpireDuration: time.Minute,
		RefreshExpire:  time.Hour * 24,
	}

	token, err := middleware.GenerateToken(1, jwtCfg)
	if err != nil {
		t.Fatalf("生成Token失败: %v", err)
	}

	refreshToken, err := middleware.GenerateRefreshToken(1, jwtCfg)
	if err != nil {
		t.Fatalf("生成刷新Token失败: %v", err)
	}

	if token == refreshToken {
		t.Error("Access Token和Refresh Token应该不同")
	}
}

func TestOptionalAuthMiddleware(t *testing.T) {
	jwtCfg := &config.JWTConfig{
		Secret:         "test-secret-optional",
		ExpireDuration: time.Hour,
		RefreshExpire:  time.Hour * 24,
	}

	router := gin.New()
	
	router.GET("/optional", middleware.OptionalAuth(jwtCfg), func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		
		if exists {
			c.JSON(200, gin.H{"authenticated": true, "user_id": userID})
		} else {
			c.JSON(200, gin.H{"authenticated": false})
		}
	})

	t.Run("不带Token访问可选认证接口", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/optional", nil)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("应该返回200，得到: %d", w.Code)
		}

		if strings.Contains(w.Body.String(), `"authenticated":false`) == false {
			t.Errorf("应该显示未认证状态: %s", w.Body.String())
		}
	})

	t.Run("带Token访问可选认证接口", func(t *testing.T) {
		token, _ := middleware.GenerateToken(999, jwtCfg)
		
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/optional", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("应该返回200，得到: %d", w.Code)
		}

		if strings.Contains(w.Body.String(), `"authenticated":true`) == false {
			t.Errorf("应该显示已认证状态: %s", w.Body.String())
		}
	})
}
