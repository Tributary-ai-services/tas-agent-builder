package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// SkillType defines the type of a skill
type SkillType string

const (
	SkillTypeMCP      SkillType = "mcp"      // MCP server tools
	SkillTypeFunction SkillType = "function"  // Custom functions (future)
	SkillTypeBuiltin  SkillType = "builtin"   // Built-in capabilities
)

// Skill represents a capability that can be assigned to agents
type Skill struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name        string         `json:"name" gorm:"uniqueIndex;not null"`
	DisplayName string         `json:"display_name" gorm:"not null"`
	Description string         `json:"description"`
	Type        SkillType      `json:"type" gorm:"type:varchar(50);not null"`
	Icon        string         `json:"icon,omitempty"`
	Tags        datatypes.JSON `json:"tags" gorm:"type:jsonb;default:'[]'"`
	Keywords    datatypes.JSON `json:"keywords" gorm:"type:jsonb;default:'[]'"`

	// MCP-specific fields
	MCPServerURL string         `json:"mcp_server_url,omitempty"`
	MCPToolNames datatypes.JSON `json:"mcp_tool_names" gorm:"type:jsonb;default:'[]'"`

	// Metadata
	IsPublic  bool   `json:"is_public" gorm:"default:true"`
	IsSystem  bool   `json:"is_system" gorm:"default:false"`
	Author    string `json:"author,omitempty"`
	Version   string `json:"version" gorm:"default:'1.0.0'"`

	CreatedAt time.Time  `json:"created_at" gorm:"not null;default:now()"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"not null;default:now()"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

func (Skill) TableName() string {
	return "agent_builder.skills"
}

// CreateSkillRequest is the request to create a new skill
type CreateSkillRequest struct {
	Name         string    `json:"name" validate:"required,min=1,max=255"`
	DisplayName  string    `json:"display_name" validate:"required,min=1,max=255"`
	Description  string    `json:"description" validate:"max=1000"`
	Type         SkillType `json:"type" validate:"required"`
	Icon         string    `json:"icon,omitempty"`
	Tags         []string  `json:"tags"`
	Keywords     []string  `json:"keywords"`
	MCPServerURL string    `json:"mcp_server_url,omitempty"`
	MCPToolNames []string  `json:"mcp_tool_names,omitempty"`
	IsPublic     bool      `json:"is_public"`
	Author       string    `json:"author,omitempty"`
	Version      string    `json:"version,omitempty"`
}

// UpdateSkillRequest is the request to update an existing skill
type UpdateSkillRequest struct {
	DisplayName  *string    `json:"display_name,omitempty"`
	Description  *string    `json:"description,omitempty"`
	Icon         *string    `json:"icon,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
	Keywords     []string   `json:"keywords,omitempty"`
	MCPServerURL *string    `json:"mcp_server_url,omitempty"`
	MCPToolNames []string   `json:"mcp_tool_names,omitempty"`
	IsPublic     *bool      `json:"is_public,omitempty"`
	Author       *string    `json:"author,omitempty"`
	Version      *string    `json:"version,omitempty"`
}

// SkillListResponse is the paginated response for skill listing
type SkillListResponse struct {
	Skills []Skill `json:"skills"`
	Total  int64   `json:"total"`
	Page   int     `json:"page"`
	Size   int     `json:"size"`
}

// SkillListFilter defines filter criteria for listing skills
type SkillListFilter struct {
	Type     *SkillType `json:"type"`
	Tags     []string   `json:"tags"`
	Search   string     `json:"search"`
	IsPublic *bool      `json:"is_public"`
	Page     int        `json:"page"`
	Size     int        `json:"size"`
}
