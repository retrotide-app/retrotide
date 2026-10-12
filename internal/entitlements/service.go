package entitlements

import (
	"context"
	"fmt"

	"retrotide/internal/model"
	"retrotide/internal/model/spec"

	"github.com/google/uuid"
)

type (
	Service interface {
		For(ctx context.Context, orgID uuid.UUID) (*Plan, error)
		CheckWithin(ctx context.Context, check spec.LimitCheck) error
	}

	service struct {
		fallback model.PlanCode
	}
)

func NewService() Service {
	return &service{fallback: model.PlanFree}
}

func (s *service) For(_ context.Context, orgID uuid.UUID) (*Plan, error) {
	plan, known := PlanFor(s.fallback)
	if !known {
		return nil, fmt.Errorf("%w: %q for organisation %s", ErrUnknownPlan, s.fallback, orgID)
	}

	return plan, nil
}

func (s *service) CheckWithin(ctx context.Context, check spec.LimitCheck) error {
	plan, err := s.For(ctx, check.OrgID)
	if err != nil {
		return err
	}

	allowed, known := plan.Allowance(check.Limit)
	if !known {
		return fmt.Errorf("%w: %q on the %s plan", ErrUnknownLimit, check.Limit, plan.Code)
	}

	if check.Current < allowed {
		return nil
	}

	return &DeniedError{Plan: plan.Code, Limit: check.Limit, Allowed: allowed, Current: check.Current}
}
