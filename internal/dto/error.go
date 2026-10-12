package dto

import (
	"retrotide/internal/model"
)

type (
	ErrorResponse struct {
		Error string `json:"error"`
		Code  string `json:"code,omitempty"`
	}

	UpgradeRequiredResponse struct {
		Error   string         `json:"error"`
		Code    string         `json:"code"`
		Limit   model.Limit    `json:"limit"`
		Allowed int            `json:"allowed"`
		Current int            `json:"current"`
		Plan    model.PlanCode `json:"plan"`
	}
)
