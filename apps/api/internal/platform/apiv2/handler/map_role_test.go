package handler

import (
	"testing"

	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/dto"

	"github.com/stretchr/testify/require"
)

func TestRoleFromListItem(t *testing.T) {
	full := roleFromListItem(dto.PublicRoleListItem{
		ID: 3, Key: "原画", Category: "art",
		NameCN: "原画", NameJA: "原画さん", NameEN: "Original Art",
	})
	require.Equal(t, "role", full.Object)
	require.Equal(t, "3", full.ID)
	require.Equal(t, "原画", full.Key)
	require.Equal(t, "art", full.Category)
	require.Equal(t, "原画", full.DisplayName)
	require.Equal(t, map[string]bool{"zh-Hans": true, "ja": true, "en": true}, keysOf(full.Localized))
	require.Equal(t, "原画さん", full.Localized["ja"].Value)
	require.False(t, full.Localized["ja"].IsMachine)
	require.False(t, full.Deprecated)

	ja := roleFromListItem(dto.PublicRoleListItem{ID: 4, Key: "2d-works", NameJA: "2D作画"})
	require.Equal(t, "2D作画", ja.DisplayName)
	require.Equal(t, map[string]bool{"ja": true}, keysOf(ja.Localized))

	bare := roleFromListItem(dto.PublicRoleListItem{ID: 5, Key: "3dcg", Deprecated: true})
	require.Equal(t, "3dcg", bare.DisplayName)
	require.NotNil(t, bare.Localized)
	require.Empty(t, bare.Localized)
	require.True(t, bare.Deprecated)
}

func keysOf(m map[string]repr.LocalizedText) map[string]bool {
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}
