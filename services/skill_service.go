package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
)

// SkillService manages skill CRUD and resolution for agents
type SkillService interface {
	Create(ctx context.Context, skill *models.Skill) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Skill, error)
	GetByName(ctx context.Context, name string) (*models.Skill, error)
	List(ctx context.Context, filter models.SkillListFilter) (*models.SkillListResponse, error)
	Update(ctx context.Context, id uuid.UUID, req models.UpdateSkillRequest) (*models.Skill, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ResolveForAgent(ctx context.Context, agent *models.Agent) ([]models.Skill, error)
	SeedDefaults(ctx context.Context) error
}
