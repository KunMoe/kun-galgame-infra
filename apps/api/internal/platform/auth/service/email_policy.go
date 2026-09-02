package service

import (
	"slices"
	"strings"

	"api/internal/platform/settings/keys"
	"api/pkg/errors"
)

func checkEmailDomainAllowed(email string) error {
	at := strings.LastIndexByte(email, '@')
	if at < 0 || at == len(email)-1 {
		return errors.NewWithCode(errors.ErrAuthInvalidEmail)
	}
	domain := strings.ToLower(strings.TrimSpace(email[at+1:]))
	if !slices.Contains(keys.AuthAllowedEmailDomains.Get(), domain) {
		return errors.NewWithCode(errors.ErrAuthEmailDomainNotAllowed)
	}
	return nil
}
