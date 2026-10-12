package authctx

import (
	"context"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	principalKey struct{}
)

func WithPrincipal(ctx context.Context, principal model.Principal) context.Context {
	if principal.ID == uuid.Nil {
		return ctx
	}

	return context.WithValue(ctx, principalKey{}, principal)
}

func Principal(ctx context.Context) (model.Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(model.Principal)

	return principal, ok
}

func UserID(ctx context.Context) (uuid.UUID, bool) {
	principal, ok := Principal(ctx)
	if !ok || principal.Kind != model.PrincipalUser {
		return uuid.Nil, false
	}

	return principal.ID, true
}
