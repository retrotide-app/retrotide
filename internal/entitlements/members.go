package entitlements

import (
	"retrotide/internal/model"
)

func CountsAsMember(principal model.Principal, membership *model.OrganisationMember) bool {
	if principal.Kind != model.PrincipalUser || membership == nil {
		return false
	}

	return membership.UserID == principal.ID
}
