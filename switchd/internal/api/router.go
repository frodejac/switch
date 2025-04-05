package api

import (
	"github.com/frodejac/switch/switchd/internal/api/handlers"
	"github.com/frodejac/switch/switchd/internal/consensus"
	"github.com/frodejac/switch/switchd/internal/rules"
	"github.com/frodejac/switch/switchd/internal/storage"
	"github.com/labstack/echo/v4"
)

type Router struct {
	featureHandler    *handlers.FeatureHandler
	membershipHandler *handlers.MembershipHandler
}

func NewRouter(store storage.Store, rules *rules.Engine, cache *rules.Cache, raft *consensus.RaftNode, membership storage.MembershipStore) *Router {
	return &Router{
		featureHandler:    handlers.NewFeatureHandler(store, rules, cache),
		membershipHandler: handlers.NewMembershipHandler(raft, membership),
	}
}

func (r *Router) SetupRoutes(e *echo.Echo) {
	// Feature flag routes
	e.GET("/:store/:key", r.featureHandler.HandleGet)
	e.PUT("/:store/:key", r.featureHandler.HandlePut)
	e.DELETE("/:store/:key", r.featureHandler.HandleDelete)

	// Store routes
	e.GET("/:store", r.featureHandler.HandleList)
	e.GET("/stores", r.featureHandler.HandleListStores)

	// Membership routes
	e.POST("/join", r.membershipHandler.HandleJoin)
	e.GET("/status", r.membershipHandler.HandleStatus)
}
