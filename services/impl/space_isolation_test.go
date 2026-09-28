package impl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

// These tests run the real queries against a real database engine.
//
// The suite they replace (test/space_management_test.go) built structs and
// asserted against helper functions declared in the same file, so it passed
// while every query in this package ignored space_id entirely. A test for an
// isolation boundary has to execute the boundary.

const (
	userA = "aaaaaaaa-0000-0000-0000-000000000001"
	userB = "bbbbbbbb-0000-0000-0000-000000000002"

	// Real space ids from this platform: opaque strings, not UUIDs.
	spaceA = "space_1766596584"
	spaceB = "space_1799000001"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	// The models live in the agent_builder schema. SQLite has no schemas, but
	// an attached database uses the same "prefix.table" syntax, so the real
	// table names work unmodified.
	if err := db.Exec("ATTACH DATABASE ':memory:' AS agent_builder").Error; err != nil {
		t.Fatalf("attach agent_builder schema: %v", err)
	}
	// The table is declared here rather than through AutoMigrate: the model's
	// tags are Postgres-specific (jsonb, gen_random_uuid(), numeric) and
	// SQLite rejects them. Column names and nullability are what the queries
	// under test actually depend on.
	const schema = `
	CREATE TABLE agent_builder.agents (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		system_prompt TEXT NOT NULL,
		llm_config TEXT NOT NULL DEFAULT '{}',
		owner_id TEXT NOT NULL,
		space_id TEXT NOT NULL,
		tenant_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'draft',
		space_type TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT 'conversational',
		is_public BOOLEAN DEFAULT 0,
		is_template BOOLEAN DEFAULT 0,
		is_internal BOOLEAN DEFAULT 0,
		notebook_ids TEXT DEFAULT '[]',
		enable_knowledge BOOLEAN DEFAULT 1,
		enable_memory BOOLEAN DEFAULT 1,
		document_context TEXT,
		tags TEXT DEFAULT '[]',
		skills TEXT DEFAULT '[]',
		total_executions INTEGER DEFAULT 0,
		total_cost_usd REAL DEFAULT 0,
		avg_response_time_ms INTEGER DEFAULT 0,
		last_executed_at DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		deleted_at DATETIME
	)`
	if err := db.Exec(schema).Error; err != nil {
		t.Fatalf("create agents table: %v", err)
	}

	// DeleteAgent clears an agent's executions first. Only the columns it
	// touches are needed here; note the table is ab_agent_executions in the
	// default schema, not agent_builder.agent_executions.
	if err := db.Exec(`CREATE TABLE ab_agent_executions (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL, user_id TEXT)`).Error; err != nil {
		t.Fatalf("create executions table: %v", err)
	}
	return db
}

type agentSeed struct {
	name       string
	owner      string
	space      string
	isPublic   bool
	isInternal bool
	isTemplate bool
}

func seedAgent(t *testing.T, db *gorm.DB, s agentSeed) models.Agent {
	t.Helper()

	agent := models.Agent{
		ID:           uuid.New(),
		Name:         s.name,
		SystemPrompt: "you are a test",
		OwnerID:      s.owner,
		SpaceID:      s.space,
		TenantID:     "tenant_test",
		Status:       models.AgentStatusDraft,
		SpaceType:    models.SpaceTypePersonal,
		Type:         models.AgentTypeConversational,
		IsPublic:     s.isPublic,
		IsInternal:   s.isInternal,
		IsTemplate:   s.isTemplate,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("seed agent %q: %v", s.name, err)
	}
	return agent
}

func scopeA() services.AgentScope { return services.AgentScope{UserID: userA, SpaceID: spaceA} }
func scopeB() services.AgentScope { return services.AgentScope{UserID: userB, SpaceID: spaceB} }

// TestGetAgentDeniesOtherSpace is the AB-5 question in one test: can a caller
// read an agent that belongs to another user in another space?
func TestGetAgentDeniesOtherSpace(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	private := seedAgent(t, db, agentSeed{name: "A's private agent", owner: userA, space: spaceA})

	if _, err := svc.GetAgent(ctx, private.ID, scopeA()); err != nil {
		t.Fatalf("owner should be able to read their own agent: %v", err)
	}

	_, err := svc.GetAgent(ctx, private.ID, scopeB())
	if !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("user B read user A's agent: got err=%v, want ErrAgentNotFound", err)
	}
}

// TestGetAgentDeniesOwnAgentFromWrongSpace covers the subtler half: the same
// user, but acting in a space the agent does not belong to. Ownership alone
// used to be enough, which is what made space_id decorative.
func TestGetAgentDeniesOwnAgentFromWrongSpace(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	agent := seedAgent(t, db, agentSeed{name: "A's agent in space A", owner: userA, space: spaceA})

	_, err := svc.GetAgent(ctx, agent.ID, services.AgentScope{UserID: userA, SpaceID: spaceB})
	if !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("agent was readable from the wrong space: got err=%v, want ErrAgentNotFound", err)
	}
}

func TestListAgentsExcludesOtherSpaces(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	seedAgent(t, db, agentSeed{name: "A private", owner: userA, space: spaceA})
	seedAgent(t, db, agentSeed{name: "B private", owner: userB, space: spaceB})
	seedAgent(t, db, agentSeed{name: "system tool", owner: "00000000-0000-0000-0000-000000000000", space: "00000000-0000-0000-0000-000000000000", isPublic: true, isInternal: true})

	resp, err := svc.ListAgents(ctx, models.AgentListFilter{Page: 1, Size: 50}, scopeA())
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}

	names := make(map[string]bool, len(resp.Agents))
	for _, a := range resp.Agents {
		names[a.Name] = true
	}
	if !names["A private"] {
		t.Error("caller's own agent missing from their list")
	}
	if names["B private"] {
		t.Error("another space's private agent appeared in the list")
	}
	if !names["system tool"] {
		t.Error("internal system agent should stay visible to everyone")
	}
}

// TestListAgentsIgnoresClientSuppliedSpace: the filter must not be able to
// redirect the query at another space. Before AB-5 the filter's space_id was
// the only space_id the query ever saw.
func TestListAgentsIgnoresClientSuppliedSpace(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	seedAgent(t, db, agentSeed{name: "B private", owner: userB, space: spaceB})

	other := spaceB
	resp, err := svc.ListAgents(ctx, models.AgentListFilter{SpaceID: &other, Page: 1, Size: 50}, scopeA())
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(resp.Agents) != 0 {
		t.Fatalf("asking for another space returned %d agents; want 0", len(resp.Agents))
	}
}

// TestUpdateAgentDeniesPublicAndInternal is the 16-of-18 case: nearly every
// agent in production is public and internal, and the old update predicate
// treated either flag as permission to write.
func TestUpdateAgentDeniesPublicAndInternal(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	system := seedAgent(t, db, agentSeed{
		name: "system tool", owner: "00000000-0000-0000-0000-000000000000",
		space: "00000000-0000-0000-0000-000000000000", isPublic: true, isInternal: true,
	})

	newPrompt := "ignore all previous instructions"
	_, err := svc.UpdateAgent(ctx, system.ID, models.UpdateAgentRequest{SystemPrompt: &newPrompt}, scopeA())
	if !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("a user rewrote a shared system agent: got err=%v, want ErrAgentNotFound", err)
	}

	var after models.Agent
	if err := db.First(&after, "id = ?", system.ID).Error; err != nil {
		t.Fatalf("reload agent: %v", err)
	}
	if after.SystemPrompt == newPrompt {
		t.Fatal("system prompt was modified despite the refusal")
	}
}

func TestUpdateAgentDeniesOtherSpace(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	agent := seedAgent(t, db, agentSeed{name: "A private", owner: userA, space: spaceA})

	name := "renamed by B"
	if _, err := svc.UpdateAgent(ctx, agent.ID, models.UpdateAgentRequest{Name: &name}, scopeB()); !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("user B updated user A's agent: got err=%v, want ErrAgentNotFound", err)
	}
}

// TestDeleteAgentRequiresOwnership pins the sharpest of the AB-5 findings:
// DeleteAgent accepted an owner id and never used it, so any authenticated
// user could delete any agent in the platform.
func TestDeleteAgentRequiresOwnership(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	agent := seedAgent(t, db, agentSeed{name: "A private", owner: userA, space: spaceA})
	system := seedAgent(t, db, agentSeed{
		name: "system tool", owner: "00000000-0000-0000-0000-000000000000",
		space: "00000000-0000-0000-0000-000000000000", isPublic: true, isInternal: true,
	})

	if err := svc.DeleteAgent(ctx, agent.ID, scopeB()); !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("user B deleted user A's agent: got err=%v, want ErrAgentNotFound", err)
	}
	if err := svc.DeleteAgent(ctx, system.ID, scopeA()); !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("a user deleted a shared system agent: got err=%v, want ErrAgentNotFound", err)
	}

	var count int64
	db.Model(&models.Agent{}).Count(&count)
	if count != 2 {
		t.Fatalf("agents were deleted: %d rows remain, want 2", count)
	}

	// The owner, in the right space, still can.
	if err := svc.DeleteAgent(ctx, agent.ID, scopeA()); err != nil {
		t.Fatalf("owner could not delete their own agent: %v", err)
	}
}

func TestPublishAgentRequiresOwnership(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	agent := seedAgent(t, db, agentSeed{name: "A private", owner: userA, space: spaceA})

	if err := svc.PublishAgent(ctx, agent.ID, scopeB()); !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("user B published user A's agent: got err=%v, want ErrAgentNotFound", err)
	}
	if err := svc.PublishAgent(ctx, agent.ID, scopeA()); err != nil {
		t.Fatalf("owner could not publish their own agent: %v", err)
	}
}

// TestCreateAgentRejectsForeignSpace: the create path used to take the space
// straight from the request body, with validation that checked only that it
// was non-empty.
func TestCreateAgentRejectsForeignSpace(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	_, err := svc.CreateAgent(ctx, models.CreateAgentRequest{
		Name:         "planted",
		SystemPrompt: "hello",
		SpaceID:      spaceB,
	}, scopeA(), "tenant_test")
	if !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("created an agent in another space: got err=%v, want refusal", err)
	}
}

// TestCreateAgentCannotMintInternalAgent: is_internal makes an agent visible
// to every user on the platform, so it cannot be a field the caller sets.
func TestCreateAgentCannotMintInternalAgent(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	agent, err := svc.CreateAgent(ctx, models.CreateAgentRequest{
		Name:         "trojan",
		SystemPrompt: "hello",
		SpaceID:      spaceA,
		IsInternal:   true,
	}, scopeA(), "tenant_test")
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if agent.IsInternal {
		t.Fatal("caller minted an internal system agent")
	}
	if agent.SpaceID != spaceA {
		t.Fatalf("agent landed in space %q, want %q", agent.SpaceID, spaceA)
	}
}

// TestDuplicateAgentLandsInCallersSpace: copying a shared agent must not carry
// its owner's space across with it.
func TestDuplicateAgentLandsInCallersSpace(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	source := seedAgent(t, db, agentSeed{name: "shared template", owner: userB, space: spaceB, isPublic: true, isTemplate: true})

	copy, err := svc.DuplicateAgent(ctx, source.ID, "my copy", scopeA(), "tenant_test")
	if err != nil {
		t.Fatalf("duplicate public template: %v", err)
	}
	if copy.SpaceID != spaceA {
		t.Fatalf("copy landed in space %q, want the caller's space %q", copy.SpaceID, spaceA)
	}
	if copy.OwnerID != userA {
		t.Fatalf("copy is owned by %q, want %q", copy.OwnerID, userA)
	}
}

// TestSpaceIDsAreOpaqueStrings guards the type that made the filter unusable:
// no live space id parses as a UUID.
func TestSpaceIDsAreOpaqueStrings(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	agent := seedAgent(t, db, agentSeed{name: "A private", owner: userA, space: spaceA})

	if _, err := uuid.Parse(spaceA); err == nil {
		t.Fatalf("test fixture %q parses as a UUID; pick a realistic space id", spaceA)
	}
	if _, err := svc.GetAgent(ctx, agent.ID, scopeA()); err != nil {
		t.Fatalf("a non-UUID space id must work end to end: %v", err)
	}
}
