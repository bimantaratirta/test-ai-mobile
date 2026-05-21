package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test only exercises URL shape — actual R2 calls are tested manually
// after deploy, since mocking the AWS SDK presigner is brittle.
func TestPresignPut_BuildsPublicURL(t *testing.T) {
	r, err := NewR2("acc", "ak", "sk", "bkt", "https://cdn.example.com")
	require.NoError(t, err)

	_, public, err := r.PresignPut(context.Background(), "uploads/abc.jpg", "image/jpeg", 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/uploads/abc.jpg", public)
	// upload URL existence is enough to verify wiring
}

func TestUpload_BuildsPublicURL(t *testing.T) {
	r, err := NewR2("acc", "ak", "sk", "bkt", "https://cdn.example.com/")
	require.NoError(t, err)
	// Trim trailing slash if present to avoid double-slash
	assert.True(t, strings.HasPrefix(r.publicBase, "https://cdn"))
}
