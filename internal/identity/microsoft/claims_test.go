package microsoft

import (
	"encoding/json"
	"testing"

	"retrotide/internal/identity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAuthority = "https://login.example"
)

func TestFlexBool_ReadsTheShapesEntraSends(t *testing.T) {
	tests := []struct {
		name string
		json string
		want bool
	}{
		{name: "json true", json: `{"xms_edov":true}`, want: true},
		{name: "json false", json: `{"xms_edov":false}`, want: false},
		{name: "the string true", json: `{"xms_edov":"true"}`, want: true},
		{name: "the string 1", json: `{"xms_edov":"1"}`, want: true},
		{name: "the string false", json: `{"xms_edov":"false"}`, want: false},
		{name: "null", json: `{"xms_edov":null}`, want: false},
		{name: "absent", json: `{}`, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			var claims microsoftClaims

			// when
			err := json.Unmarshal([]byte(tc.json), &claims)

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.want, bool(claims.DomainOwnerVerify))
		})
	}
}

func TestFlexBool_RefusesAnythingElse(t *testing.T) {
	tests := []string{`{"xms_edov":"yes"}`, `{"xms_edov":1}`, `{"xms_edov":"0"}`, `{"xms_edov":"TRUE"}`}

	for _, payload := range tests {
		t.Run(payload, func(t *testing.T) {
			// given
			var claims microsoftClaims

			// when
			err := json.Unmarshal([]byte(payload), &claims)

			// then
			require.ErrorIs(t, err, errUnreadableFlag)
		})
	}
}

func TestTenancy_Segment(t *testing.T) {
	tests := []struct {
		name       string
		tenancy    tenancy
		wantSeg    string
		wantPinned bool
	}{
		{name: "work accounts only", tenancy: tenancy{}, wantSeg: "organizations", wantPinned: false},
		{name: "work and personal accounts", tenancy: tenancy{personal: true}, wantSeg: "common", wantPinned: false},
		{name: "one tenant", tenancy: tenancy{tenantID: testTenantID}, wantSeg: testTenantID, wantPinned: true},
		{name: "one tenant ignores the personal switch", tenancy: tenancy{tenantID: testTenantID, personal: true}, wantSeg: testTenantID, wantPinned: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			admitted := tc.tenancy

			// when
			segment, pinned := admitted.segment()

			// then
			assert.Equal(t, tc.wantSeg, segment)
			assert.Equal(t, tc.wantPinned, pinned)
		})
	}
}

func TestTenancy_Admit(t *testing.T) {
	valid := func() microsoftClaims {
		return microsoftClaims{Issuer: testAuthority + "/" + testTenantID + "/v2.0", TenantID: testTenantID, ObjectID: testObjectID}
	}

	tests := []struct {
		name    string
		tenancy tenancy
		claims  func(claims *microsoftClaims)
		allowed bool
	}{
		{name: "a work account from any organisation", tenancy: tenancy{}, allowed: true},
		{name: "a work account from the pinned tenant", tenancy: tenancy{tenantID: testTenantID}, allowed: true},
		{name: "a work account from another tenant", tenancy: tenancy{tenantID: testOtherTenantID}, allowed: false},
		{name: "a personal account with the switch off", tenancy: tenancy{}, claims: func(claims *microsoftClaims) {
			claims.TenantID = personalTenantID
			claims.Issuer = testAuthority + "/" + personalTenantID + "/v2.0"
		}, allowed: false},
		{name: "a personal account with the switch on", tenancy: tenancy{personal: true}, claims: func(claims *microsoftClaims) {
			claims.TenantID = personalTenantID
			claims.Issuer = testAuthority + "/" + personalTenantID + "/v2.0"
		}, allowed: true},
		{name: "a personal account while a tenant is pinned with the switch on", tenancy: tenancy{tenantID: testTenantID, personal: true}, claims: func(claims *microsoftClaims) {
			claims.TenantID = personalTenantID
			claims.Issuer = testAuthority + "/" + personalTenantID + "/v2.0"
		}, allowed: false},
		{name: "no tid", tenancy: tenancy{}, claims: func(claims *microsoftClaims) {
			claims.TenantID = ""
		}, allowed: false},
		{name: "no oid", tenancy: tenancy{}, claims: func(claims *microsoftClaims) {
			claims.ObjectID = ""
		}, allowed: false},
		{name: "an issuer for another directory", tenancy: tenancy{}, claims: func(claims *microsoftClaims) {
			claims.Issuer = testAuthority + "/" + testOtherTenantID + "/v2.0"
		}, allowed: false},
		{name: "an issuer at another authority", tenancy: tenancy{}, claims: func(claims *microsoftClaims) {
			claims.Issuer = "https://sts.example/" + testTenantID + "/v2.0"
		}, allowed: false},
		{name: "the templated issuer", tenancy: tenancy{}, claims: func(claims *microsoftClaims) {
			claims.Issuer = testAuthority + "/{tenantid}/v2.0"
		}, allowed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			claims := valid()
			if tc.claims != nil {
				tc.claims(&claims)
			}

			// when
			err := tc.tenancy.admit(testAuthority, claims)

			// then
			if tc.allowed {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, identity.ErrAccountNotAllowed)
		})
	}
}

func TestClaims_Address(t *testing.T) {
	tests := []struct {
		name              string
		email             string
		preferredUsername string
		want              string
		wantFromEmail     bool
	}{
		{name: "the email claim", email: "ada@contoso.example", preferredUsername: "someone@contoso.example", want: "ada@contoso.example", wantFromEmail: true},
		{name: "the preferred username when it is an address", preferredUsername: "ada@contoso.example", want: "ada@contoso.example"},
		{name: "a preferred username that is a phone number", preferredUsername: "+442079460000", want: ""},
		{name: "a preferred username with a display name", preferredUsername: "Ada <ada@contoso.example>", want: ""},
		{name: "neither", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			claims := microsoftClaims{Email: tc.email, PreferredUsername: tc.preferredUsername}

			// when
			got, fromEmail := claims.address()

			// then
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantFromEmail, fromEmail)
		})
	}
}
