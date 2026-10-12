package dao

import "database/sql"

func NewSettings(db *sql.DB) SettingsDAO {
	return &settingsDAO{db: db}
}

func NewUser(db *sql.DB) UserDAO {
	return &userDAO{db: db}
}

func NewSession(db *sql.DB) SessionDAO {
	return &sessionDAO{db: db}
}

func NewGuestDevice(db *sql.DB) GuestDeviceDAO {
	return &guestDeviceDAO{db: db}
}

func NewEmailVerification(db *sql.DB) EmailVerificationDAO {
	return &emailVerificationDAO{db: db}
}

func NewPasswordReset(db *sql.DB) PasswordResetDAO {
	return &passwordResetDAO{db: db}
}

func NewNotification(db *sql.DB) NotificationDAO {
	return &notificationDAO{db: db}
}

func NewRole(db *sql.DB) RoleDAO {
	return &roleDAO{db: db}
}

func NewPermission(db *sql.DB) PermissionDAO {
	return &permissionDAO{db: db}
}

func NewAuditLog(db *sql.DB) AuditLogDAO {
	return &auditLogDAO{db: db}
}

func NewUserIdentity(db *sql.DB) UserIdentityDAO {
	return &userIdentityDAO{db: db}
}

func NewOIDCFlow(db *sql.DB) OIDCFlowDAO {
	return &oidcFlowDAO{db: db}
}

func NewInvitation(db *sql.DB) InvitationDAO {
	return &invitationDAO{db: db}
}

func NewOrganisation(db *sql.DB) OrganisationDAO {
	return &organisationDAO{db: db}
}

func NewTeam(db *sql.DB) TeamDAO {
	return &teamDAO{db: db}
}

func NewBoard(db *sql.DB) BoardDAO {
	return &boardDAO{db: db}
}

func NewBoardColumn(db *sql.DB) BoardColumnDAO {
	return &boardColumnDAO{db: db}
}

func NewCard(db *sql.DB) CardDAO {
	return &cardDAO{db: db}
}

func NewCardVote(db *sql.DB) CardVoteDAO {
	return &cardVoteDAO{db: db}
}

func NewCardComment(db *sql.DB) CardCommentDAO {
	return &cardCommentDAO{db: db}
}

func NewActionItem(db *sql.DB) ActionItemDAO {
	return &actionItemDAO{db: db}
}

func NewStats(db *sql.DB) StatsDAO {
	return &statsDAO{db: db}
}

func NewTwoFactor(db *sql.DB) TwoFactorDAO {
	return &twoFactorDAO{db: db}
}

func NewSavedTemplate(db *sql.DB) SavedTemplateDAO {
	return &savedTemplateDAO{db: db}
}

func NewCardReaction(db *sql.DB) CardReactionDAO {
	return &cardReactionDAO{db: db}
}

func NewMedia(db *sql.DB) MediaDAO {
	return &mediaDAO{db: db}
}

func NewSurvey(db *sql.DB) SurveyDAO {
	return &surveyDAO{db: db}
}
