package dto

import (
	"time"

	"retrotide/internal/config"
	"retrotide/internal/model"

	"github.com/google/uuid"
)

type (
	AdminStatTotal struct {
		Total     int `json:"total"`
		Last7Days int `json:"last_7_days"`
	}

	AdminStatTotals struct {
		Users         AdminStatTotal `json:"users"`
		VerifiedUsers AdminStatTotal `json:"verified_users"`
		Organisations AdminStatTotal `json:"organisations"`
		Teams         AdminStatTotal `json:"teams"`
		Boards        AdminStatTotal `json:"boards"`
		Cards         AdminStatTotal `json:"cards"`
	}

	AdminStatDay struct {
		Date   string `json:"date"`
		Users  int    `json:"users"`
		Boards int    `json:"boards"`
		Cards  int    `json:"cards"`
	}

	AdminActivityCounts struct {
		Users         int `json:"users"`
		Organisations int `json:"organisations"`
		Boards        int `json:"boards"`
		Cards         int `json:"cards"`
	}

	AdminActivity struct {
		Last24Hours AdminActivityCounts `json:"last_24_hours"`
		Last7Days   AdminActivityCounts `json:"last_7_days"`
		Last30Days  AdminActivityCounts `json:"last_30_days"`
	}

	AdminRecentSignup struct {
		ID            uuid.UUID `json:"id"`
		DisplayName   string    `json:"display_name"`
		Email         string    `json:"email"`
		EmailVerified bool      `json:"email_verified"`
		CreatedAt     string    `json:"created_at"`
	}

	AdminSiteHealth struct {
		SiteMode               config.SiteMode `json:"site_mode"`
		MaintenanceMode        bool            `json:"maintenance_mode"`
		EmailConfigured        bool            `json:"email_configured"`
		EmailStatus            string          `json:"email_status"`
		GoogleSignInConfigured bool            `json:"google_sign_in_configured"`
	}

	AdminStatsResponse struct {
		GeneratedAt   string               `json:"generated_at"`
		Totals        AdminStatTotals      `json:"totals"`
		Daily         []AdminStatDay       `json:"daily"`
		Activity      AdminActivity        `json:"activity"`
		RecentSignups *[]AdminRecentSignup `json:"recent_signups,omitempty"`
		Health        AdminSiteHealth      `json:"health"`
	}
)

func NewAdminStatTotals(t model.SiteTotals) AdminStatTotals {
	return AdminStatTotals{
		Users:         AdminStatTotal{Total: t.Users, Last7Days: t.UsersAdded},
		VerifiedUsers: AdminStatTotal{Total: t.VerifiedUsers, Last7Days: t.VerifiedUsersAdded},
		Organisations: AdminStatTotal{Total: t.Organisations, Last7Days: t.OrganisationsAdded},
		Teams:         AdminStatTotal{Total: t.Teams, Last7Days: t.TeamsAdded},
		Boards:        AdminStatTotal{Total: t.Boards, Last7Days: t.BoardsAdded},
		Cards:         AdminStatTotal{Total: t.Cards, Last7Days: t.CardsAdded},
	}
}

func NewAdminStatDays(days []model.SiteDay) []AdminStatDay {
	converted := make([]AdminStatDay, 0, len(days))
	for _, day := range days {
		converted = append(converted, AdminStatDay{Date: day.Day.UTC().Format(time.DateOnly), Users: day.Users, Boards: day.Boards, Cards: day.Cards})
	}

	return converted
}

func NewAdminActivity(a model.SiteActivity) AdminActivity {
	return AdminActivity{
		Last24Hours: AdminActivityCounts(a.Day),
		Last7Days:   AdminActivityCounts(a.Week),
		Last30Days:  AdminActivityCounts(a.Month),
	}
}

func NewAdminRecentSignup(u model.UserWithRole) AdminRecentSignup {
	return AdminRecentSignup{
		ID:            u.ID,
		DisplayName:   u.DisplayName,
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt.UTC().Format(time.RFC3339),
	}
}
