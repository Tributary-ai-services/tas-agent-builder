package impl

import (
	"context"
	"errors"
	"testing"

	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

// AB-13: "organization" shares an agent with people working in spaces of the
// same Aether Organization, and nobody else. "public" is everyone, and only
// an admin may set it.

const (
	orgA = "4c4d7143-e345-44f5-8fee-77902278e646" // shape of the live TAS org id
	orgB = "99999999-0000-0000-0000-00000000000b"

	spaceA2 = "space_1772236887" // a second space of org A
	userC   = "cccccccc-0000-0000-0000-000000000003"
)

func inOrg(user, space, org string) services.AgentScope {
	return services.AgentScope{UserID: user, SpaceID: space, OrganizationID: org}
}

func vis(v models.AgentVisibility) *models.AgentVisibility { return &v }

func TestOrganizationAgentReadableInSameOrgOnly(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	shared := seedAgent(t, db, agentSeed{name: "org A agent", owner: userA, space: spaceA,
		visibility: models.VisibilityOrganization, org: orgA})

	// A colleague working in another space of org A can read it.
	if _, err := svc.GetAgent(ctx, shared.ID, inOrg(userC, spaceA2, orgA)); err != nil {
		t.Fatalf("org A member could not read an org A agent: %v", err)
	}
	// A member of org B cannot.
	if _, err := svc.GetAgent(ctx, shared.ID, inOrg(userB, spaceB, orgB)); !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("org B read an org A agent: err=%v", err)
	}
	// Nor can anyone in a personal space, which has no organization. An empty
	// id must never match an empty id.
	if _, err := svc.GetAgent(ctx, shared.ID, inOrg(userB, spaceB, "")); !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("personal-space caller read an org agent: err=%v", err)
	}

	list, err := svc.ListAgents(ctx, models.AgentListFilter{Page: 1, Size: 50}, inOrg(userB, spaceB, orgB))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, a := range list.Agents {
		if a.ID == shared.ID {
			t.Fatal("org A agent listed for org B")
		}
	}
	list, err = svc.ListAgents(ctx, models.AgentListFilter{Page: 1, Size: 50}, inOrg(userC, spaceA2, orgA))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, a := range list.Agents {
		found = found || a.ID == shared.ID
	}
	if !found {
		t.Fatal("org A agent missing from an org A member's list")
	}
}

// An org agent with no organization id (created in a space with no
// HAS_SPACE edge, before AB-13) must not become readable by every caller
// whose organization is also unknown.
func TestOrganizationAgentWithoutOrgIsNotShared(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	orphan := seedAgent(t, db, agentSeed{name: "orphan", owner: userA, space: spaceA,
		visibility: models.VisibilityOrganization, org: ""})
	if _, err := svc.GetAgent(context.Background(), orphan.ID, inOrg(userB, spaceB, "")); !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("org agent with empty org id was readable: err=%v", err)
	}
}

func TestPublicRequiresAdmin(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()
	req := models.CreateAgentRequest{Name: "x", SystemPrompt: "hi", SpaceID: spaceA, Visibility: vis(models.VisibilityPublic)}

	if _, err := svc.CreateAgent(ctx, req, inOrg(userA, spaceA, orgA), "tenant_test"); !errors.Is(err, services.ErrPublicRequiresAdmin) {
		t.Fatalf("non-admin created a public agent: err=%v", err)
	}
	// The legacy flag is held to the same rule; it was the way in before.
	legacy := models.CreateAgentRequest{Name: "x", SystemPrompt: "hi", SpaceID: spaceA, IsPublic: true}
	if _, err := svc.CreateAgent(ctx, legacy, inOrg(userA, spaceA, orgA), "tenant_test"); !errors.Is(err, services.ErrPublicRequiresAdmin) {
		t.Fatalf("non-admin created a public agent via is_public: err=%v", err)
	}

	admin := inOrg(userA, spaceA, orgA)
	admin.CanPublishPublic = true
	a, err := svc.CreateAgent(ctx, req, admin, "tenant_test")
	if err != nil {
		t.Fatalf("admin could not create a public agent: %v", err)
	}
	if a.Visibility != models.VisibilityPublic || !a.IsPublic {
		t.Fatalf("visibility=%q is_public=%v, want public/true", a.Visibility, a.IsPublic)
	}

	// Update: a non-admin owner cannot promote their own agent to public.
	own := seedAgent(t, db, agentSeed{name: "mine", owner: userA, space: spaceA, org: orgA})
	if _, err := svc.UpdateAgent(ctx, own.ID, models.UpdateAgentRequest{Visibility: vis(models.VisibilityPublic)}, inOrg(userA, spaceA, orgA)); !errors.Is(err, services.ErrPublicRequiresAdmin) {
		t.Fatalf("non-admin promoted an agent to public: err=%v", err)
	}
	var after models.Agent
	db.First(&after, "id = ?", own.ID)
	if after.Visibility != models.VisibilityPrivate || after.IsPublic {
		t.Fatalf("agent changed despite refusal: %q %v", after.Visibility, after.IsPublic)
	}
}

func TestOrganizationNeedsAnOrgSpace(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	req := models.CreateAgentRequest{Name: "x", SystemPrompt: "hi", SpaceID: spaceA, Visibility: vis(models.VisibilityOrganization)}
	if _, err := svc.CreateAgent(context.Background(), req, inOrg(userA, spaceA, ""), "tenant_test"); !errors.Is(err, services.ErrOrganizationRequiresOrgSpace) {
		t.Fatalf("organization visibility accepted in a personal space: err=%v", err)
	}
	a, err := svc.CreateAgent(context.Background(), req, inOrg(userA, spaceA, orgA), "tenant_test")
	if err != nil {
		t.Fatalf("create org agent: %v", err)
	}
	if a.OrganizationID != orgA || a.Visibility != models.VisibilityOrganization || a.IsPublic {
		t.Fatalf("got org=%q vis=%q public=%v", a.OrganizationID, a.Visibility, a.IsPublic)
	}
}

func TestInvalidVisibilityRejected(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	req := models.CreateAgentRequest{Name: "x", SystemPrompt: "hi", SpaceID: spaceA, Visibility: vis("everyone")}
	if _, err := svc.CreateAgent(context.Background(), req, inOrg(userA, spaceA, orgA), "tenant_test"); !errors.Is(err, services.ErrInvalidVisibility) {
		t.Fatalf("invalid visibility accepted: err=%v", err)
	}
}

// Sharing an agent with my org fills in the org for agents created before
// AB-13, which have organization_id "".
func TestUpdateToOrganizationRecordsOrg(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()
	old := seedAgent(t, db, agentSeed{name: "pre-AB-13", owner: userA, space: spaceA})
	a, err := svc.UpdateAgent(ctx, old.ID, models.UpdateAgentRequest{Visibility: vis(models.VisibilityOrganization)}, inOrg(userA, spaceA, orgA))
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if a.OrganizationID != orgA || a.Visibility != models.VisibilityOrganization {
		t.Fatalf("got org=%q vis=%q", a.OrganizationID, a.Visibility)
	}
	if _, err := svc.GetAgent(ctx, old.ID, inOrg(userC, spaceA2, orgA)); err != nil {
		t.Fatalf("colleague cannot read the newly shared agent: %v", err)
	}
}

// Duplicating follows the read rule: an org B member cannot copy an org A
// agent, and neither can anyone copy a template that is not shared with them.
func TestDuplicateFollowsReadRule(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	ctx := context.Background()

	orgAgent := seedAgent(t, db, agentSeed{name: "org A", owner: userA, space: spaceA,
		visibility: models.VisibilityOrganization, org: orgA})
	if _, err := svc.DuplicateAgent(ctx, orgAgent.ID, "stolen", inOrg(userB, spaceB, orgB), "tenant_test"); !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("org B copied an org A agent: err=%v", err)
	}
	c, err := svc.DuplicateAgent(ctx, orgAgent.ID, "mine", inOrg(userC, spaceA2, orgA), "tenant_test")
	if err != nil {
		t.Fatalf("org A colleague could not copy: %v", err)
	}
	if c.Visibility != models.VisibilityPrivate || c.OrganizationID != orgA || c.OwnerID != userC {
		t.Fatalf("copy: vis=%q org=%q owner=%q", c.Visibility, c.OrganizationID, c.OwnerID)
	}

	privTemplate := seedAgent(t, db, agentSeed{name: "private template", owner: userA, space: spaceA, isTemplate: true, org: orgA})
	if _, err := svc.DuplicateAgent(ctx, privTemplate.ID, "x", inOrg(userB, spaceB, orgB), "tenant_test"); !errors.Is(err, services.ErrAgentNotFound) {
		t.Fatalf("a private template was copyable from another org: err=%v", err)
	}
}

func TestGetAgentsBySpaceIncludesOrgAgents(t *testing.T) {
	db := newTestDB(t)
	svc := NewAgentService(db)
	shared := seedAgent(t, db, agentSeed{name: "team tool", owner: userA, space: spaceA,
		visibility: models.VisibilityOrganization, org: orgA})
	private := seedAgent(t, db, agentSeed{name: "A only", owner: userA, space: spaceA, org: orgA})

	got, err := svc.GetAgentsBySpace(context.Background(), inOrg(userC, spaceA, orgA))
	if err != nil {
		t.Fatalf("by space: %v", err)
	}
	ids := map[string]bool{}
	for _, a := range got {
		ids[a.ID.String()] = true
	}
	if !ids[shared.ID.String()] || ids[private.ID.String()] {
		t.Fatalf("by-space list wrong: shared=%v private=%v", ids[shared.ID.String()], ids[private.ID.String()])
	}
}
