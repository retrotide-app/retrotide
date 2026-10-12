package repository

import (
	"context"
	"database/sql"
	"fmt"

	"retrotide/internal/cache"
	"retrotide/internal/dao"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	TeamRepository interface {
		dao.TeamDAO
	}

	teamRepository struct {
		dao.TeamDAO

		db            *sql.DB
		organisations dao.OrganisationDAO
		cache         *cache.Manager
	}
)

func NewTeamRepo(database *sql.DB, teamDAO dao.TeamDAO, organisations dao.OrganisationDAO, c *cache.Manager) TeamRepository {
	return &teamRepository{TeamDAO: teamDAO, db: database, organisations: organisations, cache: c}
}

func (r *teamRepository) Create(ctx context.Context, s spec.NewTeam, tx ...*sql.Tx) (*model.Team, error) {
	var created *model.Team

	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		if err := r.organisations.Lock(ctx, s.OrgID, tx); err != nil {
			return err
		}

		team, err := r.TeamDAO.Create(ctx, s, tx)
		if err != nil {
			return err
		}

		created = team

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *teamRepository) GetHeader(ctx context.Context, teamID uuid.UUID, tx ...*sql.Tx) (*model.Team, error) {
	load := func(ctx context.Context) (*model.Team, error) {
		return r.TeamDAO.GetHeader(ctx, teamID, tx...)
	}

	return r.cache.Load(ctx, cache.TeamHeader, load, teamID.String())
}

func (r *teamRepository) GetMember(ctx context.Context, s spec.TeamMemberKey, tx ...*sql.Tx) (*model.TeamMember, error) {
	load := func(ctx context.Context) (*model.TeamMember, error) {
		return r.TeamDAO.GetMember(ctx, s, tx...)
	}

	return r.cache.Load(ctx, cache.TeamMembership, load, s.OrgID.String(), s.TeamID.String(), s.UserID.String())
}

func (r *teamRepository) Rename(ctx context.Context, s spec.TeamRename, tx ...*sql.Tx) (*model.Team, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("rename team %s: %w", s.TeamID, db.ErrInboundTx)
	}

	renamed, err := r.TeamDAO.Rename(ctx, s)
	if err != nil {
		return nil, err
	}

	return renamed, forgetTenantKeys(ctx, r.cache, []string{cache.TeamHeader.Key(s.TeamID.String())})
}

func (r *teamRepository) Archive(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) (*model.Team, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("archive team %s: %w", s.TeamID, db.ErrInboundTx)
	}

	archived, err := r.TeamDAO.Archive(ctx, s)
	if err != nil {
		return nil, err
	}

	return archived, forgetTenantKeys(ctx, r.cache, []string{cache.TeamHeader.Key(s.TeamID.String())})
}

func (r *teamRepository) Delete(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) (*model.Team, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("delete team %s: %w", s.TeamID, db.ErrInboundTx)
	}

	var (
		deleted *model.Team
		keys    []string
	)

	err := db.WithTx(ctx, r.db, nil, func(tx *sql.Tx) error {
		members, err := r.TeamDAO.ListMembers(ctx, s, tx)
		if err != nil {
			return err
		}

		team, err := r.TeamDAO.Delete(ctx, s, tx)
		if err != nil {
			return err
		}

		deleted = team
		keys = append(keys, cache.TeamHeader.Key(s.TeamID.String()))
		for _, member := range members {
			keys = append(keys, cache.TeamMembership.Key(s.OrgID.String(), s.TeamID.String(), member.UserID.String()))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return deleted, forgetTenantKeys(ctx, r.cache, keys)
}

func (r *teamRepository) AddMember(ctx context.Context, s spec.NewTeamMember, tx ...*sql.Tx) (*model.TeamMember, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("add user %s to team %s: %w", s.UserID, s.TeamID, db.ErrInboundTx)
	}

	added, err := r.TeamDAO.AddMember(ctx, s)
	if err != nil {
		return nil, err
	}

	return added, forgetTenantKeys(ctx, r.cache, []string{cache.TeamMembership.Key(s.OrgID.String(), s.TeamID.String(), s.UserID.String())})
}

func (r *teamRepository) SetMemberRole(ctx context.Context, s spec.TeamMemberRoleChange, tx ...*sql.Tx) (*model.TeamMember, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("change the role of member %s of team %s: %w", s.UserID, s.TeamID, db.ErrInboundTx)
	}

	changed, err := r.TeamDAO.SetMemberRole(ctx, s)
	if err != nil {
		return nil, err
	}

	return changed, forgetTenantKeys(ctx, r.cache, []string{cache.TeamMembership.Key(s.OrgID.String(), s.TeamID.String(), s.UserID.String())})
}

func (r *teamRepository) RemoveMember(ctx context.Context, s spec.TeamMemberKey, tx ...*sql.Tx) (*model.TeamMember, error) {
	if db.Joined(tx) != nil {
		return nil, fmt.Errorf("remove member %s from team %s: %w", s.UserID, s.TeamID, db.ErrInboundTx)
	}

	removed, err := r.TeamDAO.RemoveMember(ctx, s)
	if err != nil {
		return nil, err
	}

	return removed, forgetTenantKeys(ctx, r.cache, []string{cache.TeamMembership.Key(s.OrgID.String(), s.TeamID.String(), s.UserID.String())})
}
