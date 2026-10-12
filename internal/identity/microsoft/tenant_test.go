package microsoft

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTenantID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "empty", value: "", valid: true},
		{name: "a directory id", value: testTenantID, valid: true},
		{name: "a directory id in capitals", value: "72F988BF-86F1-41AF-91AB-2D7CD011DB47", valid: true},
		{name: "a domain name", value: "contoso.onmicrosoft.com", valid: false},
		{name: "common", value: "common", valid: false},
		{name: "organizations", value: "organizations", valid: false},
		{name: "a guid without hyphens", value: "72f988bf86f141af91ab2d7cd011db47", valid: false},
		{name: "a guid in braces", value: "{72f988bf-86f1-41af-91ab-2d7cd011db47}", valid: false},
		{name: "a urn guid", value: "urn:uuid:72f988bf-86f1-41af-91ab-2d7cd011db47", valid: false},
		{name: "a guid with spaces around it", value: " 72f988bf-86f1-41af-91ab-2d7cd011db47 ", valid: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			value := tc.value

			// when
			err := ValidateTenantID(t.Context(), value)

			// then
			if tc.valid {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, errInvalidTenantID)
			assert.EqualError(t, err, "microsoft_tenant_id must be empty or a directory (tenant) ID such as 00000000-0000-0000-0000-000000000000")
		})
	}
}
