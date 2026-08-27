package handler

import (
	"api/internal/platform/devapi"

	"github.com/gofiber/fiber/v3"
)

func clientFromCtx(c fiber.Ctx) (string, bool) {
	cred := devapi.CredentialFrom(c)
	if cred == nil || cred.ClientID == "" {
		return "", false
	}
	return cred.ClientID, true
}
