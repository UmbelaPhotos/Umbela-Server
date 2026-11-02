package interceptors

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"umbela-server/internal/auth"
)

type AuthInterceptor struct {
	sessions *auth.SessionStore
}

func NewAuthInterceptor(sessions *auth.SessionStore) *AuthInterceptor {
	return &AuthInterceptor{
		sessions: sessions,
	}
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if isPublicProcedure(req.Spec().Procedure) {
			return next(ctx, req)
		}

		token := extractBearerToken(req.Header().Get("Authorization"))
		if token == "" || !i.sessions.IsValid(token) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("sesión no válida o expirada"))
		}

		return next(ctx, req)
	}
}

func (i *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if isPublicProcedure(conn.Spec().Procedure) {
			return next(ctx, conn)
		}

		token := extractBearerToken(conn.RequestHeader().Get("Authorization"))
		if token == "" || !i.sessions.IsValid(token) {
			return connect.NewError(connect.CodeUnauthenticated, errors.New("sesión no válida o expirada"))
		}

		return next(ctx, conn)
	}
}

func (i *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func isPublicProcedure(procedure string) bool {
	// Permite conectarse a Tor y loguearse sin necesidad de un token previo
	return procedure == "/umbela.v1.AuthService/Identify" ||
		procedure == "/umbela.v1.AuthService/Unlock"
}

func extractBearerToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}
