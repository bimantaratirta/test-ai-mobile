package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID uuid.UUID
}

type Verifier struct {
	keyfunc keyfunc.Keyfunc
}

// NewJWKSVerifier creates a Verifier that fetches signing keys from a JWKS
// endpoint (e.g. https://<ref>.supabase.co/auth/v1/.well-known/jwks.json).
// Keys are cached in-memory and refreshed periodically by the keyfunc lib.
func NewJWKSVerifier(ctx context.Context, jwksURL string) (*Verifier, error) {
	if jwksURL == "" {
		return nil, errors.New("jwks url is empty")
	}
	k, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("init jwks: %w", err)
	}
	return &Verifier{keyfunc: k}, nil
}

func (v *Verifier) Verify(tokenStr string) (Claims, error) {
	parsed, err := jwt.Parse(tokenStr, v.keyfunc.Keyfunc)
	if err != nil {
		return Claims{}, err
	}
	if !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}
	mapClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, errors.New("unexpected claims type")
	}
	sub, ok := mapClaims["sub"].(string)
	if !ok || sub == "" {
		return Claims{}, errors.New("missing sub claim")
	}
	uid, err := uuid.Parse(sub)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid sub uuid: %w", err)
	}
	return Claims{UserID: uid}, nil
}
