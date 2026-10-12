package main

import (
	"context"
	"fmt"
	"io/fs"
	"net"

	"retrotide/internal/actionitem"
	"retrotide/internal/admin"
	"retrotide/internal/auth"
	"retrotide/internal/authz"
	"retrotide/internal/board"
	"retrotide/internal/board/actionitems"
	"retrotide/internal/board/cardpolish"
	"retrotide/internal/board/images"
	"retrotide/internal/board/live"
	"retrotide/internal/board/management"
	"retrotide/internal/board/surveys"
	"retrotide/internal/cache"
	"retrotide/internal/config"
	"retrotide/internal/cookie"
	"retrotide/internal/dronebl"
	"retrotide/internal/email"
	"retrotide/internal/entitlements"
	"retrotide/internal/export"
	"retrotide/internal/guest"
	"retrotide/internal/health"
	"retrotide/internal/hierarchy"
	"retrotide/internal/identity"
	"retrotide/internal/identity/google"
	"retrotide/internal/identity/microsoft"
	"retrotide/internal/logger"
	"retrotide/internal/media"
	"retrotide/internal/notification"
	"retrotide/internal/og"
	"retrotide/internal/org"
	"retrotide/internal/org/invite"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
	"retrotide/internal/savedtemplate"
	"retrotide/internal/session"
	"retrotide/internal/settings"
	"retrotide/internal/siteinfo"
	"retrotide/internal/sitemap"
	"retrotide/internal/team"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"
	"retrotide/internal/ws"
)

type (
	resizableEngine interface {
		ResizeMB(maxMB int)
	}

	reconfigurableEngine interface {
		Reconfigure(rawURL string) error
	}
)

func initServices(repos *repository.Repositories, settingsSvc settings.Service, cacheMgr *cache.Manager) (*services, error) {
	if err := initCache(cacheMgr, settingsSvc); err != nil {
		return nil, fmt.Errorf("init cache: %w", err)
	}

	sessionMgr := session.NewManager(repos.Session, settingsSvc)

	authzSvc := authz.NewService(repos.Role, repos.Permission, repos.User)

	guestMgr := guest.NewManager(repos.GuestDevice, settingsSvc)

	hub := ws.NewHub()
	sessionMgr.SetDisconnector(hub)
	guestMgr.SetDisconnector(hub)

	orgAuthzSvc := orgauthz.NewService(repos.Organisation, repos.Team)

	hierarchySvc := hierarchy.NewService(hierarchy.Deps{Authz: authzSvc, Organisations: repos.Organisation, Teams: repos.Team})

	entitlementsSvc := entitlements.NewService()

	userSvc := user.NewService(repos.User, settingsSvc)

	emailSvc, err := email.NewService(settingsSvc)
	if err != nil {
		return nil, fmt.Errorf("init email: %w", err)
	}

	notificationSvc := notification.NewService(repos.Notification, repos.User, emailSvc, settingsSvc)

	droneblSvc := dronebl.NewService(settingsSvc, net.DefaultResolver)

	orgSvc := org.NewService(org.Deps{
		Organisations: repos.Organisation,
		Users:         repos.User,
		AuditLog:      repos.AuditLog,
		Hierarchy:     hierarchySvc,
		Settings:      settingsSvc,
		MaxOwned:      config.SettingMaxFreeOrgsPerOwner,
	})

	teamSvc := team.NewService(team.Deps{
		Teams:         repos.Team,
		Organisations: repos.Organisation,
		AuditLog:      repos.AuditLog,
		Hierarchy:     hierarchySvc,
		Entitlements:  entitlementsSvc,
	})

	inviteSvc := invite.NewService(invite.Deps{
		Organisations: repos.Organisation,
		Teams:         repos.Team,
		Users:         repos.User,
		AuditLog:      repos.AuditLog,
		Accounts:      userSvc,
		OrgAuthz:      orgAuthzSvc,
		Entitlements:  entitlementsSvc,
		Settings:      settingsSvc,
		Email:         emailSvc,
	})

	boardLive := live.New(live.Deps{
		Boards:   repos.Board,
		Cards:    repos.Card,
		OrgAuthz: orgAuthzSvc,
		Hub:      hub,
		Settings: settingsSvc,
		Guests:   guestMgr,
	})

	wsHandler, err := ws.NewHandler(ws.HandlerDeps{
		Hub:      hub,
		Settings: settingsSvc,
		Users:    sessionMgr,
		Guests:   guestMgr,
		Bans:     authzSvc,
		Boards:   boardLive,
	})
	if err != nil {
		return nil, fmt.Errorf("init the socket handler: %w", err)
	}

	boardSvc := board.NewService(board.Deps{
		Boards:         repos.Board,
		Columns:        repos.BoardColumn,
		Cards:          repos.Card,
		Votes:          repos.CardVote,
		Comments:       repos.CardComment,
		Teams:          repos.Team,
		Organisations:  repos.Organisation,
		Live:           boardLive,
		Events:         boardLive,
		OrgAuthz:       orgAuthzSvc,
		AuditLog:       repos.AuditLog,
		SavedTemplates: repos.SavedTemplate,
	})

	savedTemplateSvc := savedtemplate.NewService(savedtemplate.Deps{
		SavedTemplates: repos.SavedTemplate,
		Columns:        repos.BoardColumn,
		Teams:          repos.Team,
		Organisations:  repos.Organisation,
		Live:           boardLive,
		OrgAuthz:       orgAuthzSvc,
		Entitlements:   entitlementsSvc,
	})

	boardActionItemSvc := actionitems.NewService(actionitems.Deps{
		Boards:        repos.Board,
		Cards:         repos.Card,
		ActionItems:   repos.ActionItem,
		Teams:         repos.Team,
		Organisations: repos.Organisation,
		Live:          boardLive,
		Events:        boardLive,
	})

	cardPolishSvc := cardpolish.NewService(cardpolish.Deps{
		Boards:    repos.Board,
		Cards:     repos.Card,
		Reactions: repos.CardReaction,
		Live:      boardLive,
		Events:    boardLive,
	})

	boardSurveySvc := surveys.NewService(surveys.Deps{
		Boards:  repos.Board,
		Cards:   repos.Card,
		Surveys: repos.Survey,
		Live:    boardLive,
		Events:  boardLive,
	})

	mediaSvc := media.NewService(media.Deps{
		Media:        repos.Media,
		Live:         boardLive,
		Entitlements: entitlementsSvc,
		Encoder:      media.NewCWebPEncoder(),
	})

	boardImageSvc := images.NewService(images.Deps{
		Boards: repos.Board,
		Cards:  repos.Card,
		Media:  repos.Media,
		Live:   boardLive,
		Events: boardLive,
	})

	boardManagementSvc := management.NewService(management.Deps{
		Boards:        repos.Board,
		Columns:       repos.BoardColumn,
		Cards:         repos.Card,
		Teams:         repos.Team,
		Organisations: repos.Organisation,
		Live:          boardLive,
		Events:        boardLive,
		OrgAuthz:      orgAuthzSvc,
		AuditLog:      repos.AuditLog,
	})

	actionItemSvc := actionitem.NewService(actionitem.Deps{
		ActionItems:   repos.ActionItem,
		Cards:         repos.Card,
		Comments:      repos.CardComment,
		Organisations: repos.Organisation,
		OrgAuthz:      orgAuthzSvc,
	})

	exportSvc := export.NewService(export.Deps{
		Engines:       export.NewEngineManager(),
		Boards:        repos.Board,
		Teams:         repos.Team,
		Organisations: repos.Organisation,
		Users:         repos.User,
		Identities:    repos.UserIdentity,
		Cards:         repos.Card,
		Comments:      repos.CardComment,
		ActionItems:   repos.ActionItem,
		Live:          boardLive,
		OrgAuthz:      orgAuthzSvc,
	})

	guestClaimSvc := guest.NewClaimService(guest.ClaimDeps{
		Boards:   repos.Board,
		Claimer:  boardLive,
		Devices:  guestMgr,
		AuditLog: repos.AuditLog,
	})

	twoFactorSvc := twofactor.NewService(twofactor.Deps{
		TwoFactor: repos.TwoFactor,
		Users:     userSvc,
		Settings:  settingsSvc,
		AuditLog:  repos.AuditLog,
		Sessions:  sessionMgr,
	})

	twoFactorCookies := cookie.NewBuilder(settingsSvc, twofactor.ChallengeCookieName, cookie.Fixed(twofactor.ChallengeTTL))
	trustedDevices := cookie.NewBuilder(settingsSvc, twofactor.TrustedDeviceCookieName, cookie.Fixed(twofactor.TrustedDeviceTTL))

	authSvc := auth.NewService(auth.Deps{
		Users:         userSvc,
		Authz:         authzSvc,
		Sessions:      sessionMgr,
		Settings:      settingsSvc,
		Email:         emailSvc,
		UserRepo:      repos.User,
		Verifications: repos.EmailVerification,
		Resets:        repos.PasswordReset,
		Invitations:   repos.Invitation,
		AuditLog:      repos.AuditLog,
		Organisations: orgSvc,
		TwoFactor:     twoFactorSvc,
	})

	identityProviders := []identity.Provider{google.New(settingsSvc), microsoft.New(settingsSvc)}

	identitySvc, err := identity.NewService(identity.Deps{
		Providers:  identityProviders,
		Users:      userSvc,
		Authz:      authzSvc,
		Sessions:   sessionMgr,
		Settings:   settingsSvc,
		Email:      emailSvc,
		UserRepo:   repos.User,
		Identities: repos.UserIdentity,
		Flows:      repos.OIDCFlow,
		AuditLog:   repos.AuditLog,
		TwoFactor:  twoFactorSvc,
		SignIns:    authSvc,
	})
	if err != nil {
		return nil, fmt.Errorf("init identity: %w", err)
	}

	flowCookies := cookie.NewBuilder(settingsSvc, identity.FlowCookieName, cookie.Fixed(identity.FlowTTL))

	siteInfoSvc := siteinfo.NewService(siteinfo.Deps{Settings: settingsSvc, Auth: authSvc, Identity: identitySvc, Email: emailSvc, DeploymentMode: config.Cfg.DeploymentMode})

	adminSvc := admin.NewService(admin.Deps{
		Users:           repos.User,
		Roles:           repos.Role,
		Permissions:     repos.Permission,
		AuditLog:        repos.AuditLog,
		Verifications:   repos.EmailVerification,
		Invitations:     repos.Invitation,
		Organisations:   repos.Organisation,
		Teams:           repos.Team,
		Stats:           repos.Stats,
		Authz:           authzSvc,
		Hierarchy:       hierarchySvc,
		Accounts:        userSvc,
		Settings:        settingsSvc,
		Sessions:        sessionMgr,
		Email:           emailSvc,
		Entitlements:    entitlementsSvc,
		SignInProviders: identityProviders,
	})

	healthSvc, err := health.NewService(repos.DB(), cacheMgr, settingsSvc, config.Version)
	if err != nil {
		return nil, fmt.Errorf("init health checks: %w", err)
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, fmt.Errorf("open the embedded static files: %w", err)
	}

	ogResolver := og.NewResolver(og.Deps{Settings: settingsSvc, DeploymentMode: config.Cfg.DeploymentMode})
	sitemapSvc := sitemap.NewService(sitemap.Deps{Settings: settingsSvc, DeploymentMode: config.Cfg.DeploymentMode})

	return &services{
		cache:             cacheMgr,
		sessions:          sessionMgr,
		guests:            guestMgr,
		guestClaims:       guestClaimSvc,
		authz:             authzSvc,
		orgAuthz:          orgAuthzSvc,
		entitlements:      entitlementsSvc,
		user:              userSvc,
		email:             emailSvc,
		notification:      notificationSvc,
		dronebl:           droneblSvc,
		auth:              authSvc,
		identityProviders: identityProviders,
		identity:          identitySvc,
		flowCookies:       flowCookies,
		twoFactor:         twoFactorSvc,
		twoFactorCookies:  twoFactorCookies,
		trustedDevices:    trustedDevices,
		siteInfo:          siteInfoSvc,
		org:               orgSvc,
		team:              teamSvc,
		invites:           inviteSvc,
		boardLive:         boardLive,
		hub:               hub,
		wsHandler:         wsHandler,
		board:             boardSvc,
		savedTemplates:    savedTemplateSvc,
		boardActionItems:  boardActionItemSvc,
		cardPolish:        cardPolishSvc,
		boardSurveys:      boardSurveySvc,
		media:             mediaSvc,
		boardImages:       boardImageSvc,
		boardManagement:   boardManagementSvc,
		actionItems:       actionItemSvc,
		exports:           exportSvc,
		admin:             adminSvc,
		health:            healthSvc,
		staticFS:          staticFS,
		ogResolver:        ogResolver,
		sitemap:           sitemapSvc,
	}, nil
}

func initCache(manager *cache.Manager, settingsSvc settings.Service) error {
	ctx := context.Background()

	valkeyURL, err := settingsSvc.Get(ctx, config.SettingValkeyURL)
	if err != nil {
		return err
	}

	maxMB, err := settingsSvc.GetInt(ctx, config.SettingCacheInMemoryMaxMB)
	if err != nil {
		return err
	}

	for _, candidate := range manager.Engines() {
		if resizable, ok := candidate.(resizableEngine); ok {
			resizable.ResizeMB(maxMB)
		}

		configurable, ok := candidate.(reconfigurableEngine)
		if !ok {
			continue
		}

		if err := configurable.Reconfigure(valkeyURL); err != nil {
			logger.Log.Warn().Err(err).Str("engine", string(candidate.Kind())).Msg("cache engine reconfigure failed at startup")
		}
	}

	return nil
}
