package service

import (
	"context"
	"fmt"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
)

type WorkService struct {
	db      *gorm.DB
	resolve *ResolveService
}

func NewWorkService(db *gorm.DB, resolve *ResolveService) *WorkService {
	return &WorkService{db: db, resolve: resolve}
}

type ExternalAnchor struct {
	SourceID   int16
	ExternalID string
	MatchedBy  string
	EntityType int16
}

func (a ExternalAnchor) isReleaseLevel() bool { return a.EntityType == model.EntityTypeRelease }

type ClaimWorkParams struct {
	MediumID       int16
	Site           string
	ProductWorkID  int64
	DisplayName    string
	OLang          string
	ContentRating  int16
	Anchors        []ExternalAnchor
	ActorID        *int64
	RejectedAnchor func(workID int64, sourceID int16, externalID string) bool
}

type ConflictError struct {
	WorkID              int64
	OwningSite          string
	OwningProductWorkID *int64
}

func (e *ConflictError) Error() string {
	if e.OwningProductWorkID != nil {
		return fmt.Sprintf("%s: work %d is claimed by %s/%d", ErrClaimConflict.Error(), e.WorkID, e.OwningSite, *e.OwningProductWorkID)
	}
	return fmt.Sprintf("%s: work %d is claimed by %s", ErrClaimConflict.Error(), e.WorkID, e.OwningSite)
}

func (e *ConflictError) Is(target error) bool { return target == ErrClaimConflict }

func (s *WorkService) ClaimWork(ctx context.Context, params ClaimWorkParams) (int64, bool, error) {
	if params.OLang == "" {
		params.OLang = "ja"
	}
	var workID int64
	var created bool
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := repository.FindClaimed(tx, params.MediumID, params.Site, params.ProductWorkID)
		if err != nil {
			return err
		}
		if existing != nil {
			workID = existing.ID
			return nil
		}

		rejectedInAdopt := make(map[int]bool)
		for ai, anchor := range params.Anchors {
			id, found, err := repository.FindWorkIDByAnchor(tx, anchor.SourceID, anchor.ExternalID)
			if err != nil {
				return err
			}
			if !found {
				continue
			}
			canonical, err := repository.ResolveTx(tx, model.EntityTypeWork, id)
			if err != nil {
				return err
			}
			var w model.CatalogWork
			if err := tx.First(&w, canonical).Error; err != nil {
				return err
			}
			if params.RejectedAnchor != nil && params.RejectedAnchor(w.ID, anchor.SourceID, anchor.ExternalID) {
				rejectedInAdopt[ai] = true
				continue
			}
			if w.Site != nil {
				if *w.Site == params.Site && w.ProductWorkID != nil && *w.ProductWorkID == params.ProductWorkID {
					workID = w.ID
					return nil
				}
				return &ConflictError{WorkID: w.ID, OwningSite: *w.Site, OwningProductWorkID: w.ProductWorkID}
			}
			updates := map[string]any{
				"site":            params.Site,
				"product_work_id": params.ProductWorkID,
				"status":          model.WorkStatusLive,
			}
			if w.DisplayName == "" && params.DisplayName != "" {
				updates["display_name"] = params.DisplayName
			}
			if err := tx.Model(&w).Updates(updates).Error; err != nil {
				return err
			}
			workID = w.ID
			return nil
		}

		rating := params.ContentRating
		if rating == 0 && len(params.Anchors) > 0 {
			rating = s.deriveAnchorRating(ctx, params.Anchors)
		}
		w := model.CatalogWork{
			MediumID:        params.MediumID,
			Site:            &params.Site,
			ProductWorkID:   &params.ProductWorkID,
			OLang:           params.OLang,
			DisplayName:     params.DisplayName,
			ContentRating:   rating,
			Status:          model.WorkStatusLive,
			Extra:           []byte(`{}`),
			FieldProvenance: []byte(`{}`),
		}
		if err := tx.Create(&w).Error; err != nil {
			return err
		}
		var releaseID int64
		for ai, anchor := range params.Anchors {
			if rejectedInAdopt[ai] {
				continue
			}
			entityType, entityID := model.EntityTypeWork, w.ID
			if anchor.isReleaseLevel() {
				if releaseID == 0 {
					rel := model.CatalogRelease{WorkID: w.ID, Kind: model.ReleaseKindDefault, Extra: []byte(`{}`)}
					if err := tx.Create(&rel).Error; err != nil {
						return err
					}
					releaseID = rel.ID
				}
				entityType, entityID = model.EntityTypeRelease, releaseID
			}
			ref := model.CatalogExternalRef{
				EntityType: entityType,
				EntityID:   entityID,
				SourceID:   anchor.SourceID,
				ExternalID: anchor.ExternalID,
				LinkKind:   model.LinkKindExact,
				MatchedBy:  anchor.MatchedBy,
			}
			if err := tx.Create(&ref).Error; err != nil {
				return err
			}
		}
		snap, err := takeSnapshot(tx, model.EntityTypeWork, w.ID)
		if err != nil {
			return err
		}
		if err := writeRevision(tx, model.EntityTypeWork, w.ID, model.RevisionActionCreated, snap, nil, params.ActorID,
			fmt.Sprintf("claimed by %s/%d", params.Site, params.ProductWorkID)); err != nil {
			return err
		}
		workID = w.ID
		created = true
		return nil
	})
	return workID, created, err
}
