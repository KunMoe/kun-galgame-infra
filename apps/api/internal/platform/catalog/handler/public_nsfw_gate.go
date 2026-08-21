package handler

import (
	"api/internal/platform/devapi"
	"api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

const msgNSFWNotAllowed = "nsfw=1 requires a key with the NSFW capability (nsfw_allowed); apply for it on the developer portal, or drop the parameter for the sfw view"

// RequireNSFWCapability refuses nsfw=1 from a key that was never granted the
// capability, instead of degrading it to the sfw view: a caller handed a
// silently narrowed page reads it as the whole catalogue. Requests that do not
// ask for nsfw are untouched, so every sfw caller stays byte-identical.
//
// nsfwQuery is deliberately the same parser the handlers gate on — a second
// truthy set here would be a door that opens on one side only.
func RequireNSFWCapability(c fiber.Ctx) error {
	if !nsfwQuery(c) {
		return c.Next()
	}
	if cred := devapi.CredentialFrom(c); cred == nil || !cred.NSFWAllowed {
		return response.ForbiddenMsg(c, errors.ErrForbidden, msgNSFWNotAllowed)
	}
	return c.Next()
}
