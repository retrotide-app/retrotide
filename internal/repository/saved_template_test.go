package repository

import (
	"context"
	"database/sql"
	"testing"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type (
	savedTemplateRepoMocks struct {
		templates     *dao.MockSavedTemplateDAO
		organisations *dao.MockOrganisationDAO
	}
)

func newMockedSavedTemplateRepo(t *testing.T) (SavedTemplateRepository, savedTemplateRepoMocks) {
	t.Helper()

	mocks := savedTemplateRepoMocks{
		templates:     dao.NewMockSavedTemplateDAO(t),
		organisations: dao.NewMockOrganisationDAO(t),
	}

	return NewSavedTemplateRepo(nil, mocks.templates, mocks.organisations), mocks
}

func newSavedTemplateSpec() spec.NewSavedTemplate {
	ownerID := uuid.New()

	return spec.NewSavedTemplate{
		OrgID:        uuid.New(),
		OwnerID:      &ownerID,
		CreatedBy:    ownerID,
		Name:         "Our retro",
		Columns:      []model.SavedTemplateColumn{{Title: "Went well", Colour: model.ColourGreen}},
		MaxTemplates: 3,
	}
}

func TestSavedTemplateCreate_LocksTheOrganisationBeforeTheQuotaInsert(t *testing.T) {
	// given
	repo, mocks := newMockedSavedTemplateRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	newTemplate := newSavedTemplateSpec()
	created := &model.SavedTemplate{ID: uuid.New(), OrgID: newTemplate.OrgID, Name: newTemplate.Name}
	var order []string
	mocks.organisations.EXPECT().Lock(mock.Anything, newTemplate.OrgID, inTx).RunAndReturn(func(context.Context, uuid.UUID, ...*sql.Tx) error {
		order = append(order, "lock")
		return nil
	}).Once()
	mocks.templates.EXPECT().Create(mock.Anything, newTemplate, inTx).RunAndReturn(func(context.Context, spec.NewSavedTemplate, ...*sql.Tx) (*model.SavedTemplate, error) {
		order = append(order, "insert")
		return created, nil
	}).Once()

	// when
	got, err := repo.Create(t.Context(), newTemplate, tx)

	// then the count the insert compares with cannot change under it
	require.NoError(t, err)
	assert.Same(t, created, got)
	assert.Equal(t, []string{"lock", "insert"}, order)
}

func TestSavedTemplateCreate_ADeletedOrganisationIsNeverLockedForATemplate(t *testing.T) {
	// given a saved template dao mock with no expectations, so an insert fails the test
	repo, mocks := newMockedSavedTemplateRepo(t)
	tx := new(sql.Tx)
	newTemplate := newSavedTemplateSpec()
	mocks.organisations.EXPECT().Lock(mock.Anything, newTemplate.OrgID, []*sql.Tx{tx}).Return(dao.ErrNotFound).Once()

	// when
	_, err := repo.Create(t.Context(), newTemplate, tx)

	// then
	require.ErrorIs(t, err, dao.ErrNotFound)
}

func TestSavedTemplateCreate_TheQuotaRefusalComesBackFromInsideTheLock(t *testing.T) {
	// given
	repo, mocks := newMockedSavedTemplateRepo(t)
	tx := new(sql.Tx)
	inTx := []*sql.Tx{tx}
	newTemplate := newSavedTemplateSpec()
	mocks.organisations.EXPECT().Lock(mock.Anything, newTemplate.OrgID, inTx).Return(nil).Once()
	mocks.templates.EXPECT().Create(mock.Anything, newTemplate, inTx).Return(nil, dao.ErrQuotaExceeded).Once()

	// when
	got, err := repo.Create(t.Context(), newTemplate, tx)

	// then
	require.ErrorIs(t, err, dao.ErrQuotaExceeded)
	assert.Nil(t, got)
}
