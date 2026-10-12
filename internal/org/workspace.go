package org

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"retrotide/internal/audit"
	"retrotide/internal/dao"
	"retrotide/internal/dto"
	"retrotide/internal/model/spec"
	"retrotide/internal/reserved"
	"retrotide/internal/team"
)

const (
	maxSlugLength    = 40
	slugSuffixLength = 6
	slugAttempts     = 5
)

var (
	slugFormat = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`)
)

func (s *service) CreateWorkspace(ctx context.Context, request WorkspaceRequest) (*dto.WorkspaceResponse, error) {
	name, err := cleanName(request.Name)
	if err != nil {
		return nil, err
	}

	teamName, err := team.CleanName(request.TeamName)
	if err != nil {
		return nil, err
	}

	allowed, err := s.ownedAllowance(ctx, request.ActorID)
	if err != nil {
		return nil, err
	}

	base := slugBase(name)
	for attempt := range slugAttempts {
		slug := s.slugCandidate(base, attempt)
		if !usableSlug(slug) {
			continue
		}

		created, err := s.organisations.CreateWorkspace(ctx, spec.NewWorkspace{OwnerID: request.ActorID, Slug: slug, Name: name, TeamName: teamName, MaxOwned: allowed})
		if errors.Is(err, dao.ErrConflict) {
			continue
		}
		if errors.Is(err, dao.ErrQuotaExceeded) {
			return nil, ErrOwnedLimitReached
		}
		if err != nil {
			return nil, fmt.Errorf("create the workspace %q for user %s: %w", name, request.ActorID, err)
		}

		response := &dto.WorkspaceResponse{
			Organisation: dto.NewOrganisationResponse(created.Organisation, created.Owner.Role),
			Team:         dto.NewTeamResponse(created.Team, created.Lead.Role),
		}

		orgCreated := orgEntry(request.Actor, created.Organisation.ID, audit.ActionOrgCreated, map[string]string{"name": created.Organisation.Name, "slug": created.Organisation.Slug})
		teamCreated := orgEntry(request.Actor, created.Organisation.ID, audit.ActionTeamCreated, map[string]string{"name": created.Team.Name})
		teamCreated.TargetType = audit.TargetTeam
		teamCreated.TargetID = created.Team.ID.String()

		return response, s.record(ctx, orgCreated, teamCreated)
	}

	return nil, fmt.Errorf("no free address for the workspace %q after %d attempts", name, slugAttempts)
}

func normaliseSlug(raw string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if !slugFormat.MatchString(slug) {
		return "", ErrInvalidSlug
	}

	if reserved.IsPathSegment(slug) {
		return "", ErrSlugReserved
	}

	return slug, nil
}

func usableSlug(slug string) bool {
	return slugFormat.MatchString(slug) && !reserved.IsPathSegment(slug)
}

func slugBase(name string) string {
	var built strings.Builder

	pendingHyphen := false
	for _, r := range strings.ToLower(name) {
		if !isSlugRune(r) {
			pendingHyphen = built.Len() > 0

			continue
		}

		if pendingHyphen {
			built.WriteByte('-')
			pendingHyphen = false
		}

		built.WriteRune(r)
	}

	base := built.String()
	if len(base) > maxSlugLength {
		base = strings.TrimRight(base[:maxSlugLength], "-")
	}

	return base
}

func isSlugRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func (s *service) slugCandidate(base string, attempt int) string {
	if attempt == 0 {
		return base
	}

	suffix := s.slugSuffix()

	stem := base[:min(len(base), maxSlugLength-slugSuffixLength-1)]
	stem = strings.TrimRight(stem, "-")
	if stem == "" {
		return suffix
	}

	return stem + "-" + suffix
}
