package handler

import (
	"cleanmark/internal/model"
	"cleanmark/internal/repository"
	"cleanmark/pkg/response"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct{}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AdminHandler) AdminLogin(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	if req.Username != "admin" || req.Password != "admin123" {
		response.Error(c, 401, 401, "账号或密码错误")
		return
	}

	token, err := generateAdminToken()
	if err != nil {
		response.InternalError(c, "生成Token失败")
		return
	}

	response.SuccessWithMessage(c, "登录成功", gin.H{
		"token": token,
		"user": gin.H{
			"id":       0,
			"username": "admin",
			"role":     "super_admin",
			"nickname": "超级管理员",
		},
	})
}

func (h *AdminHandler) GetDashboardStats(c *gin.Context) {
	db := repository.GetDB()

	var totalUsers int64
	var totalTasks int64
	var successTasks int64
	var failedTasks int64
	var paidOrders int64
	var totalRevenue float64

	db.Model(&model.User{}).Count(&totalUsers)
	db.Model(&model.Task{}).Count(&totalTasks)
	db.Model(&model.Task{}).Where("status = ?", "success").Count(&successTasks)
	db.Model(&model.Task{}).Where("status = ?", "failed").Count(&failedTasks)
	db.Model(&model.Order{}).Where("status = ?", "paid").Count(&paidOrders)

	db.Model(&model.Order{}).Where("status = ?", "paid").Select("COALESCE(SUM(amount), 0)").Scan(&totalRevenue)

	var todayTasks int64
	var yesterdayTasks int64
	today := getTodayStart()
	yesterday := today.AddDate(0, 0, -1)

	db.Model(&model.Task{}).Where("created_at >= ?", today).Count(&todayTasks)
	db.Model(&model.Task{}).Where("created_at >= ? AND created_at < ?", yesterday, today).Count(&yesterdayTasks)

	var platformStats []struct {
		Platform string `json:"platform"`
		Count    int64  `json:"count"`
	}
	db.Model(&model.Task{}).
		Select("platform_type as platform, count(*) as count").
		Where("status = ?", "success").
		Group("platform_type").
		Order("count DESC").
		Limit(8).
		Find(&platformStats)

	var recentTasks []model.Task
	db.Where("status IN ?", []string{"success", "failed"}).
		Order("created_at DESC").
		Limit(10).
		Find(&recentTasks)

	successRate := float64(0)
	if totalTasks > 0 {
		successRate = float64(successTasks) / float64(totalTasks) * 100
	}

	response.Success(c, gin.H{
		"total_users":      totalUsers,
		"total_tasks":       totalTasks,
		"success_tasks":    successTasks,
		"failed_tasks":      failedTasks,
		"success_rate":      successRate,
		"total_orders":      paidOrders,
		"total_revenue":     totalRevenue,
		"today_tasks":       todayTasks,
		"yesterday_tasks":   yesterdayTasks,
		"platforms":         platformStats,
		"recent_tasks":      recentTasks,
	})
}

func (h *AdminHandler) GetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 50 { pageSize = 20 }

	db := repository.GetDB()
	var users []model.User
	var total int64

	query := db.Model(&model.User{})
	
	vipLevel := c.Query("vip_level")
	if vipLevel != "" {
		query = query.Where("vip_level = ?", vipLevel)
	}

	keyword := c.Query("keyword")
	if keyword != "" {
		query = query.Where("nickname LIKE ? OR phone LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&users).Error

	if err != nil {
		response.InternalError(c, "获取用户列表失败")
		return
	}

	response.PageSuccess(c, users, total, page, pageSize)
}

func (h *AdminHandler) GetUserDetail(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	db := repository.GetDB()
	var user model.User
	if err := db.First(&user, uint(userID)).Error; err != nil {
		response.NotFound(c, "用户不存在")
		return
	}

	var taskCount int64
	var successTaskCount int64
	db.Model(&model.Task{}).Where("user_id = ?", user.ID).Count(&taskCount)
	db.Model(&model.Task{}).Where("user_id = ? AND status = ?", user.ID, "success").Count(&successTaskCount)

	var orderCount int64
	var totalSpent float64
	db.Model(&model.Order{}).Where("user_id = ? AND status = ?", user.ID, "paid").Count(&orderCount)
	db.Model(&model.Order{}).Where("user_id = ? AND status = ?", user.ID, "paid").Select("COALESCE(SUM(amount), 0)").Scan(&totalSpent)

	response.Success(c, gin.H{
		"user":              user,
		"task_count":        taskCount,
		"success_task_count": successTaskCount,
		"order_count":       orderCount,
		"total_spent":       totalSpent,
	})
}

func (h *AdminHandler) UpdateUserVip(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req struct {
		VipLevel    int `json:"vip_level"`
		DailyQuota int `json:"daily_quota"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	db := repository.GetDB()
	
	updates := map[string]interface{}{
		"vip_level":   req.VipLevel,
		"daily_quota": req.DailyQuota,
	}

	if req.VipLevel > 0 {
		expireTimes := map[int]time.Duration{
			1: time.Hour * 24 * 30,
			2: time.Hour * 24 * 365,
			3: time.Hour * 24 * 365 * 100,
		}
		
		if expire, ok := expireTimes[req.VipLevel]; ok {
			expireTime := time.Now().Add(expire)
			updates["vip_expire_time"] = expireTime
		}
	} else {
		updates["vip_expire_time"] = nil
	}

	if err := db.Model(&model.User{ID: uint(userID)}).Updates(updates).Error; err != nil {
		response.InternalError(c, "更新用户失败")
		return
	}

	response.SuccessWithMessage(c, "用户VIP信息已更新", nil)
}

func (h *AdminHandler) GetTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 50 { pageSize = 20 }

	db := repository.GetDB()
	var tasks []model.Task
	var total int64

	query := db.Model(&model.Task{})

	status := c.Query("status")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	platform := c.Query("platform_type")
	if platform != "" {
		query = query.Where("platform_type = ?", platform)
	}

	userID := c.Query("user_id")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	keyword := c.Query("keyword")
	if keyword != "" {
		query = query.Where("source_url LIKE ? OR title LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error

	if err != nil {
		response.InternalError(c, "获取任务列表失败")
		return
	}

	result := make([]*model.Task, len(tasks))
	for i := range tasks {
		result[i] = &tasks[i]
	}

	response.PageSuccess(c, result, total, page, pageSize)
}

func (h *AdminHandler) GetOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 50 { pageSize = 20 }

	db := repository.GetDB()
	var orders []model.Order
	var total int64

	query := db.Model(&model.Order{})

	status := c.Query("status")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	productType := c.Query("product_type")
	if productType != "" {
		query = query.Where("product_type = ?", productType)
	}

	userID := c.Query("user_id")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&orders).Error

	if err != nil {
		response.InternalError(c, "获取订单列表失败")
		return
	}

	result := make([]*model.Order, len(orders))
	for i := range orders {
		result[i] = &orders[i]
	}

	response.PageSuccess(c, result, total, page, pageSize)
}

func (h *AdminHandler) GetSystemInfo(c *gin.Context) {
	db := repository.GetDB()

	var userCount int64
	var taskCount int64
	var orderCount int64
	var pendingOrderCount int64

	db.Model(&model.User{}).Count(&userCount)
	db.Model(&model.Task{}).Count(&taskCount)
	db.Model(&model.Order{}).Count(&orderCount)
	db.Model(&model.Order{}).Where("status = ?", "pending").Count(&pendingOrderCount)

	response.Success(c, gin.H{
		"version":         "2.0.0",
		"go_version":      "1.21",
		"database":        "SQLite",
		"user_count":      userCount,
		"task_count":       taskCount,
		"order_count":      orderCount,
		"pending_orders":  pendingOrderCount,
		"uptime":          time.Since(startTime),
	})
}

func (h *AdminHandler) GetAnalyticsData(c *gin.Context) {
	db := repository.GetDB()

	type DailyTask struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}

	var dailyTasks []DailyTask

	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		dayStart, _ := time.Parse("2006-01-02", date)
		dayEnd := dayStart.AddDate(0, 0, 1)

		var count int64
		db.Model(&model.Task{}).
			Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
			Count(&count)

		dailyTasks = append(dailyTasks, DailyTask{
			Date:  date,
			Count: count,
		})
	}

	var platformStats []struct {
		Platform string `json:"platform"`
		Count    int64  `json:"count"`
	}
	db.Model(&model.Task{}).
		Select("platform_type as platform, count(*) as count").
		Where("status = ?", "success").
		Group("platform_type").
		Order("count DESC").
		Find(&platformStats)

	type HourlyStat struct {
		Hour   int   `json:"hour"`
		Count  int64 `json:"count"`
	}
	var hourlyStats []HourlyStat

	for hour := 0; hour < 24; hour++ {
		var count int64
		db.Model(&model.Task{}).
			Where("strftime('%H', created_at) = ?", fmt.Sprintf("%02d", hour)).
			Count(&count)

		hourlyStats = append(hourlyStats, HourlyStat{
			Hour:  hour,
			Count: count,
		})
	}

	response.Success(c, gin.H{
		"daily_tasks":    dailyTasks,
		"platform_stats": platformStats,
		"hourly_stats":   hourlyStats,
	})
}

var startTime = time.Now()

func generateAdminToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(tokenBytes), nil
}

func getTodayStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
