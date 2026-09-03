package llmsuggest

import (
	"strconv"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

func loadWorkSides(db *gorm.DB, ids []int64) (map[int64]workSideDossier, error) {
	out := map[int64]workSideDossier{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, chunk := range chunkBy(ids, 500) {
		var works []struct {
			ID            int64   `gorm:"column:id"`
			DisplayName   string  `gorm:"column:display_name"`
			OLang         string  `gorm:"column:olang"`
			ContentRating int16   `gorm:"column:content_rating"`
			Site          *string `gorm:"column:site"`
			ClaimState    *int16  `gorm:"column:claim_state"`
			MediumKey     string  `gorm:"column:medium_key"`
		}
		if err := db.Raw(`SELECT w.id, w.display_name, w.olang, w.content_rating, w.site, w.claim_state, m.key AS medium_key
			FROM catalog_work w JOIN catalog_medium m ON m.id = w.medium_id
			WHERE w.id IN ?`, chunk).Scan(&works).Error; err != nil {
			return nil, err
		}
		for _, w := range works {
			side := workSideDossier{
				ID: w.ID, DisplayName: w.DisplayName, MediumKey: w.MediumKey,
				OLang: w.OLang, ContentRating: w.ContentRating,
				Titles: []workTitleEv{}, Refs: []string{}, Labels: []string{},
			}
			if siteClaimed(w.Site) {
				side.Site = derefStr(w.Site)
				side.ClaimState = w.ClaimState
			}
			out[w.ID] = side
		}

		var titles []struct {
			WorkID int64  `gorm:"column:work_id"`
			Title  string `gorm:"column:title"`
			Lang   string `gorm:"column:lang"`
			Kind   int16  `gorm:"column:kind"`
			Norm   string `gorm:"column:title_norm"`
		}
		if err := db.Raw(`SELECT work_id, title, lang, kind, title_norm
			FROM catalog_work_title WHERE work_id IN ? ORDER BY work_id, kind, id`, chunk).
			Scan(&titles).Error; err != nil {
			return nil, err
		}
		titleN := map[int64]int{}
		for _, t := range titles {
			if titleN[t.WorkID] >= 5 {
				continue
			}
			side := out[t.WorkID]
			side.Titles = append(side.Titles, workTitleEv{
				Title: t.Title, Lang: t.Lang, Official: t.Kind == model.WorkTitleKindOfficial, Norm: t.Norm,
			})
			titleN[t.WorkID]++
			out[t.WorkID] = side
		}

		var years []struct {
			WorkID int64 `gorm:"column:work_id"`
			Y      int   `gorm:"column:y"`
		}
		if err := db.Raw(`SELECT work_id, min(released_y) AS y FROM catalog_release
			WHERE deleted_at IS NULL AND released_y IS NOT NULL AND work_id IN ?
			GROUP BY work_id`, chunk).Scan(&years).Error; err != nil {
			return nil, err
		}
		for _, y := range years {
			side := out[y.WorkID]
			yy := y.Y
			side.MinReleaseYear = &yy
			out[y.WorkID] = side
		}

		var refs []struct {
			EntityID   int64  `gorm:"column:entity_id"`
			SourceKey  string `gorm:"column:source_key"`
			ExternalID string `gorm:"column:external_id"`
		}
		if err := db.Raw(`SELECT r.entity_id, s.key AS source_key, r.external_id
			FROM catalog_external_ref r
			JOIN catalog_source s ON s.id = r.source_id
			WHERE r.entity_type = ? AND r.link_kind IN (?, ?) AND r.dead_at IS NULL AND r.entity_id IN ?
			ORDER BY r.entity_id, r.link_kind, s.key, r.external_id`,
			model.EntityTypeWork, model.LinkKindExact, model.LinkKindProbable, chunk).
			Scan(&refs).Error; err != nil {
			return nil, err
		}
		refN := map[int64]int{}
		for _, r := range refs {
			if refN[r.EntityID] >= 8 {
				continue
			}
			side := out[r.EntityID]
			side.Refs = append(side.Refs, r.SourceKey+":"+r.ExternalID)
			refN[r.EntityID]++
			out[r.EntityID] = side
		}

		var labels []struct {
			WorkID int64  `gorm:"column:work_id"`
			Name   string `gorm:"column:display_name"`
		}
		if err := db.Raw(`SELECT wl.work_id, l.display_name
			FROM catalog_work_label wl
			JOIN catalog_label l ON l.id = wl.label_id AND l.deleted_at IS NULL
			WHERE wl.work_id IN ? ORDER BY wl.work_id, l.display_name`, chunk).
			Scan(&labels).Error; err != nil {
			return nil, err
		}
		labN := map[int64]int{}
		seenLab := map[string]struct{}{}
		for _, l := range labels {
			key := strconvWorkLabel(l.WorkID, l.Name)
			if _, ok := seenLab[key]; ok {
				continue
			}
			seenLab[key] = struct{}{}
			if labN[l.WorkID] >= 3 {
				continue
			}
			side := out[l.WorkID]
			side.Labels = append(side.Labels, l.Name)
			labN[l.WorkID]++
			out[l.WorkID] = side
		}
	}
	return out, nil
}

func strconvWorkLabel(id int64, name string) string {
	return strconv.FormatInt(id, 10) + "\x00" + name
}
