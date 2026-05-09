package service

import (
	"cleanmark/internal/adapter"
	"cleanmark/internal/model"
	"cleanmark/internal/repository"
	"cleanmark/internal/utils"
	"cleanmark/pkg/errors"
	"context"
	"time"

	"gorm.io/gorm"
)

type TaskService struct {
	db           *gorm.DB
	userService  *UserService
}

func NewTaskService(userService *UserService) *TaskService {
	return &TaskService{
		db:          repository.GetDB(),
		userService: userService,
	}
}

type ParseRequest struct {
	URL     string `json:"url" binding:"required"`
	Quality string `json:"quality"` // sd/hd/uhd
}

type ParseResponse struct {
	TaskID      uint              `json:"task_id"`
	Status      string            `json:"status"`
	Title       string            `json:"title"`
	CoverURL    string            `json:"cover_url"`
	VideoURL    string            `json:"video_url"`
	CleanURL    string            `json:"clean_url"`
	Duration    int               `json:"duration"`
	FileSize    int64             `json:"file_size"`
	Author      string            `json:"author"`
	Platform    string            `json:"platform"`
	ProcessTime int               `json:"process_time"`
	CreatedAt   time.Time         `json:"created_at"`
}

type BatchParseRequest struct {
	URLs    []string `json:"urls" binding:"required,min=1,max=20"`
	Quality string   `json:"quality"`
}

func (s *TaskService) Parse(userID uint, req *ParseRequest) (*ParseResponse, error) {
	cleanURL := utils.ExtractURL(req.URL)
	
	platform := utils.DetectPlatform(cleanURL)
	if platform == "" {
		return nil, errors.ErrPlatformNotSupport
	}

	err := s.userService.UseQuota(userID)
	if err != nil {
		return nil, err
	}

	task := &model.Task{
		UserID:       userID,
		SourceURL:    cleanURL,
		PlatformType: platform,
		Status:       "processing",
		Quality:      req.Quality,
	}

	if task.Quality == "" {
		task.Quality = "hd"
	}

	if err := s.db.Create(task).Error; err != nil {
		return nil, errors.ErrInternalServer
	}

	go s.processTask(task)

	return &ParseResponse{
		TaskID:    task.ID,
		Status:    task.Status,
		Platform:  platform,
		CreatedAt: task.CreatedAt,
	}, nil
}

func (s *TaskService) BatchParse(userID uint, req *BatchParseRequest) ([]*ParseResponse, error) {
	var responses []*ParseResponse
	
	for _, url := range req.URLs {
		parseReq := &ParseRequest{
			URL:     url,
			Quality: req.Quality,
		}
		
		resp, err := s.Parse(userID, parseReq)
		if err != nil {
			responses = append(responses, &ParseResponse{
				Status: "failed",
			})
			continue
		}
		
		responses = append(responses, resp)
	}
	
	return responses, nil
}

func (s *TaskService) processTask(task *model.Task) {
	startTime := time.Now()

	defer func() {
		if r := recover(); r != nil {
			s.updateTaskFailed(task.ID, "处理过程发生异常")
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adp := adapter.GetAdapter(task.PlatformType)
	if adp == nil {
		s.updateTaskFailed(task.ID, "不支持的平台类型")
		return
	}

	videoInfo, err := adp.Parse(ctx, task.SourceURL)
	if err != nil {
		s.updateTaskFailed(task.ID, err.Error())
		return
	}

	processTime := int(time.Since(startTime).Milliseconds())

	updates := map[string]interface{}{
		"status":       "success",
		"title":        videoInfo.Title,
		"cover_url":    videoInfo.CoverURL,
		"original_url": videoInfo.VideoURL,
		"clean_url":    videoInfo.VideoURL,
		"file_size":    videoInfo.FileSize,
		"duration":     videoInfo.Duration,
		"process_time": processTime,
		"completed_at": time.Now(),
	}

	s.db.Model(task).Updates(updates)
}

func (s *TaskService) updateTaskFailed(taskID uint, errMsg string) {
	now := time.Now()
	s.db.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":        "failed",
		"error_message": errMsg,
		"completed_at":  now,
	})
}

func (s *TaskService) GetTask(userID, taskID uint) (*ParseResponse, error) {
	task := &model.Task{}
	if err := s.db.Where("id = ? AND user_id = ?", taskID, userID).First(task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.ErrNotFound
		}
		return nil, errors.ErrInternalServer
	}

	return s.taskToResponse(task), nil
}

func (s *TaskService) GetTaskList(userID uint, page, pageSize int, status string) ([]*ParseResponse, int64, error) {
	var tasks []model.Task
	var total int64

	query := s.db.Where("user_id = ?", userID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Model(&model.Task{}).Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&tasks).Error

	if err != nil {
		return nil, 0, errors.ErrInternalServer
	}

	var responses []*ParseResponse
	for _, task := range tasks {
		responses = append(responses, s.taskToResponse(&task))
	}

	return responses, total, nil
}

func (s *TaskService) DeleteTask(userID, taskID uint) error {
	result := s.db.Where("id = ? AND user_id = ?", taskID, userID).Delete(&model.Task{})
	if result.Error != nil {
		return errors.ErrInternalServer
	}
	if result.RowsAffected == 0 {
		return errors.ErrNotFound
	}
	return nil
}

func (s *TaskService) taskToResponse(task *model.Task) *ParseResponse {
	return &ParseResponse{
		TaskID:      task.ID,
		Status:      task.Status,
		Title:       task.Title,
		CoverURL:    task.CoverURL,
		VideoURL:    task.OriginalURL,
		CleanURL:    task.CleanURL,
		Duration:    task.Duration,
		FileSize:    task.FileSize,
		Platform:    task.PlatformType,
		ProcessTime: task.ProcessTime,
		CreatedAt:   task.CreatedAt,
	}
}
