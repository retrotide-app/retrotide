package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	TeamDAO interface {
		Create(ctx context.Context, s spec.NewTeam, tx ...*sql.Tx) (*model.Team, error)
		GetHeader(ctx context.Context, teamID uuid.UUID, tx ...*sql.Tx) (*model.Team, error)
		GetByID(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) (*model.Team, error)
		ListForOrganisation(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.Team, error)
		ListWithCounts(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.TeamWithCounts, error)
		CountActive(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (int, error)
		Rename(ctx context.Context, s spec.TeamRename, tx ...*sql.Tx) (*model.Team, error)
		Archive(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) (*model.Team, error)
		Delete(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) (*model.Team, error)

		AddMember(ctx context.Context, s spec.NewTeamMember, tx ...*sql.Tx) (*model.TeamMember, error)
		GetMember(ctx context.Context, s spec.TeamMemberKey, tx ...*sql.Tx) (*model.TeamMember, error)
		ListMembers(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) ([]model.TeamMemberProfile, error)
		ListForMember(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) ([]model.MemberTeam, error)
		SetMemberRole(ctx context.Context, s spec.TeamMemberRoleChange, tx ...*sql.Tx) (*model.TeamMember, error)
		RemoveMember(ctx context.Context, s spec.TeamMemberKey, tx ...*sql.Tx) (*model.TeamMember, error)
	}

	teamDAO struct {
		db *sql.DB
	}
)

func toTeam(row sqlcgen.Team) *model.Team {
	return &model.Team{
		ID:         row.ID,
		OrgID:      row.OrgID,
		Name:       row.Name,
		CreatedAt:  row.CreatedAt,
		ArchivedAt: nullableTime(row.ArchivedAt),
	}
}

func toTeamMember(row sqlcgen.TeamMember) *model.TeamMember {
	return &model.TeamMember{
		OrgID:    row.OrgID,
		TeamID:   row.TeamID,
		UserID:   row.UserID,
		Role:     model.TeamRole(row.Role),
		JoinedAt: row.JoinedAt,
	}
}

func (r *teamDAO) Create(ctx context.Context, s spec.NewTeam, tx ...*sql.Tx) (*model.Team, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateTeamWithinQuota(ctx, sqlcgen.CreateTeamWithinQuotaParams{
		OrgID:    s.OrgID,
		Name:     s.Name,
		MaxTeams: clampInt32(s.MaxTeams),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("create team %q in organisation %s: %w", s.Name, s.OrgID, ErrQuotaExceeded)
	}
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create team %q in organisation %s: %w: %w", s.Name, s.OrgID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("create team %q in organisation %s: %w: %w", s.Name, s.OrgID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create team %q in organisation %s: %w", s.Name, s.OrgID, err)
	}

	return toTeam(row), nil
}

func (r *teamDAO) GetHeader(ctx context.Context, teamID uuid.UUID, tx ...*sql.Tx) (*model.Team, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetTeamHeader(ctx, teamID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get team header %s: %w", teamID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get team header %s: %w", teamID, err)
	}

	return toTeam(row), nil
}

func (r *teamDAO) GetByID(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) (*model.Team, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetTeam(ctx, sqlcgen.GetTeamParams{OrgID: s.OrgID, ID: s.TeamID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get team %s of organisation %s: %w", s.TeamID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get team %s of organisation %s: %w", s.TeamID, s.OrgID, err)
	}

	return toTeam(row), nil
}

func (r *teamDAO) ListForOrganisation(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.Team, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListTeamsForOrganisation(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list teams of organisation %s: %w", orgID, err)
	}

	teams := make([]model.Team, 0, len(rows))
	for _, row := range rows {
		teams = append(teams, *toTeam(row))
	}

	return teams, nil
}

func (r *teamDAO) ListWithCounts(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.TeamWithCounts, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListTeamsWithCountsForOrganisation(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list teams with counts of organisation %s: %w", orgID, err)
	}

	teams := make([]model.TeamWithCounts, 0, len(rows))
	for _, row := range rows {
		teams = append(teams, model.TeamWithCounts{Team: *toTeam(row.Team), MemberCount: int(row.MemberCount), BoardCount: int(row.BoardCount)})
	}

	return teams, nil
}

func (r *teamDAO) CountActive(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountActiveTeams(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("count active teams of organisation %s: %w", orgID, err)
	}

	return int(count), nil
}

func (r *teamDAO) Rename(ctx context.Context, s spec.TeamRename, tx ...*sql.Tx) (*model.Team, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RenameTeam(ctx, sqlcgen.RenameTeamParams{OrgID: s.OrgID, ID: s.TeamID, Name: s.Name})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("rename team %s of organisation %s: %w", s.TeamID, s.OrgID, ErrNotFound)
	}
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("rename team %s of organisation %s to %q: %w: %w", s.TeamID, s.OrgID, s.Name, ErrConflict, err)
	}
	if err != nil {
		return nil, fmt.Errorf("rename team %s of organisation %s: %w", s.TeamID, s.OrgID, err)
	}

	return toTeam(row), nil
}

func (r *teamDAO) Archive(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) (*model.Team, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ArchiveTeam(ctx, sqlcgen.ArchiveTeamParams{OrgID: s.OrgID, ID: s.TeamID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("archive team %s of organisation %s: %w", s.TeamID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("archive team %s of organisation %s: %w", s.TeamID, s.OrgID, err)
	}

	return toTeam(row), nil
}

func (r *teamDAO) Delete(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) (*model.Team, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteTeam(ctx, sqlcgen.DeleteTeamParams{OrgID: s.OrgID, ID: s.TeamID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("delete team %s of organisation %s: %w", s.TeamID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("delete team %s of organisation %s: %w", s.TeamID, s.OrgID, err)
	}

	return toTeam(row), nil
}

func (r *teamDAO) AddMember(ctx context.Context, s spec.NewTeamMember, tx ...*sql.Tx) (*model.TeamMember, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).AddTeamMember(ctx, sqlcgen.AddTeamMemberParams{
		OrgID:  s.OrgID,
		TeamID: s.TeamID,
		UserID: s.UserID,
		Role:   string(s.Role),
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("add user %s to team %s: %w: %w", s.UserID, s.TeamID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("add user %s to team %s of organisation %s: %w: %w", s.UserID, s.TeamID, s.OrgID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("add user %s to team %s: %w", s.UserID, s.TeamID, err)
	}

	return toTeamMember(row), nil
}

func (r *teamDAO) GetMember(ctx context.Context, s spec.TeamMemberKey, tx ...*sql.Tx) (*model.TeamMember, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetTeamMember(ctx, sqlcgen.GetTeamMemberParams{
		OrgID:  s.OrgID,
		TeamID: s.TeamID,
		UserID: s.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get member %s of team %s: %w", s.UserID, s.TeamID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get member %s of team %s: %w", s.UserID, s.TeamID, err)
	}

	return toTeamMember(row), nil
}

func (r *teamDAO) ListMembers(ctx context.Context, s spec.TeamKey, tx ...*sql.Tx) ([]model.TeamMemberProfile, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListTeamMembers(ctx, sqlcgen.ListTeamMembersParams{OrgID: s.OrgID, TeamID: s.TeamID})
	if err != nil {
		return nil, fmt.Errorf("list members of team %s: %w", s.TeamID, err)
	}

	members := make([]model.TeamMemberProfile, 0, len(rows))
	for _, row := range rows {
		members = append(members, model.TeamMemberProfile{
			TeamMember:  *toTeamMember(row.TeamMember),
			Email:       row.Email,
			DisplayName: row.DisplayName,
			OrgRole:     model.OrgRole(row.OrgRole),
			SiteRole:    role.Role(row.SiteRole),
		})
	}

	return members, nil
}

func (r *teamDAO) ListForMember(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) ([]model.MemberTeam, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListTeamsForMember(ctx, sqlcgen.ListTeamsForMemberParams{OrgID: s.OrgID, UserID: s.UserID})
	if err != nil {
		return nil, fmt.Errorf("list teams of member %s in organisation %s: %w", s.UserID, s.OrgID, err)
	}

	teams := make([]model.MemberTeam, 0, len(rows))
	for _, row := range rows {
		teams = append(teams, model.MemberTeam{
			Team:     *toTeam(row.Team),
			Role:     model.TeamRole(row.MemberRole),
			JoinedAt: row.MemberJoinedAt,
		})
	}

	return teams, nil
}

func (r *teamDAO) SetMemberRole(ctx context.Context, s spec.TeamMemberRoleChange, tx ...*sql.Tx) (*model.TeamMember, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetTeamMemberRole(ctx, sqlcgen.SetTeamMemberRoleParams{
		OrgID:  s.OrgID,
		TeamID: s.TeamID,
		UserID: s.UserID,
		Role:   string(s.Role),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("make member %s of team %s %s: %w", s.UserID, s.TeamID, s.Role, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("make member %s of team %s %s: %w", s.UserID, s.TeamID, s.Role, err)
	}

	return toTeamMember(row), nil
}

func (r *teamDAO) RemoveMember(ctx context.Context, s spec.TeamMemberKey, tx ...*sql.Tx) (*model.TeamMember, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RemoveTeamMember(ctx, sqlcgen.RemoveTeamMemberParams{
		OrgID:  s.OrgID,
		TeamID: s.TeamID,
		UserID: s.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("remove member %s from team %s: %w", s.UserID, s.TeamID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("remove member %s from team %s: %w", s.UserID, s.TeamID, err)
	}

	return toTeamMember(row), nil
}
