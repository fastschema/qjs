#include "./quickjs/quickjs.h"
#include "./quickjs/quickjs-libc.h"
#include "./quickjs/cutils.h"
#include <time.h>

JSValue JS_NewNull();
JSValue JS_NewUndefined();
JSValue JS_NewUninitialized();
JSValue JS_GetException(JSContext *ctx);
JSValue QJS_ThrowSyntaxError(JSContext *ctx, const char *fmt);
JSValue QJS_ThrowTypeError(JSContext *ctx, const char *fmt);
JSValue QJS_ThrowReferenceError(JSContext *ctx, const char *fmt);
JSValue QJS_ThrowRangeError(JSContext *ctx, const char *fmt);
JSValue QJS_ThrowInternalError(JSContext *ctx, const char *fmt);
int JS_DeletePropertyInt64(JSContext *ctx, JSValueConst obj, int64_t idx, int flags);

/* Minimum and maximum values a `signed long int' can hold. (Same as `int').  */
#undef LONG_MIN
#define LONG_MIN (-LONG_MAX - 1L)
#undef LONG_MAX
#define LONG_MAX __LONG_MAX__

#ifndef QJS_NATIVE_MODULE_SUFFIX
#ifdef _WIN32
#define QJS_NATIVE_MODULE_SUFFIX ".dll"
#else
#define QJS_NATIVE_MODULE_SUFFIX ".so"
#endif
#endif

typedef struct TimeoutArgs
{
  uint64_t start_ms;    // Start time in milliseconds
  uint64_t timeout_ms;  // Timeout duration in milliseconds
} TimeoutArgs;

typedef struct
{
  uintptr_t fn;
} InterruptHandler;

typedef struct QJSEvalOptions
{
  const void *buf;
  const uint8_t *bytecode_buf;
  size_t bytecode_len;
  const char *filename;
  int eval_flags;
} QJSEvalOptions;

typedef struct QJSRuntime
{
  JSRuntime *runtime;
  JSContext *context;
} QJSRuntime;

bool file_exists(const char *path);
void remove_trailing_slashes(char *path);
int js__has_suffix(const char *str, const char *suffix);
char *append_suffix(const char *base, const char *suffix);
bool input_is_file(QJSEvalOptions opts);
void js_set_global_objs(JSContext *ctx);
char *detect_entry_point(char *module_name);
JSValue js_std_await(JSContext *ctx, JSValue obj);
void QJS_Free(QJSRuntime *qjs);
void QJS_FreeValue(JSContext *ctx, JSValue val);
JSValue QJS_CloneValue(JSContext *ctx, JSValue val);
JSContext *QJS_GetContext(QJSRuntime *qjs);
JSRuntime *JS_GetRuntime(JSContext *ctx);

JSValue InvokeFunctionProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv);
JSValue InvokeAsyncFunctionProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv);
void SetInterruptHandler(JSRuntime *rt, void *handlerArgs);
void SetExecuteTimeout(JSRuntime *rt, uint64_t timeout_ms);

// Headers for GO exported functions
// int goInterruptHandler(JSRuntime *rt, void *handler);
// JSValue goFunctionProxy(JSContext *ctx, JSValueConst thisVal, int argc, JSValueConst *argv);
// JSValue goAsyncFunctionProxy(JSContext *ctx, JSValueConst thisVal, int argc, JSValueConst *argv);

JSValue jsFunctionProxy(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv);
#ifdef QJS_DEBUG_RUNTIME_ADDRESS
void randomize_address_space(void);
#endif

int init_qjs_proxy_value_class(JSContext *ctx);

JSContext *New_QJSContext(JSRuntime *rt);
// QJSRuntime *New_QJS(QJSRuntimeOptions);
QJSRuntime *New_QJS(
    size_t memory_limit,
    size_t max_stack_size,
    size_t max_execution_time,
    size_t gc_threshold);
void QJS_UpdateStackTop(QJSRuntime *qjs);
JSModuleDef *QJS_ModuleLoader(JSContext *ctx, const char *module_name, void *opaque);
JSValue QJS_Load(JSContext *ctx, QJSEvalOptions opts);
JSValue QJS_Eval(JSContext *ctx, QJSEvalOptions opts);
unsigned char *QJS_Compile(JSContext *c, QJSEvalOptions opts, size_t *outSize);
uint64_t *QJS_Compile2(JSContext *ctx, QJSEvalOptions opts);
QJSEvalOptions *QJS_CreateEvalOption(void *buf, uint8_t *bytecode_buf, size_t bytecode_len, char *filename, int eval_flags);

uint64_t *QJS_ToCString(JSContext *ctx, JSValueConst val);
int64_t QJS_ToInt64(JSContext *ctx, JSValue val);
int32_t QJS_ToInt32(JSContext *ctx, JSValue val);
uint32_t QJS_ToUint32(JSContext *ctx, JSValue val);
double QJS_ToFloat64(JSContext *ctx, JSValue val);
double QJS_ToEpochTime(JSContext *ctx, JSValue val);
bool QJS_IsUndefined(JSValue val);
bool QJS_IsException(JSValue val);
bool QJS_IsError(JSContext *ctx, JSValue val);
bool QJS_IsPromise(JSContext *ctx, JSValue v);
bool QJS_IsFunction(JSContext *ctx, JSValue v);
bool QJS_IsObject(JSValue v);
bool QJS_IsNumber(JSValue v);
bool QJS_IsBigInt(JSValue v);
bool QJS_IsBool(JSValue v);
bool QJS_IsNull(JSValue v);
bool QJS_IsUninitialized(JSValue val);
bool QJS_IsString(JSValue val);
bool QJS_IsSymbol(JSValue val);
bool QJS_IsArray(JSValue v);
bool QJS_IsConstructor(JSContext *ctx, JSValue v);
bool QJS_IsInstanceOf(JSContext *ctx, JSValue v, JSValue obj);
JSValue QJS_NewString(JSContext *ctx, const char *str);
JSValue QJS_CreateFunctionProxy(JSContext *ctx, uint64_t handle_id, uint64_t ctx_id, uint64_t is_async);
JSValue QJS_NewInt64(JSContext *ctx, int64_t val);
JSValue QJS_NewUint32(JSContext *ctx, uint32_t val);
JSValue QJS_NewInt32(JSContext *ctx, int32_t val);
JSValue QJS_NewBigInt64(JSContext *ctx, int64_t val);
JSValue QJS_NewBigUint64(JSContext *ctx, uint64_t val);
JSValue QJS_NewFloat64(JSContext *ctx, uint64_t bits);
uint64_t *QJS_AtomToCString(JSContext *ctx, JSAtom atom);

uint64_t *QJS_GetOwnPropertyNames(JSContext *ctx, JSValue v);
JSValue QJS_ParseJSON(JSContext *ctx, const char *buf);
JSValue QJS_NewBool(JSContext *ctx, int val);
uint64_t *QJS_GetArrayBuffer(JSContext *ctx, JSValue obj);
uint64_t *QJS_JSONStringify(JSContext *ctx, JSValue v);
int QJS_SetPropertyUint32(JSContext *ctx, JSValue this_obj, uint64_t idx, JSValue val);
JSValue QJS_GetPropertyUint32(JSContext *ctx, JSValue this_obj, uint32_t idx);
int QJS_NewAtomUInt32(JSContext *ctx, uint64_t n);
JSValue QJS_NewArrayBufferCopy(JSContext *ctx, uint64_t addr, uint64_t len);
JSValue QJS_Call(JSContext *ctx, JSValue func, JSValue this, int argc, uint64_t argv);
JSValue QJS_NewProxyValue(JSContext *ctx, int64_t proxyId);
QJSRuntime *QJS_GetRuntime();

void initialize();
