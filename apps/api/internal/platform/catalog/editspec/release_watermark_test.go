package editspec_test

import (
	"testing"

	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/model"
)

func TestReleaseEditTouchesParentWork(t *testing.T) {
	e := newReleaseEngine(t)
	work := createWork(t, "親作品")
	rel := createReleaseOn(t, work.ID)

	if err := testDB.Exec(
		`UPDATE catalog_work SET updated_at = now() - interval '1 hour' WHERE id = ?`, work.ID).Error; err != nil {
		t.Fatal(err)
	}
	var before model.CatalogWork
	if err := testDB.First(&before, work.ID).Error; err != nil {
		t.Fatal(err)
	}

	mergeOn(t, e, editspec.TypeRelease, rel.ID, map[string]any{
		editspec.FieldReleaseReleased: map[string]any{"y": float64(2021), "m": float64(6)},
	})

	got := reloadRelease(t, rel.ID)
	if got.ReleasedY == nil || *got.ReleasedY != 2021 {
		t.Fatalf("the edit must actually have landed, got %v", got.ReleasedY)
	}

	var after model.CatalogWork
	if err := testDB.First(&after, work.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("release_date rides the public work face, but the parent watermark did not move: %v -> %v",
			before.UpdatedAt, after.UpdatedAt)
	}
}
