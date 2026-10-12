package dto

import (
	"time"

	"retrotide/internal/audit"
	"retrotide/internal/model"
	"retrotide/internal/role"

	"github.com/google/uuid"
)

type (
	AdminUserResponse struct {
		ID            uuid.UUID `json:"id"`
		Email         string    `json:"email"`
		DisplayName   string    `json:"display_name"`
		Role          role.Role `json:"role,omitempty"`
		EmailVerified bool      `json:"email_verified"`
		Banned        bool      `json:"banned"`
		Locked        bool      `json:"locked"`
		CreatedAt     string    `json:"created_at"`
	}

	AdminUserListResponse struct {
		Users  []AdminUserResponse `json:"users"`
		Total  int                 `json:"total"`
		Limit  int                 `json:"limit"`
		Offset int                 `json:"offset"`
	}

	AdminUserDetailResponse struct {
		AdminUserResponse

		HasPassword  bool       `json:"has_password"`
		BanReason    string     `json:"ban_reason,omitempty"`
		BannedAt     string     `json:"banned_at,omitempty"`
		BannedBy     *uuid.UUID `json:"banned_by,omitempty"`
		LockReason   string     `json:"lock_reason,omitempty"`
		LockedAt     string     `json:"locked_at,omitempty"`
		LockedBy     *uuid.UUID `json:"locked_by,omitempty"`
		LastSeenIP   string     `json:"last_seen_ip,omitempty"`
		LastSeenIPAt string     `json:"last_seen_ip_at,omitempty"`
	}

	AdminPasswordResetResponse struct {
		Password string `json:"password"`
	}

	SetRoleRequest struct {
		Role string `json:"role"`
	}

	BanUserRequest struct {
		Reason string `json:"reason"`
	}

	LockUserRequest struct {
		Reason string `json:"reason"`
	}

	AdminSetEmailRequest struct {
		Email string `json:"email"`
	}

	AuditLogEntryResponse struct {
		ID          int64             `json:"id"`
		ActorID     *uuid.UUID        `json:"actor_id,omitempty"`
		ActorKind   audit.ActorKind   `json:"actor_kind"`
		ActorName   string            `json:"actor_name,omitempty"`
		Action      audit.Action      `json:"action"`
		TargetType  audit.TargetType  `json:"target_type"`
		TargetID    string            `json:"target_id,omitempty"`
		SubjectID   *uuid.UUID        `json:"subject_id,omitempty"`
		SubjectName string            `json:"subject_name,omitempty"`
		Details     map[string]string `json:"details"`
		CreatedAt   string            `json:"created_at"`
	}

	AuditLogListResponse struct {
		Entries []AuditLogEntryResponse `json:"entries"`
		Total   int                     `json:"total"`
		Limit   int                     `json:"limit"`
		Offset  int                     `json:"offset"`
	}

	AdminSettingsResponse struct {
		Settings map[string]string `json:"settings"`
	}

	UpdateSettingsRequest struct {
		Settings map[string]string `json:"settings"`
	}

	PermissionCatalogueItem struct {
		Permission string `json:"permission"`
		Label      string `json:"label"`
	}

	RolePermissionsItem struct {
		Role        role.Role `json:"role"`
		Label       string    `json:"label"`
		Permissions []string  `json:"permissions"`
	}

	AdminPermissionsResponse struct {
		Permissions []PermissionCatalogueItem `json:"permissions"`
		Roles       []RolePermissionsItem     `json:"roles"`
	}

	UpdatePermissionsRequest struct {
		Permissions []string `json:"permissions"`
	}
)

func NewAdminUserResponse(u model.UserWithRole) AdminUserResponse {
	return AdminUserResponse{
		ID:            u.ID,
		Email:         u.Email,
		DisplayName:   u.DisplayName,
		Role:          u.Role,
		EmailVerified: u.EmailVerified,
		Banned:        u.BannedAt != nil,
		Locked:        u.LockedAt != nil,
		CreatedAt:     u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func NewAdminUserDetailResponse(u model.UserWithRole) AdminUserDetailResponse {
	return AdminUserDetailResponse{
		AdminUserResponse: NewAdminUserResponse(u),
		HasPassword:       u.HasPassword(),
		BanReason:         u.BanReason,
		BannedAt:          optionalTime(u.BannedAt),
		BannedBy:          u.BannedBy,
		LockReason:        u.LockReason,
		LockedAt:          optionalTime(u.LockedAt),
		LockedBy:          u.LockedBy,
		LastSeenIP:        optionalString(u.LastSeenIP),
		LastSeenIPAt:      optionalTime(u.LastSeenIPAt),
	}
}

func NewAuditLogEntryResponse(e audit.Entry) AuditLogEntryResponse {
	details := e.Details
	if details == nil {
		details = map[string]string{}
	}

	return AuditLogEntryResponse{
		ID:          e.ID,
		ActorID:     e.ActorID,
		ActorKind:   e.ActorKind,
		ActorName:   e.ActorName,
		Action:      e.Action,
		TargetType:  e.TargetType,
		TargetID:    e.TargetID,
		SubjectID:   e.SubjectID,
		SubjectName: e.SubjectName,
		Details:     details,
		CreatedAt:   e.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func optionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}

func optionalString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
