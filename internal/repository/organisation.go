package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"retrotide/internal/cache"
	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	OrganisationRepository interface {
		dao.OrganisationDAO

		CreateWorkspace(ctx context.Context, s spec.NewWorkspace, tx ...*sql.Tx) (*model.Workspace, error)
		AcceptInvite(ctx context.Context, s spec.InviteAcceptance, tx ...*sql.Tx) (*model.AcceptedInvite, error)
	}

	organisationRepository struct {
		dao.OrganisationDAO

		db    *sql.DB
		users dao.UserDAO
		teams dao.TeamDAO
		cache *cache.Manager
	}

	OrganisationRepoDeps struct {
		Users dao.UserDAO
		Teams dao.TeamDAO
	}
)

const (
	firstMember = 1
	firstTeam   = 1
)

func NewOrganisationRepo(database *sql.DB, organisationDAO dao.OrganisationDAO, deps OrganisationRepoDeps, c *cache.Manager) OrganisationRepository {
	return &organisationRepository{
		OrganisationDAO: organisationDAO,
		db:              database,
		users:           deps.Users,
		teams:           deps.Teams,
		cache:           c,
	}
}

func (r *organisationRepository) CreateWorkspace(ctx context.Context, s spec.NewWorkspace, tx ...*sql.Tx) (*model.Workspace, error) {
	var created *model.Workspace

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if _, err := r.users.GetForUpdate(ctx, s.OwnerID, tx); err != nil {
			return err
		}

		organisation, err := r.OrganisationDAO.Create(ctx, spec.NewOrganisation{Slug: s.Slug, Name: s.Name, CreatedBy: s.OwnerID, MaxOwned: s.MaxOwned}, tx)
		if err != nil {
			return err
		}

		owner, err := r.OrganisationDAO.AddMember(ctx, spec.NewOrganisationMember{OrgID: organisation.ID, UserID: s.OwnerID, Role: model.OrgRoleOwner, MaxMembers: firstMember}, tx)
		if err != nil {
			return err
		}

		team, err := r.teams.Create(ctx, spec.NewTeam{OrgID: organisation.ID, Name: s.TeamName, MaxTeams: firstTeam}, tx)
		if err != nil {
			return err
		}

		lead, err := r.teams.AddMember(ctx, spec.NewTeamMember{OrgID: organisation.ID, TeamID: team.ID, UserID: s.OwnerID, Role: model.TeamRoleLead}, tx)
		if err != nil {
			return err
		}

		created = &model.Workspace{Organisation: *organisation, Owner: *owner, Team: *team, Lead: *lead}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create workspace %q for user %s: %w", s.Slug, s.OwnerID, err)
	}

	return created, nil
}

func (r *organisationRepository) AcceptInvite(ctx context.Context, s spec.InviteAcceptance, tx ...*sql.Tx) (*model.AcceptedInvite, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("accept invite for user %s: %w", s.UserID, db.ErrInboundTx)
	}

	var accepted *model.AcceptedInvite

	err := db.WithTx(ctx, r.db, nil, func(tx *sql.Tx) error {
		live, err := r.OrganisationDAO.GetLiveInviteByTokenHash(ctx, s.TokenHash, tx)
		if err != nil {
			return err
		}

		if err := r.OrganisationDAO.Lock(ctx, live.OrgID, tx); err != nil {
			return err
		}

		invite, err := r.OrganisationDAO.ConsumeInvite(ctx, spec.InviteConsumption{TokenHash: s.TokenHash, Email: s.Email}, tx)
		if err != nil {
			return err
		}

		member, err := r.OrganisationDAO.AddMember(ctx, spec.NewOrganisationMember{OrgID: invite.OrgID, UserID: s.UserID, Role: invite.Role, MaxMembers: s.MaxMembers}, tx)
		if err != nil {
			return err
		}

		accepted = &model.AcceptedInvite{Invite: *invite, Member: *member}

		if invite.TeamID == nil {
			return nil
		}

		team, err := r.teams.GetByID(ctx, spec.TeamKey{OrgID: invite.OrgID, TeamID: *invite.TeamID}, tx)
		if err != nil {
			return err
		}

		if team.Archived() {
			return nil
		}

		teamMember, err := r.teams.AddMember(ctx, spec.NewTeamMember{OrgID: invite.OrgID, TeamID: *invite.TeamID, UserID: s.UserID, Role: model.TeamRoleMember}, tx)
		if err != nil {
			return err
		}

		accepted.TeamMember = teamMember

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("accept invite for user %s: %w", s.UserID, err)
	}

	var teamIDs []uuid.UUID
	if accepted.TeamMember != nil {
		teamIDs = append(teamIDs, accepted.TeamMember.TeamID)
	}

	return accepted, forgetTenantKeys(ctx, r.cache, membershipKeys(accepted.Member.OrgID, s.UserID, teamIDs...))
}

func (r *organisationRepository) Restore(ctx context.Context, s spec.OrganisationRestore, tx ...*sql.Tx) (*model.Organisation, error) {
	var restored *model.Organisation

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if _, err := r.users.GetForUpdate(ctx, s.OwnerID, tx); err != nil {
			return err
		}

		organisation, err := r.OrganisationDAO.Restore(ctx, s, tx)
		if errors.Is(err, dao.ErrNotFound) {
			return r.unrestored(ctx, s, err, tx)
		}
		if err != nil {
			return err
		}

		restored = organisation

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("restore organisation %s for owner %s: %w", s.OrgID, s.OwnerID, err)
	}

	return restored, nil
}

func (r *organisationRepository) unrestored(ctx context.Context, s spec.OrganisationRestore, refusal error, tx *sql.Tx) error {
	found, err := r.OrganisationDAO.GetByID(ctx, s.OrgID, tx)
	if err != nil {
		return err
	}

	if found.Deleted() && found.DeletedAt.After(s.DeletedAfter) {
		return fmt.Errorf("owner %s already holds %d organisations: %w", s.OwnerID, s.MaxOwned, dao.ErrQuotaExceeded)
	}

	return refusal
}

func (r *organisationRepository) GetMember(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) (*model.OrganisationMember, error) {
	load := func(ctx context.Context) (*model.OrganisationMember, error) {
		return r.OrganisationDAO.GetMember(ctx, s, tx...)
	}

	return r.cache.Load(ctx, cache.OrgMembership, load, s.OrgID.String(), s.UserID.String())
}

func (r *organisationRepository) AddMember(ctx context.Context, s spec.NewOrganisationMember, tx ...*sql.Tx) (*model.OrganisationMember, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("add user %s to organisation %s: %w", s.UserID, s.OrgID, db.ErrInboundTx)
	}

	var added *model.OrganisationMember

	err := db.WithTx(ctx, r.db, nil, func(tx *sql.Tx) error {
		if err := r.OrganisationDAO.Lock(ctx, s.OrgID, tx); err != nil {
			return err
		}

		member, err := r.OrganisationDAO.AddMember(ctx, s, tx)
		if err != nil {
			return err
		}

		added = member

		return nil
	})
	if err != nil {
		return nil, err
	}

	return added, forgetTenantKeys(ctx, r.cache, membershipKeys(s.OrgID, s.UserID))
}

func (r *organisationRepository) SetMemberRole(ctx context.Context, s spec.OrganisationMemberRoleChange, tx ...*sql.Tx) (*model.OrganisationMember, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("change the role of member %s of organisation %s: %w", s.UserID, s.OrgID, db.ErrInboundTx)
	}

	changed, err := r.OrganisationDAO.SetMemberRole(ctx, s)
	if err != nil {
		return nil, err
	}

	return changed, forgetTenantKeys(ctx, r.cache, membershipKeys(s.OrgID, s.UserID))
}

func (r *organisationRepository) RemoveMember(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) (*model.OrganisationMember, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("remove member %s from organisation %s: %w", s.UserID, s.OrgID, db.ErrInboundTx)
	}

	var (
		removed *model.OrganisationMember
		teamIDs []uuid.UUID
	)

	err := db.WithTx(ctx, r.db, nil, func(tx *sql.Tx) error {
		memberTeams, err := r.teams.ListForMember(ctx, s, tx)
		if err != nil {
			return err
		}

		member, err := r.OrganisationDAO.RemoveMember(ctx, s, tx)
		if err != nil {
			return err
		}

		removed = member
		for _, team := range memberTeams {
			teamIDs = append(teamIDs, team.ID)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return removed, forgetTenantKeys(ctx, r.cache, membershipKeys(s.OrgID, s.UserID, teamIDs...))
}

func (r *organisationRepository) TransferOwnership(ctx context.Context, s spec.OwnershipTransfer, tx ...*sql.Tx) error {
	if db.Joined(tx) != nil {
		return fmt.Errorf("transfer organisation %s: %w", s.OrgID, db.ErrInboundTx)
	}

	err := db.WithTx(ctx, r.db, nil, func(tx *sql.Tx) error {
		if err := r.OrganisationDAO.Lock(ctx, s.OrgID, tx); err != nil {
			return err
		}

		if _, err := r.users.GetForUpdate(ctx, s.ToUserID, tx); err != nil {
			return err
		}

		return r.OrganisationDAO.TransferOwnership(ctx, s, tx)
	})
	if err != nil {
		return err
	}

	keys := append(membershipKeys(s.OrgID, s.FromUserID), membershipKeys(s.OrgID, s.ToUserID)...)

	return forgetTenantKeys(ctx, r.cache, keys)
}

func forgetTenantKeys(ctx context.Context, c *cache.Manager, keys []string) error {
	if err := c.Del(context.WithoutCancel(ctx), keys...); err != nil {
		return fmt.Errorf("%w: %w", ErrStaleTenantCache, err)
	}

	return nil
}

func membershipKeys(orgID uuid.UUID, userID uuid.UUID, teamIDs ...uuid.UUID) []string {
	keys := make([]string, 0, len(teamIDs)+1)
	keys = append(keys, cache.OrgMembership.Key(orgID.String(), userID.String()))

	for _, teamID := range teamIDs {
		keys = append(keys, cache.TeamMembership.Key(orgID.String(), teamID.String(), userID.String()))
	}

	return keys
}
