package handler

import (
	"context"
	"strings"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/devapi"

	"github.com/gofiber/fiber/v3"
)

type ctxKey string

const (
	ctxUserID   ctxKey = "v2_user_id"
	ctxClientID ctxKey = "v2_client_id"
)

func userIDFrom(ctx context.Context) int64 {
	v, _ := ctx.Value(ctxUserID).(int64)
	return v
}

func clientIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxClientID).(string)
	return v
}

func requireUser(ctx context.Context) (int64, string, error) {
	uid := userIDFrom(ctx)
	client := clientIDFrom(ctx)
	if uid <= 0 {
		return 0, "", problem.New(problem.CodeUserIdentityRequired, "", "", "this operation requires a user access token.")
	}
	if client == "" {
		return 0, "", problem.New(problem.CodeUserIdentityRequired, "", "", "the access token is missing a client id.")
	}
	return uid, client, nil
}

func userAuth(lookup func(context.Context, string) (int64, string, error)) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		if !strings.HasPrefix(path, "/v2/me/") && !strings.HasPrefix(path, "/v2/moderation/") {
			return c.Next()
		}
		h := c.Get("Authorization")
		const pfx = "Bearer "
		if h == "" {
			return problem.WriteFiberError(c, problem.New(problem.CodeMissingCredential, problem.RequestID(c), problem.Instance(c),
				"Authorization Bearer token is required."))
		}
		if !strings.HasPrefix(h, pfx) {
			return problem.WriteFiberError(c, problem.New(problem.CodeInvalidCredential, problem.RequestID(c), problem.Instance(c),
				"Authorization Bearer token is invalid."))
		}
		token := strings.TrimSpace(h[len(pfx):])
		if token == "" {
			return problem.WriteFiberError(c, problem.New(problem.CodeInvalidCredential, problem.RequestID(c), problem.Instance(c),
				"Authorization Bearer token is invalid."))
		}
		if devapi.HasKeyPrefix(token) {
			return problem.WriteFiberError(c, problem.New(problem.CodeInvalidCredential, problem.RequestID(c), problem.Instance(c),
				"this face requires a user access token, not an application key."))
		}
		if lookup != nil {
			uid, clientID, err := lookup(c.Context(), token)
			if err != nil {
				return problem.WriteFiberError(c, problem.New(problem.CodeInvalidCredential, problem.RequestID(c), problem.Instance(c),
					"Authorization Bearer token is invalid."))
			}
			if uid <= 0 {
				return problem.WriteFiberError(c, problem.New(problem.CodeUserIdentityRequired, problem.RequestID(c), problem.Instance(c),
					"this operation requires a user access token."))
			}
			c.Locals("user_id", uid)
			c.Locals("token_client_id", clientID)
		}
		return c.Next()
	}
}
