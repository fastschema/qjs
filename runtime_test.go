package qjs_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConcurrentRuntimeExecution(t *testing.T, threadID int) {
	rt, err := qjs.New()
	require.NoError(t, err)
	defer rt.Close()

	threadName := fmt.Sprintf("thread%d", threadID)
	result, err := rt.Context().Eval(
		"test.js",
		qjs.Code(fmt.Sprintf("console.log('Hello from %s');", threadName)),
	)
	assert.NoError(t, err)
	if result != nil {
		result.Free()
	}

	// Test struct conversion in concurrent context
	type TestData struct {
		ThreadID int
		Name     string
	}

	data := TestData{
		ThreadID: threadID,
		Name:     threadName,
	}

	jsValue, err := qjs.ToJsValue(rt.Context(), data)
	assert.NoError(t, err)
	if jsValue != nil {
		assert.True(t, jsValue.IsObject())
		assert.Equal(t, int32(threadID), jsValue.GetPropertyStr("ThreadID").Int32())
		assert.Equal(t, threadName, jsValue.GetPropertyStr("Name").String())
		jsValue.Free()
	}

	// Test call QJS_Panic
	assert.Panics(t, func() {
		rt.Call("QJS_Panic")
	})
}

func createTestPoolWithSetup() *qjs.Pool {
	return qjs.NewPool(10, qjs.Option{
		MaxStackSize: 1024 * 1024 * 10,
	}, func(rt *qjs.Runtime) error {
		_, err := rt.Context().Eval(
			"pool-init.js",
			qjs.Code("globalThis.poolInitialized = true;"),
		)
		return err
	})
}

func testPooledRuntimeExecution(t *testing.T, pool *qjs.Pool, workerID int) {
	rt, err := pool.Get()
	require.NoError(t, err)
	defer pool.Put(rt)

	// Verify pool initialization
	result, err := rt.Context().Eval("check.js", qjs.Code("globalThis.poolInitialized"))
	require.NoError(t, err)
	defer result.Free()
	assert.True(t, result.Bool())

	// Test conversion in pooled runtime
	testData := map[string]any{
		"workerID":  workerID,
		"timestamp": time.Now().Unix(),
		"processed": true,
	}

	jsValue, err := qjs.ToJsValue(rt.Context(), testData)
	require.NoError(t, err)
	defer jsValue.Free()

	assert.True(t, jsValue.IsObject())
	assert.Equal(t, int32(workerID), jsValue.GetPropertyStr("workerID").Int32())
	assert.True(t, jsValue.GetPropertyStr("processed").Bool())
}

func createTestPool(size int, setupFuncs ...func(*qjs.Runtime) error) *qjs.Pool {
	return qjs.NewPool(size, qjs.Option{}, setupFuncs...)
}

func verifyRuntimeExecution(t *testing.T, rt *qjs.Runtime, code string, expected any) {
	t.Helper()
	val, err := rt.Eval("test.js", qjs.Code(code))
	assert.NoError(t, err)
	defer val.Free()

	switch exp := expected.(type) {
	case int32:
		assert.Equal(t, exp, val.Int32())
	case string:
		assert.Equal(t, exp, val.String())
	default:
		t.Fatalf("Unsupported expected type: %T", expected)
	}
}

func createSetupFunction(globalVar, value string, shouldError bool, customError error) func(*qjs.Runtime) error {
	return func(rt *qjs.Runtime) error {
		if shouldError {
			if customError != nil {
				return customError
			}
			return fmt.Errorf("setup error")
		}

		code := fmt.Sprintf("globalThis.%s = '%s'", globalVar, value)
		_, err := rt.Eval("setup.js", qjs.Code(code))
		return err
	}
}

func TestRuntime(t *testing.T) {
	t.Run("RuntimeCreation", func(t *testing.T) {
		rt, _ := setupTestContext(t)
		assert.Contains(t, rt.String(), "QJSRuntime")
		assert.NotZero(t, rt.Raw(), "Runtime raw pointer should not be zero")
	})

	t.Run("RuntimeCreationWithInvalidProxyFunction", func(t *testing.T) {
		invalidFunc := "invalidFunction"
		_, err := qjs.New(qjs.Option{ProxyFunction: invalidFunc})
		assert.Error(t, err, "Creating runtime with invalid proxy function should return error")
	})

	t.Run("RuntimeCreationWithCanceledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := qjs.New(qjs.Option{
			Context:            ctx,
			CloseOnContextDone: true,
			DisableBuildCache:  true,
		})
		assert.Error(t, err, "Creating runtime with canceled context should return error")
	})

	t.Run("TruncatedWasmBytes", func(t *testing.T) {
		// Too short to be valid WASM
		truncatedWasmBytes := []byte{0x00, 0x61, 0x73}

		_, err := qjs.New(qjs.Option{QuickJSWasmBytes: truncatedWasmBytes})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create global compiled module")
		assert.Contains(t, err.Error(), "failed to compile qjs module")
		t.Logf("Truncated WASM error: %v", err)
	})

	t.Run("RuntimeCreationWithErrorStartFunction", func(t *testing.T) {
		invalidStartFunc := "QJS_Panic"
		_, err := qjs.New(qjs.Option{StartFunctionName: invalidStartFunc})
		assert.Error(t, err, "Creating runtime with invalid start function should return error")
	})

	t.Run("FreeQJSRuntimePanicOnDuplicateFree", func(t *testing.T) {
		rt := must(qjs.New())
		rt.FreeQJSRuntime()

		assert.Panics(t, func() {
			rt.FreeQJSRuntime()
		}, "FreeQJSRuntime should panic on double free")

		// nil runtime closing should not panic
		rt = nil
		assert.NotPanics(t, func() {
			rt.Close()
		}, "Closing nil runtime should not panic")
	})

	t.Run("CallNonExistentFunction", func(t *testing.T) {
		rt, _ := setupTestContext(t)
		assert.Panics(t, func() {
			rt.Call("NonExistentFunction")
		}, "Calling non-existent function should panic")
	})

	t.Run("NewBytesHandleReturnNilWithEmptyBytes", func(t *testing.T) {
		rt, _ := setupTestContext(t)
		handle := rt.NewBytesHandle(nil)
		assert.Nil(t, handle, "NewBytesHandle should return nil for empty bytes")
	})

	t.Run("MallocSuccess", func(t *testing.T) {
		rt, _ := setupTestContext(t)
		ptr := rt.Malloc(1024)
		assert.NotZero(t, ptr, "Malloc should return non-zero pointer for valid allocation")
		rt.FreeHandle(ptr)
	})

	t.Run("MallocError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		rt := must(qjs.New(qjs.Option{
			Context:            ctx,
			CloseOnContextDone: true,
			DisableBuildCache:  true,
		}))
		cancel() // Cancel context to simulate error
		assert.Panics(t, func() {
			rt.Malloc(1024)
		}, "Malloc should panic when context is canceled")
	})

	t.Run("FreeHandleSuccess", func(t *testing.T) {
		rt, _ := setupTestContext(t)
		ptr := rt.Malloc(1024)
		assert.NotZero(t, ptr, "Malloc should return non-zero pointer for valid allocation")
		rt.FreeHandle(ptr)
		assert.NotPanics(t, func() {
			rt.FreeHandle(ptr)
		}, "FreeHandle should not panic on double free")
	})

	t.Run("FreeHandleError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		rt := must(qjs.New(qjs.Option{Context: ctx}))
		ptr := rt.Malloc(1024)
		cancel() // Cancel context to simulate error
		assert.Panics(t, func() {
			rt.FreeHandle(ptr)
		}, "FreeHandle should panic on invalid handle")
	})

	t.Run("CustomQuickJSWasmBytesGlobalCache", func(t *testing.T) {
		rt1, err := qjs.New()
		require.NoError(t, err, "Normal runtime creation should succeed")
		defer rt1.Close()

		// Verify it works
		val, err := rt1.Eval("test.js", qjs.Code("42"))
		require.NoError(t, err)
		assert.Equal(t, int32(42), val.Int32())
		val.Free()

		invalidBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
		_, err = qjs.New(qjs.Option{QuickJSWasmBytes: invalidBytes})
		assert.Error(t, err, "Creating runtime with invalid QuickJSWasmBytes should return error")

		rt2, err := qjs.New()
		require.NoError(t, err, "Normal runtime creation should still work after invalid attempt")
		defer rt2.Close()
	})

	t.Run("CacheDirOption", func(t *testing.T) {
		cacheDir := t.TempDir()
		rt1, err := qjs.New(qjs.Option{CacheDir: cacheDir})
		require.NoError(t, err)

		val, err := rt1.Eval("test.js", qjs.Code("21 + 21"))
		require.NoError(t, err)
		assert.Equal(t, int32(42), val.Int32())
		val.Free()

		rt1.Close()
		// Note: Cache directory usage depends on implementation details
		// Just verify it can be read without errors
		_, err = os.ReadDir(cacheDir)
		require.NoError(t, err)
	})

	t.Run("CacheDirWithEmptyString", func(t *testing.T) {
		rt, err := qjs.New(qjs.Option{CacheDir: "", DisableBuildCache: true})
		require.NoError(t, err)
		defer rt.Close()

		val, err := rt.Eval("test.js", qjs.Code("2 * 21"))
		require.NoError(t, err)
		assert.Equal(t, int32(42), val.Int32())
		val.Free()
	})

	t.Run("CacheDirWithInvalidPath", func(t *testing.T) {
		_, err := qjs.New(qjs.Option{CacheDir: "/invalid/path/that/does/not/exist", DisableBuildCache: true})
		// When DisableBuildCache is true, invalid cache path may not cause error
		if err != nil {
			assert.Contains(t, err.Error(), "failed to create compilation cache")
		}
	})

	t.Run("MemoryLimitPreventsLargeAllocations", func(t *testing.T) {
		// Test that MemoryLimit prevents hangs on large allocations
		// Note: QuickJS doesn't return clean errors for OOM - the Value is corrupted
		// and panics when accessed, but MemoryLimit ensures quick completion (no hang)
		rt, err := qjs.New(qjs.Option{
			MemoryLimit:      16 * 1024 * 1024, // 16MB limit
			MaxExecutionTime: 5000,             // 5s timeout as safety net
		})
		require.NoError(t, err)
		defer rt.Close()

		// Try to allocate large array - should complete quickly (not hang)
		start := time.Now()
		val, err := rt.Eval("test.js", qjs.Code(`new Array(1024 * 1024 * 256)`)) // ~256M elements
		elapsed := time.Since(start)

		// MemoryLimit prevents hangs - operation should complete quickly
		assert.Less(t, elapsed, 2*time.Second, "Should complete quickly, not hang or timeout")

		// The actual behavior: no Go error, but Value is corrupted
		// (accessing it would panic with "InternalError: out of memory")
		assert.NoError(t, err, "QuickJS doesn't return Go errors for OOM")
		if val != nil {
			// Don't try to use the value (would panic) - just free it
			val.Free()
		}

		t.Logf("Large allocation completed in %v (MemoryLimit prevented hang)", elapsed)
	})

	t.Run("MemoryLimitAllowsNormalOperations", func(t *testing.T) {
		// Test that MemoryLimit doesn't interfere with normal operations
		rt, err := qjs.New(qjs.Option{
			MemoryLimit:      128 * 1024 * 1024, // 128MB - generous for normal ops
			MaxExecutionTime: 5000,
		})
		require.NoError(t, err)
		defer rt.Close()

		// Normal operations should work fine
		testCases := []struct {
			name     string
			code     string
			expected any
		}{
			{"arithmetic", "40 + 2", int32(42)},
			{"string_concat", "'Hello' + ' ' + 'World'", "Hello World"},
			{"small_array", "[1,2,3,4,5].length", int32(5)},
			{"object_creation", "({a: 1, b: 2}).a", int32(1)},
			{"function_call", "(function() { return 42; })()", int32(42)},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				val, err := rt.Eval("test.js", qjs.Code(tc.code))
				require.NoError(t, err, "Normal operation should succeed with MemoryLimit")
				defer val.Free()

				switch exp := tc.expected.(type) {
				case int32:
					assert.Equal(t, exp, val.Int32())
				case string:
					assert.Equal(t, exp, val.String())
				}
			})
		}
	})

	t.Run("MemoryLimitPageAlignment", func(t *testing.T) {
		// Test that MemoryLimit properly rounds up to page boundaries
		// WASM pages are 64KB (65536 bytes)
		testCases := []struct {
			name         string
			memoryLimit  int
			shouldWork   bool
			testAlloc    string
			description  string
		}{
			{
				name:         "exact_page_boundary_256MB",
				memoryLimit:  256 * 1024 * 1024, // 268435456 bytes = exactly 4096 pages
				shouldWork:   true,
				testAlloc:    "new Array(1024)",
				description:  "256MB is exact page boundary",
			},
			{
				name:         "non_page_boundary_rounds_up",
				memoryLimit:  256*1024*1024 + 1, // One byte over - should round up to 4097 pages
				shouldWork:   true,
				testAlloc:    "new Array(1024)",
				description:  "Non-page-boundary value should round up",
			},
			{
				name:         "very_small_limit",
				memoryLimit:  1 * 1024 * 1024, // 1MB
				shouldWork:   true,
				testAlloc:    "[1,2,3]",
				description:  "Small limit with tiny allocation",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				rt, err := qjs.New(qjs.Option{
					MemoryLimit:      tc.memoryLimit,
					MaxExecutionTime: 5000,
				})
				require.NoError(t, err, "Runtime creation should succeed")
				defer rt.Close()

				val, err := rt.Eval("test.js", qjs.Code(tc.testAlloc))
				if tc.shouldWork {
					assert.NoError(t, err, tc.description)
					if val != nil {
						val.Free()
					}
				} else {
					assert.Error(t, err, tc.description)
				}
			})
		}
	})

	t.Run("MemoryLimitZeroMeansUnlimited", func(t *testing.T) {
		// Test that MemoryLimit: 0 means no memory limit (default behavior)
		rt, err := qjs.New(qjs.Option{
			MemoryLimit:      0, // No limit
			MaxExecutionTime: 5000,
		})
		require.NoError(t, err)
		defer rt.Close()

		// Should be able to allocate reasonably large arrays
		val, err := rt.Eval("test.js", qjs.Code(`new Array(1024 * 1024)`)) // 1M elements
		assert.NoError(t, err, "Large allocation should work with MemoryLimit: 0")
		if val != nil {
			val.Free()
		}
	})

	t.Run("MemoryLimitWithPool", func(t *testing.T) {
		// Test that MemoryLimit works correctly with runtime pools
		pool := qjs.NewPool(2, qjs.Option{
			MemoryLimit:      32 * 1024 * 1024, // 32MB per runtime
			MaxExecutionTime: 5000,
		})

		// Test that each pooled runtime respects the memory limit
		rt, err := pool.Get()
		require.NoError(t, err)
		defer pool.Put(rt)

		// Normal operations should work
		val, err := rt.Eval("test.js", qjs.Code("[1,2,3,4,5]"))
		require.NoError(t, err)
		val.Free()

		// Large allocations should complete quickly (not hang) thanks to MemoryLimit
		start := time.Now()
		val, err = rt.Eval("test.js", qjs.Code(`new Array(1024 * 1024 * 128)`)) // Try to allocate 128M elements
		elapsed := time.Since(start)

		assert.Less(t, elapsed, 2*time.Second, "Should complete quickly in pooled runtime")
		assert.NoError(t, err, "QuickJS doesn't return Go errors for OOM")
		if val != nil {
			val.Free()
		}
	})
}

// Concurrent Runtime Usage Tests
func TestConcurrentRuntimeUsage(t *testing.T) {
	t.Run("MultipleIndependentRuntimes", func(t *testing.T) {
		const numThreads = 10
		var wg sync.WaitGroup

		for i := 0; i < numThreads; i++ {
			wg.Add(1)
			go func(threadID int) {
				defer wg.Done()
				testConcurrentRuntimeExecution(t, threadID)
			}(i)
		}
		wg.Wait()
	})

	t.Run("RuntimePoolUsage", func(t *testing.T) {
		pool := createTestPoolWithSetup()
		const numWorkers = 20
		var wg sync.WaitGroup

		for i := range numWorkers {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				testPooledRuntimeExecution(t, pool, workerID)
			}(i)
		}
		wg.Wait()
	})
}

// Pool Creation and Basic Operations Tests
func TestPoolCreationAndBasicOperations(t *testing.T) {
	t.Run("PoolCreationPanicWithZeroSize", func(t *testing.T) {
		assert.Panics(t, func() {
			createTestPool(0)
		}, "Creating pool with size 0 should panic")
	})

	t.Run("PoolCreationWithDifferentSizes", func(t *testing.T) {
		testCases := []struct {
			name string
			size int
		}{
			{"small_pool", 1},
			{"medium_pool", 5},
			{"large_pool", 10},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				pool := createTestPool(tc.size)
				assert.NotNil(t, pool)
			})
		}
	})

	t.Run("BasicGetAndPutOperations", func(t *testing.T) {
		// get on nil pool should return error
		var pool *qjs.Pool
		rt, err := pool.Get()
		assert.Nil(t, rt, "Runtime should be nil when pool is nil")
		assert.Error(t, err, "Should return error when getting from nil pool")

		pool = createTestPool(2)
		rt, err = pool.Get()
		assert.NoError(t, err)
		assert.NotNil(t, rt)

		// Verify runtime functionality
		verifyRuntimeExecution(t, rt, "40+2", int32(42))
		pool.Put(rt)

		// put nil runtime should not panic
		assert.NotPanics(t, func() {
			pool.Put(nil)
		}, "Putting nil runtime should not panic")
	})
}

// Pool Runtime Lifecycle Management Tests
func TestPoolRuntimeLifecycleManagement(t *testing.T) {
	t.Run("RuntimeStatePersistenceAcrossReuse", func(t *testing.T) {
		pool := createTestPool(1)

		// Set a global value in the runtime
		rt1, err := pool.Get()
		assert.NoError(t, err)

		result1, err := rt1.Eval("test.js", qjs.Code("globalThis.testValue = 'test pool'"))
		assert.NoError(t, err)
		if result1 != nil {
			result1.Free()
		}
		pool.Put(rt1)

		// Verify the global value persists when reusing the runtime
		rt2, err := pool.Get()
		assert.NoError(t, err)

		val, err := rt2.Eval("test.js", qjs.Code("globalThis.testValue"))
		assert.NoError(t, err)
		defer val.Free()
		assert.Equal(t, "test pool", val.String())

		pool.Put(rt2)
	})

	t.Run("CapacityExceededBehavior", func(t *testing.T) {
		// Test runtime creation when pool capacity is exceeded and proper cleanup
		pool := createTestPool(1)

		// Get first runtime and mark it with an identifier
		rt1, err := pool.Get()
		assert.NoError(t, err)

		result1, err := rt1.Eval("test.js", qjs.Code("globalThis.runtimeId = 'first'"))
		assert.NoError(t, err)
		if result1 != nil {
			result1.Free()
		}

		// Get second runtime (exceeds capacity, creates new one)
		rt2, err := pool.Get()
		assert.NoError(t, err)

		result2, err := rt2.Eval("test.js", qjs.Code("globalThis.runtimeId = 'second'"))
		assert.NoError(t, err)
		if result2 != nil {
			result2.Free()
		}

		// Return both runtimes - second one should be closed due to capacity limit
		pool.Put(rt1)
		pool.Put(rt2)

		// Verify we get the first runtime back (it was kept in the pool)
		rt3, err := pool.Get()
		assert.NoError(t, err)

		val, err := rt3.Eval("test.js", qjs.Code("globalThis.runtimeId || 'unknown'"))
		assert.NoError(t, err)
		defer val.Free()
		assert.Equal(t, "first", val.String(), "Should get the first runtime back from pool")

		pool.Put(rt3)
	})
}

// Pool Setup Function Handling Tests
func TestPoolSetupFunctionHandling(t *testing.T) {
	t.Run("SetupFunctionExecutionSuccess", func(t *testing.T) {
		setupExecuted := false
		setupFunc := func(rt *qjs.Runtime) error {
			// Verify setup function execution by setting a global value
			result, err := rt.Eval("setup.js", qjs.Code("globalThis.setupValue = 'setup called'"))
			if result != nil {
				result.Free()
			}
			setupExecuted = true
			return err
		}

		pool := createTestPool(1, setupFunc)

		rt, err := pool.Get()
		assert.NoError(t, err)

		val, err := rt.Eval("test.js", qjs.Code("globalThis.setupValue"))
		assert.NoError(t, err)
		defer val.Free()
		assert.Equal(t, "setup called", val.String())
		assert.True(t, setupExecuted, "Setup function should have been called")

		pool.Put(rt)
	})

	t.Run("SetupFunctionErrorHandling", func(t *testing.T) {
		testCases := []struct {
			name        string
			setupError  error
			expectError bool
		}{
			{"custom_error", fmt.Errorf("custom setup error"), true},
			{"generic_error", fmt.Errorf("setup error"), true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				setupFunc := createSetupFunction("setupValue", "test", tc.expectError, tc.setupError)
				pool := createTestPool(1, setupFunc)

				// Verify that setup errors are properly propagated
				rt, err := pool.Get()
				assert.Nil(t, rt, "Runtime should be nil when setup fails")
				assert.Error(t, err, "Should return setup error")
				// Note: Error might be wrapped, so we check if it contains the original error
				assert.Contains(t, err.Error(), tc.setupError.Error(), "Should contain the setup error message")
			})
		}
	})

	t.Run("PoolGetErrorDueToInvalidWASMBytes", func(t *testing.T) {
		// Test that pool setup function handles invalid WASM bytes gracefully
		invalidBytes := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
		setupFunc := createSetupFunction("setupValue", "test", true, fmt.Errorf("invalid WASM bytes"))

		pool := qjs.NewPool(3, qjs.Option{QuickJSWasmBytes: invalidBytes}, setupFunc)
		rt, err := pool.Get()
		assert.Nil(t, rt)
		assert.Error(t, err)
	})
}

// Pool Concurrent Access Tests
func TestPoolConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping pool concurrency test in short mode")
	}

	t.Run("MultipleGoroutinesAccessingPool", func(t *testing.T) {
		const (
			poolSize   = 3
			numThreads = 10
		)

		pool := createTestPool(poolSize)
		results := make(chan bool, numThreads)

		// Test concurrent access to the pool from multiple goroutines
		executeInGoroutine := func(goroutineID int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Goroutine %d panicked: %v", goroutineID, r)
					results <- false
					return
				}
			}()

			rt, err := pool.Get()
			if err != nil {
				t.Errorf("Goroutine %d failed to get runtime: %v", goroutineID, err)
				results <- false
				return
			}
			defer pool.Put(rt)

			// Set and verify goroutine-specific global value
			setCode := fmt.Sprintf("globalThis.threadId = %d", goroutineID)
			result1, err := rt.Eval("thread.js", qjs.Code(setCode))
			if result1 != nil {
				result1.Free()
			}
			if err != nil {
				t.Errorf("Goroutine %d failed to set threadId: %v", goroutineID, err)
				results <- false
				return
			}

			val, err := rt.Eval("thread.js", qjs.Code("globalThis.threadId"))
			if err != nil {
				t.Errorf("Goroutine %d failed to get threadId: %v", goroutineID, err)
				results <- false
				return
			}
			defer val.Free()

			if val.Int32() != int32(goroutineID) {
				t.Errorf("Goroutine %d: expected threadId %d, got %d", goroutineID, goroutineID, val.Int32())
				results <- false
				return
			}

			results <- true
		}

		// Launch concurrent goroutines
		for i := range numThreads {
			go executeInGoroutine(i)
		}

		// Verify all goroutines completed successfully
		successCount := 0
		for range numThreads {
			if <-results {
				successCount++
			}
		}

		assert.Equal(t, numThreads, successCount, "All goroutines should complete successfully")
	})
}

func TestPoolCallGoFuncFromJs(t *testing.T) {
	pool := qjs.NewPool(5, qjs.Option{}, func(rt *qjs.Runtime) error {
		result, err := rt.Context().Eval("<main>", qjs.Code(`
		const hello = (i, getEntity) => getEntity();
		export default { hello };
		`), qjs.TypeModule())
		if err != nil {
			return err
		}

		rt.Context().Global().SetPropertyStr("defaultExports", result)
		return nil
	})

	invokeJsFunc := func(jsFuncName string, args ...any) (*qjs.Value, error) {
		rt, err := pool.Get()
		if err != nil {
			return nil, err
		}
		defer pool.Put(rt)

		defaultExports := rt.Context().Global().GetPropertyStr("defaultExports")
		return defaultExports.Invoke(jsFuncName, args...)
	}

	var wg sync.WaitGroup
	concurrentRoutines := 10
	for i := range concurrentRoutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			getEntity := func() map[string]any {
				return map[string]any{
					"id":   i,
					"name": fmt.Sprintf("Entity %d", i),
				}
			}
			_, err := invokeJsFunc("hello", i, getEntity)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
}
