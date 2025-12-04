package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/adiecho/oci-panel/internal/database"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/google/uuid"
)

type TaskService struct {
	ociService *OCIService
	stopChan   chan struct{}
	running    bool
	mutex      sync.Mutex
	taskTimers map[string]*time.Timer
	timerMutex sync.RWMutex
}

func NewTaskService(ociService *OCIService) *TaskService {
	return &TaskService{
		ociService: ociService,
		stopChan:   make(chan struct{}),
		taskTimers: make(map[string]*time.Timer),
	}
}

func (s *TaskService) Start() {
	s.mutex.Lock()
	if s.running {
		s.mutex.Unlock()
		return
	}
	s.running = true
	s.stopChan = make(chan struct{})
	s.mutex.Unlock()

	go s.loadAndStartTasks()
	log.Println("Task service started")
}

func (s *TaskService) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	close(s.stopChan)
	s.running = false

	s.timerMutex.Lock()
	for _, timer := range s.taskTimers {
		timer.Stop()
	}
	s.taskTimers = make(map[string]*time.Timer)
	s.timerMutex.Unlock()

	log.Println("Task service stopped")
}

func (s *TaskService) loadAndStartTasks() {
	db := database.GetDB()
	var tasks []models.OciCreateTask
	db.Where("status = ?", "running").Find(&tasks)

	for _, task := range tasks {
		s.scheduleTask(task)
	}
}

func (s *TaskService) scheduleTask(task models.OciCreateTask) {
	s.timerMutex.Lock()
	defer s.timerMutex.Unlock()

	if existingTimer, ok := s.taskTimers[task.ID]; ok {
		existingTimer.Stop()
	}

	interval := time.Duration(task.Interval) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}

	timer := time.AfterFunc(interval, func() {
		s.executeTask(task.ID)
	})
	s.taskTimers[task.ID] = timer
}

func (s *TaskService) executeTask(taskID string) {
	db := database.GetDB()
	var task models.OciCreateTask
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		log.Printf("Task %s not found: %v", taskID, err)
		return
	}

	if task.Status != "running" {
		return
	}

	var user models.OciUser
	if err := db.Where("id = ?", task.UserID).First(&user).Error; err != nil {
		s.logTaskExecution(taskID, "error", fmt.Sprintf("配置不存在: %v", err))
		return
	}

	var sshKey models.SSHKey
	if err := db.Where("id = ?", task.SSHKeyID).First(&sshKey).Error; err != nil {
		s.logTaskExecution(taskID, "error", fmt.Sprintf("SSH密钥不存在: %v", err))
		return
	}

	ctx := context.Background()
	err := s.ociService.CreateInstance(ctx, &user, task.OciRegion, task.Architecture, task.OperationSystem,
		task.Ocpus, task.Memory, task.Disk, sshKey.PublicKey, task.ImageId)

	now := time.Now()
	task.ExecuteCount++
	task.LastExecuteTime = &now

	if err != nil {
		task.LastMessage = fmt.Sprintf("创建失败: %v", err)
		s.logTaskExecution(taskID, "error", task.LastMessage)
	} else {
		task.SuccessCount++
		task.LastMessage = "创建成功"
		task.Status = "completed"
		s.logTaskExecution(taskID, "success", "实例创建成功")
	}

	db.Save(&task)

	if task.Status == "running" {
		s.scheduleTask(task)
	} else {
		s.removeTaskTimer(taskID)
	}
}

func (s *TaskService) logTaskExecution(taskID, status, message string) {
	db := database.GetDB()
	logEntry := models.TaskLog{
		ID:          uuid.New().String(),
		TaskID:      taskID,
		Status:      status,
		Message:     message,
		ExecuteTime: time.Now(),
	}
	db.Create(&logEntry)
}

func (s *TaskService) removeTaskTimer(taskID string) {
	s.timerMutex.Lock()
	defer s.timerMutex.Unlock()

	if timer, ok := s.taskTimers[taskID]; ok {
		timer.Stop()
		delete(s.taskTimers, taskID)
	}
}

func (s *TaskService) AddTask(task *models.OciCreateTask) error {
	db := database.GetDB()
	if err := db.Create(task).Error; err != nil {
		return err
	}

	if task.Status == "running" {
		s.scheduleTask(*task)
	}
	return nil
}

func (s *TaskService) StartTask(taskID string) error {
	db := database.GetDB()
	var task models.OciCreateTask
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		return err
	}

	task.Status = "running"
	if err := db.Save(&task).Error; err != nil {
		return err
	}

	s.scheduleTask(task)
	return nil
}

func (s *TaskService) StopTask(taskID string) error {
	db := database.GetDB()
	if err := db.Model(&models.OciCreateTask{}).Where("id = ?", taskID).Update("status", "stopped").Error; err != nil {
		return err
	}

	s.removeTaskTimer(taskID)
	return nil
}

func (s *TaskService) DeleteTask(taskID string) error {
	s.removeTaskTimer(taskID)

	db := database.GetDB()
	db.Where("task_id = ?", taskID).Delete(&models.TaskLog{})
	return db.Where("id = ?", taskID).Delete(&models.OciCreateTask{}).Error
}

func (s *TaskService) GetTaskLogs(taskID string, page, pageSize int) ([]models.TaskLog, int64, error) {
	db := database.GetDB()
	var logs []models.TaskLog
	var total int64

	query := db.Model(&models.TaskLog{}).Where("task_id = ?", taskID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Order("execute_time DESC").Limit(pageSize).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (s *TaskService) ClearTaskLogs(taskID string) error {
	db := database.GetDB()
	return db.Where("task_id = ?", taskID).Delete(&models.TaskLog{}).Error
}
