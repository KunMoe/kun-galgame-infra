package handler

import (
	"context"

	siteModel "api/internal/platform/site/model"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
)

type ctxKey string

const ctxKeyAdminID ctxKey = "catalog:admin_user_id"

type OAuthClientLookup interface {
	FindByClientID(ctx context.Context, clientID string) (*siteModel.OAuthClient, error)
}

func isThirdPartyClient(client *siteModel.OAuthClient) bool {
	return client != nil && client.OwnerUserID != nil
}

func AdminBridge(ctx huma.Context, next func(huma.Context)) {
	fc := humafiber.Unwrap(ctx)
	if id, ok := fc.Locals("user_id").(uint); ok {
		ctx = huma.WithValue(ctx, ctxKeyAdminID, int64(id))
	}
	next(ctx)
}

func adminIDFromCtx(ctx context.Context) int64 {
	id, _ := ctx.Value(ctxKeyAdminID).(int64)
	return id
}
