package controllers

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

type (
	FRoute struct {
		Method  string
		Path    string
		Before  gin.HandlersChain
		Handler gin.HandlerFunc
	}
)

func RegisterRoutes(r gin.IRouter, routes []FRoute) {
	for _, route := range routes {
		chain := append(slices.Clone(route.Before), route.Handler)

		r.Handle(route.Method, route.Path, chain...)
		if route.Method == http.MethodGet {
			r.Handle(http.MethodHead, route.Path, chain...)
		}
	}
}
