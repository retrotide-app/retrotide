package main

import (
	"cmp"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

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
	"retrotide/internal/controllers"
	"retrotide/internal/cookie"
	"retrotide/internal/dronebl"
	"retrotide/internal/email"
	"retrotide/internal/entitlements"
	"retrotide/internal/export"
	"retrotide/internal/guest"
	"retrotide/internal/health"
	"retrotide/internal/identity"
	"retrotide/internal/logger"
	"retrotide/internal/media"
	"retrotide/internal/middleware"
	"retrotide/internal/notification"
	"retrotide/internal/og"
	"retrotide/internal/org"
	"retrotide/internal/org/invite"
	"retrotide/internal/orgauthz"
	"retrotide/internal/repository"
	"retrotide/internal/routes"
	"retrotide/internal/savedtemplate"
	"retrotide/internal/session"
	"retrotide/internal/settings"
	"retrotide/internal/siteinfo"
	"retrotide/internal/sitemap"
	"retrotide/internal/team"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"
	"retrotide/internal/utils"
	"retrotide/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

const (
	drainTimeout      = 15 * time.Second
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 3 * time.Minute
	maxHeaderBytes    = 16 << 10
)

var (
	//go:embed all:static
	staticFiles embed.FS
)

type (
	services struct {
		cache             *cache.Manager
		sessions          *session.Manager
		guests            *guest.Manager
		guestClaims       guest.ClaimService
		authz             authz.Service
		orgAuthz          orgauthz.Service
		entitlements      entitlements.Service
		user              user.Service
		email             email.Service
		notification      notification.Service
		dronebl           dronebl.Service
		auth              auth.Service
		identityProviders []identity.Provider
		identity          identity.Service
		flowCookies       *cookie.Builder
		twoFactor         twofactor.Service
		twoFactorCookies  *cookie.Builder
		trustedDevices    *cookie.Builder
		siteInfo          siteinfo.Service
		org               org.Service
		team              team.Service
		invites           invite.Service
		boardLive         live.Service
		hub               *ws.Hub
		wsHandler         http.Handler
		board             board.Service
		savedTemplates    savedtemplate.Service
		boardActionItems  actionitems.Service
		cardPolish        cardpolish.Service
		boardSurveys      surveys.Service
		media             media.Service
		boardImages       images.Service
		boardManagement   management.Service
		actionItems       actionitem.Service
		exports           export.Service
		admin             admin.Service
		health            health.Service
		staticFS          fs.FS
		ogResolver        og.Resolver
		sitemap           sitemap.Service
	}
)

func initServer() (*http.Server, int, func(), error) {
	cacheMgr, err := cache.New(prometheus.DefaultRegisterer)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("init cache: %w", err)
	}

	repos, settingsSvc, err := initDatabase(cacheMgr)
	if err != nil {
		return nil, 0, nil, err
	}

	svc, err := initServices(repos, settingsSvc, cacheMgr)
	if err != nil {
		return nil, 0, nil, err
	}

	if err := svc.board.RestoreTimers(context.Background()); err != nil {
		return nil, 0, nil, fmt.Errorf("restore the board timers: %w", err)
	}

	engine, bodyLimit, err := initApp(svc, repos, settingsSvc)
	if err != nil {
		return nil, 0, nil, err
	}

	stopJobs := registerListeners(settingsSvc, svc, repos, bodyLimit)

	srv := &http.Server{
		Addr:              config.Cfg.ListenAddr,
		Handler:           engine,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          log.New(logger.NewLevelWriter(zerolog.ErrorLevel), "", 0),
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		defer cancel()

		stopJobs(ctx)

		if err := svc.board.StopTimers(ctx); err != nil {
			logger.Ctx(ctx).Warn().Err(err).Msg("board timers did not stop in time")
		}

		if err := svc.boardLive.Drain(ctx); err != nil {
			logger.Ctx(ctx).Warn().Err(err).Msg("board outboxes did not drain in time")
		}

		if err := svc.hub.Shutdown(ctx); err != nil {
			logger.Ctx(ctx).Warn().Err(err).Msg("sockets did not close in time")
		}

		if err := svc.cache.Close(); err != nil {
			logger.Ctx(ctx).Warn().Err(err).Msg("cache close failed")
		}
	}

	return srv, len(engine.Routes()), cleanup, nil
}

func initApp(svc *services, repos *repository.Repositories, settingsSvc settings.Service) (*gin.Engine, *middleware.BodyLimit, error) {
	engine, err := utils.NewEngine(utils.DefaultTrustedProxies)
	if err != nil {
		return nil, nil, fmt.Errorf("build engine: %w", err)
	}

	storedLimit, err := settingsSvc.Get(context.Background(), config.SettingMaxBodySize)
	if err != nil {
		return nil, nil, fmt.Errorf("read the request body limit: %w", err)
	}

	maxBodyBytes, err := middleware.ParseBodyLimit(storedLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("apply the stored request body limit: %w", err)
	}

	bodyLimit := middleware.NewBodyLimit(maxBodyBytes)

	deps := middleware.Deps{
		Settings:   settingsSvc,
		BodyLimit:  bodyLimit,
		DroneBL:    svc.dronebl,
		Sessions:   svc.sessions,
		Guests:     svc.guests,
		Authz:      svc.authz,
		LastSeenIP: middleware.NewLastSeenIP(repos.User),
	}
	if err := middleware.Setup(engine, deps); err != nil {
		return nil, nil, fmt.Errorf("install middleware: %w", err)
	}

	guards := middleware.NewGuards(middleware.GuardDeps{Settings: settingsSvc, Sessions: svc.sessions, Guests: svc.guests, Authz: svc.authz, OrgAuthz: svc.orgAuthz})

	ctrl := &controllers.Service{
		HealthService:        svc.health,
		AuthService:          svc.auth,
		UserService:          svc.user,
		IdentityService:      svc.identity,
		TwoFactorService:     svc.twoFactor,
		NotificationService:  svc.notification,
		SiteInfoService:      svc.siteInfo,
		AdminService:         svc.admin,
		AuthzService:         svc.authz,
		OrgService:           svc.org,
		TeamService:          svc.team,
		InviteService:        svc.invites,
		EntitlementsService:  svc.entitlements,
		BoardLive:            svc.boardLive,
		BoardService:         svc.board,
		SavedTemplateService: svc.savedTemplates,
		BoardActionItems:     svc.boardActionItems,
		BoardCardPolish:      svc.cardPolish,
		BoardSurveys:         svc.boardSurveys,
		MediaService:         svc.media,
		BoardImages:          svc.boardImages,
		BoardManagement:      svc.boardManagement,
		ActionItemService:    svc.actionItems,
		ExportService:        svc.exports,
		Sessions:             svc.sessions,
		Guests:               svc.guests,
		GuestClaims:          svc.guestClaims,
		FlowCookies:          svc.flowCookies,
		TwoFactorCookies:     svc.twoFactorCookies,
		TrustedDevices:       svc.trustedDevices,
		Guards:               guards,
		WSHandler:            svc.wsHandler,
		StaticFS:             svc.staticFS,
		OGResolver:           svc.ogResolver,
		SitemapService:       svc.sitemap,
	}

	registerAllRoutes(engine, ctrl, guards)

	logRoutes(engine)

	return engine, bodyLimit, nil
}

func registerAllRoutes(engine *gin.Engine, ctrl *controllers.Service, guards *middleware.Guards) {
	routes.PublicRoutes(ctrl, engine)
	controllers.RegisterRoutes(engine, pprofRoutes(guards))
}

func logRoutes(engine *gin.Engine) {
	registered := engine.Routes()

	if logger.Log.Debug().Enabled() {
		slices.SortFunc(registered, func(a, b gin.RouteInfo) int {
			return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Method, b.Method))
		})

		methodWidth := len("METHOD")
		pathWidth := len("PATH")
		for _, route := range registered {
			methodWidth = max(methodWidth, len(route.Method))
			pathWidth = max(pathWidth, len(route.Path))
		}

		border := "+" + strings.Repeat("-", methodWidth+2) + "+" + strings.Repeat("-", pathWidth+2) + "+"

		var table strings.Builder
		fmt.Fprintf(&table, "\n%s\n| %-*s | %-*s |\n%s\n", border, methodWidth, "METHOD", pathWidth, "PATH", border)
		for _, route := range registered {
			fmt.Fprintf(&table, "| %-*s | %-*s |\n", methodWidth, route.Method, pathWidth, route.Path)
		}
		table.WriteString(border)

		logger.Log.Debug().Msgf("registered routes:%s", table.String())
	}

	logger.Log.Info().Int("count", len(registered)).Msg("routes mounted")
}
