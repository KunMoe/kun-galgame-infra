package handler

import (
	"strings"

	"api/pkg/errors"

	"github.com/danielgtaylor/huma/v2"
)

type Envelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}

type houseError struct {
	status  int
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *houseError) Error() string             { return e.Message }
func (e *houseError) GetStatus() int            { return e.status }
func (e *houseError) ContentType(string) string { return "application/json" }

func InstallErrorEnvelope() {
	huma.NewError = func(status int, message string, errs ...error) huma.StatusError {
		if len(errs) > 0 {
			parts := make([]string, 0, len(errs)+1)
			if message != "" {
				parts = append(parts, message)
			}
			for _, e := range errs {
				if e != nil {
					parts = append(parts, e.Error())
				}
			}
			message = strings.Join(parts, "; ")
		}
		return &houseError{status: status, Code: statusToCode(status), Message: message}
	}
}

// Kept byte-identical to the catalog and news faces' mapping. huma.NewError is a
// package global and all three faces are mounted in the catalog PROCESS, so
// every InstallErrorEnvelope call writes the same variable and the last one
// wins; a divergent mapping here would silently change what the other two
// return depending on mount order.
func statusToCode(status int) int {
	switch status {
	case 400, 422:
		return errors.ErrValidationFailed
	case 401:
		return errors.ErrAuthUnauthorized
	case 403:
		return errors.ErrForbidden
	case 404:
		return errors.ErrNotFound
	default:
		return errors.ErrInternalServer
	}
}
