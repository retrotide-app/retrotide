package spec

import (
	"time"

	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	NewOrganisation struct {
		Slug      string
		Name      string
		CreatedBy uuid.UUID
		MaxOwned  int
	}

	OrganisationListing struct {
		Search string
		Limit  int
		Offset int
	}

	NewWorkspace struct {
		OwnerID  uuid.UUID
		Slug     string
		Name     string
		TeamName string
		MaxOwned int
	}

	OrganisationUpdate struct {
		OrgID       uuid.UUID
		Slug        string
		Name        string
		AllowGuests bool
	}

	OrganisationRestore struct {
		OrgID        uuid.UUID
		OwnerID      uuid.UUID
		DeletedAfter time.Time
		MaxOwned     int
	}

	OrganisationMemberKey struct {
		OrgID  uuid.UUID
		UserID uuid.UUID
	}

	NewOrganisationMember struct {
		OrgID      uuid.UUID
		UserID     uuid.UUID
		Role       model.OrgRole
		MaxMembers int
	}

	OrganisationMemberRoleChange struct {
		OrgID  uuid.UUID
		UserID uuid.UUID
		Role   model.OrgRole
	}

	OwnershipTransfer struct {
		OrgID      uuid.UUID
		FromUserID uuid.UUID
		ToUserID   uuid.UUID
		MaxOwned   int
	}

	NewOrganisationInvite struct {
		OrgID     uuid.UUID
		TeamID    *uuid.UUID
		TokenHash string
		Email     *string
		Role      model.OrgRole
		MaxUses   *int
		InvitedBy uuid.UUID
		ExpiresAt time.Time
	}

	OrganisationInviteKey struct {
		OrgID    uuid.UUID
		InviteID uuid.UUID
	}

	InviteConsumption struct {
		TokenHash string
		Email     string
	}

	InviteAcceptance struct {
		TokenHash  string
		UserID     uuid.UUID
		Email      string
		MaxMembers int
	}

	OrgPermissionCheck struct {
		OrgID      uuid.UUID
		UserID     uuid.UUID
		Permission model.TenantPermission
	}

	LimitCheck struct {
		OrgID   uuid.UUID
		Limit   model.Limit
		Current int
	}
)
