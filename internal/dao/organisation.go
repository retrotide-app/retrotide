package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	OrganisationDAO interface {
		Create(ctx context.Context, s spec.NewOrganisation, tx ...*sql.Tx) (*model.Organisation, error)
		GetByID(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (*model.Organisation, error)
		Lock(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) error
		Update(ctx context.Context, s spec.OrganisationUpdate, tx ...*sql.Tx) (*model.Organisation, error)
		SoftDelete(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (*model.Organisation, error)
		Restore(ctx context.Context, s spec.OrganisationRestore, tx ...*sql.Tx) (*model.Organisation, error)
		PurgeDeletedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error)
		ListForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.UserOrganisation, error)
		CountOwnedByUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int, error)
		ListOwnedWithOtherMembers(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.Organisation, error)
		LockAllOfMember(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]uuid.UUID, error)
		SoftDeleteOwnedAlone(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int64, error)
		ListForStaff(ctx context.Context, s spec.OrganisationListing, tx ...*sql.Tx) ([]model.StaffOrganisation, error)
		CountForStaff(ctx context.Context, search string, tx ...*sql.Tx) (int, error)
		GetForStaff(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (*model.StaffOrganisation, error)

		AddMember(ctx context.Context, s spec.NewOrganisationMember, tx ...*sql.Tx) (*model.OrganisationMember, error)
		GetMember(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) (*model.OrganisationMember, error)
		ListMembers(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.OrganisationMemberProfile, error)
		CountMembers(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (int, error)
		SetMemberRole(ctx context.Context, s spec.OrganisationMemberRoleChange, tx ...*sql.Tx) (*model.OrganisationMember, error)
		RemoveMember(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) (*model.OrganisationMember, error)
		TransferOwnership(ctx context.Context, s spec.OwnershipTransfer, tx ...*sql.Tx) error

		CreateInvite(ctx context.Context, s spec.NewOrganisationInvite, tx ...*sql.Tx) (*model.OrganisationInvite, error)
		GetInvite(ctx context.Context, s spec.OrganisationInviteKey, tx ...*sql.Tx) (*model.OrganisationInvite, error)
		ListInvites(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.OrganisationInvite, error)
		RevokeInvite(ctx context.Context, s spec.OrganisationInviteKey, tx ...*sql.Tx) (*model.OrganisationInvite, error)
		GetLiveInviteByTokenHash(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.InviteDetails, error)
		ConsumeInvite(ctx context.Context, s spec.InviteConsumption, tx ...*sql.Tx) (*model.OrganisationInvite, error)
		DeleteExpiredInvites(ctx context.Context, tx ...*sql.Tx) (int64, error)
	}

	organisationDAO struct {
		db *sql.DB
	}
)

func toOrganisation(row sqlcgen.Organisation) *model.Organisation {
	return &model.Organisation{
		ID:          row.ID,
		Slug:        row.Slug,
		Name:        row.Name,
		AllowGuests: row.AllowGuests,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   nullableTime(row.DeletedAt),
	}
}

func toOrganisationMember(row sqlcgen.OrganisationMember) *model.OrganisationMember {
	return &model.OrganisationMember{
		OrgID:    row.OrgID,
		UserID:   row.UserID,
		Role:     model.OrgRole(row.Role),
		JoinedAt: row.JoinedAt,
	}
}

func toOrganisationInvite(row sqlcgen.OrganisationInvite) *model.OrganisationInvite {
	return &model.OrganisationInvite{
		ID:        row.ID,
		OrgID:     row.OrgID,
		TeamID:    row.TeamID,
		TokenHash: row.TokenHash,
		Email:     row.Email,
		Role:      model.OrgRole(row.Role),
		MaxUses:   nullableInt(row.MaxUses),
		UseCount:  int(row.UseCount),
		InvitedBy: row.InvitedBy,
		ExpiresAt: row.ExpiresAt,
		RevokedAt: nullableTime(row.RevokedAt),
		CreatedAt: row.CreatedAt,
	}
}

func (r *organisationDAO) Create(ctx context.Context, s spec.NewOrganisation, tx ...*sql.Tx) (*model.Organisation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateOrganisationWithinOwnerQuota(ctx, sqlcgen.CreateOrganisationWithinOwnerQuotaParams{
		Slug:      s.Slug,
		Name:      s.Name,
		CreatedBy: s.CreatedBy,
		MaxOwned:  clampInt32(s.MaxOwned),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("create organisation %q for user %s: %w", s.Slug, s.CreatedBy, ErrQuotaExceeded)
	}
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create organisation %q: %w: %w", s.Slug, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("create organisation %q for user %s: %w: %w", s.Slug, s.CreatedBy, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create organisation %q: %w", s.Slug, err)
	}

	return toOrganisation(row), nil
}

func (r *organisationDAO) GetByID(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (*model.Organisation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetOrganisation(ctx, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get organisation %s: %w", orgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get organisation %s: %w", orgID, err)
	}

	return toOrganisation(row), nil
}

func (r *organisationDAO) Lock(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) error {
	_, err := sqlcgen.New(db.TxOrDB(r.db, tx)).LockOrganisation(ctx, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lock organisation %s: %w", orgID, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("lock organisation %s: %w", orgID, err)
	}

	return nil
}

func (r *organisationDAO) Update(ctx context.Context, s spec.OrganisationUpdate, tx ...*sql.Tx) (*model.Organisation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).UpdateOrganisation(ctx, sqlcgen.UpdateOrganisationParams{
		ID:          s.OrgID,
		Slug:        s.Slug,
		Name:        s.Name,
		AllowGuests: s.AllowGuests,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("update organisation %s: %w", s.OrgID, ErrNotFound)
	}
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("update organisation %s to slug %q: %w: %w", s.OrgID, s.Slug, ErrConflict, err)
	}
	if err != nil {
		return nil, fmt.Errorf("update organisation %s: %w", s.OrgID, err)
	}

	return toOrganisation(row), nil
}

func (r *organisationDAO) SoftDelete(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (*model.Organisation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SoftDeleteOrganisation(ctx, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("delete organisation %s: %w", orgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("delete organisation %s: %w", orgID, err)
	}

	return toOrganisation(row), nil
}

func (r *organisationDAO) Restore(ctx context.Context, s spec.OrganisationRestore, tx ...*sql.Tx) (*model.Organisation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RestoreOrganisationWithinOwnerQuota(ctx, sqlcgen.RestoreOrganisationWithinOwnerQuotaParams{
		ID:           s.OrgID,
		DeletedAfter: s.DeletedAfter,
		OwnerID:      s.OwnerID,
		MaxOwned:     clampInt32(s.MaxOwned),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("restore organisation %s within the allowance of owner %s: %w", s.OrgID, s.OwnerID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("restore organisation %s: %w", s.OrgID, err)
	}

	return toOrganisation(row), nil
}

func (r *organisationDAO) PurgeDeletedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error) {
	purged, err := sqlcgen.New(db.TxOrDB(r.db, tx)).PurgeOrganisationsDeletedBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge organisations deleted before %s: %w", cutoff.Format(time.RFC3339), err)
	}

	return purged, nil
}

func (r *organisationDAO) ListForUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.UserOrganisation, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListOrganisationsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list organisations of user %s: %w", userID, err)
	}

	organisations := make([]model.UserOrganisation, 0, len(rows))
	for _, row := range rows {
		organisations = append(organisations, model.UserOrganisation{
			Organisation: *toOrganisation(row.Organisation),
			Role:         model.OrgRole(row.MemberRole),
			JoinedAt:     row.MemberJoinedAt,
		})
	}

	return organisations, nil
}

func (r *organisationDAO) CountOwnedByUser(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountOrganisationsOwnedByUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count organisations owned by user %s: %w", userID, err)
	}

	return int(count), nil
}

func (r *organisationDAO) ListOwnedWithOtherMembers(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]model.Organisation, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListOrganisationsOwnedWithOtherMembers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list organisations user %s owns beside other members: %w", userID, err)
	}

	organisations := make([]model.Organisation, 0, len(rows))
	for _, row := range rows {
		organisations = append(organisations, *toOrganisation(row))
	}

	return organisations, nil
}

func (r *organisationDAO) LockAllOfMember(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) ([]uuid.UUID, error) {
	locked, err := sqlcgen.New(db.TxOrDB(r.db, tx)).LockOrganisationsOfMember(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("lock the organisations of user %s: %w", userID, err)
	}

	return locked, nil
}

func (r *organisationDAO) SoftDeleteOwnedAlone(ctx context.Context, userID uuid.UUID, tx ...*sql.Tx) (int64, error) {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SoftDeleteOrganisationsOwnedAlone(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("delete the organisations user %s owns alone: %w", userID, err)
	}

	return deleted, nil
}

func (r *organisationDAO) ListForStaff(ctx context.Context, s spec.OrganisationListing, tx ...*sql.Tx) ([]model.StaffOrganisation, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListOrganisationsForStaff(ctx, sqlcgen.ListOrganisationsForStaffParams{
		Pattern:    containsPattern(s.Search),
		PageLimit:  clampInt32(s.Limit),
		PageOffset: clampInt32(s.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list organisations for staff: %w", err)
	}

	listed := make([]model.StaffOrganisation, 0, len(rows))
	for _, row := range rows {
		listed = append(listed, toStaffOrganisation(row))
	}

	return listed, nil
}

func (r *organisationDAO) CountForStaff(ctx context.Context, search string, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountOrganisationsForStaff(ctx, containsPattern(search))
	if err != nil {
		return 0, fmt.Errorf("count organisations for staff: %w", err)
	}

	return int(count), nil
}

func (r *organisationDAO) GetForStaff(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (*model.StaffOrganisation, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetOrganisationForStaff(ctx, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get organisation %s for staff: %w", orgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get organisation %s for staff: %w", orgID, err)
	}

	return new(toStaffOrganisation(sqlcgen.ListOrganisationsForStaffRow(row))), nil
}

func toStaffOrganisation(row sqlcgen.ListOrganisationsForStaffRow) model.StaffOrganisation {
	return model.StaffOrganisation{
		Organisation:     *toOrganisation(row.Organisation),
		OwnerID:          row.OwnerID,
		OwnerDisplayName: nullableString(row.OwnerDisplayName),
		OwnerEmail:       row.OwnerEmail,
		MemberCount:      int(row.MemberCount),
		TeamCount:        int(row.TeamCount),
		BoardCount:       int(row.BoardCount),
	}
}

func (r *organisationDAO) AddMember(ctx context.Context, s spec.NewOrganisationMember, tx ...*sql.Tx) (*model.OrganisationMember, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).AddOrganisationMemberWithinQuota(ctx, sqlcgen.AddOrganisationMemberWithinQuotaParams{
		OrgID:      s.OrgID,
		UserID:     s.UserID,
		Role:       string(s.Role),
		MaxMembers: clampInt32(s.MaxMembers),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("add user %s to organisation %s: %w", s.UserID, s.OrgID, ErrQuotaExceeded)
	}
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("add user %s to organisation %s as %s: %w: %w", s.UserID, s.OrgID, s.Role, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("add user %s to organisation %s: %w: %w", s.UserID, s.OrgID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("add user %s to organisation %s: %w", s.UserID, s.OrgID, err)
	}

	return toOrganisationMember(row), nil
}

func (r *organisationDAO) GetMember(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) (*model.OrganisationMember, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetOrganisationMember(ctx, sqlcgen.GetOrganisationMemberParams{
		OrgID:  s.OrgID,
		UserID: s.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get member %s of organisation %s: %w", s.UserID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get member %s of organisation %s: %w", s.UserID, s.OrgID, err)
	}

	return toOrganisationMember(row), nil
}

func (r *organisationDAO) ListMembers(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.OrganisationMemberProfile, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListOrganisationMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list members of organisation %s: %w", orgID, err)
	}

	members := make([]model.OrganisationMemberProfile, 0, len(rows))
	for _, row := range rows {
		members = append(members, model.OrganisationMemberProfile{
			OrganisationMember: *toOrganisationMember(row.OrganisationMember),
			Email:              row.Email,
			DisplayName:        row.DisplayName,
			SiteRole:           role.Role(row.SiteRole),
		})
	}

	return members, nil
}

func (r *organisationDAO) CountMembers(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountOrganisationMembers(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("count members of organisation %s: %w", orgID, err)
	}

	return int(count), nil
}

func (r *organisationDAO) SetMemberRole(ctx context.Context, s spec.OrganisationMemberRoleChange, tx ...*sql.Tx) (*model.OrganisationMember, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).SetOrganisationMemberRole(ctx, sqlcgen.SetOrganisationMemberRoleParams{
		OrgID:  s.OrgID,
		UserID: s.UserID,
		Role:   string(s.Role),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("make member %s of organisation %s %s: %w", s.UserID, s.OrgID, s.Role, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("make member %s of organisation %s %s: %w", s.UserID, s.OrgID, s.Role, err)
	}

	return toOrganisationMember(row), nil
}

func (r *organisationDAO) RemoveMember(ctx context.Context, s spec.OrganisationMemberKey, tx ...*sql.Tx) (*model.OrganisationMember, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RemoveOrganisationMember(ctx, sqlcgen.RemoveOrganisationMemberParams{
		OrgID:  s.OrgID,
		UserID: s.UserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("remove member %s from organisation %s: %w", s.UserID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("remove member %s from organisation %s: %w", s.UserID, s.OrgID, err)
	}

	return toOrganisationMember(row), nil
}

func (r *organisationDAO) TransferOwnership(ctx context.Context, s spec.OwnershipTransfer, tx ...*sql.Tx) error {
	err := db.WithTx(ctx, r.db, tx, func(tx *sql.Tx) error {
		queries := sqlcgen.New(tx)

		demoted, err := queries.DemoteOrganisationOwner(ctx, sqlcgen.DemoteOrganisationOwnerParams{OrgID: s.OrgID, UserID: s.FromUserID})
		if err != nil {
			return fmt.Errorf("demote owner %s: %w", s.FromUserID, err)
		}

		if demoted == 0 {
			return fmt.Errorf("demote owner %s: %w", s.FromUserID, ErrNotFound)
		}

		promoted, err := queries.PromoteOrganisationOwnerWithinOwnerQuota(ctx, sqlcgen.PromoteOrganisationOwnerWithinOwnerQuotaParams{
			OrgID:    s.OrgID,
			UserID:   s.ToUserID,
			MaxOwned: clampInt32(s.MaxOwned),
		})
		if err != nil {
			return fmt.Errorf("promote member %s: %w", s.ToUserID, err)
		}

		if promoted == 0 {
			return unpromoted(ctx, queries, s)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("transfer organisation %s from %s to %s: %w", s.OrgID, s.FromUserID, s.ToUserID, err)
	}

	return nil
}

func unpromoted(ctx context.Context, queries *sqlcgen.Queries, s spec.OwnershipTransfer) error {
	_, err := queries.GetOrganisationMember(ctx, sqlcgen.GetOrganisationMemberParams{OrgID: s.OrgID, UserID: s.ToUserID})
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("promote member %s: %w", s.ToUserID, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("read member %s after the promotion matched nothing: %w", s.ToUserID, err)
	}

	return fmt.Errorf("new owner %s already holds %d organisations: %w", s.ToUserID, s.MaxOwned, ErrQuotaExceeded)
}

func (r *organisationDAO) CreateInvite(ctx context.Context, s spec.NewOrganisationInvite, tx ...*sql.Tx) (*model.OrganisationInvite, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CreateOrganisationInvite(ctx, sqlcgen.CreateOrganisationInviteParams{
		OrgID:     s.OrgID,
		TeamID:    s.TeamID,
		TokenHash: s.TokenHash,
		Email:     s.Email,
		Role:      string(s.Role),
		MaxUses:   toNullInt32(s.MaxUses),
		InvitedBy: nullableUUID(s.InvitedBy),
		ExpiresAt: s.ExpiresAt,
	})
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("create invite to organisation %s: %w: %w", s.OrgID, ErrConflict, err)
	}
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("create invite to organisation %s: %w: %w", s.OrgID, ErrNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("create invite to organisation %s: %w", s.OrgID, err)
	}

	return toOrganisationInvite(row), nil
}

func (r *organisationDAO) GetInvite(ctx context.Context, s spec.OrganisationInviteKey, tx ...*sql.Tx) (*model.OrganisationInvite, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetOrganisationInvite(ctx, sqlcgen.GetOrganisationInviteParams{
		OrgID: s.OrgID,
		ID:    s.InviteID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get invite %s of organisation %s: %w", s.InviteID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get invite %s of organisation %s: %w", s.InviteID, s.OrgID, err)
	}

	return toOrganisationInvite(row), nil
}

func (r *organisationDAO) ListInvites(ctx context.Context, orgID uuid.UUID, tx ...*sql.Tx) ([]model.OrganisationInvite, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListOrganisationInvites(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list invites of organisation %s: %w", orgID, err)
	}

	invites := make([]model.OrganisationInvite, 0, len(rows))
	for _, row := range rows {
		invites = append(invites, *toOrganisationInvite(row))
	}

	return invites, nil
}

func (r *organisationDAO) RevokeInvite(ctx context.Context, s spec.OrganisationInviteKey, tx ...*sql.Tx) (*model.OrganisationInvite, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).RevokeOrganisationInvite(ctx, sqlcgen.RevokeOrganisationInviteParams{
		OrgID: s.OrgID,
		ID:    s.InviteID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("revoke invite %s of organisation %s: %w", s.InviteID, s.OrgID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("revoke invite %s of organisation %s: %w", s.InviteID, s.OrgID, err)
	}

	return toOrganisationInvite(row), nil
}

func (r *organisationDAO) GetLiveInviteByTokenHash(ctx context.Context, tokenHash string, tx ...*sql.Tx) (*model.InviteDetails, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).GetLiveOrganisationInviteByTokenHash(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get invite by token: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get invite by token: %w", err)
	}

	return &model.InviteDetails{
		OrganisationInvite: *toOrganisationInvite(row.OrganisationInvite),
		OrgName:            row.OrgName,
		OrgSlug:            row.OrgSlug,
		TeamName:           row.TeamName,
		InviterName:        row.InviterName,
	}, nil
}

func (r *organisationDAO) ConsumeInvite(ctx context.Context, s spec.InviteConsumption, tx ...*sql.Tx) (*model.OrganisationInvite, error) {
	row, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ConsumeOrganisationInvite(ctx, sqlcgen.ConsumeOrganisationInviteParams{
		TokenHash: s.TokenHash,
		Email:     s.Email,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("consume invite: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("consume invite: %w", err)
	}

	return toOrganisationInvite(row), nil
}

func (r *organisationDAO) DeleteExpiredInvites(ctx context.Context, tx ...*sql.Tx) (int64, error) {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteExpiredOrganisationInvites(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired invites: %w", err)
	}

	return deleted, nil
}
