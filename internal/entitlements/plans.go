package entitlements

import (
	"maps"

	"retrotide/internal/model"
)

type (
	Plan struct {
		Code   model.PlanCode
		Limits map[model.Limit]int
	}
)

var (
	planCatalogue = map[model.PlanCode]Plan{
		model.PlanFree: {
			Code: model.PlanFree,
			Limits: map[model.Limit]int{
				model.LimitTeams:          1,
				model.LimitMembers:        10,
				model.LimitSavedTemplates: 3,
				model.LimitCardImageBytes: 5242880,
				model.LimitMediaBytes:     1073741824,
			},
		},
	}
)

func PlanFor(code model.PlanCode) (*Plan, bool) {
	plan, ok := planCatalogue[code]
	if !ok {
		return nil, false
	}

	return &Plan{Code: plan.Code, Limits: maps.Clone(plan.Limits)}, true
}

func (p *Plan) Allowance(limit model.Limit) (int, bool) {
	allowed, ok := p.Limits[limit]

	return allowed, ok
}
