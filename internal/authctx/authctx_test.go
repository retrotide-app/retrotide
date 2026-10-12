package authctx

import (
	"testing"

	"retrotide/internal/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	otherKind model.PrincipalKind = "guest"
)

func TestPrincipal_ReturnsTheStoredPrincipal(t *testing.T) {
	// given
	stored := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
	ctx := WithPrincipal(t.Context(), stored)

	// when
	got, ok := Principal(ctx)

	// then
	assert.True(t, ok)
	assert.Equal(t, stored, got)
}

func TestPrincipal_ReportsNoneOnABareContext(t *testing.T) {
	// given
	ctx := t.Context()

	// when
	got, ok := Principal(ctx)

	// then
	assert.False(t, ok)
	assert.Equal(t, model.Principal{}, got)
}

func TestWithPrincipal_NeverStoresAPrincipalWithoutAnID(t *testing.T) {
	// given
	parent := t.Context()

	// when
	ctx := WithPrincipal(parent, model.Principal{Kind: model.PrincipalUser})

	// then
	_, ok := Principal(ctx)
	assert.False(t, ok)
	assert.Equal(t, parent, ctx)
}

func TestWithPrincipal_ReplacesAnEarlierPrincipal(t *testing.T) {
	// given
	first := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
	second := model.Principal{Kind: model.PrincipalUser, ID: uuid.New()}
	ctx := WithPrincipal(t.Context(), first)

	// when
	ctx = WithPrincipal(ctx, second)

	// then
	got, ok := Principal(ctx)
	assert.True(t, ok)
	assert.Equal(t, second, got)
}

func TestUserID_ReturnsTheIDOfAUserPrincipal(t *testing.T) {
	// given
	userID := uuid.New()
	ctx := WithPrincipal(t.Context(), model.Principal{Kind: model.PrincipalUser, ID: userID})

	// when
	got, ok := UserID(ctx)

	// then
	assert.True(t, ok)
	assert.Equal(t, userID, got)
}

func TestUserID_ReportsNoneForAnotherKindOfPrincipal(t *testing.T) {
	// given
	ctx := WithPrincipal(t.Context(), model.Principal{Kind: otherKind, ID: uuid.New()})

	// when
	got, ok := UserID(ctx)

	// then
	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, got)
}

func TestUserID_ReportsNoneOnABareContext(t *testing.T) {
	// given
	ctx := t.Context()

	// when
	got, ok := UserID(ctx)

	// then
	assert.False(t, ok)
	assert.Equal(t, uuid.Nil, got)
}
