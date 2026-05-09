package tests

import (
	"cleanmark/internal/model"
	"testing"
	"time"
)

func TestUserModel(t *testing.T) {
	db := setupTestDB(t)

	user := model.User{
		OpenID:     "test_openid_001",
		Nickname:   "测试用户",
		VipLevel:   0,
		DailyQuota: 3,
		UsedQuota:  0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err := db.Create(&user).Error
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	if user.ID == 0 {
		t.Error("用户ID不应该为0")
	}

	var foundUser model.User
	err = db.First(&foundUser, user.ID).Error
	if err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}

	if foundUser.Nickname != "测试用户" {
		t.Errorf("用户昵称不正确，得到: %s", foundUser.Nickname)
	}

	if foundUser.VipLevel != 0 {
		t.Errorf("VIP等级不正确，得到: %d", foundUser.VipLevel)
	}
}

func TestTaskModel(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	task := model.Task{
		UserID:       1,
		SourceURL:    "https://www.douyin.com/test",
		PlatformType: "douyin",
		Status:       "pending",
		Title:        "测试视频",
		Quality:      "hd",
		CreatedAt:    now,
	}

	err := db.Create(&task).Error
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}

	if task.ID == 0 {
		t.Error("任务ID不应该为0")
	}

	var foundTask model.Task
	err = db.First(&foundTask, task.ID).Error
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}

	if foundTask.Status != "pending" {
		t.Errorf("任务状态不正确，得到: %s", foundTask.Status)
	}

	if foundTask.PlatformType != "douyin" {
		t.Errorf("平台类型不正确，得到: %s", foundTask.PlatformType)
	}
}

func TestOrderModel(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	order := model.Order{
		UserID:     1,
		OrderNo:    "TEST2024010100001",
		ProductType: "monthly",
		Amount:     19.9,
		PayMethod:  "wechat",
		Status:     "pending",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := db.Create(&order).Error
	if err != nil {
		t.Fatalf("创建订单失败: %v", err)
	}

	if order.ID == 0 {
		t.Error("订单ID不应该为0")
	}

	var foundOrder model.Order
	err = db.First(&foundOrder, order.ID).Error
	if err != nil {
		t.Fatalf("查询订单失败: %v", err)
	}

	if foundOrder.OrderNo != "TEST2024010100001" {
		t.Errorf("订单号不正确，得到: %s", foundOrder.OrderNo)
	}

	if foundOrder.Amount != 19.9 {
		t.Errorf("金额不正确，得到: %f", foundOrder.Amount)
	}
}

func TestUserTaskRelation(t *testing.T) {
	db := setupTestDB(t)

	user := model.User{
		OpenID:     "relation_test_001",
		Nickname:   "关联测试用户",
		VipLevel:   1,
		DailyQuota: 999,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	db.Create(&user)

	task1 := model.Task{
		UserID:       user.ID,
		SourceURL:    "https://www.douyin.com/task1",
		PlatformType: "douyin",
		Status:       "success",
		Title:        "任务1",
		CreatedAt:    time.Now(),
	}
	db.Create(&task1)

	task2 := model.Task{
		UserID:       user.ID,
		SourceURL:    "https://www.kuaishou.com/task2",
		PlatformType: "kuaishou",
		Status:       "success",
		Title:        "任务2",
		CreatedAt:    time.Now(),
	}
	db.Create(&task2)

	var taskCount int64
	db.Model(&model.Task{}).Where("user_id = ?", user.ID).Count(&taskCount)

	if taskCount != 2 {
		t.Errorf("用户的任务数量应该是2，得到: %d", taskCount)
	}

	var tasks []model.Task
	db.Where("user_id = ?", user.ID).Find(&tasks)

	if len(tasks) != 2 {
		t.Errorf("查询到的任务数量应该是2，得到: %d", len(tasks))
	}
}
