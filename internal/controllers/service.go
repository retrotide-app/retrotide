package controllers

import (
	"io/fs"
	"net/http"

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
	"retrotide/internal/cookie"
	"retrotide/internal/entitlements"
	"retrotide/internal/export"
	"retrotide/internal/guest"
	"retrotide/internal/health"
	"retrotide/internal/identity"
	"retrotide/internal/media"
	"retrotide/internal/middleware"
	"retrotide/internal/notification"
	"retrotide/internal/og"
	"retrotide/internal/org"
	"retrotide/internal/org/invite"
	"retrotide/internal/savedtemplate"
	"retrotide/internal/session"
	"retrotide/internal/siteinfo"
	"retrotide/internal/sitemap"
	"retrotide/internal/team"
	"retrotide/internal/twofactor"
	"retrotide/internal/user"
)

type (
	Service struct {
		HealthService        health.Service
		AuthService          auth.Service
		UserService          user.Service
		IdentityService      identity.Service
		TwoFactorService     twofactor.Service
		NotificationService  notification.Service
		SiteInfoService      siteinfo.Service
		AdminService         admin.Service
		AuthzService         authz.Service
		OrgService           org.Service
		TeamService          team.Service
		InviteService        invite.Service
		EntitlementsService  entitlements.Service
		BoardLive            live.Service
		BoardService         board.Service
		SavedTemplateService savedtemplate.Service
		BoardActionItems     actionitems.Service
		BoardCardPolish      cardpolish.Service
		BoardSurveys         surveys.Service
		MediaService         media.Service
		BoardImages          images.Service
		BoardManagement      management.Service
		ActionItemService    actionitem.Service
		ExportService        export.Service
		Sessions             *session.Manager
		Guests               *guest.Manager
		GuestClaims          guest.ClaimService
		FlowCookies          *cookie.Builder
		TwoFactorCookies     *cookie.Builder
		TrustedDevices       *cookie.Builder
		Guards               *middleware.Guards
		WSHandler            http.Handler
		StaticFS             fs.FS
		OGResolver           og.Resolver
		SitemapService       sitemap.Service
	}
)

func (s *Service) GetAPIRoutes() []FRoute {
	var all []FRoute
	all = append(all, s.getAllAuthRoutes()...)
	all = append(all, s.getAllIdentityRoutes()...)
	all = append(all, s.getAllTwoFactorRoutes()...)
	all = append(all, s.getAllNotificationRoutes()...)
	all = append(all, s.getAllSiteInfoRoutes()...)
	all = append(all, s.getAllAdminRoutes()...)
	all = append(all, s.getAllOrganisationRoutes()...)
	all = append(all, s.getAllTeamRoutes()...)
	all = append(all, s.getAllInviteRoutes()...)
	all = append(all, s.getAllBoardRoutes()...)
	all = append(all, s.getAllBoardLinkRoutes()...)
	all = append(all, s.getAllGuestClaimRoutes()...)
	all = append(all, s.getAllBoardColumnRoutes()...)
	all = append(all, s.getAllBoardColumnFacilitationRoutes()...)
	all = append(all, s.getAllBoardCardRoutes()...)
	all = append(all, s.getAllBoardVoteRoutes()...)
	all = append(all, s.getAllBoardCommentRoutes()...)
	all = append(all, s.getAllBoardFacilitationRoutes()...)
	all = append(all, s.getAllBoardSpotlightRoutes()...)
	all = append(all, s.getAllBoardCardPolishRoutes()...)
	all = append(all, s.getAllBoardSurveyRoutes()...)
	all = append(all, s.getAllSurveyTemplateRoutes()...)
	all = append(all, s.getAllMediaRoutes()...)
	all = append(all, s.getAllBoardImageRoutes()...)
	all = append(all, s.getAllBoardManagementRoutes()...)
	all = append(all, s.getAllActionItemRoutes()...)
	all = append(all, s.getAllExportRoutes()...)
	all = append(all, s.getAllBoardTemplateRoutes()...)
	all = append(all, s.getAllSavedTemplateRoutes()...)
	all = append(all, s.getAllSocketRoutes()...)

	return all
}

func (s *Service) GetPageRoutes() []FRoute {
	var all []FRoute
	all = append(all, s.getAllHealthRoutes()...)
	all = append(all, s.getAllSitemapRoutes()...)
	all = append(all, s.getAllUploadRoutes()...)
	all = append(all, s.getAllStaticRoutes()...)

	return all
}
