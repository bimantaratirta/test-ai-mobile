package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type ctxKey struct{}

func WithUserID(ctx context.Context, uid uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, uid)
}

func UserIDFrom(ctx context.Context) (uuid.UUID, error) {
	v, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("no user in context")
	}
	return v, nil
}
