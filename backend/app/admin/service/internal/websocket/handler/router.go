package handler

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// HandlerFunc is a function that handles a WebSocket message
type HandlerFunc func(*websocket.Client, *protocol.UnifiedMessage) error

// Router routes messages to appropriate handlers
type Router struct {
	handlers map[string]HandlerFunc
	log      *log.Helper
}

// NewRouter creates a new message router
func NewRouter(logger log.Logger) *Router {
	return &Router{
		handlers: make(map[string]HandlerFunc),
		log:      log.NewHelper(log.With(logger, "module", "websocket/router")),
	}
}

// Register registers a handler for an action
func (r *Router) Register(action string, handler HandlerFunc) {
	r.handlers[action] = handler
	r.log.Infof("Registered handler for action: %s", action)
}

// HandleMessage routes a message to the appropriate handler
func (r *Router) HandleMessage(client *websocket.Client, msg *protocol.UnifiedMessage) error {
	r.log.Infof("Received message: action=%s, client=%s, isActor=%v, data=%v", msg.Action, client.ID, client.IsActor, msg.Data)

	handler, ok := r.handlers[msg.Action]
	if !ok {
		r.log.Warnf("No handler found for action: %s", msg.Action)
		resp := protocol.NewErrorResponse(404, fmt.Sprintf("Unknown action: %s", msg.Action))
		return client.SendResponse(resp, msg.Action)
	}

	r.log.Infof("Routing message to handler: action=%s, client=%s", msg.Action, client.ID)
	return handler(client, msg)
}
