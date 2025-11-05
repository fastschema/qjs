#include "qjs.h"

#ifdef __wasm__
// When compiling for WASM, declare the imported host function.
// The function is imported from the "env" module under the name "jsModuleLoaderProxy".
__attribute__((import_module("env"), import_name("jsModuleLoaderProxy"))) extern uint64_t jsModuleLoaderProxy(uint32_t ctx, uint32_t module_name, uint64_t callback_id);
#endif

// The actual module loader function that will be called by QuickJS
// The callback_id is passed via the opaque pointer
JSModuleDef *GoModuleLoaderProxy(JSContext *ctx, const char *module_name, void *opaque)
{
#ifdef __wasm__
  // Extract the callback_id from the opaque pointer
  uint64_t callback_id = (uint64_t)(uintptr_t)opaque;

  if (callback_id == 0)
  {
    JS_ThrowInternalError(ctx, "Module loader callback not set");
    return NULL;
  }

  // Call the Go callback through the imported host function
  // The Go function compiles and loads the module, returning JSModuleDef* as uint64
  uint64_t result = jsModuleLoaderProxy((uint32_t)(uintptr_t)ctx, (uint32_t)(uintptr_t)module_name, callback_id);

  return (JSModuleDef *)(uintptr_t)result;
#else
  JS_ThrowInternalError(ctx, "Module loader proxy not implemented for native builds");
  return NULL;
#endif
}

void QJS_SetModuleLoaderCallback(QJSRuntime *qjs, uint64_t callback_id)
{
  // Pass the callback_id as the opaque pointer - this is the idiomatic C pattern
  JS_SetModuleLoaderFunc(qjs->runtime, NULL, GoModuleLoaderProxy, (void *)(uintptr_t)callback_id);
}
