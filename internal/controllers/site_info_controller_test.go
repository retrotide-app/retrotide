package controllers

import (
	"net/http"
	"testing"

	"retrotide/internal/controllers/utils/testutil"
	"retrotide/internal/dto"
	"retrotide/internal/siteinfo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type (
	siteInfoHarness struct {
		*testutil.Harness
		siteInfo *siteinfo.MockService
	}
)

func newSiteInfoHarness(t *testing.T) *siteInfoHarness {
	t.Helper()

	h := testutil.NewHarness(t)

	s := &siteInfoHarness{Harness: h, siteInfo: siteinfo.NewMockService(t)}

	ctrl := &Service{SiteInfoService: s.siteInfo, Guards: h.Guards}
	RegisterRoutes(h.Engine.Group("/api/v1"), ctrl.getAllSiteInfoRoutes())

	return s
}

func TestSiteInfoController_AnswersEveryVisitorWithTheSiteInfo(t *testing.T) {
	// given no session, because the shell reads this before anyone signs in
	h := newSiteInfoHarness(t)
	h.siteInfo.EXPECT().Get(mock.Anything).Return(dto.SiteInfoResponse{
		SiteName:          "Harbour Retros",
		DeploymentMode:    "self_hosted",
		SiteMode:          "private",
		RegistrationOpen:  false,
		MaintenanceMode:   true,
		TurnstileEnabled:  true,
		TurnstileSiteKey:  "0x4AAAAAAAtestsitekey",
		IdentityProviders: []dto.IdentityProviderResponse{},
	}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/site-info").Record()

	// then a false registration_open is sent rather than left out, so the SPA hides the link
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"site_name": "Harbour Retros",
		"deployment_mode": "self_hosted",
		"site_mode": "private",
		"registration_open": false,
		"email_enabled": false,
		"maintenance_mode": true,
		"turnstile_enabled": true,
		"turnstile_site_key": "0x4AAAAAAAtestsitekey",
		"identity_providers": [],
		"password_sign_in": false
	}`, rec.Body.String())
}

func TestSiteInfoController_ReportsAPublicSiteWithItsProviders(t *testing.T) {
	// given
	h := newSiteInfoHarness(t)
	h.siteInfo.EXPECT().Get(mock.Anything).Return(dto.SiteInfoResponse{
		SiteName:          "Harbour Retros",
		DeploymentMode:    "hosted",
		SiteMode:          "public",
		RegistrationOpen:  true,
		EmailEnabled:      true,
		IdentityProviders: []dto.IdentityProviderResponse{{ID: "google", Name: "Google"}},
		PasswordSignIn:    true,
	}, nil)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/site-info").Record()

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"site_name": "Harbour Retros",
		"deployment_mode": "hosted",
		"site_mode": "public",
		"registration_open": true,
		"email_enabled": true,
		"maintenance_mode": false,
		"turnstile_enabled": false,
		"turnstile_site_key": "",
		"identity_providers": [{"id": "google", "name": "Google"}],
		"password_sign_in": true
	}`, rec.Body.String())
}

func TestSiteInfoController_ReportsASiteInfoThatCannotBeRead(t *testing.T) {
	// given
	h := newSiteInfoHarness(t)
	h.siteInfo.EXPECT().Get(mock.Anything).Return(dto.SiteInfoResponse{}, errServiceDown)

	// when
	rec := h.NewRequest(http.MethodGet, "/api/v1/site-info").Record()

	// then the shell sees a failure rather than a default site
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"failed to read the site info"}`, rec.Body.String())
}
