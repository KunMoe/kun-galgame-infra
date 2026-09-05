package price

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/japanese"
)

const getchuPricedPage = `<html><body>
<table><tr><td>定価：</td>
    <td align="top">￥3,800 (税込￥4,180)</td>
</tr></table>
<div class="cart_block3"><nobr><span>価格（税込）：</span><span class="redb2">￥3,970</span></nobr><!--<span class="taxin">￥3,609</span>--><br><button onclick="location.href='https://ssl.getchu.com/comike/cart.phtml?gc=gc&article_id=1371244'">カートに入れる</button></div>
</body></html>`

const getchuSuspendedPage = `<html><body>
<table><tr><td>定価：</td>
    <td align="top">￥8,800 (税込￥9,680)</td>
</tr></table>
<div class="cart_block3">ご注文の受付は停止中です</div>
</body></html>`

func eucJP(t *testing.T, s string) []byte {
	t.Helper()
	b, err := japanese.EUCJP.NewEncoder().Bytes([]byte(s))
	require.NoError(t, err)
	return b
}

func TestGetchuFetch(t *testing.T) {
	var gotPath, gotCookie, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotCookie = r.Header.Get("Cookie")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html; charset=EUC-JP")
		_, _ = w.Write(eucJP(t, getchuPricedPage))
	}))
	t.Cleanup(srv.Close)

	f := NewGetchu(srv.URL)
	out, err := f.Fetch(context.Background(), "jp", []string{"1371244"})
	require.NoError(t, err)
	require.Equal(t, "/item/1371244/", gotPath)
	require.Equal(t, "getchu_adalt_flag=getchu.com", gotCookie)
	require.Equal(t, "NextMoe-PriceBot/1.0 (+https://www.kungal.com)", gotUA)
	up, ok := out["1371244"]
	require.True(t, ok)
	require.True(t, up.Found)
	require.Equal(t, "JPY", up.Currency)
	require.Equal(t, int64(418000), up.ListMinor)
	require.Equal(t, int64(397000), up.CurrentMinor)
	require.Equal(t, 5, up.DiscountPercent)
	require.Equal(t, "https://www.getchu.com/item/1371244/", up.URL)
	require.Nil(t, up.SaleEndsAt)
	require.Empty(t, up.Converted)
}

func TestGetchuCartPriceWithoutListRow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(eucJP(t, `<html><body><div class="cart_block3"><nobr><span>価格（税込）：</span><span class="redb2">￥2,090</span></nobr></div></body></html>`))
	}))
	t.Cleanup(srv.Close)
	f := NewGetchu(srv.URL)
	out, err := f.Fetch(context.Background(), "jp", []string{"1371246"})
	require.NoError(t, err)
	up := out["1371246"]
	require.True(t, up.Found)
	require.Equal(t, int64(209000), up.ListMinor)
	require.Equal(t, int64(209000), up.CurrentMinor)
	require.Zero(t, up.DiscountPercent)
}

func TestGetchuOrderingSuspended(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(eucJP(t, getchuSuspendedPage))
	}))
	t.Cleanup(srv.Close)
	f := NewGetchu(srv.URL)
	out, err := f.Fetch(context.Background(), "jp", []string{"16084"})
	require.NoError(t, err)
	up, ok := out["16084"]
	require.True(t, ok)
	require.False(t, up.Found)
	require.Equal(t, "https://www.getchu.com/item/16084/", up.URL)
}

func TestGetchuNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	f := NewGetchu(srv.URL)
	out, err := f.Fetch(context.Background(), "jp", []string{"7678960"})
	require.NoError(t, err)
	up, ok := out["7678960"]
	require.True(t, ok)
	require.False(t, up.Found)
	require.Equal(t, "https://www.getchu.com/item/7678960/", up.URL)
}

func TestGetchuAgeGateRedirectIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://www.getchu.com/php/attestation.html?aurl="+r.URL.String())
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	f := NewGetchu(srv.URL)
	_, err := f.Fetch(context.Background(), "jp", []string{"1371244"})
	require.ErrorContains(t, err, "status 302")
}

func TestGetchuUnrecognizedPageIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body>maintenance</body></html>`))
	}))
	t.Cleanup(srv.Close)
	f := NewGetchu(srv.URL)
	_, err := f.Fetch(context.Background(), "jp", []string{"1371244"})
	require.ErrorContains(t, err, "no price markup")
}

func TestGetchuAccepts(t *testing.T) {
	f := NewGetchu("")
	require.True(t, f.Accepts("1371244"))
	require.False(t, f.Accepts("RJ149770"))
	require.False(t, f.Accepts(""))
	require.Equal(t, "getchu", f.Source())
	require.Equal(t, []string{"jp"}, f.Regions())
	require.Equal(t, 1, f.Batch())
}
