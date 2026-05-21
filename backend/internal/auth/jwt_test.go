package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-must-be-long-enough"

func signTestToken(t *testing.T, sub uuid.UUID, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub.String(),
		"aud": "authenticated",
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	})
	s, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return s
}

func TestVerify_Valid(t *testing.T) {
	v := NewVerifier(testSecret)
	uid := uuid.New()
	tok := signTestToken(t, uid, time.Now().Add(time.Hour))

	claims, err := v.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, uid, claims.UserID)
}

func TestVerify_Expired(t *testing.T) {
	v := NewVerifier(testSecret)
	tok := signTestToken(t, uuid.New(), time.Now().Add(-time.Hour))

	_, err := v.Verify(tok)
	require.Error(t, err)
}

func TestVerify_BadSignature(t *testing.T) {
	v := NewVerifier("different-secret")
	tok := signTestToken(t, uuid.New(), time.Now().Add(time.Hour))

	_, err := v.Verify(tok)
	require.Error(t, err)
}

func TestVerify_Malformed(t *testing.T) {
	v := NewVerifier(testSecret)
	_, err := v.Verify("not-a-jwt")
	require.Error(t, err)
}
