package model

type (
	PlanCode string

	Limit string
)

const (
	PlanFree PlanCode = "free"
)

const (
	LimitTeams          Limit = "teams"
	LimitMembers        Limit = "members"
	LimitSavedTemplates Limit = "saved_templates"
	LimitCardImageBytes Limit = "card_image_bytes"
)
