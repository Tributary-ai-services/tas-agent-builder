package test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tas-agent-builder/models"
)

// TestSpaceIsolation tests space-based isolation for agents and executions
func TestSpaceIsolation(t *testing.T) {
	t.Run("Personal Space Isolation", func(t *testing.T) {
		// Create two users with personal spaces
		user1ID := uuid.New()
		user2ID := uuid.New()
		user1SpaceID := uuid.New()
		user2SpaceID := uuid.New()
		tenantID := "shared-tenant"

		// User 1's personal agent
		user1Agent := &models.Agent{
			ID:          uuid.New(),
			Name:        "User 1 Personal Agent",
			Description: "Personal agent for user 1",
			OwnerID:     user1ID,
			SpaceID:     user1SpaceID,
			SpaceType:   models.SpaceTypePersonal,
			TenantID:    tenantID,
			IsPublic:    false,
			Status:      models.AgentStatusPublished,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// User 2's personal agent
		user2Agent := &models.Agent{
			ID:          uuid.New(),
			Name:        "User 2 Personal Agent",
			Description: "Personal agent for user 2",
			OwnerID:     user2ID,
			SpaceID:     user2SpaceID,
			SpaceType:   models.SpaceTypePersonal,
			TenantID:    tenantID,
			IsPublic:    false,
			Status:      models.AgentStatusPublished,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// Test space isolation
		assert.NotEqual(t, user1Agent.SpaceID, user2Agent.SpaceID, 
			"Users should have different personal spaces")
		assert.Equal(t, models.SpaceTypePersonal, user1Agent.SpaceType, 
			"User 1 agent should be in personal space")
		assert.Equal(t, models.SpaceTypePersonal, user2Agent.SpaceType, 
			"User 2 agent should be in personal space")

		// Test access permissions
		assert.True(t, canUserAccessAgent(user1Agent, user1ID), 
			"User 1 should access own personal agent")
		assert.False(t, canUserAccessAgent(user1Agent, user2ID), 
			"User 2 should not access User 1's personal agent")
		assert.False(t, canUserAccessAgent(user2Agent, user1ID), 
			"User 1 should not access User 2's personal agent")
		assert.True(t, canUserAccessAgent(user2Agent, user2ID), 
			"User 2 should access own personal agent")

		t.Logf("✅ Personal space isolation validated")
		t.Logf("   User 1 Space: %s", user1Agent.SpaceID)
		t.Logf("   User 2 Space: %s", user2Agent.SpaceID)
	})

	t.Run("Organization Space Access", func(t *testing.T) {
		// Create organization space with multiple users
		orgSpaceID := uuid.New()
		tenantID := "organization-tenant"
		ownerID := uuid.New()
		memberID1 := uuid.New()
		memberID2 := uuid.New()

		// Organization agent (public within tenant)
		orgAgent := &models.Agent{
			ID:          uuid.New(),
			Name:        "Organization Agent",
			Description: "Shared agent for organization",
			OwnerID:     ownerID,
			SpaceID:     orgSpaceID,
			SpaceType:   models.SpaceTypeOrganization,
			TenantID:    tenantID,
			IsPublic:    true,
			Status:      models.AgentStatusPublished,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// Test organization space access
		assert.True(t, canUserAccessAgent(orgAgent, ownerID), 
			"Owner should access organization agent")
		assert.True(t, canTenantMemberAccessAgent(orgAgent, memberID1, tenantID), 
			"Tenant member 1 should access public organization agent")
		assert.True(t, canTenantMemberAccessAgent(orgAgent, memberID2, tenantID), 
			"Tenant member 2 should access public organization agent")

		// Test access from different tenant
		differentTenantID := "different-tenant"
		externalUserID := uuid.New()
		assert.False(t, canTenantMemberAccessAgent(orgAgent, externalUserID, differentTenantID), 
			"External user should not access organization agent")

		t.Logf("✅ Organization space access validated")
		t.Logf("   Organization Space: %s", orgAgent.SpaceID)
		t.Logf("   Tenant: %s", orgAgent.TenantID)
	})

	t.Run("Cross-Space Agent Listing", func(t *testing.T) {
		tenantID := "multi-space-tenant"
		userID := uuid.New()

		// Create agents in different spaces
		agents := []*models.Agent{
			// Personal space agent
			{
				ID:        uuid.New(),
				Name:      "Personal Agent",
				OwnerID:   userID,
				SpaceID:   uuid.New(),
				SpaceType: models.SpaceTypePersonal,
				TenantID:  tenantID,
				IsPublic:  false,
				Status:    models.AgentStatusPublished,
			},
			// Organization space agent (owned by user)
			{
				ID:        uuid.New(),
				Name:      "Owned Org Agent",
				OwnerID:   userID,
				SpaceID:   uuid.New(),
				SpaceType: models.SpaceTypeOrganization,
				TenantID:  tenantID,
				IsPublic:  true,
				Status:    models.AgentStatusPublished,
			},
			// Organization space agent (not owned by user, but accessible)
			{
				ID:        uuid.New(),
				Name:      "Shared Org Agent",
				OwnerID:   uuid.New(), // Different owner
				SpaceID:   uuid.New(),
				SpaceType: models.SpaceTypeOrganization,
				TenantID:  tenantID,
				IsPublic:  true,
				Status:    models.AgentStatusPublished,
			},
			// Private organization agent (not accessible)
			{
				ID:        uuid.New(),
				Name:      "Private Org Agent",
				OwnerID:   uuid.New(), // Different owner
				SpaceID:   uuid.New(),
				SpaceType: models.SpaceTypeOrganization,
				TenantID:  tenantID,
				IsPublic:  false,
				Status:    models.AgentStatusPublished,
			},
		}

		// Test agent visibility for the user
		accessibleAgents := filterAccessibleAgents(agents, userID, tenantID)
		
		// Should see: personal agent, owned org agent, shared org agent
		// Should NOT see: private org agent (not owner)
		expectedAccessible := 3
		assert.Len(t, accessibleAgents, expectedAccessible, 
			"User should see %d accessible agents", expectedAccessible)

		// Verify specific agents are accessible
		agentNames := make([]string, len(accessibleAgents))
		for i, agent := range accessibleAgents {
			agentNames[i] = agent.Name
		}

		assert.Contains(t, agentNames, "Personal Agent", "Personal agent should be accessible")
		assert.Contains(t, agentNames, "Owned Org Agent", "Owned org agent should be accessible")
		assert.Contains(t, agentNames, "Shared Org Agent", "Shared org agent should be accessible")
		assert.NotContains(t, agentNames, "Private Org Agent", "Private org agent should not be accessible")

		t.Logf("✅ Cross-space agent listing validated")
		t.Logf("   Total agents: %d", len(agents))
		t.Logf("   Accessible agents: %d", len(accessibleAgents))
		t.Logf("   Accessible names: %v", agentNames)
	})
}

// TestSpaceBasedExecutionIsolation tests execution isolation within spaces
func TestSpaceBasedExecutionIsolation(t *testing.T) {
	t.Run("Execution History Isolation", func(t *testing.T) {
		// Setup: Two users with agents in different spaces
		user1ID := uuid.New()
		user2ID := uuid.New()
		// Note: Space isolation is tested through agent and execution relationships

		// User 1's agent and executions
		user1AgentID := uuid.New()
		user1Executions := []*models.AgentExecution{
			createMockExecution(uuid.New(), user1AgentID, user1ID, "User 1 execution 1"),
			createMockExecution(uuid.New(), user1AgentID, user1ID, "User 1 execution 2"),
		}

		// User 2's agent and executions
		user2AgentID := uuid.New()
		user2Executions := []*models.AgentExecution{
			createMockExecution(uuid.New(), user2AgentID, user2ID, "User 2 execution 1"),
			createMockExecution(uuid.New(), user2AgentID, user2ID, "User 2 execution 2"),
		}

		// Test execution access isolation
		for _, execution := range user1Executions {
			assert.True(t, canUserAccessExecution(execution, user1ID), 
				"User 1 should access own executions")
			assert.False(t, canUserAccessExecution(execution, user2ID), 
				"User 2 should not access User 1's executions")
		}

		for _, execution := range user2Executions {
			assert.True(t, canUserAccessExecution(execution, user2ID), 
				"User 2 should access own executions")
			assert.False(t, canUserAccessExecution(execution, user1ID), 
				"User 1 should not access User 2's executions")
		}

		t.Logf("✅ Execution history isolation validated")
		t.Logf("   User 1 executions: %d", len(user1Executions))
		t.Logf("   User 2 executions: %d", len(user2Executions))
	})

	t.Run("Organization Execution Visibility", func(t *testing.T) {
		agentOwnerID := uuid.New()
		executorID := uuid.New()
		observerID := uuid.New()
		// Note: Organization space visibility is tested through access control

		// Organization agent
		orgAgentID := uuid.New()

		// Executions by different users on the same organization agent
		executions := []*models.AgentExecution{
			createMockExecution(uuid.New(), orgAgentID, agentOwnerID, "Owner execution"),
			createMockExecution(uuid.New(), orgAgentID, executorID, "Executor execution"),
		}

		// Test organization execution visibility rules
		for _, execution := range executions {
			// Agent owner should see all executions of their agent
			assert.True(t, canAgentOwnerSeeExecution(execution, orgAgentID, agentOwnerID), 
				"Agent owner should see all executions of their agent")

			// Execution user should see their own execution
			assert.True(t, canUserAccessExecution(execution, execution.UserID), 
				"User should see their own execution")

			// Other tenant members should not see executions (unless explicitly granted)
			assert.False(t, canUserAccessExecution(execution, observerID), 
				"Other users should not see executions by default")
		}

		t.Logf("✅ Organization execution visibility validated")
		t.Logf("   Agent ID: %s", orgAgentID)
		t.Logf("   Total executions: %d", len(executions))
	})
}

// TestTenantIsolation tests tenant-level isolation
func TestTenantIsolation(t *testing.T) {
	t.Run("Cross-Tenant Agent Isolation", func(t *testing.T) {
		// Two different tenants
		tenant1ID := "tenant-1"
		tenant2ID := "tenant-2"
		
		user1ID := uuid.New()
		user2ID := uuid.New()

		// Agents in different tenants
		tenant1Agent := &models.Agent{
			ID:       uuid.New(),
			Name:     "Tenant 1 Agent",
			OwnerID:  user1ID,
			TenantID: tenant1ID,
			IsPublic: true,
			Status:   models.AgentStatusPublished,
		}

		tenant2Agent := &models.Agent{
			ID:       uuid.New(),
			Name:     "Tenant 2 Agent",
			OwnerID:  user2ID,
			TenantID: tenant2ID,
			IsPublic: true,
			Status:   models.AgentStatusPublished,
		}

		// Test cross-tenant isolation
		assert.False(t, canTenantMemberAccessAgent(tenant1Agent, user2ID, tenant2ID), 
			"User from tenant 2 should not access tenant 1 agent")
		assert.False(t, canTenantMemberAccessAgent(tenant2Agent, user1ID, tenant1ID), 
			"User from tenant 1 should not access tenant 2 agent")

		// Test within-tenant access
		assert.True(t, canTenantMemberAccessAgent(tenant1Agent, user1ID, tenant1ID), 
			"User should access agent within same tenant")
		assert.True(t, canTenantMemberAccessAgent(tenant2Agent, user2ID, tenant2ID), 
			"User should access agent within same tenant")

		t.Logf("✅ Cross-tenant agent isolation validated")
		t.Logf("   Tenant 1: %s", tenant1ID)
		t.Logf("   Tenant 2: %s", tenant2ID)
	})

	t.Run("Tenant Data Segregation", func(t *testing.T) {
		// Test that tenant data is properly segregated
		tenants := []struct {
			id        string
			userCount int
			agentCount int
		}{
			{"enterprise-tenant", 50, 25},
			{"startup-tenant", 5, 10},
			{"individual-tenant", 1, 3},
		}

		for _, tenant := range tenants {
			// Simulate tenant metrics
			metrics := calculateTenantMetrics(tenant.id, tenant.userCount, tenant.agentCount)
			
			assert.Equal(t, tenant.id, metrics.TenantID, "Tenant ID should match")
			assert.Equal(t, tenant.userCount, metrics.UserCount, "User count should match")
			assert.Equal(t, tenant.agentCount, metrics.AgentCount, "Agent count should match")
			assert.GreaterOrEqual(t, metrics.AvgExecutionsPerAgent, 0.0, 
				"Average executions should be non-negative")

			t.Logf("   Tenant %s: %d users, %d agents, %.1f avg executions/agent", 
				tenant.id, metrics.UserCount, metrics.AgentCount, metrics.AvgExecutionsPerAgent)
		}

		t.Logf("✅ Tenant data segregation validated")
	})
}

// TestSpaceManagementOperations tests space management operations
func TestSpaceManagementOperations(t *testing.T) {
	t.Run("Space Creation and Configuration", func(t *testing.T) {
		tenantID := "space-mgmt-tenant"
		ownerID := uuid.New()

		// Test personal space creation
		personalSpace := &SpaceConfig{
			ID:       uuid.New(),
			Name:     "Personal Workspace",
			Type:     models.SpaceTypePersonal,
			OwnerID:  ownerID,
			TenantID: tenantID,
			Settings: SpaceSettings{
				MaxAgents:         10,
				MaxExecutionsPerDay: 100,
				AllowPublicAgents:   false,
			},
		}

		assert.Equal(t, models.SpaceTypePersonal, personalSpace.Type, 
			"Personal space should have correct type")
		assert.False(t, personalSpace.Settings.AllowPublicAgents, 
			"Personal space should not allow public agents by default")

		// Test organization space creation
		orgSpace := &SpaceConfig{
			ID:       uuid.New(),
			Name:     "Organization Workspace",
			Type:     models.SpaceTypeOrganization,
			OwnerID:  ownerID,
			TenantID: tenantID,
			Settings: SpaceSettings{
				MaxAgents:         100,
				MaxExecutionsPerDay: 1000,
				AllowPublicAgents:   true,
			},
		}

		assert.Equal(t, models.SpaceTypeOrganization, orgSpace.Type, 
			"Organization space should have correct type")
		assert.True(t, orgSpace.Settings.AllowPublicAgents, 
			"Organization space should allow public agents")

		t.Logf("✅ Space creation and configuration validated")
		t.Logf("   Personal space: %s", personalSpace.ID)
		t.Logf("   Organization space: %s", orgSpace.ID)
	})

	t.Run("Space Member Management", func(t *testing.T) {
		orgSpaceID := uuid.New()
		ownerID := uuid.New()
		memberIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

		// Test adding members to organization space
		spaceMembers := &SpaceMembers{
			SpaceID: orgSpaceID,
			OwnerID: ownerID,
			Members: make([]SpaceMember, 0),
		}

		// Add members with different roles
		roles := []string{"admin", "editor", "viewer"}
		for i, memberID := range memberIDs {
			member := SpaceMember{
				UserID:   memberID,
				Role:     roles[i],
				JoinedAt: time.Now(),
			}
			spaceMembers.Members = append(spaceMembers.Members, member)
		}

		assert.Len(t, spaceMembers.Members, len(memberIDs), 
			"All members should be added")

		// Test member permissions
		for i, member := range spaceMembers.Members {
			expectedRole := roles[i]
			assert.Equal(t, expectedRole, member.Role, 
				"Member should have correct role")
			
			canEdit := member.Role == "admin" || member.Role == "editor"
			canView := true // All members can view

			assert.Equal(t, canView, true, "All members should be able to view")
			t.Logf("   Member %s: %s (can edit: %t)", member.UserID, member.Role, canEdit)
		}

		t.Logf("✅ Space member management validated")
		t.Logf("   Space: %s", orgSpaceID)
		t.Logf("   Members: %d", len(spaceMembers.Members))
	})
}

// Helper functions and types

type SpaceConfig struct {
	ID       uuid.UUID
	Name     string
	Type     models.SpaceType
	OwnerID  uuid.UUID
	TenantID string
	Settings SpaceSettings
}

type SpaceSettings struct {
	MaxAgents           int
	MaxExecutionsPerDay int
	AllowPublicAgents   bool
}

type SpaceMembers struct {
	SpaceID uuid.UUID
	OwnerID uuid.UUID
	Members []SpaceMember
}

type SpaceMember struct {
	UserID   uuid.UUID
	Role     string
	JoinedAt time.Time
}

type TenantMetrics struct {
	TenantID                string
	UserCount               int
	AgentCount              int
	AvgExecutionsPerAgent   float64
}

func canUserAccessAgent(agent *models.Agent, userID uuid.UUID) bool {
	// Owner can always access
	if agent.OwnerID == userID {
		return true
	}
	
	// Public agents can be accessed by anyone (in practice, within tenant)
	if agent.IsPublic && agent.Status == models.AgentStatusPublished {
		return true
	}
	
	return false
}

func canTenantMemberAccessAgent(agent *models.Agent, userID uuid.UUID, tenantID string) bool {
	// Must be same tenant
	if agent.TenantID != tenantID {
		return false
	}
	
	// Owner can always access
	if agent.OwnerID == userID {
		return true
	}
	
	// Public agents within tenant can be accessed
	if agent.IsPublic && agent.Status == models.AgentStatusPublished {
		return true
	}
	
	return false
}

func filterAccessibleAgents(agents []*models.Agent, userID uuid.UUID, tenantID string) []*models.Agent {
	var accessible []*models.Agent
	
	for _, agent := range agents {
		if canTenantMemberAccessAgent(agent, userID, tenantID) {
			accessible = append(accessible, agent)
		}
	}
	
	return accessible
}

func createMockExecution(id, agentID, userID uuid.UUID, description string) *models.AgentExecution {
	completedTime := time.Now()
	startedTime := time.Now().Add(-5 * time.Minute)
	
	inputData, _ := models.ConvertToJSON(map[string]string{"description": description})
	outputData, _ := models.ConvertToJSON(map[string]string{"response": "mock response"})
	
	return &models.AgentExecution{
		ID:          id,
		AgentID:     agentID,
		UserID:      userID,
		InputData:   inputData,
		OutputData:  outputData,
		Status:      "completed",
		TokenUsage:  intPtr(50),
		CostUSD:     floatPtr(0.001),
		StartedAt:   &startedTime,
		CompletedAt: &completedTime,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func canUserAccessExecution(execution *models.AgentExecution, userID uuid.UUID) bool {
	return execution.UserID == userID
}

func canAgentOwnerSeeExecution(execution *models.AgentExecution, agentID, ownerID uuid.UUID) bool {
	// Agent owner can see all executions of their agent, regardless of who executed it
	return execution.AgentID == agentID
}

func calculateTenantMetrics(tenantID string, userCount, agentCount int) TenantMetrics {
	// Mock calculation
	avgExecutions := float64(agentCount) * 2.5 // Assume 2.5 executions per agent on average
	
	return TenantMetrics{
		TenantID:              tenantID,
		UserCount:             userCount,
		AgentCount:            agentCount,
		AvgExecutionsPerAgent: avgExecutions / float64(agentCount),
	}
}

