package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestKeyAndJWKSServer(t *testing.T) (priv *ecdsa.PrivateKey, kid, jwksURL string) {
	t.Helper()
	p, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	xBytes := make([]byte, 32)
	yBytes := make([]byte, 32)
	p.X.FillBytes(xBytes)
	p.Y.FillBytes(yBytes)

	kid = "test-key-" + uuid.NewString()
	jwks := map[string]any{
		"keys": []map[string]any{{
			"alg":     "ES256",
			"crv":     "P-256",
			"kty":     "EC",
			"use":     "sig",
			"kid":     kid,
			"x":       base64.RawURLEncoding.EncodeToString(xBytes),
			"y":       base64.RawURLEncoding.EncodeToString(yBytes),
			"key_ops": []string{"verify"},
		}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return p, kid, srv.URL
}

func signES256(t *testing.T, priv *ecdsa.PrivateKey, kid string, sub uuid.UUID, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": sub.String(),
		"aud": "authenticated",
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	})
	tok.Header["kid"] = kid
	s, err := tok.SignedString(priv)
	require.NoError(t, err)
	return s
}

func TestVerify_Valid(t *testing.T) {
	priv, kid, jwksURL := newTestKeyAndJWKSServer(t)
	v, err := NewJWKSVerifier(context.Background(), jwksURL)
	require.NoError(t, err)

	uid := uuid.New()
	tok := signES256(t, priv, kid, uid, time.Now().Add(time.Hour))

	claims, err := v.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, uid, claims.UserID)
}

func TestVerify_Expired(t *testing.T) {
	priv, kid, jwksURL := newTestKeyAndJWKSServer(t)
	v, err := NewJWKSVerifier(context.Background(), jwksURL)
	require.NoError(t, err)

	tok := signES256(t, priv, kid, uuid.New(), time.Now().Add(-time.Hour))
	_, err = v.Verify(tok)
	require.Error(t, err)
}

func TestVerify_UnknownKID(t *testing.T) {
	priv1, _, _ := newTestKeyAndJWKSServer(t)
	_, _, jwksURL2 := newTestKeyAndJWKSServer(t)
	v, err := NewJWKSVerifier(context.Background(), jwksURL2)
	require.NoError(t, err)

	tok := signES256(t, priv1, "unknown-kid", uuid.New(), time.Now().Add(time.Hour))
	_, err = v.Verify(tok)
	require.Error(t, err)
}

func TestVerify_Malformed(t *testing.T) {
	_, _, jwksURL := newTestKeyAndJWKSServer(t)
	v, err := NewJWKSVerifier(context.Background(), jwksURL)
	require.NoError(t, err)
	_, err = v.Verify("not-a-jwt")
	require.Error(t, err)
}
