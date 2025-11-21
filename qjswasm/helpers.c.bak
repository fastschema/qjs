#include "qjs.h"
#include <sys/stat.h>
#include <limits.h>
#include <float.h>

#ifdef QJS_DEBUG_RUNTIME_ADDRESS
/**
 * Allocates a random-sized block of memory to randomize address space layout.
 * This helps in debugging by ensuring runtime objects are allocated at different
 * addresses on each run, making pointer-related bugs more apparent.
 */
void randomize_address_space(void)
{
    static unsigned int call_counter = 0;
    int stack_variable;

    // Generate entropy from multiple sources for better randomization
    unsigned int entropy = (unsigned int)((uintptr_t)&stack_variable ^ // Stack address (varies per call)
                                          (uintptr_t)time(NULL) ^      // Current time
                                          (uintptr_t)clock() ^         // Clock ticks (higher resolution)
                                          (++call_counter)             // Incremental counter
    );

    // Allocate 1-1024 bytes to perturb the address space
    size_t allocation_size = (entropy % 1024) + 1;
    volatile void *random_allocation = malloc(allocation_size);

    // Note: Intentionally not freeing to affect subsequent allocations
    // Touch the allocated memory to ensure it's not optimized away
    if (random_allocation)
    {
        *((volatile char *)random_allocation) = 0;
    }
}
#endif

JSValue JS_NewNull() { return JS_NULL; }
JSValue JS_NewUndefined() { return JS_UNDEFINED; }
JSValue JS_NewUninitialized() { return JS_UNINITIALIZED; }

JSValue QJS_ThrowSyntaxError(JSContext *ctx, const char *fmt)
{
    return JS_ThrowSyntaxError(ctx, "%s", fmt);
}
JSValue QJS_ThrowTypeError(JSContext *ctx, const char *fmt)
{
    return JS_ThrowTypeError(ctx, "%s", fmt);
}
JSValue QJS_ThrowReferenceError(JSContext *ctx, const char *fmt)
{
    return JS_ThrowReferenceError(ctx, "%s", fmt);
}
JSValue QJS_ThrowRangeError(JSContext *ctx, const char *fmt)
{
    return JS_ThrowRangeError(ctx, "%s", fmt);
}
JSValue QJS_ThrowInternalError(JSContext *ctx, const char *fmt)
{
    return JS_ThrowInternalError(ctx, "%s", fmt);
}

// int QJS_TimeoutHandler(JSRuntime *rt, void *opaque)
// {
//     TimeoutArgs *ts = (TimeoutArgs *)opaque;
//     time_t timeout = ts->timeout;
//     time_t start = ts->start;
//     if (timeout <= 0)
//     {
//         return 0;
//     }

//     time_t now = time(NULL);
//     if (now - start > timeout)
//     {
//         free(ts);
//         return 1;
//     }

//     return 0;
// }

// void SetExecuteTimeout(JSRuntime *rt, time_t timeout)
// {
//     TimeoutArgs *ts = malloc(sizeof(TimeoutArgs));
//     ts->start = time(NULL);
//     ts->timeout = timeout;
//     JS_SetInterruptHandler(rt, &QJS_TimeoutHandler, ts);
// }

// int QJS_InterruptHandler(JSRuntime *rt, void *handlerArgs)
// {
//     return goInterruptHandler(rt, handlerArgs);
// }

// void SetInterruptHandler(JSRuntime *rt, void *handlerArgs)
// {
//     JS_SetInterruptHandler(rt, &QJS_InterruptHandler, handlerArgs);
// }

// Copied from "quickjs/qjs.c"
#ifndef countof
#define countof(x) (sizeof(x) / sizeof((x)[0]))
#ifndef endof
#define endof(x) ((x) + countof(x))
#endif
#endif

static JSValue js_gc(
    JSContext *ctx,
    JSValue this_val,
    int argc,
    JSValue *argv)
{
    JS_RunGC(JS_GetRuntime(ctx));
    return JS_UNDEFINED;
}

static JSValue js_navigator_get_userAgent(JSContext *ctx, JSValue this_val)
{
    char version[32];
    snprintf(version, sizeof(version), "quickjs-ng/%s", JS_GetVersion());
    return JS_NewString(ctx, version);
}

static const JSCFunctionListEntry global_obj[] = {
    JS_CFUNC_DEF("gc", 0, js_gc),
};

static const JSCFunctionListEntry navigator_proto_funcs[] = {
    JS_CGETSET_DEF2("userAgent", js_navigator_get_userAgent, NULL, JS_PROP_CONFIGURABLE | JS_PROP_ENUMERABLE),
    JS_PROP_STRING_DEF("[Symbol.toStringTag]", "Navigator", JS_PROP_CONFIGURABLE),
};

void js_set_global_objs(JSContext *ctx)
{
    JSValue global = JS_GetGlobalObject(ctx);
    JS_SetPropertyFunctionList(
        ctx,
        global,
        global_obj,
        countof(global_obj));
    JSValue navigator_proto = JS_NewObject(ctx);
    JS_SetPropertyFunctionList(
        ctx,
        navigator_proto,
        navigator_proto_funcs,
        countof(navigator_proto_funcs));
    JSValue navigator = JS_NewObjectProto(ctx, navigator_proto);
    JS_DefinePropertyValueStr(
        ctx,
        global,
        "navigator",
        navigator,
        JS_PROP_CONFIGURABLE | JS_PROP_ENUMERABLE);

    JS_FreeValue(ctx, global);
    JS_FreeValue(ctx, navigator_proto);

    js_std_add_helpers(ctx, 0, 0);

    /* make 'std' and 'os' visible to non module code */
    const char *str =
        "import * as bjson from 'qjs:bjson';\n"
        "import * as std from 'qjs:std';\n"
        "import * as os from 'qjs:os';\n"
        "globalThis.bjson = bjson;\n"
        "globalThis.std = std;\n"
        "globalThis.os = os;\n"
        "globalThis.setTimeout = os.setTimeout;\n"
        "globalThis.setInterval = os.setInterval;\n"
        "globalThis.clearTimeout = os.clearTimeout;\n"
        "globalThis.clearInterval = os.clearInterval;\n";

    QJSEvalOptions opts = {
        .buf = str,
        .filename = "<init_global>",
        .eval_flags = JS_EVAL_TYPE_MODULE};

    JSValue val = QJS_Eval(ctx, opts);
    if (JS_IsException(val))
    {
        js_std_dump_error(ctx);
        exit(1);
    }
    JS_FreeValue(ctx, val);
}

bool file_exists(const char *path)
{
    struct stat path_stat;
    // Perform a stat() system call to get information about the path
    if (stat(path, &path_stat) != 0)
    {
        // If stat() returns a non-zero value, an error occurred (e.g., the path doesn't exist)
        return false;
    }
    // Use the S_ISREG macro to check if the path is a regular file
    return S_ISREG(path_stat.st_mode);
}

// Remove trailing slashes from a given path
void remove_trailing_slashes(char *path)
{
    size_t len = strlen(path);
    while (len > 0 && path[len - 1] == '/')
    {
        path[--len] = '\0';
    }
}

/* Allocate a new string by appending `suffix` to `base` */
char *append_suffix(const char *base, const char *suffix)
{
    size_t base_len = strlen(base);
    size_t suffix_len = strlen(suffix);
    char *result = malloc(base_len + suffix_len + 1);
    if (result)
    {
        strcpy(result, base);
        strcat(result, suffix);
    }
    return result;
}

bool input_is_file(QJSEvalOptions opts)
{
    bool has_bytecode = opts.bytecode_buf != NULL;
    bool has_script = opts.buf != NULL && strlen(opts.buf) > 0;
    return !has_bytecode && !has_script;
}

char *detect_entry_point(char *module_name)
{
    char *module_path = strdup(module_name);
    if (!module_path)
        return NULL;

    remove_trailing_slashes(module_path);

    if (file_exists(module_path))
        return module_path;

    const char *suffixes[] = {".js", ".mjs", "/index.js", "/index.mjs"};
    size_t num_suffixes = sizeof(suffixes) / sizeof(suffixes[0]);

    for (size_t i = 0; i < num_suffixes; i++)
    {
        char *candidate = append_suffix(module_path, suffixes[i]);
        if (!candidate)
            continue; // Allocation failure: skip to next suffix
        if (file_exists(candidate))
        {
            free(module_path);
            return candidate;
        }
    }
    free(module_path);

    return module_name;
}

bool QJS_IsUndefined(JSValue val)
{
    return JS_IsUndefined(val);
}

bool QJS_IsException(JSValue val)
{
    return JS_IsException(val);
}

bool QJS_IsError(JSContext *ctx, JSValue val)
{
    return JS_IsError(ctx, val);
}

bool QJS_IsUninitialized(JSValue val)
{
    return JS_IsUninitialized(val);
}

bool QJS_IsString(JSValue val)
{
    return JS_IsString(val);
}

bool QJS_IsSymbol(JSValue val)
{
    return JS_IsSymbol(val);
}

bool QJS_IsPromise(JSContext *ctx, JSValue v)
{
    JSPromiseStateEnum state = JS_PromiseState(ctx, v);
    if (state == JS_PROMISE_PENDING ||
        state == JS_PROMISE_FULFILLED ||
        state == JS_PROMISE_REJECTED)
    {
        return true;
    }
    return false;
}

bool QJS_IsFunction(JSContext *ctx, JSValue v)
{
    return JS_IsFunction(ctx, v);
}

bool QJS_IsConstructor(JSContext *ctx, JSValue v)
{
    return JS_IsConstructor(ctx, v);
}

bool QJS_IsInstanceOf(JSContext *ctx, JSValue v, JSValue obj)
{
    return JS_IsInstanceOf(ctx, v, obj) == 0 ? false : true;
}

bool QJS_IsObject(JSValue v)
{
    return JS_IsObject(v);
}

bool QJS_IsNumber(JSValue v)
{
    return JS_IsNumber(v);
}

bool QJS_IsBigInt(JSValue v)
{
    return JS_IsBigInt(v);
}

bool QJS_IsBool(JSValue v)
{
    return JS_IsBool(v);
}

bool QJS_IsNull(JSValue v)
{
    return JS_IsNull(v);
}

bool QJS_IsArray(JSValue v)
{
    return JS_IsArray(v);
}

/**
 * Converts a JSValue to a C string and returns a packed uint64_t value
 * containing both the string's memory address (high 32 bits) and length (low 32 bits).
 * NOTE: The caller must free the string memory after use with JS_FreeCString().
 * NOTE: This assumes a 32-bit memory model (as in WebAssembly 1.0).
 */
uint64_t *QJS_ToCString(JSContext *ctx, JSValueConst val)
{
    const char *str = JS_ToCString(ctx, val);
    if (!str)
    {
        return NULL;
    }

    size_t len = strlen(str);
    uint64_t *result = malloc(sizeof(uint64_t));
    if (!result)
    {
        JS_FreeCString(ctx, str);
        return NULL; // Allocation failure
    }

    // Store the address of the string in the high 32 bits and the length in the low 32 bits
    *result = ((uint64_t)(uintptr_t)str << 32) | (uint32_t)len;
    return result;
}

int64_t QJS_ToInt64(JSContext *ctx, JSValue val)
{
    int64_t i64;
    int ret = JS_ToInt64(ctx, &i64, val);
    if (ret != 0)
    {
        return 0;
    }
    return i64;
}

int32_t QJS_ToInt32(JSContext *ctx, JSValue val)
{
    int32_t i32;
    int ret = JS_ToInt32(ctx, &i32, val);
    if (ret != 0)
    {
        return 0;
    }

    return i32;
}

uint32_t QJS_ToUint32(JSContext *ctx, JSValue val)
{
    uint32_t u32;
    int ret = JS_ToUint32(ctx, &u32, val);
    if (ret != 0)
    {
        return 0;
    }
    return u32;
}

double QJS_ToFloat64(JSContext *ctx, JSValue val)
{
    double d;
    int ret = JS_ToFloat64(ctx, &d, val);
    if (ret != 0)
    {
        return 0.0;
    }
    return d;
}

double QJS_ToEpochTime(JSContext *ctx, JSValue val)
{
    // val is JSValue create from JS_NewDate
    // First check if val is a Date object
    if (!JS_IsDate(val))
    {
        return 0.0; // Return 0 for non-Date objects
    }

    // Get the "getTime" method from the Date object
    JSAtom getTime_atom = JS_NewAtom(ctx, "getTime");
    JSValue getTime_func = JS_GetProperty(ctx, val, getTime_atom);
    JS_FreeAtom(ctx, getTime_atom);

    if (JS_IsException(getTime_func))
    {
        JS_FreeValue(ctx, getTime_func);
        return 0.0;
    }

    // Call getTime() method
    JSValue result = JS_Call(ctx, getTime_func, val, 0, NULL);
    JS_FreeValue(ctx, getTime_func);

    if (JS_IsException(result))
    {
        JS_FreeValue(ctx, result);
        return 0.0;
    }

    // Convert the result to double (milliseconds since epoch)
    double epoch_ms = QJS_ToFloat64(ctx, result);
    JS_FreeValue(ctx, result);

    return epoch_ms;
}

JSValue QJS_NewString(JSContext *ctx, const char *str)
{
    return JS_NewString(ctx, str);
}

JSValue QJS_NewInt64(JSContext *ctx, int64_t val)
{
    return JS_NewInt64(ctx, val);
}

JSValue QJS_NewUint32(JSContext *ctx, uint32_t val)
{
    return JS_NewUint32(ctx, val);
}

JSValue QJS_NewInt32(JSContext *ctx, int32_t val)
{
    return JS_NewInt32(ctx, val);
}

JSValue QJS_NewBigInt64(JSContext *ctx, int64_t val)
{
    return JS_NewBigInt64(ctx, val);
}

JSValue QJS_NewBigUint64(JSContext *ctx, uint64_t val)
{
    return JS_NewBigUint64(ctx, val);
}

JSValue QJS_NewFloat64(JSContext *ctx, uint64_t bits)
{
    double val = uint64_as_float64(bits); // Using the helper function from cutils.h
    return JS_NewFloat64(ctx, val);
}

/**
 * Converts a JSAtom to a C string and returns a packed uint64_t value
 * containing both the string's memory address (high 32 bits) and length (low 32 bits).
 * NOTE: The caller must free the string memory after use.
 */
uint64_t *QJS_AtomToCString(JSContext *ctx, JSAtom atom)
{
    const char *str = JS_AtomToCString(ctx, atom);
    if (!str)
    {
        return NULL;
    }

    size_t len = strlen(str);
    uint64_t *result = malloc(sizeof(uint64_t));
    if (!result)
    {
        JS_FreeCString(ctx, str);
        return NULL; // Allocation failure
    }
    // Store the address of the string in the high 32 bits and the length in the low 32 bits
    *result = ((uint64_t)(uintptr_t)str << 32) | (uint32_t)len;
    return result;
}

// returns a packed uint64_t value containing both the string's memory address (high 32 bits) and length (low 32 bits).
uint64_t *QJS_GetOwnPropertyNames(JSContext *ctx, JSValue v)
{
    JSPropertyEnum *ptr;
    uint32_t size;

    int result = JS_GetOwnPropertyNames(
        ctx,
        &ptr,
        &size,
        v,
        JS_GPN_STRING_MASK | JS_GPN_SYMBOL_MASK | JS_GPN_PRIVATE_MASK);

    if (result < 0)
    {
        return NULL;
    }

    uint64_t *packed_result = malloc(sizeof(uint64_t));
    if (!packed_result)
    {
        // Free the property array if allocation fails
        js_free(ctx, ptr);
        return NULL;
    }

    // Pack the pointer and size into the allocated memory
    *packed_result = ((uint64_t)(uintptr_t)ptr << 32) | (uint32_t)size;
    return packed_result;
}

JSValue QJS_ParseJSON(JSContext *ctx, const char *buf)
{
    size_t len = strlen(buf);
    JSValue obj = JS_ParseJSON(ctx, buf, len, "<parse_json>");
    return obj;
}

JSValue QJS_NewBool(JSContext *ctx, int val)
{
    return JS_NewBool(ctx, val == 0 ? 0 : 1);
}

uint64_t *QJS_GetArrayBuffer(JSContext *ctx, JSValue obj)
{
    size_t len = 0;
    uint8_t *arr = JS_GetArrayBuffer(ctx, &len, obj);

    if (!arr)
    {
        return NULL;
    }

    uint64_t *result = malloc(sizeof(uint64_t));
    if (!result)
    {
        return NULL; // Allocation failure
    }

    // Store the address of the arr in the high 32 bits and the length in the low 32 bits
    *result = ((uint64_t)(uintptr_t)arr << 32) | (uint32_t)len;
    return result;
}

uint64_t *QJS_JSONStringify(JSContext *ctx, JSValue v)
{
    JSValue ref = JS_JSONStringify(ctx, v, JS_NewNull(), JS_NewNull());
    const char *ptr = JS_ToCString(ctx, ref);

    if (!ptr)
    {
        JS_FreeValue(ctx, ref);
        return NULL;
    }

    size_t len = strlen(ptr);
    uint64_t *result = malloc(sizeof(uint64_t));
    if (!result)
    {
        JS_FreeValue(ctx, ref);
        JS_FreeCString(ctx, ptr);
        return NULL; // Allocation failure
    }

    // Store the address of the string in the high 32 bits and the length in the low 32 bits
    *result = ((uint64_t)(uintptr_t)ptr << 32) | (uint32_t)len;
    JS_FreeValue(ctx, ref);
    JS_FreeCString(ctx, ptr);
    return result;
}

int QJS_SetPropertyUint32(JSContext *ctx, JSValue this_obj, uint64_t idx, JSValue val)
{
    uint32_t idx32 = (uint32_t)idx;
    return JS_SetPropertyUint32(ctx, this_obj, idx32, val);
}

JSValue QJS_GetPropertyUint32(JSContext *ctx, JSValue this_obj, uint32_t idx)
{
    uint32_t idx32 = (uint32_t)idx;
    return JS_GetPropertyUint32(ctx, this_obj, idx32);
}

int QJS_NewAtomUInt32(JSContext *ctx, uint64_t n)
{
    uint32_t n32 = (uint32_t)n;
    return JS_NewAtomUInt32(ctx, n32);
}

JSValue QJS_NewArrayBufferCopy(JSContext *ctx, uint64_t addr, uint64_t len)
{
    uint8_t *arr = (uint8_t *)(uintptr_t)addr;
    return JS_NewArrayBufferCopy(ctx, arr, len);
}

JSValue QJS_Call(JSContext *ctx, JSValue func, JSValue this, int argc, uint64_t argv)
{
    JSValue *js_argv = (JSValue *)(uintptr_t)argv;
    return JS_Call(ctx, func, this, argc, js_argv);
}

void QJS_Panic()
{
    // Handle panic situation
    fprintf(stderr, "QJS Panic: Unrecoverable error occurred\n");
    abort();
}
