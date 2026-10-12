package main

import (
	"net/http"
	"net/http/pprof"

	"retrotide/internal/authz"
	"retrotide/internal/controllers"
	"retrotide/internal/middleware"

	"github.com/gin-gonic/gin"
)

func pprofRoutes(guards *middleware.Guards) []controllers.FRoute {
	before := gin.HandlersChain{guards.Permission(authz.PermManageSettings)}

	return []controllers.FRoute{
		{Method: http.MethodGet, Path: "/debug/pprof/", Before: before, Handler: gin.WrapF(pprof.Index)},
		{Method: http.MethodGet, Path: "/debug/pprof/cmdline", Before: before, Handler: gin.WrapF(pprof.Cmdline)},
		{Method: http.MethodGet, Path: "/debug/pprof/profile", Before: before, Handler: gin.WrapF(pprof.Profile)},
		{Method: http.MethodGet, Path: "/debug/pprof/symbol", Before: before, Handler: gin.WrapF(pprof.Symbol)},
		{Method: http.MethodPost, Path: "/debug/pprof/symbol", Before: before, Handler: gin.WrapF(pprof.Symbol)},
		{Method: http.MethodGet, Path: "/debug/pprof/trace", Before: before, Handler: gin.WrapF(pprof.Trace)},
		{Method: http.MethodGet, Path: "/debug/pprof/:profile", Before: before, Handler: gin.WrapF(pprof.Index)},
	}
}
