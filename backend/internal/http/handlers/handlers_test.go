//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bimantara/ai-image/backend/internal/auth"
	"github.com/bimantara/ai-image/backend/internal/db"
	"github.com/bimantara/ai-image/backend/internal/generation"
	apphttp "github.com/bimantara/ai-image/backend/internal/http"
	"github.com/bimantara/ai-image/backend/internal/providers"
	"github.com/bimantara/ai-image/backend/internal/providers/mock"
	"github.com/bimantara/ai-image/backend/internal/ratelimit"
	"github.com/bimantara/ai-image/backend/internal/storage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "test-secret-must-be-long-enough"

func mintJWT(t *testing.T, uid uuid.UUID) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": uid.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func newTestServer(t *testing.T) (*httptest.Server, *db.DB, uuid.UUID) {
	d := db.NewTestDB(t)
	uid := uuid.New()
	_, err := d.Pool.Exec(context.Background(),
		`insert into auth.users (id) values ($1)`, uid)
	require.NoError(t, err)

	reg := providers.NewRegistry()
	reg.Register(db.ModeRealistic, mock.New("mock"))
	reg.Register(db.ModeInspirational, mock.New("mock"))
	gate := ratelimit.New(ratelimit.Config{DB: d, PerDay: 10, PerWeek: 30, GlobalCostCap: 25, MaxConcurrent: 3})
	svc := generation.NewService(generation.Deps{DB: d, Registry: reg, Gate: gate})
	s3, _ := storage.NewS3("https://cdn.test", "auto", "ak", "sk", "bkt", "https://cdn.test")

	router := apphttp.NewRouter(apphttp.Deps{
		JWTVerifier: auth.NewVerifier(secret),
		DB:          d,
		S3:          s3,
		Generation:  svc,
	})
	return httptest.NewServer(router), d, uid
}

func TestHealth(t *testing.T) {
	srv, _, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

func TestGenerate_RequiresAuth(t *testing.T) {
	srv, _, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/generate", "application/json",
		bytes.NewBufferString(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 401, resp.StatusCode)
}

func TestRedeemInvite_HappyPath(t *testing.T) {
	srv, d, uid := newTestServer(t)
	defer srv.Close()

	invites := db.Invites{DB: d}
	require.NoError(t, invites.Create(context.Background(), "OK1"))

	req, _ := http.NewRequest("POST", srv.URL+"/redeem-invite",
		bytes.NewBufferString(`{"code":"OK1"}`))
	req.Header.Set("Authorization", "Bearer "+mintJWT(t, uid))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

func TestGenerate_HappyPath(t *testing.T) {
	srv, d, uid := newTestServer(t)
	defer srv.Close()
	profiles := db.Profiles{DB: d}
	require.NoError(t, profiles.Upsert(context.Background(), uid, "x", ""))

	body, _ := json.Marshal(map[string]string{
		"input_image_url": "https://cdn/x.jpg",
		"mode":            "realistic",
		"prompt":          "Scandinavian coffee shop with oak",
		"style_preset":    "scandinavian",
	})
	req, _ := http.NewRequest("POST", srv.URL+"/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+mintJWT(t, uid))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
}

func TestGenerate_InvalidPrompt(t *testing.T) {
	srv, d, uid := newTestServer(t)
	defer srv.Close()
	profiles := db.Profiles{DB: d}
	require.NoError(t, profiles.Upsert(context.Background(), uid, "x", ""))

	body, _ := json.Marshal(map[string]string{
		"input_image_url": "https://cdn/x.jpg",
		"mode":            "realistic",
		"prompt":          "x",
	})
	req, _ := http.NewRequest("POST", srv.URL+"/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+mintJWT(t, uid))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, 400, resp.StatusCode)
}
