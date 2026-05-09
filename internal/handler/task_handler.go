package handler

import (
	"cleanmark/internal/model"
	"cleanmark/internal/repository"
	"cleanmark/internal/service"
	"cleanmark/internal/utils"
	"cleanmark/pkg/errors"
	"cleanmark/pkg/response"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	taskService *service.TaskService
}

func NewTaskHandler(taskService *service.TaskService) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
	}
}

func (h *TaskHandler) Parse(c *gin.Context) {
	var req service.ParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	result, err := h.taskService.Parse(uid, &req)
	if err != nil {
		appErr := err.(*errors.AppError)
		
		if appErr.Code == 429 {
			response.Error(c, http.StatusTooManyRequests, appErr.Code, appErr.Message)
			return
		}
		
		response.Error(c, http.StatusBadRequest, appErr.Code, appErr.Message)
		return
	}

	response.SuccessWithMessage(c, "解析任务已提交", result)
}

func (h *TaskHandler) BatchParse(c *gin.Context) {
	var req service.BatchParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	results, err := h.taskService.BatchParse(uid, &req)
	if err != nil {
		appErr := err.(*errors.AppError)
		response.Error(c, http.StatusInternalServerError, appErr.Code, appErr.Message)
		return
	}

	response.SuccessWithMessage(c, "批量任务已提交", results)
}

func (h *TaskHandler) GetTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	result, err := h.taskService.GetTask(uid, uint(taskID))
	if err != nil {
		appErr := err.(*errors.AppError)
		response.Error(c, http.StatusNotFound, appErr.Code, appErr.Message)
		return
	}

	response.Success(c, result)
}

func (h *TaskHandler) GetTaskList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	list, total, err := h.taskService.GetTaskList(uid, page, pageSize, status)
	if err != nil {
		response.InternalError(c, "获取任务列表失败")
		return
	}

	response.PageSuccess(c, list, total, page, pageSize)
}

func (h *TaskHandler) DeleteTask(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint)

	err = h.taskService.DeleteTask(uid, uint(taskID))
	if err != nil {
		appErr := err.(*errors.AppError)
		response.Error(c, http.StatusNotFound, appErr.Code, appErr.Message)
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

func (h *TaskHandler) DetectPlatform(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请提供有效的URL")
		return
	}

	platform := utils.DetectPlatform(req.URL)
	if platform == "" {
		response.Error(c, http.StatusBadRequest, 400, "不支持的平台或无效的链接")
		return
	}

	platformNames := map[string]string{
		"douyin":      "抖音",
		"kuaishou":    "快手",
		"xiaohongshu": "小红书",
		"bilibili":    "B站",
		"weibo":       "微博",
	}

	name, ok := platformNames[platform]
	if !ok {
		name = platform
	}

	response.Success(c, gin.H{
		"platform":     platform,
		"platform_name": name,
		"supported":    true,
	})
}

func (h *TaskHandler) DownloadProxy(c *gin.Context) {
	taskIDStr := c.Param("taskId")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}

	task := &model.Task{}
	db := repository.GetDB()
	if err := db.First(task, uint(taskID)).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	if task.Status != "success" || task.CleanURL == "" {
		response.BadRequest(c, "视频尚未解析完成或下载链接不可用")
		return
	}

	platform := task.PlatformType
	
	if platform == "weibo" || platform == "xiaohongshu" {
		result := gin.H{
			"url":      task.CleanURL,
			"title":    task.Title,
			"platform": platform,
			"message":  "由于平台防盗链限制，请点击下方链接直接打开视频",
		}
		c.JSON(200, result)
		return
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	req, err := http.NewRequest("GET", task.CleanURL, nil)
	if err != nil {
		response.InternalError(c, "创建下载请求失败")
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", getReferer(task.PlatformType))
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	
	if task.PlatformType == "xiaohongshu" {
		req.Header.Set("Origin", "https://www.xiaohongshu.com")
		req.Header.Set("Sec-Fetch-Dest", "video")
		req.Header.Set("Sec-Fetch-Mode", "no-cors")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
	}
	
	if task.PlatformType == "bilibili" {
		req.Header.Set("Origin", "https://www.bilibili.com")
		req.Header.Set("Sec-Fetch-Dest", "video")
		req.Header.Set("Sec-Fetch-Mode", "no-cors")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
	}
	
	if task.PlatformType == "weibo" {
		req.Header.Set("Origin", "https://video.weibo.com")
		req.Header.Set("Sec-Fetch-Dest", "video")
		req.Header.Set("Sec-Fetch-Mode", "no-cors")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
	}

	resp, err := client.Do(req)
	if err != nil {
		response.InternalError(c, "下载文件失败")
		return
	}
	defer resp.Body.Close()

	filename := generateFilename(task.Title, task.PlatformType)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Cache-Control", "no-cache")
	c.Header("Expires", "0")

	io.Copy(c.Writer, resp.Body)
}

func (h *TaskHandler) GetStats(c *gin.Context) {
	db := repository.GetDB()

	var totalTasks int64
	var successTasks int64
	var failedTasks int64
	var totalUsers int64

	db.Model(&model.Task{}).Count(&totalTasks)
	db.Model(&model.Task{}).Where("status = ?", "success").Count(&successTasks)
	db.Model(&model.Task{}).Where("status = ?", "failed").Count(&failedTasks)
	db.Model(&model.User{}).Count(&totalUsers)

	var platformStats []struct {
		Platform string `json:"platform"`
		Count    int64  `json:"count"`
	}

	db.Model(&model.Task{}).
		Select("platform_type as platform, count(*) as count").
		Where("status = ?", "success").
		Group("platform_type").
		Find(&platformStats)

	response.Success(c, gin.H{
		"total_tasks":   totalTasks,
		"success_tasks":  successTasks,
		"failed_tasks":   failedTasks,
		"total_users":    totalUsers,
		"success_rate":   float64(successTasks) / float64(totalTasks+1) * 100,
		"platforms":      platformStats,
	})
}

func getReferer(platform string) string {
	referers := map[string]string{
		"douyin":      "https://www.douyin.com/",
		"kuaishou":    "https://www.kuaishou.com/",
		"xiaohongshu": "https://www.xiaohongshu.com/",
		"bilibili":    "https://www.bilibili.com/",
		"weibo":       "https://video.weibo.com/",
	}
	
	if ref, ok := referers[platform]; ok {
		return ref
	}
	return ""
}

func generateFilename(title, platform string) string {
	suffixes := map[string]string{
		"douyin":      "_douyin",
		"kuaishou":    "_kuaishou",
		"xiaohongshu": "_xhs",
		"bilibili":    "_bilibili",
	}

	suffix, ok := suffixes[platform]
	if !ok {
		suffix = "_video"
	}

	if title == "" || len(title) > 50 {
		title = "cleanmark_video"
	} else {
		title = sanitizeFilename(title)
	}

	return title + suffix + ".mp4"
}

func sanitizeFilename(s string) string {
	replacements := map[rune]string{
		'/': "_",
		'\\': "_",
		':': "_",
		'*': "_",
		'?': "_",
		'"': "_",
		'<': "_",
		'>': "_",
		'|': "_",
		' ': "_",
	}

	result := make([]rune, 0, len(s))
	for _, r := range s {
		if repl, ok := replacements[r]; ok {
			result = append(result, []rune(repl)...)
		} else {
			result = append(result, r)
		}
	}

	return string(result)
}
