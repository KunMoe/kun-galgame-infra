package imageclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetaBatchDecodesSexualAndKeepsUngradedNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"metas":{
			"aa":{"width":800,"height":600,"thumbhash":"th","sexual":2},
			"bb":{"width":100,"height":100,"thumbhash":"th2","sexual":0},
			"cc":{"width":10,"height":10}
		}}}`))
	}))
	defer srv.Close()

	metas, err := New(Config{BaseURL: srv.URL}).MetaBatch(context.Background(), []string{"aa", "bb", "cc"})
	if err != nil {
		t.Fatalf("meta-batch: %v", err)
	}
	if metas["aa"].Sexual == nil || *metas["aa"].Sexual != 2 {
		t.Fatalf("aa sexual = %v, want 2", metas["aa"].Sexual)
	}
	if metas["bb"].Sexual == nil || *metas["bb"].Sexual != 0 {
		t.Fatalf("bb sexual = %v, want an explicit 0", metas["bb"].Sexual)
	}
	if metas["cc"].Sexual != nil {
		t.Fatalf("cc sexual = %v, want nil for an ungraded image", *metas["cc"].Sexual)
	}
}
