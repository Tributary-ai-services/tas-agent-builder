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

// visibleToScope narrows a query to the agents the caller may READ.
//
// Three disjoint reasons an agent is readable:
//   - it is the caller's own, in the space they were verified into
//   - it is an internal system agent, which belongs to no space by design
//   - it is public, which is an explicit choice by its owner to share it
//     beyond their space (templates, the shared library)
//
// Note what this is NOT: it does not make every agent in a space readable by
// every member. That would be a widening, and nothing here knows space roles.
// The rule is the old ownership rule with space added as a second condition,
// so it can only ever return a subset of what it used to.
func visibleToScope(query *gorm.DB, scope services.AgentScope) *gorm.DB {
	return query.Where(
		"((owner_id = ? AND space_id = ?) OR is_internal = true OR is_public = true)",
		scope.UserID, scope.SpaceID,
	)
}

// writableByScope narrows a query to the agents the caller may MODIFY.
//
// Ownership inside the verified space, and nothing else. In particular there
// is no is_public or is_internal escape hatch: those flags used to appear in
// the update predicate, which made all 16 shared system agents writable — and
// their system prompts and notebook bindings rewritable — by any authenticated
// user (AB-5).
func writableByScope(query *gorm.DB, scope services.AgentScope) *gorm.DB {
	return query.Where("owner_id = ? AND space_id = ?", scope.UserID, scope.SpaceID)
}

func (s *agentServiceImpl) CreateAgent(ctx context.Context, req models.CreateAgentRequest, scope services.AgentScope, tenantID string) (*models.Agent, error) {
	// The agent is created in the space the caller was verified into. A
	// request naming a different space is refused rather than quietly
	// corrected, because the two mean different things to the caller and
	// silently moving their agent is its own kind of wrong.
	if req.SpaceID != "" && req.SpaceID != scope.SpaceID {
		return nil, fmt.Errorf("%w: cannot create an agent in another space", services.ErrAgentNotFound)
	}

	// Default SpaceType to personal if not specified
	spaceType := req.SpaceType
	if spaceType == "" {
		spaceType = models.SpaceTypePersonal
	}

	// Default Type to conversational if not specified
	agentType := req.Type
	if agentType == "" {
		agentType = models.AgentTypeConversational
	}

	// Default EnableKnowledge to true if not explicitly set
	// Since bool defaults to false, we check if notebooks are provided as a hint
	enableKnowledge := req.EnableKnowledge
	if !enableKnowledge && len(req.NotebookIDs) > 0 {
		enableKnowledge = true
	}

	// Default EnableMemory to true for conversational agents
	enableMemory := req.EnableMemory
	if !enableMemory && agentType == models.AgentTypeConversational {
		enableMemory = true
	}

	agent := &models.Agent{
		ID:           uuid.New(),
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		LLMConfig:    req.LLMConfig,
		OwnerID:      scope.UserID,
		SpaceID:      scope.SpaceID,
		SpaceType:    spaceType,
		Type:         agentType,
		TenantID:     tenantID,
		Status:       models.AgentStatusDraft,
		IsPublic:     req.IsPublic,
		IsTemplate:   req.IsTemplate,
		// IsInternal is NOT taken from the request. Internal agents are the
		// shared system tools every user can see and run; letting a caller
		// mint one would be a way to plant an agent in everyone's list.
		// They are seeded, not created through this API.
		IsInternal:      false,
		EnableKnowledge: enableKnowledge,
		EnableMemory:    enableMemory,
		DocumentContext: req.DocumentContext,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Set default document context if knowledge is enabled but no config provided
	if enableKnowledge && agent.DocumentContext == nil {
		agent.DocumentContext = &models.DocumentContextConfig{
			Strategy:            models.ContextStrategyVector,
			Scope:               models.DocumentScopeAll,
			IncludeSubNotebooks: false,
			MaxContextTokens:    8000,
			TopK:                10,
			MinScore:            0.7,
			VectorWeight:        0.5,
			FullDocWeight:       0.5,
		}
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

	if len(req.Skills) > 0 {
		skillsJSON, err := models.ConvertToJSON(req.Skills)
		if err != nil {
			return nil, fmt.Errorf("failed to convert skills: %w", err)
		}
		agent.Skills = skillsJSON
	}

	if err := s.db.WithContext(ctx).Create(agent).Error; err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return agent, nil
}

func (s *agentServiceImpl) GetAgent(ctx context.Context, id uuid.UUID, scope services.AgentScope) (*models.Agent, error) {
	var agent models.Agent

	query := visibleToScope(s.db.WithContext(ctx).Where("id = ?", id), scope)

	if err := query.First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, services.ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return &agent, nil
}

func (s *agentServiceImpl) GetAgentByOwner(ctx context.Context, id uuid.UUID, scope services.AgentScope) (*models.Agent, error) {
	var agent models.Agent

	query := writableByScope(s.db.WithContext(ctx).Where("id = ?", id), scope)
	if err := query.First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, services.ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return &agent, nil
}

func (s *agentServiceImpl) UpdateAgent(ctx context.Context, id uuid.UUID, req models.UpdateAgentRequest, scope services.AgentScope) (*models.Agent, error) {
	var agent models.Agent

	// Owner, in the verified space. Being public or internal is not a licence
	// to edit — that is what made every shared agent editable by anyone.
	if err := writableByScope(s.db.WithContext(ctx).Where("id = ?", id), scope).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, services.ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to find agent: %w", err)
	}

	updates := make(map[string]any)

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
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}
	if req.IsTemplate != nil {
		updates["is_template"] = *req.IsTemplate
	}
	// is_internal is deliberately not updatable: an agent cannot promote
	// itself into the shared system set. Those rows come from seed migrations.

	// Knowledge configuration updates
	if req.EnableKnowledge != nil {
		updates["enable_knowledge"] = *req.EnableKnowledge
	}
	if req.EnableMemory != nil {
		updates["enable_memory"] = *req.EnableMemory
	}
	if req.DocumentContext != nil {
		updates["document_context"] = req.DocumentContext
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

	if req.Skills != nil {
		skillsJSON, err := models.ConvertToJSON(req.Skills)
		if err != nil {
			return nil, fmt.Errorf("failed to convert skills: %w", err)
		}
		updates["skills"] = skillsJSON
	}

	updates["updated_at"] = time.Now()

	if err := s.db.WithContext(ctx).Model(&agent).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	// Reload the agent to get updated values
	if err := s.db.WithContext(ctx).First(&agent, id).Error; err != nil {
		return nil, fmt.Errorf("failed to reload agent: %w", err)
	}

	return &agent, nil
}

func (s *agentServiceImpl) DeleteAgent(ctx context.Context, id uuid.UUID, scope services.AgentScope) error {
	// Confirm the caller owns this agent in the space they are acting in.
	// This check used to be absent entirely — ownerID was accepted and never
	// used, so any authenticated user could delete any agent, including the
	// seeded system ones (AB-5).
	var count int64
	if err := writableByScope(s.db.WithContext(ctx).Model(&models.Agent{}).Where("id = ?", id), scope).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to look up agent: %w", err)
	}
	if count == 0 {
		return services.ErrAgentNotFound
	}

	// Delete related records first (executions, etc.) to avoid foreign key constraint errors
	// Delete agent executions
	if err := s.db.WithContext(ctx).Where("agent_id = ?", id).Delete(&models.AgentExecution{}).Error; err != nil {
		return fmt.Errorf("failed to delete agent executions: %w", err)
	}

	// Now delete the agent itself, under the same predicate as the check
	// above so that the authorization and the delete cannot disagree.
	result := writableByScope(s.db.WithContext(ctx).Where("id = ?", id), scope).Delete(&models.Agent{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete agent: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return services.ErrAgentNotFound
	}

	return nil
}

func (s *agentServiceImpl) ListAgents(ctx context.Context, filter models.AgentListFilter, scope services.AgentScope) (*models.AgentListResponse, error) {
	query := visibleToScope(s.db.WithContext(ctx).Model(&models.Agent{}), scope)

	// The remaining filters narrow what is already visible. filter.SpaceID is
	// NOT one of them: the space is the scope, so honouring a second,
	// caller-supplied space here is at best redundant and at worst the bug
	// this change exists to remove.
	if filter.OwnerID != nil {
		query = query.Where("owner_id = ?", *filter.OwnerID)
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
	if filter.IsInternal != nil {
		query = query.Where("is_internal = ?", *filter.IsInternal)
	}

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchPattern, searchPattern)
	}

	// Filter by tags - check if the agent's tags JSON array contains the specified tags
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			// Use PostgreSQL JSONB @> operator to check if tags array contains the specified tag
			query = query.Where("tags @> ?", fmt.Sprintf(`["%s"]`, tag))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count agents: %w", err)
	}

	page := max(filter.Page, 1)
	size := max(filter.Size, 1)
	size = min(size, 100)
	if filter.Size < 1 {
		size = 20
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

func (s *agentServiceImpl) PublishAgent(ctx context.Context, id uuid.UUID, scope services.AgentScope) error {
	result := writableByScope(s.db.WithContext(ctx).Model(&models.Agent{}).Where("id = ?", id), scope).
		Updates(map[string]any{
			"status":     models.AgentStatusPublished,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to publish agent: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return services.ErrAgentNotFound
	}

	return nil
}

func (s *agentServiceImpl) UnpublishAgent(ctx context.Context, id uuid.UUID, scope services.AgentScope) error {
	result := writableByScope(s.db.WithContext(ctx).Model(&models.Agent{}).Where("id = ?", id), scope).
		Updates(map[string]any{
			"status":     models.AgentStatusDraft,
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to unpublish agent: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return services.ErrAgentNotFound
	}

	return nil
}

func (s *agentServiceImpl) DuplicateAgent(ctx context.Context, sourceID uuid.UUID, newName string, scope services.AgentScope, tenantID string) (*models.Agent, error) {
	var sourceAgent models.Agent

	// Readable as a source: the caller's own agent in this space, a system
	// agent, a public one, or a template. Templates are a sharing mechanism
	// like is_public, so they stay copyable across spaces.
	query := s.db.WithContext(ctx).Where("id = ?", sourceID).
		Where("((owner_id = ? AND space_id = ?) OR is_internal = true OR is_public = true OR is_template = true)",
			scope.UserID, scope.SpaceID)

	if err := query.First(&sourceAgent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, services.ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get source agent: %w", err)
	}

	newAgent := sourceAgent
	newAgent.ID = uuid.New()
	newAgent.Name = newName
	newAgent.OwnerID = scope.UserID
	// The copy lands in the caller's space, not the source's. Copying a
	// public agent used to carry its owner's space_id across with it.
	newAgent.SpaceID = scope.SpaceID
	newAgent.TenantID = tenantID
	// A copy is never a system agent, whatever it was copied from.
	newAgent.IsInternal = false
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

// GetAgentsBySpace returns the agents of the caller's verified space.
//
// It used to take a uuid.UUID, which no live space id has ever been — they are
// opaque strings such as "space_1766596584" — so this could not be called with
// a real space at all. The space now comes from the verified scope, which
// removes both the type error and the question of whose space it is.
func (s *agentServiceImpl) GetAgentsBySpace(ctx context.Context, scope services.AgentScope) ([]models.Agent, error) {
	var agents []models.Agent

	query := s.db.WithContext(ctx).
		Where("space_id = ? AND (owner_id = ? OR is_public = true)", scope.SpaceID, scope.UserID)

	if err := query.Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("failed to get agents by space: %w", err)
	}

	return agents, nil
}

func (s *agentServiceImpl) GetPublicAgents(ctx context.Context, filter models.AgentListFilter, scope services.AgentScope) (*models.AgentListResponse, error) {
	filter.IsPublic = &[]bool{true}[0]
	return s.ListAgents(ctx, filter, scope)
}

func (s *agentServiceImpl) GetAgentTemplates(ctx context.Context, filter models.AgentListFilter, scope services.AgentScope) (*models.AgentListResponse, error) {
	filter.IsTemplate = &[]bool{true}[0]
	return s.ListAgents(ctx, filter, scope)
}

// GetInternalAgents returns all internal (system) agents available to all users
func (s *agentServiceImpl) GetInternalAgents(ctx context.Context) ([]models.Agent, error) {
	var agents []models.Agent

	if err := s.db.WithContext(ctx).
		Where("is_internal = ? AND deleted_at IS NULL", true).
		Order("name ASC").
		Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("failed to get internal agents: %w", err)
	}

	return agents, nil
}

// GetInternalAgent returns a specific internal agent by ID (no ownership check needed)
func (s *agentServiceImpl) GetInternalAgent(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	var agent models.Agent

	if err := s.db.WithContext(ctx).
		Where("id = ? AND is_internal = ? AND deleted_at IS NULL", id, true).
		First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("internal agent not found")
		}
		return nil, fmt.Errorf("failed to get internal agent: %w", err)
	}

	return &agent, nil
}
