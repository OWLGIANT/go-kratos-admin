package middleware

import (
	"net/http"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	authnEngine "github.com/tx7do/kratos-authn/engine"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/pkg/jwt"
	"go-wind-admin/pkg/middleware/auth"
)

// AuthMiddleware handles WebSocket authentication
type AuthMiddleware struct {
	authenticator authnEngine.Authenticator
	log           *log.Helper
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(authenticator authnEngine.Authenticator, logger log.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		authenticator: authenticator,
		log:           log.NewHelper(log.With(logger, "module", "websocket/auth")),
	}
}

// Authenticate authenticates a WebSocket connection and returns user info
func (m *AuthMiddleware) Authenticate(r *http.Request) (userID, tenantID uint32, username string, err error) {
	// Extract token from query parameter or header
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("Authorization")
		if token != "" && strings.HasPrefix(token, "Bearer ") {
			token = token[7:]
		}
	}

	if token == "" {
		m.log.Error("WebSocket auth: missing token")
		return 0, 0, "", auth.ErrMissingJwtToken
	}

	// Verify token using authenticator
	claims, err := m.authenticator.AuthenticateToken(token)
	if err != nil {
		m.log.Errorf("WebSocket auth: failed to authenticate token: %v", err)
		return 0, 0, "", err
	}

	// Extract user information from claims
	tokenPayload, err := jwt.NewUserTokenPayloadWithClaims(claims)
	if err != nil {
		m.log.Errorf("WebSocket auth: failed to extract user info: %v", err)
		return 0, 0, "", err
	}

	// Check token expiration
	if jwt.IsTokenExpired(claims) {
		m.log.Errorf("WebSocket auth: token expired for user %d", tokenPayload.UserId)
		return 0, 0, "", auth.ErrAccessTokenExpired
	}

	// Check token validity
	if jwt.IsTokenNotValidYet(claims) {
		m.log.Errorf("WebSocket auth: token not valid yet for user %d", tokenPayload.UserId)
		return 0, 0, "", auth.ErrInvalidRequest
	}

	userID = tokenPayload.UserId
	tenantID = tokenPayload.GetTenantId()
	username = tokenPayload.GetUsername()

	m.log.Infof("WebSocket auth: authenticated user %s (id=%d, tenant=%d)", username, userID, tenantID)

	return userID, tenantID, username, nil
}

// AuthenticateClient authenticates a client and sets user info
func (m *AuthMiddleware) AuthenticateClient(client *websocket.Client, r *http.Request) error {
	userID, tenantID, username, err := m.Authenticate(r)
	if err != nil {
		return err
	}

	client.SetUserInfo(userID, tenantID, username)
	return nil
}
