package impl

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
	"gorm.io/gorm"
	"gorm.io/datatypes"
)

type ExecutionServiceImpl struct {
	db            *gorm.DB
	routerService services.RouterService
}

func NewExecutionService(db *gorm.DB, routerService services.RouterService) services.ExecutionService {
	return &ExecutionServiceImpl{
		db:            db,
		routerService: routerService,
	}
}

func (s *ExecutionServiceImpl) StartExecution(ctx context.Context, req models.StartExecutionRequest, userID uuid.UUID) (*models.AgentExecution, error) {
	// Create execution record
	execution := &models.AgentExecution{
		AgentID:   req.AgentID,
		UserID:    userID,
		SessionID: req.SessionID,
		Status:    models.ExecutionStatusQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Marshal input data
	inputData, err := json.Marshal(req.InputData)
	if err != nil {
		return nil, err
	}
	execution.InputData = datatypes.JSON(inputData)

	// Save to database
	if err := s.db.Create(execution).Error; err != nil {
		return nil, err
	}

	return execution, nil
}

func (s *ExecutionServiceImpl) GetExecution(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.AgentExecution, error) {
	var execution models.AgentExecution
	
	err := s.db.Where("id = ? AND user_id = ?", id, userID).
		Preload("Agent").
		First(&execution).Error
	
	if err != nil {
		return nil, err
	}
	
	return &execution, nil
}

func (s *ExecutionServiceImpl) ListExecutions(ctx context.Context, filter models.ExecutionListFilter, userID uuid.UUID) (*models.ExecutionListResponse, error) {
	query := s.db.Model(&models.AgentExecution{}).Where("user_id = ?", userID)

	// Apply filters
	if filter.AgentID != nil {
		query = query.Where("agent_id = ?", *filter.AgentID)
	}
	if filter.SessionID != nil {
		query = query.Where("session_id = ?", *filter.SessionID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", *filter.EndDate)
	}
	if filter.WithRetries != nil && *filter.WithRetries {
		query = query.Where("retry_attempts > 0")
	}
	if filter.WithFallback != nil && *filter.WithFallback {
		query = query.Where("fallback_used = true")
	}

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// Apply pagination
	offset := (filter.Page - 1) * filter.Size
	query = query.Offset(offset).Limit(filter.Size)

	// Order by created_at desc
	query = query.Order("created_at DESC")

	// Preload agent data
	query = query.Preload("Agent")

	var executions []models.AgentExecution
	if err := query.Find(&executions).Error; err != nil {
		return nil, err
	}

	return &models.ExecutionListResponse{
		Executions: executions,
		Total:      total,
		Page:       filter.Page,
		Size:       filter.Size,
	}, nil
}

func (s *ExecutionServiceImpl) CancelExecution(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	result := s.db.Model(&models.AgentExecution{}).
		Where("id = ? AND user_id = ? AND status IN (?)", id, userID, []string{
			string(models.ExecutionStatusQueued),
			string(models.ExecutionStatusRunning),
		}).
		Updates(map[string]interface{}{
			"status":       models.ExecutionStatusCancelled,
			"completed_at": time.Now(),
			"updated_at":   time.Now(),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (s *ExecutionServiceImpl) GetExecutionsByAgent(ctx context.Context, agentID uuid.UUID, userID uuid.UUID, limit int) ([]models.AgentExecution, error) {
	var executions []models.AgentExecution
	
	err := s.db.Where("agent_id = ? AND user_id = ?", agentID, userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&executions).Error
	
	return executions, err
}

func (s *ExecutionServiceImpl) GetExecutionsBySession(ctx context.Context, sessionID string, userID uuid.UUID) ([]models.AgentExecution, error) {
	var executions []models.AgentExecution
	
	err := s.db.Where("session_id = ? AND user_id = ?", sessionID, userID).
		Order("created_at ASC").
		Find(&executions).Error
	
	return executions, err
}