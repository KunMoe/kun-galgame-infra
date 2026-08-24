package handler

import (
	"path"
	"strings"

	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/dto"
)

func workFromListItem(it dto.PublicWorkListItem) repr.Work {
	var cover, banner *repr.Image
	if it.Covers != nil {
		cover = imageFromSlot(it.Covers.Portrait)
		banner = imageFromSlot(it.Covers.Banner)
	}
	if cover == nil {
		cover = imageFromURL(it.Cover, "")
	}
	w, _ := repr.NewWork(
		it.ID, it.Medium, it.DisplayName, it.OLang, it.ContentRating, releaseStatus(it.ReleaseDate),
		it.Updated, it.Updated, optString(it.Latin), localizedFrom(it.Localized),
		it.ReleaseDate, releasePrecision(it.ReleaseDate), cover, banner, claimFrom(it.ClaimedBy),
	)
	return w
}

func workFromDetail(rec dto.PublicCatalogWork, include []string) repr.Work {
	var cover, banner *repr.Image
	if rec.CoverSlots != nil {
		cover = imageFromSlot(rec.CoverSlots.Portrait)
		banner = imageFromSlot(rec.CoverSlots.Banner)
	}
	created := rec.Created
	if created == "" {
		created = rec.Updated
	}
	w, _ := repr.NewWork(
		rec.ID, rec.Medium, rec.DisplayName, rec.OLang, rec.ContentRating, releaseStatus(rec.ReleaseDate),
		created, rec.Updated, optString(rec.Latin), localizedFrom(rec.Localized),
		rec.ReleaseDate, releasePrecision(rec.ReleaseDate), cover, banner, claimFrom(rec.ClaimedBy),
	)
	attachWorkIncludes(&w, rec, include)
	return w
}

func attachWorkIncludes(w *repr.Work, rec dto.PublicCatalogWork, include []string) {
	want := map[string]bool{}
	for _, t := range include {
		want[t] = true
	}
	if want["titles"] {
		w.Titles = ptrSlice(cap100(titlesFrom(rec.Titles)))
	}
	if want["refs"] {
		w.Refs = ptrSlice(cap100(refsFrom(rec.Refs)))
	}
	if want["relations"] {
		w.Relations = ptrSlice(cap100(relationsFrom(rec.Relations)))
	}
	if want["credits"] {
		w.Credits = ptrSlice(cap100(creditGroupsFrom(rec.Credits)))
	}
	if want["releases"] {
		w.Releases = ptrSlice(cap100(releasesFrom(rec.Releases)))
	}
	if want["popularity"] {
		w.Popularity = ptrSlice(cap100(popularityFrom(rec.Popularity)))
	}
	if want["ratings"] {
		w.Ratings = ptrSlice(cap100(ratingsFrom(rec.Ratings)))
	}
	if want["tags"] {
		w.Tags = ptrSlice(cap100(workTagsFrom(rec.Tags)))
	}
	if want["playtimes"] {
		w.Playtimes = ptrSlice(cap100(playtimesFrom(rec.Playtimes)))
	}
	if want["series"] {
		w.Series = ptrSlice(cap100(workSeriesFrom(rec.Series)))
	}
	if want["platforms"] {
		w.Platforms = ptrSlice(cap100(platformsFrom(rec.Platforms)))
	}
	if want["intros"] {
		w.Intros = ptrSlice(cap100(introsFrom(rec.Intros)))
	}
	if want["covers"] {
		w.Covers = ptrSlice(cap100(coversFrom(rec.Covers)))
	}
	if want["screenshots"] {
		w.Screenshots = ptrSlice(cap100(screenshotsFrom(rec.Screenshots)))
	}
	if want["characters"] {
		w.Characters = ptrSlice(cap100(workCharactersFrom(rec.Characters)))
	}
	if want["companies"] {
		w.Companies = ptrSlice(cap100(workCompaniesFrom(rec.Labels)))
	}
	if want["engines"] {
		w.Engines = ptrSlice(cap100(workEnginesFrom(rec.Engines)))
	}
	if want["links"] {
		w.Links = ptrSlice(cap100(workLinksFrom(rec.Links)))
	}
}

func claimFrom(c *dto.PublicClaimedBy) *repr.Claim {
	if c == nil {
		return nil
	}
	return &repr.Claim{
		Site: c.Site, SiteWorkID: repr.ID(c.WorkID), State: c.State, ContentLimit: c.ContentLimit,
	}
}

func localizedFrom(in map[string]dto.PublicLocalizedName) map[string]repr.LocalizedText {
	out := map[string]repr.LocalizedText{}
	for k, v := range in {
		out[k] = repr.LocalizedText{Value: v.Value, IsMachine: v.Machine}
	}
	return out
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func releaseStatus(date *string) string {
	if date == nil || *date == "" {
		return "unknown"
	}
	return "released"
}

func releasePrecision(date *string) *string {
	if date == nil || *date == "" {
		return nil
	}
	p := "day"
	return &p
}

func imageFromSlot(slot *dto.PublicCoverSlot) *repr.Image {
	if slot == nil {
		return nil
	}
	return imageFromURLMeta(slot.URL, slot.Source, slot.Width, slot.Height, slot.Thumbhash, slot.Sexual)
}

func imageFromURL(url, source string) *repr.Image {
	return imageFromURLMeta(url, source, 0, 0, "", 0)
}

func imageFromURLMeta(url, source string, width, height int, thumbhash string, sexual int16) *repr.Image {
	h := hashFromURL(url)
	if url == "" || h == "" {
		return nil
	}
	img := &repr.Image{URL: url, Hash: h, Source: source}
	if width > 0 {
		w := width
		img.Width = &w
	}
	if height > 0 {
		ht := height
		img.Height = &ht
	}
	if thumbhash != "" {
		th := thumbhash
		img.Thumbhash = &th
	}
	if width > 0 || height > 0 || thumbhash != "" || sexual != 0 {
		if sx, ok := repr.Sexual(&sexual); ok {
			img.Sexual = sx
		}
	}
	return img
}

func hashFromURL(url string) string {
	base := path.Base(url)
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	if len(base) == 64 {
		return base
	}
	return ""
}
