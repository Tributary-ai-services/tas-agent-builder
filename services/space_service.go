package services

import (
	"context"
	"errors"
)

// ErrSpaceAccessDenied is returned when the caller is not a member of the
// space they asked to act in. It is a definite "no" from the authority, as
// opposed to a failure to reach it.
var ErrSpaceAccessDenied = errors.New("space access denied")

// ErrSpaceCheckUnavailable is returned when membership could not be
// determined — aether-be unreachable, timed out, or answering unexpectedly.
// Callers must fail closed on it: an isolation check that cannot run is not
// an isolation check that passed.
var ErrSpaceCheckUnavailable = errors.New("space membership check unavailable")

// SpaceMembership is what this service is allowed to know about a caller's
// standing in a space. It deliberately omits the space's AudiModal API key,
// which aether-be holds and never returns.
type SpaceMembership struct {
	SpaceID     string   `json:"space_id"`
	SpaceType   string   `json:"space_type"`
	TenantID    string   `json:"tenant_id"`
	UserRole    string   `json:"user_role"`
	Permissions []string `json:"permissions"`
}

// SpaceVerifier answers "is this caller a member of this space?".
//
// Agents are stored here keyed by space_id, but membership lives in aether-be
// (Neo4j). Rather than copy that data — a second definition of membership is a
// second thing to drift — this service asks aether-be, passing through the
// caller's own bearer token so the answer is about the caller and not about a
// privileged service account.
type SpaceVerifier interface {
	// VerifyMembership returns the caller's membership, ErrSpaceAccessDenied
	// if they are not a member, or ErrSpaceCheckUnavailable if the authority
	// could not be consulted.
	//
	// spaceType may be empty, in which case aether-be tries each type.
	VerifyMembership(ctx context.Context, userID, bearerToken, spaceID, spaceType string) (*SpaceMembership, error)
}
