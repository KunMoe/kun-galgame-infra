package service

import (
	"context"
	"strings"

	"api/internal/platform/catalog/model"
)

var DisplayLimitTokens = []string{model.DisplayLimitKeySFW, model.DisplayLimitKeyNSFW}

func IsDisplayLimit(tok string) bool {
	for _, v := range DisplayLimitTokens {
		if tok == v {
			return true
		}
	}
	return false
}

const displayNSFWSQL = `w.display_nsfw`

func displayLimitWhere(limits []string) (string, []any) {
	if len(limits) == 0 {
		return "", nil
	}
	ors := make([]string, 0, len(limits))
	args := make([]any, 0, len(limits))
	for _, lim := range limits {
		switch lim {
		case model.DisplayLimitKeyNSFW:
			ors = append(ors, "("+claimedSQL+" AND "+displayNSFWSQL+") OR (NOT "+claimedSQL+" AND w.content_rating = ?)")
			args = append(args, model.ContentRatingR18)
		case model.DisplayLimitKeySFW:
			ors = append(ors, "("+claimedSQL+" AND NOT "+displayNSFWSQL+") OR (NOT "+claimedSQL+" AND w.content_rating <> ?)")
			args = append(args, model.ContentRatingR18)
		default:
			ors = append(ors, "false")
		}
	}
	return "((" + strings.Join(ors, ") OR (") + "))", args
}

// The cover gates must classify a work exactly as claimed_by.content_limit
// does: gating on the raw column left unclaimed r18 works (display_nsfw is
// never edited for them) hiding their covers even from nsfw viewers.
func effectiveDisplayNSFW(site *string, productWorkID *int64, displayNSFW bool, contentRating int16) bool {
	return model.DisplayLimitKey(site, productWorkID, displayNSFW, contentRating) == model.DisplayLimitKeyNSFW
}

func (s *ReadService) loadDisplayNSFW(ctx context.Context, subjects []claimSubject) (map[int64]bool, error) {
	out := make(map[int64]bool, len(subjects))
	if len(subjects) == 0 {
		return out, nil
	}
	workIDs := make([]int64, 0, len(subjects))
	for _, sub := range subjects {
		workIDs = append(workIDs, sub.WorkID)
	}
	var ids []int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT id FROM catalog_work WHERE id IN ? AND display_nsfw`, workIDs).Scan(&ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}
