package handler

import (
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/editing"

	"github.com/stretchr/testify/require"
)

// Every keyed lane points into the proposal's patch document, decide-time
// errors included — the ValidationError lane alone used "/"+key until the
// edit UI reported the inconsistency.
func TestProposalErrPointersSharePatchPrefix(t *testing.T) {
	cases := []struct {
		err     error
		code    string
		pointer string
		reason  string
	}{
		{
			&editing.ValidationError{Key: "catalog.work.titles", Reason: "must be an array"},
			problem.CodeValidationFailed, "/patch/catalog.work.titles", problem.ReasonUnknownValue,
		},
		{
			&editing.PermissionError{Key: "catalog.work.credits", Action: "propose"},
			problem.CodePermissionRequired, "/patch/catalog.work.credits", problem.ReasonNotPermitted,
		},
		{
			&editing.UnknownFieldError{Key: "catalog.work.nope"},
			problem.CodeValidationFailed, "/patch/catalog.work.nope", problem.ReasonUnknownValue,
		},
		{
			&editing.LockedFieldError{Key: "catalog.work.olang"},
			problem.CodeValidationFailed, "/patch/catalog.work.olang", problem.ReasonImmutable,
		},
		{
			&editing.ConflictError{Keys: []string{"catalog.work.titles"}},
			problem.CodeValidationFailed, "/patch/catalog.work.titles", problem.ReasonInconsistentWith,
		},
	}
	for _, c := range cases {
		p, ok := proposalErr(c.err).(*problem.Problem)
		require.True(t, ok, "%T", c.err)
		require.Equal(t, c.code, p.Code, "%T", c.err)
		require.Len(t, p.Errors, 1, "%T", c.err)
		require.Equal(t, c.pointer, p.Errors[0].Pointer, "%T", c.err)
		require.Equal(t, c.reason, p.Errors[0].Reason, "%T", c.err)
		require.NotEmpty(t, p.Errors[0].Detail, "%T", c.err)
	}
}
