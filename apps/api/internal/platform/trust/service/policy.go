package service

import (
	"sync"
	"time"

	"api/internal/platform/settings/keys"
	"api/internal/platform/trust/model"

	"gorm.io/gorm"
)

type PlatformDefaults struct {
	ScanMode           int16
	SampleRate         float64
	AggregateThreshold float32
	AutoHideEnabled    bool
}

type ResolvedPolicy struct {
	ScanMode           int16
	SampleRate         float64
	FlagThreshold      *float32
	AggregateThreshold float32
	AutoHideEnabled    bool
}

type DefaultsSource func() PlatformDefaults

type PolicyService struct {
	db  *gorm.DB
	src DefaultsSource
	now func() time.Time

	mu       sync.Mutex
	cache    map[string]model.TrustSitePolicy
	loaded   bool
	loadedAt time.Time
}

func DefaultAggregateThreshold() float32 {
	return float32(keys.TrustAggregateThreshold.Get())
}

func NewPolicyService(db *gorm.DB, defaults PlatformDefaults) *PolicyService {
	return NewPolicyServiceFrom(db, func() PlatformDefaults { return defaults })
}

func NewPolicyServiceFrom(db *gorm.DB, src DefaultsSource) *PolicyService {
	return &PolicyService{
		db:  db,
		src: src,
		now: time.Now,
	}
}

func (s *PolicyService) Defaults() PlatformDefaults { return s.src() }

func (s *PolicyService) Resolve(site string) ResolvedPolicy {
	d := s.Defaults()
	resolved := ResolvedPolicy{
		ScanMode:           d.ScanMode,
		SampleRate:         d.SampleRate,
		AggregateThreshold: d.AggregateThreshold,
		AutoHideEnabled:    d.AutoHideEnabled,
	}
	row, ok := s.lookup(site)
	if !ok {
		return resolved
	}
	if row.ScanMode != nil {
		resolved.ScanMode = *row.ScanMode
	}
	if row.SampleRate != nil {
		resolved.SampleRate = *row.SampleRate
	}
	if row.AggregateThreshold != nil {
		resolved.AggregateThreshold = *row.AggregateThreshold
	}
	if row.AutoHideEnabled != nil {
		resolved.AutoHideEnabled = *row.AutoHideEnabled
	}
	resolved.FlagThreshold = row.FlagThreshold
	return resolved
}

func (s *PolicyService) lookup(site string) (model.TrustSitePolicy, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded || s.now().Sub(s.loadedAt) >= time.Duration(keys.TrustPolicyCacheTTLSeconds.Get())*time.Second {
		var rows []model.TrustSitePolicy
		if err := s.db.Find(&rows).Error; err != nil {
			if s.loaded {
				row, ok := s.cache[site]
				return row, ok
			}
			return model.TrustSitePolicy{}, false
		}
		cache := make(map[string]model.TrustSitePolicy, len(rows))
		for _, r := range rows {
			cache[r.Site] = r
		}
		s.cache, s.loaded, s.loadedAt = cache, true, s.now()
	}
	row, ok := s.cache[site]
	return row, ok
}

func (s *PolicyService) Invalidate() {
	s.mu.Lock()
	s.loaded = false
	s.mu.Unlock()
}

func (s *PolicyService) List() ([]model.TrustSitePolicy, error) {
	var rows []model.TrustSitePolicy
	if err := s.db.Order("site").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *PolicyService) Upsert(p *model.TrustSitePolicy) error {
	now := s.now()
	p.UpdatedAt = now
	if err := s.db.Exec(`
		INSERT INTO trust_site_policy
		    (site, scan_mode, sample_rate, flag_threshold, aggregate_threshold,
		     auto_hide_enabled, note, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (site) DO UPDATE SET
		    scan_mode           = EXCLUDED.scan_mode,
		    sample_rate         = EXCLUDED.sample_rate,
		    flag_threshold      = EXCLUDED.flag_threshold,
		    aggregate_threshold = EXCLUDED.aggregate_threshold,
		    auto_hide_enabled   = EXCLUDED.auto_hide_enabled,
		    note                = EXCLUDED.note,
		    updated_at          = EXCLUDED.updated_at`,
		p.Site, p.ScanMode, p.SampleRate, p.FlagThreshold, p.AggregateThreshold,
		p.AutoHideEnabled, p.Note, now, now).Error; err != nil {
		return err
	}
	s.Invalidate()
	return nil
}

func MaxScanSampleRate() float64 { return maxScanSampleRate }
