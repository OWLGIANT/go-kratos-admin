package middleware

import (
	"runtime/debug"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// RecoveryMiddleware handles panic recovery for WebSocket handlers
type RecoveryMiddleware struct {
	log *log.Helper
}

// NewRecoveryMiddleware creates a new recovery middleware
func NewRecoveryMiddleware(logger log.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{
		log: log.NewHelper(log.With(logger, "module", "websocket/recovery")),
	}
}

// Recover wraps a handler function with panic recovery
func (m *RecoveryMiddleware) Recover(handler func(*websocket.Client, *protocol.UnifiedMessage) error) func(*websocket.Client, *protocol.UnifiedMessage) error {
	return func(client *websocket.Client, msg *protocol.UnifiedMessage) (err error) {
		defer func() {
			if r := recover(); r != nil {
				m.log.Errorf("WebSocket handler panic: %v\n%s", r, debug.Stack())

				// Send error response to client
				resp := protocol.NewErrorResponse(500, "Internal server error")
				client.SendResponse(resp, msg.Action)

				err = nil // Don't propagate panic as error
			}
		}()

		return handler(client, msg)
	}
}
