package impl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

type agentServiceImpl struct {
	db *gorm.DB
}

func NewAgentService(db *gorm.DB) services.AgentService {
	return &agentServiceImpl{
		db: db,
	}
}

func (s *agentServiceImpl) CreateAgent(ctx context.Context, req models.CreateAgentRequest, ownerID string, tenantID string) (*models.Agent, error) {
	agent := &models.Agent{
		ID:           uuid.New(),
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		LLMConfig:    req.LLMConfig,
		OwnerID:      ownerID,
		SpaceID:      req.SpaceID,
		TenantID:     tenantID,
		Status:       models.AgentStatusDraft,
		IsPublic:     req.IsPublic,
		IsTemplate:   req.IsTemplate,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if len(req.NotebookIDs) > 0 {
		notebookJSON, err := models.ConvertToJSON(req.NotebookIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert notebook IDs: %w", err)
		}
		agent.NotebookIDs = notebookJSON
	}

	if len(req.Tags) > 0 {
		tagsJSON, err := models.ConvertToJSON(req.Tags)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tags: %w", err)
		}
		agent.Tags = tagsJSON
	}

	if err := s.db.WithContext(ctx).Create(agent).Error; err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return agent, nil
}

func (s *agentServiceImpl) GetAgent(ctx context.Context, id uuid.UUID, userID string) (*models.Agent, error) {
	var agent models.Agent
	
	query := s.db.WithContext(ctx).Where("id = ?", id)
	query = query.Where("(owner_id = ? OR is_public = true)", userID)
	
	if err := query.First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("agent not found or access denied")
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return &agent, nil
}

func (s *agentServiceImpl) GetAgentByOwner(ctx context.Context, id uuid.UUID, ownerID string) (*models.Agent, error) {
	var agent models.Agent
	
	if err := s.db.WithContext(ctx).Where("id = ? AND owner_id = ?", id, ownerID).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("agent not found")
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return &agent, nil
}

func (s *agentServiceImpl) UpdateAgent(ctx context.Context, id uuid.UUID, req models.UpdateAgentRequest, ownerID string) (*models.Agent, error) {
	var agent models.Agent
	
	if err := s.db.WithContext(ctx).Where("id = ? AND owner_id = ?", id, ownerID).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("agent not found")
		}
		return nil, fmt.Errorf("failed to find agent: %w", err)
	}

	updates := make(map[string]interface{})
	
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.SystemPrompt != nil {
		updates["system_prompt"] = *req.SystemPrompt
	}
	if req.LLMConfig != nil {
		updates["llm_config"] = *req.LLMConfig
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}
	if req.IsTemplate != nil {
		updates["is_template"] = *req.IsTemplate
	}
	
	if req.NotebookIDs != nil {
		notebookJSON, err := models.ConvertToJSON(req.NotebookIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to convert notebook IDs: %w", err)
		}
		updates["notebook_ids"] = notebookJSON
	}
	
	if req.Tags != nil {
		tagsJSON, err := models.ConvertToJSON(req.Tags)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tags: %w", err)
		}
		updates["tags"] = tagsJSON
	}
	
	updates["updated_at"] = time.Now()

	if err := s.db.WithContext(ctx).Model(&agent).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	return &agent, nil
}

func (s *agentServiceImpl) DeleteAgent(ctx context.Context, id uuid.UUID, ownerID string) error {
	result := s.db.WithContext(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.Agent{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to delete agent: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("agent not found or access denied")
	}

	return nil
}

func (s *agentServiceImpl) ListAgents(ctx context.Context, filter models.AgentListFilter, userID string) (*models.AgentListResponse, error) {
	query := s.db.WithContext(ctx).Model(&models.Agent{})
	
	query = query.Where("(owner_id = ? OR is_public = true)", userID)
	
	if filter.OwnerID != nil {
		query = query.Where("owner_id = ?", *filter.OwnerID)
	}
	if filter.SpaceID != nil {
		query = query.Where("space_id = ?", *filter.SpaceID)
	}
	if filter.TenantID != nil {
		query = query.Where("tenant_id = ?", *filter.TenantID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.SpaceType != nil {
		query = query.Where("space_type = ?", *filter.SpaceType)
	}
	if filter.IsPublic != nil {
		query = query.Where("is_public = ?", *filter.IsPublic)
	}
	if filter.IsTemplate != nil {
		query = query.Where("is_template = ?", *filter.IsTemplate)
	}
	
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchPattern, searchPattern)
	}
	
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count agents: %w", err)
	}
	
	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.Size
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	
	offset := (page - 1) * size
	
	var agents []models.Agent
	if err := query.Offset(offset).Limit(size).Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return &models.AgentListResponse{
		Agents: agents,
		Total:  total,
		Page:   page,
		Size:   size,
	}, nil
}

func (s *agentServiceImpl) PublishAgent(ctx context.Context, id uuid.UUID, ownerID string) error {
	result := s.db.WithContext(ctx).Model(&models.Agent{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Updates(map[string]interface{}{
			"status":     models.AgentStatusPublished,
			"updated_at": time.Now(),
		})
		
	if result.Error != nil {
		return fmt.Errorf("failed to publish agent: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("agent not found or access denied")
	}
	
	return nil
}

func (s *agentServiceImpl) UnpublishAgent(ctx context.Context, id uuid.UUID, ownerID string) error {
	result := s.db.WithContext(ctx).Model(&models.Agent{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Updates(map[string]interface{}{
			"status":     models.AgentStatusDraft,
			"updated_at": time.Now(),
		})
		
	if result.Error != nil {
		return fmt.Errorf("failed to unpublish agent: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return fmt.Errorf("agent not found or access denied")
	}
	
	return nil
}

func (s *agentServiceImpl) DuplicateAgent(ctx context.Context, sourceID uuid.UUID, newName string, userID string, tenantID string) (*models.Agent, error) {
	var sourceAgent models.Agent
	
	query := s.db.WithContext(ctx).Where("id = ?", sourceID)
	query = query.Where("(owner_id = ? OR is_public = true OR is_template = true)", userID)
	
	if err := query.First(&sourceAgent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("source agent not found or access denied")
		}
		return nil, fmt.Errorf("failed to get source agent: %w", err)
	}
	
	newAgent := sourceAgent
	newAgent.ID = uuid.New()
	newAgent.Name = newName
	newAgent.OwnerID = userID
	newAgent.TenantID = tenantID
	newAgent.Status = models.AgentStatusDraft
	newAgent.IsPublic = false
	newAgent.IsTemplate = false
	newAgent.TotalExecutions = 0
	newAgent.TotalCostUSD = 0
	newAgent.AvgResponseTimeMs = 0
	newAgent.LastExecutedAt = nil
	newAgent.CreatedAt = time.Now()
	newAgent.UpdatedAt = time.Now()
	newAgent.DeletedAt = nil

	if err := s.db.WithContext(ctx).Create(&newAgent).Error; err != nil {
		return nil, fmt.Errorf("failed to duplicate agent: %w", err)
	}

	return &newAgent, nil
}

func (s *agentServiceImpl) GetAgentsBySpace(ctx context.Context, spaceID uuid.UUID, userID string) ([]models.Agent, error) {
	var agents []models.Agent
	
	query := s.db.WithContext(ctx).Where("space_id = ?", spaceID)
	query = query.Where("(owner_id = ? OR is_public = true)", userID)
	
	if err := query.Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("failed to get agents by space: %w", err)
	}

	return agents, nil
}

func (s *agentServiceImpl) GetPublicAgents(ctx context.Context, filter models.AgentListFilter) (*models.AgentListResponse, error) {
	filter.IsPublic = &[]bool{true}[0]
	return s.ListAgents(ctx, filter, "")
}

func (s *agentServiceImpl) GetAgentTemplates(ctx context.Context, filter models.AgentListFilter) (*models.AgentListResponse, error) {
	filter.IsTemplate = &[]bool{true}[0]
	return s.ListAgents(ctx, filter, "")
}