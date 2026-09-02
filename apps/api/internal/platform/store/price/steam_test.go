package price

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSteamFetch(t *testing.T) {
	const body = `{
		"1680880":{"success":true,"data":{"price_overview":{"currency":"JPY","initial":968000,"final":968000,"discount_percent":0,"initial_formatted":"","final_formatted":"¥ 9,680"}}},
		"2598690":{"success":true,"data":[]},
		"999999999":{"success":false}
	}`
	var gotPath, gotAppIDs, gotCC, gotFilters string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		q := r.URL.Query()
		gotAppIDs = q.Get("appids")
		gotCC = q.Get("cc")
		gotFilters = q.Get("filters")
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	f := NewSteam("Test-UA/1.0", []string{"jp"}, srv.URL)
	out, err := f.Fetch(context.Background(), "jp", []string{"1680880", "2598690", "999999999"})
	require.NoError(t, err)
	require.Equal(t, "/api/appdetails", gotPath)
	require.Equal(t, "1680880,2598690,999999999", gotAppIDs)
	require.Equal(t, "jp", gotCC)
	require.Equal(t, "price_overview", gotFilters)

	found, ok := out["1680880"]
	require.True(t, ok)
	require.True(t, found.Found)
	require.Equal(t, int64(968000), found.ListMinor)
	require.Equal(t, int64(968000), found.CurrentMinor)
	require.Equal(t, "JPY", found.Currency)

	free, ok := out["2598690"]
	require.True(t, ok)
	require.False(t, free.Found)

	missing, ok := out["999999999"]
	require.True(t, ok)
	require.False(t, missing.Found)
}
