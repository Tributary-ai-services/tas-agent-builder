package auth

import (
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents JWT token claims
type Claims struct {
	Sub               string                 `json:"sub"`
	Iss               string                 `json:"iss"`
	Aud               interface{}            `json:"aud"`
	Exp               int64                  `json:"exp"`
	Iat               int64                  `json:"iat"`
	Email             string                 `json:"email"`
	EmailVerified     bool                   `json:"email_verified"`
	PreferredUsername string                 `json:"preferred_username"`
	Name              string                 `json:"name"`
	GivenName         string                 `json:"given_name"`
	FamilyName        string                 `json:"family_name"`
	RealmAccess       RealmAccess            `json:"realm_access"`
	ResourceAccess    map[string]interface{} `json:"resource_access"`
	Groups            []string               `json:"groups"`
	jwt.RegisteredClaims
}

// RealmAccess represents realm access information
type RealmAccess struct {
	Roles []string `json:"roles"`
}

// JWKS represents the JSON Web Key Set response from Keycloak
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
}

// JWTValidator handles JWT token validation
type JWTValidator struct {
	secret         []byte
	allowedIssuers []string
	jwksURL        string
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(secret string, allowedIssuers []string) *JWTValidator {
	return &JWTValidator{
		secret:         []byte(secret),
		allowedIssuers: allowedIssuers,
		// jwksURL will be derived from the token's issuer claim dynamically
		jwksURL:        "",
	}
}

// ValidateToken validates a JWT token string and returns claims
func (v *JWTValidator) ValidateToken(tokenString string) (*Claims, error) {
	// Remove Bearer prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// First, parse without validation to get the issuer for JWKS URL
	parser := jwt.NewParser()
	unverifiedToken, _, err := parser.ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract issuer from unverified claims to build JWKS URL
	unverifiedClaims, ok := unverifiedToken.Claims.(*Claims)
	if !ok {
		return nil, errors.New("failed to extract claims from token")
	}

	// Build JWKS URL from issuer (e.g., https://keycloak.example.com/realms/aether -> .../protocol/openid-connect/certs)
	jwksURL := unverifiedClaims.Iss + "/protocol/openid-connect/certs"

	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Get the key ID from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("token missing kid header")
		}

		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); ok {
			// For RSA tokens (Keycloak), fetch the public key using dynamic JWKS URL
			return v.getRSAPublicKeyFromURL(kid, jwksURL)
		} else if _, ok := token.Method.(*jwt.SigningMethodHMAC); ok {
			// For HMAC tokens, use the secret
			return v.secret, nil
		}

		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Validate expiration
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, errors.New("token has expired")
	}

	// Validate issuer if specified
	if len(v.allowedIssuers) > 0 {
		validIssuer := false
		for _, allowedIss := range v.allowedIssuers {
			if claims.Iss == allowedIss {
				validIssuer = true
				break
			}
		}
		if !validIssuer {
			return nil, fmt.Errorf("invalid issuer: %s", claims.Iss)
		}
	}

	return claims, nil
}

// ExtractUserContext extracts user context from JWT claims
func (v *JWTValidator) ExtractUserContext(claims *Claims) (userID, tenantID string) {
	// Use the subject as user ID
	userID = claims.Sub
	
	// For tenant ID, we can use a combination of user ID and a default tenant
	// In production, this might come from custom claims or be mapped differently
	if userID != "" {
		// Create a deterministic tenant ID based on user ID
		// This ensures the same user always gets the same tenant
		tenantID = fmt.Sprintf("tenant_%s", userID[:min(len(userID), 10)])
	} else {
		// Fallback to default tenant
		tenantID = "default-tenant"
	}
	
	return userID, tenantID
}

// getRSAPublicKeyFromURL fetches the RSA public key from a dynamic JWKS endpoint
func (v *JWTValidator) getRSAPublicKeyFromURL(kid string, jwksURL string) (*rsa.PublicKey, error) {
	// Create HTTP client that skips TLS verification for internal/self-signed certs
	// This is needed when Keycloak uses Let's Encrypt or self-signed certificates
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	// Fetch JWKS from the dynamically determined URL
	resp, err := client.Get(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", jwksURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Find the key with matching kid
	for _, key := range jwks.Keys {
		if key.Kid == kid && key.Kty == "RSA" {
			return v.parseRSAPublicKey(key)
		}
	}

	return nil, fmt.Errorf("no RSA key found with kid: %s in JWKS from %s", kid, jwksURL)
}

// getRSAPublicKey fetches the RSA public key from the configured JWKS endpoint (deprecated, use getRSAPublicKeyFromURL)
func (v *JWTValidator) getRSAPublicKey(kid string) (*rsa.PublicKey, error) {
	if v.jwksURL == "" {
		return nil, errors.New("no JWKS URL configured")
	}
	return v.getRSAPublicKeyFromURL(kid, v.jwksURL)
}

// parseRSAPublicKey converts JWK to RSA public key
func (v *JWTValidator) parseRSAPublicKey(jwk JWK) (*rsa.PublicKey, error) {
	// Decode modulus (n)
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode exponent (e)
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert to big integers
	n := big.NewInt(0).SetBytes(nBytes)
	e := big.NewInt(0).SetBytes(eBytes)

	// Create RSA public key
	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}

// min helper function for Go versions without built-in min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}