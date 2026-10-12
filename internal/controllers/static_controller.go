package controllers

import (
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"retrotide/internal/controllers/utils"
	"retrotide/internal/dto"
	"retrotide/internal/og"

	"github.com/gin-gonic/gin"
)

const (
	shellDocument    = "index.html"
	shellContentType = "text/html; charset=utf-8"
	composeType      = "application/yaml"
	composeFilename  = `attachment; filename="docker-compose.yml"`
	apiPrefix        = "/api"
	assetsDir        = "assets"
	notFoundMessage  = "not found"
)

func init() {
	if err := mime.AddExtensionType(".webmanifest", "application/manifest+json"); err != nil {
		panic(err)
	}
}

func (s *Service) getAllStaticRoutes() []FRoute {
	return []FRoute{
		{Method: http.MethodGet, Path: "/assets/*filepath", Handler: s.serveAsset},
		{Method: http.MethodGet, Path: "/self-host/docker-compose.yml", Handler: s.serveSelfHostCompose},
	}
}

func (s *Service) serveSelfHostCompose(c *gin.Context) {
	c.Header("Content-Disposition", composeFilename)
	c.Data(http.StatusOK, composeType, s.SelfHostCompose)
}

func (s *Service) serveAsset(c *gin.Context) {
	name := assetsDir + path.Clean(c.Param("filepath"))

	s.serveEmbedded(c, name)
}

func (s *Service) SPA(c *gin.Context) {
	method := c.Request.Method
	if method != http.MethodGet && method != http.MethodHead {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: notFoundMessage})
		return
	}

	requested := c.Request.URL.Path
	if requested == apiPrefix || strings.HasPrefix(requested, apiPrefix+"/") {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: notFoundMessage})
		return
	}

	if path.Ext(requested) != "" {
		s.serveEmbedded(c, strings.TrimPrefix(requested, "/"))
		return
	}

	s.serveShell(c, requested)
}

func (s *Service) serveShell(c *gin.Context, requested string) {
	if hasDotDotSegment(requested) {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: notFoundMessage})
		return
	}

	shell, err := fs.ReadFile(s.StaticFS, shellDocument)
	if err != nil {
		utils.InternalError(c, "read the embedded shell", err)
		return
	}

	meta, err := s.OGResolver.Resolve(c.Request.Context(), requested)
	if err != nil {
		utils.InternalError(c, "failed to resolve the page meta", err)
		return
	}

	c.Data(http.StatusOK, shellContentType, og.Render(shell, meta))
}

func (s *Service) serveEmbedded(c *gin.Context, name string) {
	if hasDotDotSegment(c.Request.URL.Path) {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: notFoundMessage})
		return
	}

	cleaned := path.Clean(name)

	info, err := fs.Stat(s.StaticFS, cleaned)
	if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrInvalid) {
		utils.InternalError(c, "stat embedded file", err)
		return
	}

	if err != nil || info.IsDir() {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.ErrorResponse{Error: notFoundMessage})
		return
	}

	c.Request.Header.Del("Range")

	http.ServeFileFS(c.Writer, c.Request, s.StaticFS, cleaned)
}

func hasDotDotSegment(urlPath string) bool {
	for segment := range strings.FieldsFuncSeq(urlPath, isPathSeparator) {
		if segment == ".." {
			return true
		}
	}

	return false
}

func isPathSeparator(r rune) bool {
	return r == '/' || r == '\\'
}
