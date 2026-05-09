package handler

import (
	"cleanmark/internal/service"
	"cleanmark/pkg/errors"
	"cleanmark/pkg/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (h *UserHandler) WechatLogin(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	result, err := h.userService.WechatLogin(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.(*errors.AppError).Code, err.Error())
		return
	}

	response.SuccessWithMessage(c, "登录成功", result)
}

func (h *UserHandler) PhoneLogin(c *gin.Context) {
	var req service.PhoneLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	result, err := h.userService.PhoneLogin(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.(*errors.AppError).Code, err.Error())
		return
	}

	response.SuccessWithMessage(c, "登录成功", result)
}

func (h *UserHandler) GetUserInfo(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	user, err := h.userService.GetUserInfo(uid)
	if err != nil {
		appErr := err.(*errors.AppError)
		response.Error(c, http.StatusNotFound, appErr.Code, appErr.Message)
		return
	}

	remaining, dailyQuota, _ := h.userService.CheckDailyQuota(uid)
	userData := map[string]interface{}{
		"user":           user,
		"remaining_quota": remaining,
		"daily_quota":    dailyQuota,
	}

	response.Success(c, userData)
}

func (h *UserHandler) RefreshToken(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	token, refreshToken, err := h.userService.RefreshToken(uid)
	if err != nil {
		appErr := err.(*errors.AppError)
		response.Error(c, http.StatusUnauthorized, appErr.Code, appErr.Message)
		return
	}

	response.Success(c, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
	})
}

func (h *UserHandler) GetQuotaInfo(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Success(c, gin.H{
			"remaining_quota": 3,
			"daily_quota":    3,
			"is_vip":         false,
		})
		return
	}
	
	uid := userID.(uint)
	remaining, dailyQuota, err := h.userService.CheckDailyQuota(uid)
	if err != nil {
		response.InternalError(c, "获取额度信息失败")
		return
	}

	user, err := h.userService.GetUserInfo(uid)
	if err != nil {
		response.InternalError(c, "获取用户信息失败")
		return
	}

	response.Success(c, gin.H{
		"remaining_quota": remaining,
		"daily_quota":    dailyQuota,
		"is_vip":         user.VipLevel > 0,
	})
}
