package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
	"gorm.io/datatypes"
)

// SkillHandlers handles skill CRUD HTTP endpoints
type SkillHandlers struct {
	skillService services.SkillService
}

// NewSkillHandlers creates a new SkillHandlers instance
func NewSkillHandlers(skillService services.SkillService) *SkillHandlers {
	return &SkillHandlers{
		skillService: skillService,
	}
}

// CreateSkill handles POST /api/v1/skills
func (h *SkillHandlers) CreateSkill(c *gin.Context) {
	var req models.CreateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if req.Name == "" || req.DisplayName == "" || req.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, display_name, and type are required"})
		return
	}

	// Validate type
	switch req.Type {
	case models.SkillTypeMCP, models.SkillTypeFunction, models.SkillTypeBuiltin:
		// valid
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'mcp', 'function', or 'builtin'"})
		return
	}

	// Build skill model
	skill := &models.Skill{
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
		Type:         req.Type,
		Icon:         req.Icon,
		MCPServerURL: req.MCPServerURL,
		IsPublic:     req.IsPublic,
		Author:       req.Author,
		Version:      req.Version,
	}

	if skill.Version == "" {
		skill.Version = "1.0.0"
	}

	if req.Tags != nil {
		tagsJSON, _ := json.Marshal(req.Tags)
		skill.Tags = datatypes.JSON(tagsJSON)
	}
	if req.Keywords != nil {
		keywordsJSON, _ := json.Marshal(req.Keywords)
		skill.Keywords = datatypes.JSON(keywordsJSON)
	}
	if req.MCPToolNames != nil {
		toolNamesJSON, _ := json.Marshal(req.MCPToolNames)
		skill.MCPToolNames = datatypes.JSON(toolNamesJSON)
	}

	if err := h.skillService.Create(c.Request.Context(), skill); err != nil {
		log.Printf("[SKILLS] Failed to create skill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create skill: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"skill": skill})
}

// ListSkills handles GET /api/v1/skills
func (h *SkillHandlers) ListSkills(c *gin.Context) {
	filter := models.SkillListFilter{
		Search: c.Query("search"),
		Page:   1,
		Size:   50,
	}

	if typeStr := c.Query("type"); typeStr != "" {
		t := models.SkillType(typeStr)
		filter.Type = &t
	}
	if tagsStr := c.Query("tags"); tagsStr != "" {
		filter.Tags = splitTags(tagsStr)
	}
	if pageStr := c.Query("page"); pageStr != "" {
		var page int
		if _, err := parseIntParam(pageStr, &page); err == nil && page > 0 {
			filter.Page = page
		}
	}
	if sizeStr := c.Query("size"); sizeStr != "" {
		var size int
		if _, err := parseIntParam(sizeStr, &size); err == nil && size > 0 {
			filter.Size = size
		}
	}

	result, err := h.skillService.List(c.Request.Context(), filter)
	if err != nil {
		log.Printf("[SKILLS] Failed to list skills: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list skills"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetSkill handles GET /api/v1/skills/:id
func (h *SkillHandlers) GetSkill(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid skill ID"})
		return
	}

	skill, err := h.skillService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Skill not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"skill": skill})
}

// UpdateSkill handles PUT /api/v1/skills/:id
func (h *SkillHandlers) UpdateSkill(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid skill ID"})
		return
	}

	var req models.UpdateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	skill, err := h.skillService.Update(c.Request.Context(), id, req)
	if err != nil {
		log.Printf("[SKILLS] Failed to update skill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update skill: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"skill": skill})
}

// DeleteSkill handles DELETE /api/v1/skills/:id
func (h *SkillHandlers) DeleteSkill(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid skill ID"})
		return
	}

	if err := h.skillService.Delete(c.Request.Context(), id); err != nil {
		if err.Error() == "skill not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Skill not found"})
			return
		}
		if contains(err.Error(), "cannot delete system skill") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[SKILLS] Failed to delete skill: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete skill"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Skill deleted successfully"})
}

// helpers

func splitTags(s string) []string {
	parts := make([]string, 0)
	for _, p := range splitString(s, ",") {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	result := make([]string, 0)
	for len(s) > 0 {
		idx := indexOf(s, sep)
		if idx < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	return result
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func contains(s, sub string) bool {
	return indexOf(s, sub) >= 0
}

func parseIntParam(s string, out *int) (bool, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
		n = n*10 + int(c-'0')
	}
	*out = n
	return true, nil
}
