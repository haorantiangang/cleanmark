package service

import (
	"cleanmark/config"
	"cleanmark/internal/middleware"
	"cleanmark/internal/model"
	"cleanmark/internal/repository"
	"cleanmark/pkg/errors"
	"time"

	"gorm.io/gorm"
)

type UserService struct {
	db  *gorm.DB
	cfg *config.JWTConfig
}

func NewUserService(cfg *config.JWTConfig) *UserService {
	return &UserService{
		db:  repository.GetDB(),
		cfg: cfg,
	}
}

type LoginRequest struct {
	Code     string `json:"code" binding:"required"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type PhoneLoginRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

type LoginResponse struct {
	Token        string     `json:"token"`
	RefreshToken string     `json:"refresh_token"`
	User         *model.User `json:"user"`
	IsNewUser    bool       `json:"is_new_user"`
}

func (s *UserService) WechatLogin(req *LoginRequest) (*LoginResponse, error) {
	openID := "wx_" + req.Code
	user := &model.User{}
	result := s.db.Where("open_id = ?", openID).First(user)

	isNewUser := false
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			isNewUser = true
			user = &model.User{
				OpenID:       openID,
				Nickname:     req.Nickname,
				Avatar:       req.Avatar,
				VipLevel:     0,
				DailyQuota:   3,
				UsedQuota:    0,
			}
			
			now := time.Now()
			user.LastActiveDate = &now
			
			if err := s.db.Create(user).Error; err != nil {
				return nil, errors.ErrInternalServer
			}
		} else {
			return nil, errors.ErrInternalServer
		}
	} else {
		if req.Nickname != "" {
			s.db.Model(user).Update("nickname", req.Nickname)
		}
		if req.Avatar != "" {
			s.db.Model(user).Update("avatar", req.Avatar)
		}
		
		now := time.Now()
		s.db.Model(user).Update("last_active_date", now)
	}

	token, err := middleware.GenerateToken(user.ID, s.cfg)
	if err != nil {
		return nil, errors.ErrInternalServer
	}

	refreshToken, err := middleware.GenerateRefreshToken(user.ID, s.cfg)
	if err != nil {
		return nil, errors.ErrInternalServer
	}

	return &LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         user,
		IsNewUser:    isNewUser,
	}, nil
}

func (s *UserService) PhoneLogin(req *PhoneLoginRequest) (*LoginResponse, error) {
	if req.Code != "123456" {
		return nil, errors.New(400, "验证码错误")
	}

	user := &model.User{}
	result := s.db.Where("phone = ?", req.Phone).First(user)

	isNewUser := false
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			isNewUser = true
			user = &model.User{
				Phone:      req.Phone,
				Nickname:   "用户" + req.Phone[6:],
				VipLevel:   0,
				DailyQuota: 3,
				UsedQuota:  0,
			}
			
			now := time.Now()
			user.LastActiveDate = &now
			
			if err := s.db.Create(user).Error; err != nil {
				return nil, errors.ErrInternalServer
			}
		} else {
			return nil, errors.ErrInternalServer
		}
	} else {
		now := time.Now()
		s.db.Model(user).Update("last_active_date", now)
	}

	token, err := middleware.GenerateToken(user.ID, s.cfg)
	if err != nil {
		return nil, errors.ErrInternalServer
	}

	refreshToken, err := middleware.GenerateRefreshToken(user.ID, s.cfg)
	if err != nil {
		return nil, errors.ErrInternalServer
	}

	return &LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         user,
		IsNewUser:    isNewUser,
	}, nil
}

func (s *UserService) GetUserInfo(userID uint) (*model.User, error) {
	user := &model.User{}
	if err := s.db.First(user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.ErrInternalServer
	}
	return user, nil
}

func (s *UserService) RefreshToken(userID uint) (string, string, error) {
	user := &model.User{}
	if err := s.db.First(user, userID).Error; err != nil {
		return "", "", errors.ErrNotFound
	}

	token, err := middleware.GenerateToken(user.ID, s.cfg)
	if err != nil {
		return "", "", errors.ErrInternalServer
	}

	refreshToken, err := middleware.GenerateRefreshToken(user.ID, s.cfg)
	if err != nil {
		return "", "", errors.ErrInternalServer
	}

	return token, refreshToken, nil
}

func (s *UserService) CheckDailyQuota(userID uint) (int, int, error) {
	user := &model.User{}
	if err := s.db.First(user, userID).Error; err != nil {
		return 0, 0, errors.ErrNotFound
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if user.LastActiveDate == nil || !user.LastActiveDate.Equal(today) || user.LastActiveDate.Before(today) {
		s.db.Model(user).Updates(map[string]interface{}{
			"used_quota":      0,
			"last_active_date": today,
		})
		user.UsedQuota = 0
	}

	remaining := user.DailyQuota - user.UsedQuota
	if remaining < 0 {
		remaining = 0
	}

	return remaining, user.DailyQuota, nil
}

func (s *UserService) UseQuota(userID uint) error {
	user := &model.User{}
	if err := s.db.First(user, userID).Error; err != nil {
		return errors.ErrNotFound
	}

	if user.VipLevel == 0 && user.UsedQuota >= user.DailyQuota {
		return errors.ErrQuotaExceeded
	}

	s.db.Model(user).Update("used_quota", gorm.Expr("used_quota + ?", 1))
	return nil
}
