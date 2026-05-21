package providers

import (
	"fmt"

	"github.com/bimantara/ai-image/backend/internal/db"
)

type Registry struct{ byMode map[db.JobMode]ImageProvider }

func NewRegistry() *Registry { return &Registry{byMode: map[db.JobMode]ImageProvider{}} }

func (r *Registry) Register(mode db.JobMode, p ImageProvider) { r.byMode[mode] = p }

func (r *Registry) For(mode db.JobMode) (ImageProvider, error) {
	p, ok := r.byMode[mode]
	if !ok {
		return nil, fmt.Errorf("no provider for mode %q", mode)
	}
	return p, nil
}
