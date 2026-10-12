package admin

import (
	"context"
	"fmt"

	"retrotide/internal/audit"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
)

func (s *service) GetAuditLog(ctx context.Context, listing spec.AuditLogListing) (*dto.AuditLogListResponse, error) {
	limit, offset := clampPage(listing.Limit, listing.Offset, defaultAuditPageSize)
	page := spec.AuditLogListing{Action: listing.Action, Limit: limit, Offset: offset}

	entries, err := s.auditLog.ListSite(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("list the audit log: %w", err)
	}

	total, err := s.auditLog.CountSite(ctx, page.Action)
	if err != nil {
		return nil, fmt.Errorf("count the audit log: %w", err)
	}

	return &dto.AuditLogListResponse{Entries: toAuditLogEntries(entries), Total: total, Limit: limit, Offset: offset}, nil
}

func (s *service) GetUserAuditLog(ctx context.Context, listing spec.AuditLogSubjectListing) (*dto.AuditLogListResponse, error) {
	limit, offset := clampPage(listing.Limit, listing.Offset, defaultAuditPageSize)
	page := spec.AuditLogSubjectListing{SubjectID: listing.SubjectID, Limit: limit, Offset: offset}

	entries, err := s.auditLog.ListForSubject(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("list the audit log for user %s: %w", listing.SubjectID, err)
	}

	total, err := s.auditLog.CountForSubject(ctx, page.SubjectID)
	if err != nil {
		return nil, fmt.Errorf("count the audit log for user %s: %w", listing.SubjectID, err)
	}

	return &dto.AuditLogListResponse{Entries: toAuditLogEntries(entries), Total: total, Limit: limit, Offset: offset}, nil
}

func toAuditLogEntries(entries []audit.Entry) []dto.AuditLogEntryResponse {
	converted := make([]dto.AuditLogEntryResponse, 0, len(entries))
	for _, entry := range entries {
		converted = append(converted, dto.NewAuditLogEntryResponse(entry))
	}

	return converted
}
