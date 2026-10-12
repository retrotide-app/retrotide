package repository

import "database/sql"

type (
	Repositories struct {
		db                *sql.DB
		Settings          SettingsRepository
		User              UserRepository
		Session           SessionRepository
		GuestDevice       GuestDeviceRepository
		EmailVerification EmailVerificationRepository
		PasswordReset     PasswordResetRepository
		Notification      NotificationRepository
		Role              RoleRepository
		Permission        PermissionRepository
		AuditLog          AuditLogRepository
		UserIdentity      UserIdentityRepository
		OIDCFlow          OIDCFlowRepository
		Invitation        InvitationRepository
		Organisation      OrganisationRepository
		Team              TeamRepository
		Board             BoardRepository
		BoardColumn       BoardColumnRepository
		Card              CardRepository
		CardVote          CardVoteRepository
		CardComment       CardCommentRepository
		ActionItem        ActionItemRepository
		Stats             StatsRepository
		TwoFactor         TwoFactorRepository
		SavedTemplate     SavedTemplateRepository
		CardReaction      CardReactionRepository
		Media             MediaRepository
		Survey            SurveyRepository
	}
)

func (r *Repositories) DB() *sql.DB {
	return r.db
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{db: db}
}
