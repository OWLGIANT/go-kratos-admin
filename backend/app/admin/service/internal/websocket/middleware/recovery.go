package middleware

import (
	"runtime/debug"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// RecoveryMiddleware 处理 WebSocket 处理器的 panic 恢复
type RecoveryMiddleware struct {
	log *log.Helper
}

// NewRecoveryMiddleware 创建新的恢复中间件
func NewRecoveryMiddleware(logger log.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{
		log: log.NewHelper(log.With(logger, "module", "websocket/recovery")),
	}
}

// Recover 包装处理器函数以进行 panic 恢复
func (m *RecoveryMiddleware) Recover(handler func(*websocket.Client, *protocol.Command) error) func(*websocket.Client, *protocol.Command) error {
	return func(client *websocket.Client, cmd *protocol.Command) (err error) {
		defer func() {
			if r := recover(); r != nil {
				m.log.Errorf("WebSocket handler panic: %v\n%s", r, debug.Stack())

				// 发送错误响应给客户端
				errCmd := protocol.NewErrorCommand(500, "Internal server error")
				errCmd.RequestID = cmd.RequestID
				errCmd.Seq = cmd.Seq
				client.SendCommand(errCmd)

				err = nil // 不将 panic 作为错误传播
			}
		}()

		return handler(client, cmd)
	}
}
