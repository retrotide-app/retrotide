package dao

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"fmt"
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/dao/sqlcgen"
	"retrotide/internal/db"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	AuditLogDAO interface {
		Create(ctx context.Context, entry audit.NewEntry, tx ...*sql.Tx) error
		ListSite(ctx context.Context, s spec.AuditLogListing, tx ...*sql.Tx) ([]audit.Entry, error)
		CountSite(ctx context.Context, action audit.Action, tx ...*sql.Tx) (int, error)
		ListForSubject(ctx context.Context, s spec.AuditLogSubjectListing, tx ...*sql.Tx) ([]audit.Entry, error)
		CountForSubject(ctx context.Context, subjectID uuid.UUID, tx ...*sql.Tx) (int, error)
		ClearIPsBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error)
		DeleteCreatedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error)
	}

	auditLogDAO struct {
		db *sql.DB
	}
)

func toAuditEntries(rows []sqlcgen.ListSiteAuditLogRow) ([]audit.Entry, error) {
	entries := make([]audit.Entry, 0, len(rows))
	for _, row := range rows {
		var details map[string]string
		if err := json.Unmarshal(row.Details, &details); err != nil {
			return nil, fmt.Errorf("decode the details of audit entry %d: %w", row.ID, err)
		}

		entries = append(entries, audit.Entry{
			ID:          row.ID,
			OrgID:       row.OrgID,
			ActorID:     row.ActorID,
			ActorKind:   audit.ActorKind(row.ActorKind),
			ActorName:   row.ActorName,
			Action:      audit.Action(row.Action),
			TargetType:  audit.TargetType(row.TargetType),
			TargetID:    row.TargetID,
			SubjectID:   row.SubjectID,
			SubjectName: row.SubjectName,
			Details:     details,
			IP:          nullableString(row.Ip),
			CreatedAt:   row.CreatedAt,
		})
	}

	return entries, nil
}

func (r *auditLogDAO) Create(ctx context.Context, entry audit.NewEntry, tx ...*sql.Tx) error {
	details, err := json.Marshal(entry.Details)
	if err != nil {
		return fmt.Errorf("encode the details of a %s audit entry: %w", entry.Action, err)
	}

	err = sqlcgen.New(db.TxOrDB(r.db, tx)).CreateAuditLogEntry(ctx, sqlcgen.CreateAuditLogEntryParams{
		OrgID:      nullableUUID(entry.OrgID),
		ActorID:    nullableUUID(entry.ActorID),
		ActorKind:  string(entry.ActorKind),
		Action:     string(entry.Action),
		TargetType: string(entry.TargetType),
		TargetID:   entry.TargetID,
		SubjectID:  nullableUUID(entry.SubjectID),
		Details:    details,
		Ip:         sql.NullString{String: entry.IP, Valid: entry.IP != ""},
	})
	if err != nil {
		return fmt.Errorf("create a %s audit entry: %w", entry.Action, err)
	}

	return nil
}

func (r *auditLogDAO) ListSite(ctx context.Context, s spec.AuditLogListing, tx ...*sql.Tx) ([]audit.Entry, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListSiteAuditLog(ctx, sqlcgen.ListSiteAuditLogParams{
		Action:     string(s.Action),
		PageLimit:  int32(s.Limit),
		PageOffset: int32(s.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list the site audit log: %w", err)
	}

	return toAuditEntries(rows)
}

func (r *auditLogDAO) CountSite(ctx context.Context, action audit.Action, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountSiteAuditLog(ctx, string(action))
	if err != nil {
		return 0, fmt.Errorf("count the site audit log: %w", err)
	}

	return int(count), nil
}

func (r *auditLogDAO) ListForSubject(ctx context.Context, s spec.AuditLogSubjectListing, tx ...*sql.Tx) ([]audit.Entry, error) {
	rows, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ListSiteAuditLogForSubject(ctx, sqlcgen.ListSiteAuditLogForSubjectParams{
		SubjectID:  new(s.SubjectID),
		PageLimit:  int32(s.Limit),
		PageOffset: int32(s.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list the audit log for user %s: %w", s.SubjectID, err)
	}

	converted := make([]sqlcgen.ListSiteAuditLogRow, 0, len(rows))
	for _, row := range rows {
		converted = append(converted, sqlcgen.ListSiteAuditLogRow(row))
	}

	return toAuditEntries(converted)
}

func (r *auditLogDAO) CountForSubject(ctx context.Context, subjectID uuid.UUID, tx ...*sql.Tx) (int, error) {
	count, err := sqlcgen.New(db.TxOrDB(r.db, tx)).CountSiteAuditLogForSubject(ctx, new(subjectID))
	if err != nil {
		return 0, fmt.Errorf("count the audit log for user %s: %w", subjectID, err)
	}

	return int(count), nil
}

func (r *auditLogDAO) ClearIPsBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error) {
	cleared, err := sqlcgen.New(db.TxOrDB(r.db, tx)).ClearAuditLogIPsBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("clear audit log ips before %s: %w", cutoff.Format(time.RFC3339), err)
	}

	return cleared, nil
}

func (r *auditLogDAO) DeleteCreatedBefore(ctx context.Context, cutoff time.Time, tx ...*sql.Tx) (int64, error) {
	deleted, err := sqlcgen.New(db.TxOrDB(r.db, tx)).DeleteAuditLogCreatedBefore(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete audit log entries created before %s: %w", cutoff.Format(time.RFC3339), err)
	}

	return deleted, nil
}
