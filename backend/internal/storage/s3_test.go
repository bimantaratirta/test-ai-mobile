package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresignPut_BuildsPublicURL(t *testing.T) {
	s, err := NewS3("https://is3.cloudhost.id", "id-jkt-01", "ak", "sk", "bkt", "https://cdn.example.com")
	require.NoError(t, err)

	_, public, err := s.PresignPut(context.Background(), "uploads/abc.jpg", "image/jpeg", 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/uploads/abc.jpg", public)
}

func TestNewS3_DefaultsRegionToAuto(t *testing.T) {
	s, err := NewS3("https://is3.cloudhost.id", "", "ak", "sk", "bkt", "https://cdn")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(s.publicBase, "https://"))
}
