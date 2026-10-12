package model

import (
	"time"

	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	OrgRole string

	Organisation struct {
		ID          uuid.UUID
		Slug        string
		Name        string
		AllowGuests bool
		CreatedBy   *uuid.UUID
		CreatedAt   time.Time
		UpdatedAt   time.Time
		DeletedAt   *time.Time
	}

	UserOrganisation struct {
		Organisation

		Role     OrgRole
		JoinedAt time.Time
	}

	StaffOrganisation struct {
		Organisation

		OwnerID          *uuid.UUID
		OwnerDisplayName *string
		OwnerEmail       *string
		MemberCount      int
		TeamCount        int
		BoardCount       int
	}

	OrganisationMember struct {
		OrgID    uuid.UUID
		UserID   uuid.UUID
		Role     OrgRole
		JoinedAt time.Time
	}

	OrganisationMemberProfile struct {
		OrganisationMember

		Email       string
		DisplayName string
		SiteRole    role.Role
	}

	OrganisationInvite struct {
		ID        uuid.UUID
		OrgID     uuid.UUID
		TeamID    *uuid.UUID
		TokenHash string
		Email     *string
		Role      OrgRole
		MaxUses   *int
		UseCount  int
		InvitedBy *uuid.UUID
		ExpiresAt time.Time
		RevokedAt *time.Time
		CreatedAt time.Time
	}

	InviteDetails struct {
		OrganisationInvite

		OrgName     string
		OrgSlug     string
		TeamName    *string
		InviterName string
	}

	AcceptedInvite struct {
		Invite     OrganisationInvite
		Member     OrganisationMember
		TeamMember *TeamMember
	}

	Workspace struct {
		Organisation Organisation
		Owner        OrganisationMember
		Team         Team
		Lead         TeamMember
	}
)

const (
	OrgRoleOwner  OrgRole = "owner"
	OrgRoleAdmin  OrgRole = "admin"
	OrgRoleMember OrgRole = "member"
)

const (
	OrganisationNameMaxRunes  = 80
	OrganisationRestoreWindow = 30 * 24 * time.Hour
)

func (o Organisation) Deleted() bool {
	return o.DeletedAt != nil
}

func (o Organisation) RestorableUntil() *time.Time {
	if o.DeletedAt == nil {
		return nil
	}

	return new(o.DeletedAt.Add(OrganisationRestoreWindow))
}

func (i OrganisationInvite) Expired(now time.Time) bool {
	return !now.Before(i.ExpiresAt)
}

func (i OrganisationInvite) BoundToEmail() bool {
	return i.Email != nil
}
