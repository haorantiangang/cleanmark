package tests

import (
	"cleanmark/internal/model"
	"cleanmark/internal/repository"
	"cleanmark/internal/service"
	"cleanmark/config"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func initServiceDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&model.User{}, &model.Task{}, &model.Order{})
	if err != nil {
		return nil, err
	}

	repository.SetTestDB(db)
	return db, nil
}

func TestWechatLogin(t *testing.T) {
	db, err := initServiceDB()
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:         "test-login-secret",
			ExpireDuration: time.Hour,
			RefreshExpire:  time.Hour * 24,
		},
	}

	svc := service.NewUserService(&cfg.JWT)

	t.Run("新用户登录", func(t *testing.T) {
		req := &service.LoginRequest{
			Code:     "new_user_test_001",
			Nickname: "新用户",
			Avatar:   "https://example.com/avatar.jpg",
		}

		resp, err := svc.WechatLogin(req)
		if err != nil {
			t.Fatalf("微信登录失败: %v", err)
		}

		if resp == nil {
			t.Fatal("响应不应该为nil")
		}

		if resp.Token == "" {
			t.Error("Token不应该为空")
		}

		if resp.RefreshToken == "" {
			t.Error("RefreshToken不应该为空")
		}

		if resp.User == nil {
			t.Error("User信息不应该为nil")
		}

		if !resp.IsNewUser {
			t.Error("IsNewUser应该为true")
		}

		if resp.User.Nickname != "新用户" {
			t.Errorf("昵称不正确，得到: %s", resp.User.Nickname)
		}
	})

	t.Run("老用户登录", func(t *testing.T) {
		req := &service.LoginRequest{
			Code:     "returning_user_001",
			Nickname: "老用户",
		}

		resp1, err := svc.WechatLogin(req)
		if err != nil {
			t.Fatalf("第一次登录失败: %v", err)
		}

		if !resp1.IsNewUser {
			t.Error("第一次登录应该是新用户")
		}

		resp2, err := svc.WechatLogin(req)
		if err != nil {
			t.Fatalf("第二次登录失败: %v", err)
		}

		if resp2.IsNewUser {
			t.Error("第二次登录不应该是新用户")
		}

		if resp2.User.ID != resp1.User.ID {
			t.Error("两次登录应该返回同一个用户")
		}
	})
}

func TestPhoneLogin(t *testing.T) {
	db, _ := initServiceDB()
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:         "test-phone-secret",
			ExpireDuration: time.Hour,
			RefreshExpire:  time.Hour * 24,
		},
	}

	svc := service.NewUserService(&cfg.JWT)

	t.Run("验证码错误", func(t *testing.T) {
		req := &service.PhoneLoginRequest{
			Phone: "13800138000",
			Code:  "000000",
		}

		_, err := svc.PhoneLogin(req)
		if err == nil {
			t.Error("验证码错误时应该返回错误")
		}
	})

	t.Run("正确验证码登录", func(t *testing.T) {
		req := &service.PhoneLoginRequest{
			Phone: "13800138001",
			Code:  "123456",
		}

		resp, err := svc.PhoneLogin(req)
		if err != nil {
			t.Fatalf("手机登录失败: %v", err)
		}

		if resp == nil || resp.User == nil {
			t.Fatal("响应和用户信息不应该为nil")
		}

		if resp.User.Phone != "13800138001" {
			t.Errorf("手机号不正确，得到: %s", resp.User.Phone)
		}
	})
}

func TestCheckDailyQuota(t *testing.T) {
	db, _ := initServiceDB()
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:         "test-quota-secret",
			ExpireDuration: time.Hour,
			RefreshExpire:  time.Hour * 24,
		},
	}

	svc := service.NewUserService(&cfg.JWT)

	loginReq := &service.LoginRequest{
		Code:     "quota_test_user",
		Nickname: "额度测试用户",
	}

	loginResp, err := svc.WechatLogin(loginReq)
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	userID := loginResp.User.ID

	t.Run("检查初始额度", func(t *testing.T) {
		remaining, dailyQuota, err := svc.CheckDailyQuota(userID)
		if err != nil {
			t.Fatalf("检查额度失败: %v", err)
		}

		if remaining != 3 {
			t.Errorf("剩余额度应该是3，得到: %d", remaining)
		}

		if dailyQuota != 3 {
			t.Errorf("每日额度应该是3，得到: %d", dailyQuota)
		}
	})

	t.Run("使用额度", func(t *testing.T) {
		err := svc.UseQuota(userID)
		if err != nil {
			t.Fatalf("使用额度失败: %v", err)
		}

		remaining, _, err := svc.CheckDailyQuota(userID)
		if err != nil {
			t.Fatalf("检查额度失败: %v", err)
		}

		if remaining != 2 {
			t.Errorf("使用后剩余额度应该是2，得到: %d", remaining)
		}
	})
}

func TestGetUserInfo(t *testing.T) {
	db, _ := initServiceDB()
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:         "test-user-info-secret",
			ExpireDuration: time.Hour,
			RefreshExpire:  time.Hour * 24,
		},
	}

	svc := service.NewUserService(&cfg.JWT)

	loginReq := &service.LoginRequest{
		Code:     "get_info_test",
		Nickname: "信息查询用户",
	}

	loginResp, err := svc.WechatLogin(loginReq)
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	userID := loginResp.User.ID

	user, err := svc.GetUserInfo(userID)
	if err != nil {
		t.Fatalf("获取用户信息失败: %v", err)
	}

	if user == nil {
		t.Fatal("用户信息不应该为nil")
	}

	if user.ID != userID {
		t.Errorf("用户ID不匹配，期望: %d，得到: %d", userID, user.ID)
	}

	if user.Nickname != "信息查询用户" {
		t.Errorf("昵称不匹配，期望: 信息查询用户，得到: %s", user.Nickname)
	}
}

func TestRefreshTokenService(t *testing.T) {
	db, _ := initServiceDB()
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:         "test-refresh-token-secret",
			ExpireDuration: time.Minute,
			RefreshExpire:  time.Hour * 24,
		},
	}

	svc := service.NewUserService(&cfg.JWT)

	loginReq := &service.LoginRequest{
		Code:     "refresh_token_test",
		Nickname: "刷新Token测试",
	}

	loginResp, err := svc.WechatLogin(loginReq)
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	userID := loginResp.User.ID

	newToken, newRefreshToken, err := svc.RefreshToken(userID)
	if err != nil {
		t.Fatalf("刷新Token失败: %v", err)
	}

	if newToken == "" {
		t.Error("新Token不应该为空")
	}

	if newRefreshToken == "" {
		t.Error("新RefreshToken不应该为空")
	}

	if newToken == loginResp.Token {
		t.Error("新Token应该与旧Token不同")
	}
}

func TestNonExistentUser(t *testing.T) {
	db, _ := initServiceDB()
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:         "test-nonexistent-secret",
			ExpireDuration: time.Hour,
			RefreshExpire:  time.Hour * 24,
		},
	}

	svc := service.NewUserService(&cfg.JWT)

	_, err := svc.GetUserInfo(999999)
	if err == nil {
		t.Error("查询不存在的用户应该返回错误")
	}

	_, _, err = svc.RefreshToken(888888)
	if err == nil {
		t.Error("刷新不存在用户的Token应该返回错误")
	}
}
