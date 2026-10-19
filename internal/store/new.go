package store

import (
	"database/sql"

	"retrotide/internal/cache"
	"retrotide/internal/dao"
	"retrotide/internal/repository"
)

func New(db *sql.DB, c *cache.Manager) *repository.Repositories {
	repos := repository.NewRepositories(db)
	userDAO := dao.NewUser(db)
	organisationDAO := dao.NewOrganisation(db)
	teamDAO := dao.NewTeam(db)
	boardDAO := dao.NewBoard(db)
	columnDAO := dao.NewBoardColumn(db)
	cardDAO := dao.NewCard(db)
	voteDAO := dao.NewCardVote(db)
	commentDAO := dao.NewCardComment(db)
	actionItemDAO := dao.NewActionItem(db)
	reactionDAO := dao.NewCardReaction(db)
	surveyDAO := dao.NewSurvey(db)
	twoFactorDAO := dao.NewTwoFactor(db)

	repos.Settings = repository.NewSettingsRepo(dao.NewSettings(db), c)
	repos.Session = repository.NewSessionRepo(dao.NewSession(db))
	repos.GuestDevice = repository.NewGuestDeviceRepo(dao.NewGuestDevice(db))
	repos.EmailVerification = repository.NewEmailVerificationRepo(db, dao.NewEmailVerification(db))
	repos.PasswordReset = repository.NewPasswordResetRepo(db, dao.NewPasswordReset(db))
	repos.Notification = repository.NewNotificationRepo(dao.NewNotification(db))
	repos.Role = repository.NewRoleRepo(dao.NewRole(db), c)
	repos.Permission = repository.NewPermissionRepo(dao.NewPermission(db), c)
	repos.AuditLog = repository.NewAuditLogRepo(dao.NewAuditLog(db))
	repos.OIDCFlow = repository.NewOIDCFlowRepo(dao.NewOIDCFlow(db))
	repos.UserIdentity = repository.NewUserIdentityRepo(db, dao.NewUserIdentity(db), repository.UserIdentityRepoDeps{
		Users:    userDAO,
		Roles:    repos.Role,
		Sessions: repos.Session,
	})
	repos.User = repository.NewUserRepo(db, userDAO, repository.UserRepoDeps{
		Roles:         repos.Role,
		Sessions:      repos.Session,
		Verifications: repos.EmailVerification,
		Resets:        repos.PasswordReset,
		Identities:    repos.UserIdentity,
		Organisations: organisationDAO,
		TwoFactor:     twoFactorDAO,
	})
	repos.Invitation = repository.NewInvitationRepo(db, dao.NewInvitation(db), userDAO)
	repos.Organisation = repository.NewOrganisationRepo(db, organisationDAO, repository.OrganisationRepoDeps{
		Users: userDAO,
		Teams: teamDAO,
	}, c)
	repos.Team = repository.NewTeamRepo(db, teamDAO, organisationDAO, c)
	repos.Board = repository.NewBoardRepo(db, boardDAO, repository.BoardRepoDeps{
		Columns:     columnDAO,
		Cards:       cardDAO,
		Votes:       voteDAO,
		Comments:    commentDAO,
		ActionItems: actionItemDAO,
		Reactions:   reactionDAO,
		Surveys:     surveyDAO,
	})
	repos.BoardColumn = repository.NewBoardColumnRepo(db, columnDAO, boardDAO, voteDAO)
	repos.Card = repository.NewCardRepo(db, cardDAO, boardDAO, voteDAO)
	repos.CardVote = repository.NewCardVoteRepo(db, voteDAO, repository.CardVoteRepoDeps{
		Boards: boardDAO,
		Cards:  cardDAO,
	})
	repos.CardComment = repository.NewCardCommentRepo(db, commentDAO, boardDAO)
	repos.ActionItem = repository.NewActionItemRepo(db, actionItemDAO, boardDAO)
	repos.Stats = repository.NewStatsRepo(dao.NewStats(db))
	repos.TwoFactor = repository.NewTwoFactorRepo(db, twoFactorDAO, repository.TwoFactorRepoDeps{
		Sessions:      repos.Session,
		Verifications: repos.EmailVerification,
	})
	repos.SavedTemplate = repository.NewSavedTemplateRepo(db, dao.NewSavedTemplate(db), organisationDAO)
	repos.CardReaction = repository.NewCardReactionRepo(db, reactionDAO, boardDAO)
	repos.Media = repository.NewMediaRepo(db, dao.NewMedia(db), organisationDAO)
	repos.Survey = repository.NewSurveyRepo(db, surveyDAO, boardDAO)

	return repos
}
