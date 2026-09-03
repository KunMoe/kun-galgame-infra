package osengine

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func canonJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var norm any
	require.NoError(t, json.Unmarshal(raw, &norm))
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	require.NoError(t, enc.Encode(norm))
	return buf.String()
}

func TestIndexBodiesGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/index_bodies.golden.json")
	require.NoError(t, err)
	var want map[string]any
	require.NoError(t, json.Unmarshal(raw, &want))
	require.Len(t, want, 8)
	require.Len(t, IndexUIDs, 8)
	for _, uid := range IndexUIDs {
		require.Contains(t, want, uid)
		got, err := IndexBody(uid)
		require.NoError(t, err)
		require.Equal(t, canonJSON(t, want[uid]), canonJSON(t, got), uid)
	}
}

func TestQueriesGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/queries.golden.json")
	require.NoError(t, err)
	var cases []struct {
		Name        string          `json:"name"`
		Q           string          `json:"q"`
		Limit       int             `json:"limit"`
		Entity      bool            `json:"entity"`
		SearchIntro bool            `json:"search_intro"`
		Body        json.RawMessage `json:"body"`
		Index       string          `json:"index"`
	}
	require.NoError(t, json.Unmarshal(raw, &cases))
	require.Len(t, cases, 43)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var got map[string]any
			if tc.Entity {
				got = EntityBody(tc.Q, tc.Limit)
			} else {
				got = WorksBody(tc.Q, tc.Limit, tc.SearchIntro)
			}
			var want any
			require.NoError(t, json.Unmarshal(tc.Body, &want))
			require.Equal(t, canonJSON(t, want), canonJSON(t, got))
		})
	}
}
