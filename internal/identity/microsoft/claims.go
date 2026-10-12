package microsoft

import (
	"fmt"
	"net/mail"
	"strings"

	"retrotide/internal/identity"
)

type (
	flexBool bool

	microsoftClaims struct {
		Issuer            string   `json:"iss"`
		TenantID          string   `json:"tid"`
		ObjectID          string   `json:"oid"`
		Email             string   `json:"email"`
		PreferredUsername string   `json:"preferred_username"`
		Name              string   `json:"name"`
		DomainOwnerVerify flexBool `json:"xms_edov"`
	}

	tenancy struct {
		tenantID string
		personal bool
	}
)

const (
	segmentCommon        = "common"
	segmentOrganizations = "organizations"
)

func (b *flexBool) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "null":
		return nil
	case "true", `"true"`, `"1"`:
		*b = true
	case "false", `"false"`:
		*b = false
	default:
		return fmt.Errorf("%w: %s", errUnreadableFlag, data)
	}

	return nil
}

func (t tenancy) segment() (string, bool) {
	if t.tenantID != "" {
		return t.tenantID, true
	}

	if t.personal {
		return segmentCommon, false
	}

	return segmentOrganizations, false
}

func (t tenancy) admit(authority string, claims microsoftClaims) error {
	if !isGUID(claims.TenantID) {
		return fmt.Errorf("%w: the id token carries no directory (tenant) id", identity.ErrAccountNotAllowed)
	}

	if claims.ObjectID == "" {
		return fmt.Errorf("%w: the id token carries no object id", identity.ErrAccountNotAllowed)
	}

	if claims.Issuer != authority+"/"+claims.TenantID+"/v2.0" {
		return fmt.Errorf("%w: the id token was issued by %q, not by the directory it names", identity.ErrAccountNotAllowed, claims.Issuer)
	}

	if t.tenantID != "" {
		if !strings.EqualFold(claims.TenantID, t.tenantID) {
			return fmt.Errorf("%w: the account belongs to directory %s, not to the one this site admits", identity.ErrAccountNotAllowed, claims.TenantID)
		}

		return nil
	}

	if strings.EqualFold(claims.TenantID, personalTenantID) && !t.personal {
		return fmt.Errorf("%w: personal Microsoft accounts are turned off", identity.ErrAccountNotAllowed)
	}

	return nil
}

func (c microsoftClaims) address() (string, bool) {
	if c.Email != "" {
		return c.Email, true
	}

	parsed, err := mail.ParseAddress(c.PreferredUsername)
	if err != nil || parsed.Address != c.PreferredUsername {
		return "", false
	}

	return parsed.Address, false
}

func (c microsoftClaims) subject() string {
	return strings.ToLower(c.TenantID) + ":" + strings.ToLower(c.ObjectID)
}
