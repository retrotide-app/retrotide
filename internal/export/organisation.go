package export

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"retrotide/internal/board/live"
	"retrotide/internal/dto"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	organisationBoards struct {
		service   *service
		principal model.Principal
		orgID     uuid.UUID
		headers   []model.BoardHeader
		teams     map[uuid.UUID]string
		at        int
		pending   *dto.OrganisationExportBoard
	}
)

const (
	jsonIndent  = "  "
	boardIndent = jsonIndent + jsonIndent
)

func newOrganisationBoards(s *service, viewer spec.OrganisationMemberKey, headers []model.BoardHeader, teams []model.Team) *organisationBoards {
	names := make(map[uuid.UUID]string, len(teams))
	for _, team := range teams {
		names[team.ID] = team.Name
	}

	return &organisationBoards{
		service:   s,
		principal: model.Principal{Kind: model.PrincipalUser, ID: viewer.UserID},
		orgID:     viewer.OrgID,
		headers:   headers,
		teams:     names,
	}
}

func (b *organisationBoards) load(ctx context.Context) error {
	b.pending = nil

	for b.at < len(b.headers) {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("stop the export of organisation %s after %d of %d boards: %w", b.orgID, b.at, len(b.headers), err)
		}

		header := b.headers[b.at]
		b.at++

		if header.OrgID != b.orgID {
			continue
		}

		board, err := b.exportable(ctx, header)
		if err != nil {
			return err
		}

		if board == nil {
			continue
		}

		entry := dto.OrganisationExportBoard{Archived: header.Archived(), Board: *board}
		if header.TeamID != nil {
			entry.Team = b.teams[*header.TeamID]
		}

		b.pending = &entry

		return nil
	}

	return nil
}

func (b *organisationBoards) exportable(ctx context.Context, header model.BoardHeader) (*dto.BoardExport, error) {
	access, err := b.service.live.Resolve(ctx, spec.BoardAccessQuery{Principal: b.principal, BoardID: header.ID})
	if errors.Is(err, live.ErrNoAccess) || errors.Is(err, live.ErrRemoved) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve access to board %s for the export of organisation %s: %w", header.ID, b.orgID, err)
	}

	if !slices.Contains(access.Capabilities, model.BoardCapabilityExport) {
		return nil, nil
	}

	board, err := b.service.project(ctx, *access)
	if errors.Is(err, live.ErrNoAccess) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return board, nil
}

func (b *organisationBoards) write(ctx context.Context, w io.Writer, header dto.OrganisationExportHeader) error {
	organisation, err := indentedJSON(header, jsonIndent)
	if err != nil {
		return fmt.Errorf("encode the organisation of the export: %w", err)
	}

	if _, err := fmt.Fprintf(w, "{\n%s\"organisation\": %s,\n%s\"boards\": [", jsonIndent, organisation, jsonIndent); err != nil {
		return fmt.Errorf("write the opening of the organisation export: %w", err)
	}

	written := 0
	for b.pending != nil {
		entry, err := indentedJSON(b.pending, boardIndent)
		if err != nil {
			return fmt.Errorf("encode board %d of the organisation export: %w", written+1, err)
		}

		separator := ",\n"
		if written == 0 {
			separator = "\n"
		}

		if _, err := fmt.Fprintf(w, "%s%s%s", separator, boardIndent, entry); err != nil {
			return fmt.Errorf("write board %d of the organisation export: %w", written+1, err)
		}

		written++

		if err := b.load(ctx); err != nil {
			return err
		}
	}

	closing := "]\n}\n"
	if written > 0 {
		closing = "\n" + jsonIndent + closing
	}

	if _, err := io.WriteString(w, closing); err != nil {
		return fmt.Errorf("write the end of the organisation export: %w", err)
	}

	return nil
}

func indentedJSON(value any, prefix string) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetIndent(prefix, jsonIndent)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}
