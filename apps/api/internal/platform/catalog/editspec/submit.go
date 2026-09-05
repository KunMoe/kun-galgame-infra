package editspec

import (
	"context"
	"fmt"
	"strings"

	catmodel "api/internal/platform/catalog/model"
	"api/internal/platform/editing"

	"gorm.io/gorm"
)

var submissionFields = map[string]struct{}{
	FieldWorkDisplayName:   {},
	FieldWorkOLang:         {},
	FieldWorkContentRating: {},
	FieldWorkTitles:        {},
	FieldWorkIntros:        {},
	FieldWorkDisplayNSFW:   {},
	FieldWorkTagIDs:        {},
	FieldWorkLabels:        {},
	FieldWorkEngineIDs:     {},
	FieldWorkSeriesIDs:     {},
	FieldWorkLinks:         {},
}

func SubmissionFieldKeys() []string {
	out := make([]string, 0, len(submissionFields))
	for _, f := range workFieldSpecs() {
		if _, ok := submissionFields[f.Key]; ok {
			out = append(out, f.Key)
		}
	}
	return out
}

type SubmissionFieldError struct {
	Field string
	Err   error
}

func (e *SubmissionFieldError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s is not a submittable field (accepted: %s)",
			e.Field, strings.Join(SubmissionFieldKeys(), ", "))
	}
	return fmt.Sprintf("%s: %v", e.Field, e.Err)
}

func (e *SubmissionFieldError) Unwrap() error { return e.Err }

type SubmissionAnchor struct {
	SourceKey  string
	ExternalID string
}

// SubmissionTitleStrings extracts the title strings from a submitted
// catalog.work.titles value without validating it — the mint's duplicate gate
// wants every name the caller asserted, and a shape error is still caught by
// parseTitles when the fields are applied.
func SubmissionTitleStrings(value any) []string {
	arr, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		obj, ok := el.(map[string]any)
		if !ok {
			continue
		}
		if title, ok := obj["title"].(string); ok && strings.TrimSpace(title) != "" {
			out = append(out, title)
		}
	}
	return out
}

func SubmissionAnchorsOf(value any) []SubmissionAnchor {
	urls, err := parseLinks(value)
	if err != nil {
		return nil
	}
	out := make([]SubmissionAnchor, 0, len(urls))
	for _, u := range urls {
		cl, ok := classifyWorkLink(u)
		if !ok || cl.LinkKind != catmodel.LinkKindProbable {
			continue
		}
		out = append(out, SubmissionAnchor{SourceKey: cl.SourceKey, ExternalID: cl.ExternalID})
	}
	return out
}

func ApplyWorkFields(ctx context.Context, tx *gorm.DB, workID int64, values map[string]any) error {
	for key := range values {
		if _, ok := submissionFields[key]; !ok {
			return &SubmissionFieldError{Field: key}
		}
	}
	for _, spec := range workFieldSpecs() {
		value, present := values[spec.Key]
		if !present {
			continue
		}
		if spec.Validate != nil {
			if err := spec.Validate(value); err != nil {
				return &SubmissionFieldError{Field: spec.Key, Err: err}
			}
		}
		if err := editing.ApplyField(ctx, tx, &spec, workID, value); err != nil {
			return err
		}
	}
	return nil
}
