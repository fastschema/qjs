#include "qjs.h"

JSContext *New_QJSContext(JSRuntime *rt)
{
  JSContext *ctx;
  ctx = JS_NewContext(rt);
  js_init_module_std(ctx, "qjs:std");
  js_init_module_os(ctx, "qjs:os");
  js_init_module_bjson(ctx, "qjs:bjson");
  js_set_global_objs(ctx);

  return ctx;
}

QJSRuntime *New_QJS(
    size_t memory_limit,
    size_t max_stack_size,
    size_t max_execution_time,
    size_t gc_threshold)
{
  JSRuntime *runtime;
  JSContext *ctx;

#ifdef QJS_DEBUG_RUNTIME_ADDRESS
  randomize_address_space();
#endif

  runtime = JS_NewRuntime();

  if (!runtime)
    return NULL;

  if (memory_limit > 0)
    JS_SetMemoryLimit(runtime, memory_limit);

  if (gc_threshold > 0)
    JS_SetGCThreshold(runtime, gc_threshold);

  // WASM STACK OVERFLOW FIX: Now safe to use with js_check_stack_overflow_wasm()
  // Uses frame-depth counting instead of C stack pointers
  // JS_SetMaxStackSize() just sets rt->stack_size, which js_check_stack_overflow_wasm() reads
  // to determine max recursion depth via frame count (not C stack checking)
  if (max_stack_size > 0)
    JS_SetMaxStackSize(runtime, max_stack_size);

  // Set up execution timeout handler (uses JS_SetInterruptHandler internally)
  // max_execution_time is in milliseconds
  if (max_execution_time > 0)
    SetExecuteTimeout(runtime, (uint64_t)max_execution_time);

  /* setup the the worker context */
  js_std_set_worker_new_context_func(New_QJSContext);
  /* initialize the standard objects */
  js_std_init_handlers(runtime);
  /* loader for ES6 modules */
  JS_SetModuleLoaderFunc(runtime, NULL, QJS_ModuleLoader, NULL);
  /* exit on unhandled promise rejections */
  // JS_SetHostPromiseRejectionTracker(runtime, js_std_promise_rejection_tracker, NULL);

  ctx = New_QJSContext(runtime);
  if (!ctx)
  {
    JS_FreeRuntime(runtime);
    return NULL;
  }

  // Initialize QJS_PROXY_VALUE class
  if (init_qjs_proxy_value_class(ctx) < 0)
  {
    JS_FreeContext(ctx);
    JS_FreeRuntime(runtime);
    return NULL;
  }

  QJSRuntime *qjs = (QJSRuntime *)malloc(sizeof(QJSRuntime));
  if (!qjs)
  {
    JS_FreeContext(ctx);
    JS_FreeRuntime(runtime);
    return NULL;
  }

  qjs->runtime = runtime;
  qjs->context = ctx;

  return qjs;
}

void QJS_FreeValue(JSContext *ctx, JSValue val)
{
  JS_FreeValue(ctx, val);
}

void QJS_Free(QJSRuntime *qjs)
{
  JS_FreeContext(qjs->context);
  JS_FreeRuntime(qjs->runtime);
  free(qjs);
}

JSValue QJS_CloneValue(JSContext *ctx, JSValue val)
{
  return JS_DupValue(ctx, val);
}

JSContext *QJS_GetContext(QJSRuntime *qjs)
{
  return qjs->context;
}

void QJS_UpdateStackTop(QJSRuntime *qjs)
{
  JS_UpdateStackTop(qjs->runtime);
}

QJSRuntime *qjs = NULL;

QJSRuntime *QJS_GetRuntime()
{
  return qjs;
}

void initialize()
{
  if (qjs != NULL)
    return;
  size_t memory_limit = 0;
  size_t gc_threshold = 0;
  size_t max_stack_size = 0;
  qjs = New_QJS(memory_limit, max_stack_size, 0, gc_threshold);
}
