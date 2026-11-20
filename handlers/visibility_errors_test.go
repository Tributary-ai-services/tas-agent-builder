package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/tas-agent-builder/services"
)

// The codes are a contract with aether-be and the aether UI (AB-13), which
// read a top-level "code" field. Pin both the status and the body shape.
func TestVisibilityErrorsKeepTheirContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{services.ErrPublicRequiresAdmin, http.StatusForbidden, "visibility_public_requires_admin"},
		{services.ErrOrganizationRequiresOrgSpace, http.StatusBadRequest, "visibility_organization_requires_org_space"},
		{services.ErrInvalidVisibility, http.StatusBadRequest, "visibility_invalid"},
		// Wrapped errors still map: the service may add context.
		{fmt.Errorf("update: %w", services.ErrPublicRequiresAdmin), http.StatusForbidden, "visibility_public_requires_admin"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		respondAgentError(c, tc.err, "x")
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != tc.status || body["code"] != tc.code {
			t.Errorf("%v: got %d %v, want %d code=%s", tc.err, w.Code, body, tc.status, tc.code)
		}
	}
}

func TestRealmRoleGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if hasRealmRole(c, "admin") {
		t.Fatal("no roles in context must mean no role")
	}
	c.Set("realm_roles", []string{"user", "admin"})
	if !hasRealmRole(c, "admin") || hasRealmRole(c, "aiqg-admin") {
		t.Fatal("role match is exact")
	}
}
