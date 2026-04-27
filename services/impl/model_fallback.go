package impl

import (
	"context"
	"strings"
)

// detectTier returns a canonical family tier for a model ID so deprecated
// snapshots can be mapped forward to current ones. Matching is substring-based
// because Anthropic and OpenAI both embed the family in the model ID
// (e.g. claude-3-haiku-20240307 → haiku, claude-haiku-4-5-20251001 → haiku).
func detectTier(modelID string) string {
	m := strings.ToLower(modelID)
	switch {
	case strings.Contains(m, "haiku"):
		return "haiku"
	case strings.Contains(m, "sonnet"):
		return "sonnet"
	case strings.Contains(m, "opus"):
		return "opus"
	case strings.Contains(m, "gpt-4o-mini"):
		return "gpt-4o-mini"
	case strings.Contains(m, "gpt-4o"):
		return "gpt-4o"
	case strings.Contains(m, "gpt-4"):
		return "gpt-4"
	case strings.Contains(m, "gpt-3.5"):
		return "gpt-3.5"
	}
	return ""
}

// isModelGoneError reports whether an error response from the router looks
// like the configured model is no longer accepted — either the provider's
// own 404 (deprecated snapshot), the router's own "model not supported" if
// the model was dropped from config, or "provider not healthy" stemming from
// a failing provider-side health probe against a deprecated model.
func isModelGoneError(statusCode int, body []byte) bool {
	if statusCode != 400 && statusCode != 404 && statusCode != 503 {
		return false
	}
	b := strings.ToLower(string(body))
	return strings.Contains(b, "not_found") ||
		strings.Contains(b, "model not") ||
		strings.Contains(b, "not healthy") ||
		strings.Contains(b, "not supported")
}

// resolveFallbackModel asks the router which models the provider currently
// exposes, then picks one of the same tier as originalModel. Returns "" when
// no same-tier model is available — the caller must then surface the original
// error rather than silently routing to an unrelated tier.
func (s *routerServiceImpl) resolveFallbackModel(ctx context.Context, provider, originalModel string) string {
	if provider == "" || originalModel == "" {
		return ""
	}
	tier := detectTier(originalModel)
	if tier == "" {
		return ""
	}
	models, err := s.GetProviderModels(ctx, provider)
	if err != nil || len(models) == 0 {
		return ""
	}
	for _, m := range models {
		if m.Name != originalModel && detectTier(m.Name) == tier {
			return m.Name
		}
	}
	return ""
}
