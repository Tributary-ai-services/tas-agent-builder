package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/services"
)

// Cache lifetimes for membership answers.
//
// Positive answers are cached for a minute: space membership changes rarely,
// and a stale "yes" only survives until the entry expires. Negative answers
// are cached far more briefly — a user who has just been added to a space
// should not be locked out for a minute, and a short negative TTL is still
// enough to absorb a retry storm from a client looping on 403s.
const (
	membershipTTL         = 60 * time.Second
	membershipNegativeTTL = 10 * time.Second

	// maxMembershipEntries bounds the cache. The key includes the user id, so
	// an unbounded map is a memory leak proportional to the number of users
	// who have ever called. At the cap the cache is cleared rather than
	// evicted one by one: this is a latency optimisation, not a store, and a
	// full rebuild costs one round trip per active caller.
	maxMembershipEntries = 10000
)

type membershipCacheEntry struct {
	membership *services.SpaceMembership
	denied     bool
	expiresAt  time.Time
}

type spaceVerifierImpl struct {
	baseURL string
	client  *http.Client

	mu    sync.RWMutex
	cache map[string]membershipCacheEntry
}

// NewSpaceVerifier builds a verifier that consults aether-be.
//
// baseURL is aether-be's origin (scheme://host:port). The /api/v1 prefix is
// appended here, and a baseURL that already carries it is accepted too, since
// the two URL environment variables in this repo disagree on that point.
func NewSpaceVerifier(cfg *config.AetherConfig) services.SpaceVerifier {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &spaceVerifierImpl{
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
		client:  &http.Client{Timeout: timeout},
		cache:   make(map[string]membershipCacheEntry),
	}
}

func membershipCacheKey(userID, spaceID, spaceType string) string {
	return userID + "\x00" + spaceID + "\x00" + spaceType
}

func (s *spaceVerifierImpl) lookup(key string) (membershipCacheEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return membershipCacheEntry{}, false
	}
	return entry, true
}

func (s *spaceVerifierImpl) store(key string, entry membershipCacheEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cache) >= maxMembershipEntries {
		s.cache = make(map[string]membershipCacheEntry, maxMembershipEntries)
	}
	s.cache[key] = entry
}

// membershipResponse mirrors aether-be's SpaceMembershipResponse.
type membershipResponse struct {
	Member      bool     `json:"member"`
	SpaceID     string   `json:"space_id"`
	SpaceType   string   `json:"space_type"`
	TenantID    string   `json:"tenant_id"`
	UserRole    string   `json:"user_role"`
	Permissions []string `json:"permissions"`
}

func (s *spaceVerifierImpl) VerifyMembership(ctx context.Context, userID, bearerToken, spaceID, spaceType string) (*services.SpaceMembership, error) {
	if userID == "" || spaceID == "" {
		return nil, services.ErrSpaceAccessDenied
	}

	key := membershipCacheKey(userID, spaceID, spaceType)
	if entry, ok := s.lookup(key); ok {
		if entry.denied {
			return nil, services.ErrSpaceAccessDenied
		}
		return entry.membership, nil
	}

	endpoint := s.baseURL
	if !strings.HasSuffix(endpoint, "/api/v1") {
		endpoint += "/api/v1"
	}
	endpoint += "/spaces/" + url.PathEscape(spaceID) + "/membership"
	if spaceType != "" {
		endpoint += "?space_type=" + url.QueryEscape(spaceType)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", services.ErrSpaceCheckUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("[SPACE] membership check failed for space=%s: %v", spaceID, err)
		return nil, fmt.Errorf("%w: %v", services.ErrSpaceCheckUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var body membershipResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return nil, fmt.Errorf("%w: decoding response: %v", services.ErrSpaceCheckUnavailable, err)
		}
		if !body.Member {
			s.store(key, membershipCacheEntry{denied: true, expiresAt: time.Now().Add(membershipNegativeTTL)})
			return nil, services.ErrSpaceAccessDenied
		}
		membership := &services.SpaceMembership{
			SpaceID:     body.SpaceID,
			SpaceType:   body.SpaceType,
			TenantID:    body.TenantID,
			UserRole:    body.UserRole,
			Permissions: body.Permissions,
		}
		s.store(key, membershipCacheEntry{membership: membership, expiresAt: time.Now().Add(membershipTTL)})
		return membership, nil

	case http.StatusForbidden, http.StatusNotFound:
		s.store(key, membershipCacheEntry{denied: true, expiresAt: time.Now().Add(membershipNegativeTTL)})
		return nil, services.ErrSpaceAccessDenied

	case http.StatusUnauthorized:
		// The token this service was handed was rejected upstream. That is a
		// denial of this caller, not an outage, but it is not cached: the
		// caller may simply need to refresh a token.
		return nil, services.ErrSpaceAccessDenied

	default:
		log.Printf("[SPACE] membership check returned %d for space=%s", resp.StatusCode, spaceID)
		return nil, fmt.Errorf("%w: status %d", services.ErrSpaceCheckUnavailable, resp.StatusCode)
	}
}
