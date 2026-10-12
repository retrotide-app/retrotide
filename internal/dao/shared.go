package dao

import (
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
	checkViolation      = "23514"
)

var (
	likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
)

func nullableUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}

	return new(id)
}

func nullableTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}

	return new(value.Time)
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}

	return new(value.String)
}

func toNullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}

	return sql.NullString{String: *value, Valid: true}
}

func nullableInt(value sql.NullInt32) *int {
	if !value.Valid {
		return nil
	}

	return new(int(value.Int32))
}

func toNullInt32(value *int) sql.NullInt32 {
	if value == nil {
		return sql.NullInt32{}
	}

	return sql.NullInt32{Int32: clampInt32(*value), Valid: true}
}

func toNullBool(value *bool) sql.NullBool {
	if value == nil {
		return sql.NullBool{}
	}

	return sql.NullBool{Bool: *value, Valid: true}
}

func toNullInt16(value *int) sql.NullInt16 {
	if value == nil {
		return sql.NullInt16{}
	}

	clamped := min(max(*value, 0), math.MaxInt16)

	return sql.NullInt16{Int16: int16(clamped), Valid: true}
}

func clampInt32(value int) int32 {
	clamped := min(max(value, 0), math.MaxInt32)

	return int32(clamped)
}

func textOrNull(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}

	return sql.NullString{String: value, Valid: true}
}

func isUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)

	return ok && pgErr.Code == uniqueViolation
}

func isForeignKeyViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)

	return ok && pgErr.Code == foreignKeyViolation
}

func isCheckViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)

	return ok && pgErr.Code == checkViolation
}

func toNullTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}

	return sql.NullTime{Time: *value, Valid: true}
}

func containsPattern(search string) string {
	if search == "" {
		return ""
	}

	return "%" + likeEscaper.Replace(search) + "%"
}
