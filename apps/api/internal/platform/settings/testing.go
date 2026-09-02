package settings

import "testing"

func Override[T any](tb testing.TB, k *Key[T], v T) {
	tb.Helper()
	prev := k.current.Load()
	k.apply(v, SourceDB)
	tb.Cleanup(func() {
		k.current.Store(prev)
	})
}
