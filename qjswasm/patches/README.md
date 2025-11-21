# QuickJS Patches

This directory contains patches that are automatically applied to the QuickJS submodule during the build process.

## How It Works

1. **Build Process**: When you run `make build` or `make build-debug`, the Makefile automatically:
   - Applies all `.patch` files from this directory to the QuickJS submodule
   - Builds the WASM binary with the patched code
   - Cleans up by restoring the original files

2. **Patch Application**: Patches are applied in alphabetical order using `git apply`.

3. **Manual Control**:
   ```bash
   make apply-patches   # Apply all patches manually
   make clean-patches   # Remove applied patches (restore originals)
   ```

## Current Patches

### quickjs-wasm-stack-overflow.patch

> **🚨 CRITICAL PATCH**: Prevents WASM heap corruption by replacing C stack pointer checks with frame-depth counting.

**Purpose**: Replace unsafe C stack pointer detection with WASM-safe frame-depth counting to prevent "out of bounds memory access" crashes.

**What it changes**:

1. **Replaces `js_check_stack_overflow()` with `js_check_stack_overflow_wasm()`** (23 call sites)
   - ❌ **OLD**: Used `js_get_stack_pointer()` which causes "out of bounds memory access" in WASM
   - ✅ **NEW**: Counts `JSStackFrame` depth by traversing `rt->current_stack_frame` linked list

2. **Changes default max depth from 1000 to 256 frames** (commit 43284e4)
   - **Why 256 frames?** Binary search testing discovered exact corruption threshold:
     - ✅ **379KB (379 frames)**: Clean exception, VM still usable
     - ❌ **380KB (380 frames)**: WASM heap corruption, VM permanently broken
   - **Safety margin**: 256 frames (~256KB) provides 120KB buffer below the 379KB limit

3. **Implements safe frame-depth calculation**:
   ```c
   int max_depth = 256;  // Safe default (was 1000 - UNSAFE!)

   if (rt->stack_size > 0) {
       // Convert bytes to frames: max_frames = stack_size_bytes / 1024
       // Example: 256KB (262144 bytes) → 256 frames
       int calculated_depth = (int)(rt->stack_size / 1024);
       max_depth = calculated_depth < 100000 ? calculated_depth : 100000;
   }

   return js_get_call_depth(rt) >= max_depth;
   ```

**Why needed**:

The original `js_check_stack_overflow()` used C stack pointers:
```c
static inline bool js_check_stack_overflow(JSRuntime *rt, size_t alloca_size) {
    uintptr_t sp = js_get_stack_pointer() - alloca_size;
    return unlikely(sp < rt->stack_limit);
}
```

This **does not work in WASM** because:
- C stack pointers are WASM linear memory offsets (not real addresses)
- `--stack-first` places C stack at byte 0
- Only 20 pages (1,280KB) initial memory allocated
- QuickJS heap starts around byte 380,000
- C stack growing beyond 380KB collides with heap → "out of bounds memory access" panic

**Root cause: WASM memory layout**:
```
╔════════════════════════════════════════════════════════════════════════╗
║                     WASM Linear Memory (20 pages = 1,280KB)            ║
╠════════════════════════════════════════════════════════════════════════╣
║ Byte 0: C Stack starts here (--stack-first)                           ║
║   ↓                                                                    ║
║ [C Stack grows UP toward higher addresses]                            ║
║   ↓ (256KB safe limit)                                                ║
║   ↓ (379KB maximum safe)                                              ║
║   ↓ (380KB = CORRUPTION BOUNDARY) ⚠️                                   ║
║                                                                        ║
║ ~Byte 380,000: QuickJS heap begins                                    ║
║   [Heap data structures]                                              ║
║   [JavaScript objects]                                                ║
║   ...                                                                  ║
║ Byte 1,310,720: End of initial memory                                 ║
╚════════════════════════════════════════════════════════════════════════╝
```

**Testing results**:

| MaxStackSize | Frames | C Stack Size | Result |
|--------------|--------|--------------|--------|
| 10KB | 10 | ~10KB | ✅ Clean exception |
| 256KB | 256 | ~256KB | ✅ Clean exception (safe default) |
| 379KB | 379 | ~379KB | ✅ Clean exception (maximum safe) |
| 380KB | 380 | ~380KB | ❌ **WASM corruption** |
| 500KB | 500 | ~500KB | ❌ **WASM corruption** |
| 1000KB (default) | 1000 | ~1000KB | ❌ **WASM corruption** |

**Protection layers**:

1. **Layer 1 (PRIMARY)**: QuickJS frame counting
   - Catches at ~256 frames (if `MaxStackSize` configured)
   - **This patch implements this layer**
   - **ONLY effective protection**

2. **Layer 3 (CORRUPTION - NOT PROTECTION)**: WASM memory bounds
   - Triggers at ~380 frames (~380KB C stack)
   - Results in permanent VM corruption
   - This is the **failure mode**, not a defense

3. **Layer 2 (UNREACHABLE)**: wazero call depth limit
   - Would catch at 134M calls
   - Never executes (corruption happens at 0.0003% of this limit)

**Result**:
- ✅ MaxStackSize option now works correctly for all byte values
- ✅ Safe default (256 frames) prevents corruption without configuration
- ✅ Formula-based calculation: `max_frames = stack_size_bytes / 1024`
- ✅ Applications can configure higher limits safely (up to 379KB)
- ✅ Clean "RangeError: stack overflow" exceptions instead of WASM corruption

## Adding New Patches

To add a new patch:

1. Make your changes to files in the `qjswasm/quickjs/` submodule
2. Create a patch file:
   ```bash
   cd qjswasm/quickjs
   git diff > ../patches/my-new-patch.patch
   ```
3. Test that it applies cleanly:
   ```bash
   make clean-patches  # Clean existing patches
   make apply-patches  # Should apply all patches including yours
   ```
4. Commit the patch file to the repository

## Patch Naming Convention

Use descriptive names that indicate what the patch does:
- `quickjs-<feature>-<description>.patch`
- Example: `quickjs-wasm-stack-overflow.patch`

## Troubleshooting

**Patch fails to apply:**
- Check that the patch was created from the correct base commit
- Verify the submodule is at the expected commit (check `.gitmodules`)
- Try applying manually to see the exact error:
  ```bash
  cd qjswasm/quickjs
  git apply ../patches/problematic-patch.patch
  ```

**Build fails after adding patch:**
- Ensure the patch doesn't introduce syntax errors
- Check that all modified files are restored by `make clean-patches`
- Test building without the patch to isolate the issue
