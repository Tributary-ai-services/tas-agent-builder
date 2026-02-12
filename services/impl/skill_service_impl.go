package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type skillServiceImpl struct {
	db *gorm.DB
}

// NewSkillService creates a new SkillService implementation
func NewSkillService(db *gorm.DB) services.SkillService {
	return &skillServiceImpl{db: db}
}

func (s *skillServiceImpl) Create(ctx context.Context, skill *models.Skill) error {
	if skill.ID == uuid.Nil {
		skill.ID = uuid.New()
	}
	skill.CreatedAt = time.Now()
	skill.UpdatedAt = time.Now()

	if err := s.db.WithContext(ctx).Create(skill).Error; err != nil {
		return fmt.Errorf("failed to create skill: %w", err)
	}
	return nil
}

func (s *skillServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Skill, error) {
	var skill models.Skill
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&skill).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("skill not found")
		}
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}
	return &skill, nil
}

func (s *skillServiceImpl) GetByName(ctx context.Context, name string) (*models.Skill, error) {
	var skill models.Skill
	if err := s.db.WithContext(ctx).Where("name = ? AND deleted_at IS NULL", name).First(&skill).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("skill not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}
	return &skill, nil
}

func (s *skillServiceImpl) List(ctx context.Context, filter models.SkillListFilter) (*models.SkillListResponse, error) {
	query := s.db.WithContext(ctx).Model(&models.Skill{}).Where("deleted_at IS NULL")

	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.IsPublic != nil {
		query = query.Where("is_public = ?", *filter.IsPublic)
	}
	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(description) LIKE ?",
			searchTerm, searchTerm, searchTerm)
	}
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			query = query.Where("tags @> ?", datatypes.JSON(fmt.Sprintf(`[%q]`, tag)))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count skills: %w", err)
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	size := filter.Size
	if size < 1 {
		size = 50
	}

	var skills []models.Skill
	if err := query.Order("name ASC").Offset((page - 1) * size).Limit(size).Find(&skills).Error; err != nil {
		return nil, fmt.Errorf("failed to list skills: %w", err)
	}

	return &models.SkillListResponse{
		Skills: skills,
		Total:  total,
		Page:   page,
		Size:   size,
	}, nil
}

func (s *skillServiceImpl) Update(ctx context.Context, id uuid.UUID, req models.UpdateSkillRequest) (*models.Skill, error) {
	skill, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Tags != nil {
		tagsJSON, _ := json.Marshal(req.Tags)
		updates["tags"] = datatypes.JSON(tagsJSON)
	}
	if req.Keywords != nil {
		keywordsJSON, _ := json.Marshal(req.Keywords)
		updates["keywords"] = datatypes.JSON(keywordsJSON)
	}
	if req.MCPServerURL != nil {
		updates["mcp_server_url"] = *req.MCPServerURL
	}
	if req.MCPToolNames != nil {
		toolNamesJSON, _ := json.Marshal(req.MCPToolNames)
		updates["mcp_tool_names"] = datatypes.JSON(toolNamesJSON)
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}
	if req.Author != nil {
		updates["author"] = *req.Author
	}
	if req.Version != nil {
		updates["version"] = *req.Version
	}
	updates["updated_at"] = time.Now()

	if err := s.db.WithContext(ctx).Model(skill).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update skill: %w", err)
	}

	return s.GetByID(ctx, id)
}

func (s *skillServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	skill, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if skill.IsSystem {
		return fmt.Errorf("cannot delete system skill: %s", skill.Name)
	}

	now := time.Now()
	if err := s.db.WithContext(ctx).Model(skill).Update("deleted_at", &now).Error; err != nil {
		return fmt.Errorf("failed to delete skill: %w", err)
	}
	return nil
}

// ResolveForAgent returns the skills for an agent, combining explicit assignment and auto-detection from system prompt
func (s *skillServiceImpl) ResolveForAgent(ctx context.Context, agent *models.Agent) ([]models.Skill, error) {
	skillMap := make(map[string]*models.Skill)

	// 1. Load explicitly assigned skills
	var explicitNames []string
	if agent.Skills != nil {
		if err := json.Unmarshal(agent.Skills, &explicitNames); err != nil {
			log.Printf("[SKILLS] Warning: failed to parse agent skills JSON: %v", err)
		}
	}

	for _, name := range explicitNames {
		skill, err := s.GetByName(ctx, name)
		if err != nil {
			log.Printf("[SKILLS] Warning: explicit skill %q not found: %v", name, err)
			continue
		}
		skillMap[skill.Name] = skill
	}

	// 2. Auto-detect from system prompt keywords
	if agent.SystemPrompt != "" {
		promptLower := strings.ToLower(agent.SystemPrompt)

		var allSkills []models.Skill
		if err := s.db.WithContext(ctx).Where("deleted_at IS NULL").Find(&allSkills).Error; err != nil {
			log.Printf("[SKILLS] Warning: failed to load skills for auto-detect: %v", err)
		} else {
			for i := range allSkills {
				if _, exists := skillMap[allSkills[i].Name]; exists {
					continue // Already explicitly assigned
				}

				var keywords []string
				if err := json.Unmarshal(allSkills[i].Keywords, &keywords); err != nil {
					continue
				}

				for _, kw := range keywords {
					if strings.Contains(promptLower, strings.ToLower(kw)) {
						skillMap[allSkills[i].Name] = &allSkills[i]
						break
					}
				}
			}
		}
	}

	// Collect results
	result := make([]models.Skill, 0, len(skillMap))
	for _, skill := range skillMap {
		result = append(result, *skill)
	}

	log.Printf("[SKILLS] Resolved %d skills for agent %s (explicit: %d, auto-detected: %d)",
		len(result), agent.ID, len(explicitNames), len(result)-len(explicitNames))

	return result, nil
}

// SeedDefaults inserts built-in skills if they don't exist
func (s *skillServiceImpl) SeedDefaults(ctx context.Context) error {
	defaults := []models.Skill{
		{
			Name:        "visual_generation",
			DisplayName: "Visual Generation",
			Description: "Generate diagrams, mind maps, flowcharts, and visual content from text descriptions using Napkin AI",
			Type:        models.SkillTypeMCP,
			Icon:        "Image",
			Tags:        mustJSON([]string{"visual", "diagram", "chart", "mindmap", "napkin"}),
			Keywords:    mustJSON([]string{"visual", "diagram", "chart", "graph", "mindmap", "infographic", "illustration", "draw", "flowchart"}),
			MCPServerURL: "http://napkin-mcp.tas-mcp-servers.svc.cluster.local:8087",
			MCPToolNames: mustJSON([]string{"generate_visual", "list_styles", "get_visual_status", "download_visual", "list_visuals", "delete_visual"}),
			IsPublic:    true,
			IsSystem:    true,
			Author:      "TAS Platform",
			Version:     "1.0.0",
		},
	}

	for _, skill := range defaults {
		var existing models.Skill
		result := s.db.WithContext(ctx).Where("name = ?", skill.Name).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			skill.ID = uuid.New()
			skill.CreatedAt = time.Now()
			skill.UpdatedAt = time.Now()
			if err := s.db.WithContext(ctx).Create(&skill).Error; err != nil {
				log.Printf("[SKILLS] Warning: failed to seed skill %q: %v", skill.Name, err)
			} else {
				log.Printf("[SKILLS] Seeded default skill: %s", skill.Name)
			}
		} else if result.Error == nil {
			log.Printf("[SKILLS] Default skill %q already exists, skipping", skill.Name)
		}
	}

	return nil
}

// mustJSON marshals a value to datatypes.JSON, panicking on error
func mustJSON(v any) datatypes.JSON {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustJSON: %v", err))
	}
	return datatypes.JSON(b)
}
