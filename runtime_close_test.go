package qjs_test

import (
	"runtime"
	"testing"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each New instantiates host modules into the shared compilation cache under a
// pointer-derived ID; they are only released when the wazero runtime closes.
func TestRuntimeCloseReleasesHostModules(t *testing.T) {
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapInuse
	}
	run := func() {
		rt, err := qjs.New()
		require.NoError(t, err)
		v, err := rt.Context().Eval("t.js", qjs.Code(`1+1`))
		require.NoError(t, err)
		v.Free()
		rt.Close()
	}

	run()
	before := heap()
	for range 200 {
		run()
	}
	assert.Less(t, int64(heap())-int64(before), int64(2<<20), "heap grew across New/Close cycles")
}
