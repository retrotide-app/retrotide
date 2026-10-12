package settings

import (
	"errors"
	"testing"

	"retrotide/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReadSiteMode_ReadsEitherMode(t *testing.T) {
	for _, want := range []config.SiteMode{config.SiteModePublic, config.SiteModePrivate} {
		t.Run(string(want), func(t *testing.T) {
			// given
			svc := NewMockService(t)
			svc.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return(string(want), nil)

			// when
			mode, err := ReadSiteMode(t.Context(), svc)

			// then
			require.NoError(t, err)
			assert.Equal(t, want, mode)
		})
	}
}

func TestReadSiteMode_ReportsASettingThatCannotBeRead(t *testing.T) {
	// given
	errDatabaseDown := errors.New("database down")
	svc := NewMockService(t)
	svc.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("", errDatabaseDown)

	// when
	mode, err := ReadSiteMode(t.Context(), svc)

	// then a failed read is never taken for public
	require.ErrorIs(t, err, errDatabaseDown)
	assert.Empty(t, mode)
}

func TestReadSiteMode_RefusesAnUnknownStoredValue(t *testing.T) {
	// given a row edited by hand, which validation on write never lets through
	svc := NewMockService(t)
	svc.EXPECT().Get(mock.Anything, config.SettingSiteMode).Return("friends", nil)

	// when
	mode, err := ReadSiteMode(t.Context(), svc)

	// then it is neither mode, so nothing opens up
	require.EqualError(t, err, `unknown site mode "friends"`)
	assert.Empty(t, mode)
}
