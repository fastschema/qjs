macro(add_qjs_libc_if_needed target)
    if(NOT QJS_BUILD_LIBC)
        target_sources(${target} PRIVATE quickjs-libc.c)
    endif()
endmacro()
macro(add_static_if_needed target)
    if(QJS_BUILD_CLI_STATIC OR MINGW)
        target_link_options(${target} PRIVATE -static)
        if(MINGW)
            target_link_options(${target} PRIVATE -static-libgcc)
        endif()
    endif()
endmacro()

if(CMAKE_SYSTEM_NAME STREQUAL "WASI")
    add_compile_definitions(
        _WASI_EMULATED_PROCESS_CLOCKS
        _WASI_EMULATED_SIGNAL
    )
    add_link_options(
        -lwasi-emulated-process-clocks
        -lwasi-emulated-signal
    )
endif()

# Optional debug flag for runtime address randomization
option(QJS_DEBUG_RUNTIME_ADDRESS "Enable runtime address randomization debugging" OFF)
if(QJS_DEBUG_RUNTIME_ADDRESS)
    add_compile_definitions(QJS_DEBUG_RUNTIME_ADDRESS)
endif()

if(NOT CMAKE_SYSTEM_NAME STREQUAL "WASI")
    list(APPEND qjs_libs ${CMAKE_THREAD_LIBS_INIT})
endif()

add_executable(qjswasm
    # gen/repl.c
    # gen/standalone.c
    ../eval.c
    ../function.c
    ../helpers.c
    ../proxy.c
    ../qjs.c
    ../module_loader.c
)

add_qjs_libc_if_needed(qjswasm)
add_static_if_needed(qjswasm)

set_target_properties(qjswasm PROPERTIES
    OUTPUT_NAME "qjswasm"
)

target_link_options(qjswasm PRIVATE 
    "LINKER:--export=js_std_await"
    "LINKER:--export=New_QJS"
    "LINKER:--export=New_QJSContext"
    "LINKER:--export=QJS_FreeValue"
    "LINKER:--export=QJS_Free"
    "LINKER:--export=QJS_CloneValue"
    "LINKER:--export=QJS_GetContext"
    "LINKER:--export=JS_FreeContext"
    "LINKER:--export=JS_NewDate"
    "LINKER:--export=JS_NewNull"
    "LINKER:--export=JS_NewUndefined"
    "LINKER:--export=JS_NewUninitialized"
    "LINKER:--export=QJS_NewString"
    "LINKER:--export=QJS_ToCString"
    "LINKER:--export=JS_FreeCString"
    "LINKER:--export=QJS_ToInt64"
    "LINKER:--export=QJS_ToInt32"
    "LINKER:--export=QJS_ToUint32"
    "LINKER:--export=QJS_ToFloat64"
    "LINKER:--export=JS_GetGlobalObject"
    "LINKER:--export=JS_HasException"
    "LINKER:--export=JS_GetException"
    "LINKER:--export=JS_GetPropertyStr"
    "LINKER:--export=QJS_IsUndefined"
    "LINKER:--export=QJS_IsException"
    "LINKER:--export=QJS_UpdateStackTop"
    "LINKER:--export=JS_SetPropertyStr"
    # "LINKER:--export=JS_Call"
    "LINKER:--export=JS_CallConstructor"
    "LINKER:--export=QJS_CreateFunctionProxy"
    "LINKER:--export=QJS_NewInt64"
    "LINKER:--export=QJS_NewUint32"
    "LINKER:--export=QJS_NewInt32"
    "LINKER:--export=JS_NewError"
    "LINKER:--export=JS_Throw"
    "LINKER:--export=JS_FreeValue"
    "LINKER:--export=JS_NewAtom"
    "LINKER:--export=JS_AtomToValue"
    "LINKER:--export=JS_FreeAtom"
    "LINKER:--export=JS_NewObject"
    "LINKER:--export=JS_ValueToAtom"
    "LINKER:--export=JS_GetProperty"
    "LINKER:--export=JS_SetProperty"
    "LINKER:--export=JS_HasProperty"
    "LINKER:--export=JS_NewAtomUInt32"
    "LINKER:--export=JS_DeleteProperty"
    "LINKER:--export=JS_ToObject"
    "LINKER:--export=JS_ToBool"
    "LINKER:--export=JS_NewArrayBufferCopy"
    "LINKER:--export=JS_NewArray"
    "LINKER:--export=JS_SetPropertyUint32"
    "LINKER:--export=QJS_ThrowSyntaxError"
    "LINKER:--export=QJS_ThrowTypeError"
    "LINKER:--export=QJS_ThrowReferenceError"
    "LINKER:--export=QJS_ThrowRangeError"
    "LINKER:--export=QJS_ThrowInternalError"

    "LINKER:--export=QJS_ModuleLoader"
    "LINKER:--export=QJS_SetModuleLoaderCallback"
    "LINKER:--export=QJS_Load"
    "LINKER:--export=QJS_Eval"
    "LINKER:--export=QJS_Compile"
    "LINKER:--export=QJS_Compile2"
    "LINKER:--export=QJS_CreateEvalOption"
    "LINKER:--export=QJS_CloneValue"
    "LINKER:--export=QJS_AtomToCString"
    "LINKER:--export=QJS_IsPromise"
    "LINKER:--export=QJS_IsObject"
    "LINKER:--export=JS_IsDate"
    "LINKER:--export=QJS_IsFunction"
    "LINKER:--export=QJS_IsError"
    "LINKER:--export=QJS_IsNumber"
    "LINKER:--export=QJS_IsBigInt"
    "LINKER:--export=QJS_IsBool"
    "LINKER:--export=QJS_IsNull"
    "LINKER:--export=QJS_IsUninitialized"
    "LINKER:--export=QJS_IsString"
    "LINKER:--export=QJS_IsSymbol"
    "LINKER:--export=QJS_IsArray"
    "LINKER:--export=QJS_IsConstructor"
    "LINKER:--export=QJS_IsInstanceOf"
    "LINKER:--export=QJS_GetOwnPropertyNames"
    "LINKER:--export=QJS_ParseJSON"
    "LINKER:--export=QJS_NewBool"
    "LINKER:--export=QJS_NewBigInt64"
    "LINKER:--export=QJS_NewBigUint64"
    "LINKER:--export=QJS_NewFloat64"
    "LINKER:--export=QJS_NewProxyValue"
    "LINKER:--export=QJS_GetArrayBuffer"
    "LINKER:--export=QJS_JSONStringify"
    "LINKER:--export=QJS_ToInt32"
    "LINKER:--export=QJS_ToEpochTime"
    "LINKER:--export=QJS_SetPropertyUint32"
    "LINKER:--export=QJS_GetPropertyUint32"
    "LINKER:--export=QJS_NewAtomUInt32"
    "LINKER:--export=QJS_NewArrayBufferCopy"
    "LINKER:--export=QJS_Call"
    "LINKER:--export=QJS_GetRuntime"
    "LINKER:--export=QJS_Panic"

    "LINKER:--export=malloc"
    "LINKER:--export=free"
    "LINKER:--export=initialize"
)

target_compile_options(qjswasm PRIVATE "-fvisibility=default")

target_compile_definitions(qjswasm PRIVATE ${qjs_defines})

target_link_libraries(qjswasm qjs)

if(NOT WIN32)
    set_target_properties(qjswasm PROPERTIES ENABLE_EXPORTS TRUE)
endif()

if(QJS_BUILD_CLI_WITH_MIMALLOC OR QJS_BUILD_CLI_WITH_STATIC_MIMALLOC)
    find_package(mimalloc REQUIRED)
    if(QJS_BUILD_CLI_WITH_STATIC_MIMALLOC)
        target_link_libraries(qjswasm mimalloc-static)
    else()
        target_link_libraries(qjswasm mimalloc)
    endif()
endif()

set(CMAKE_BUILD_TYPE "Release" CACHE STRING "" FORCE)
set(CMAKE_INTERPROCEDURAL_OPTIMIZATION ON)  # enables -flto for clang/wasm-ld

add_compile_options(-O3 -DNDEBUG)
add_link_options(-O3)

add_link_options("LINKER:--stack-first")
add_link_options("LINKER:--initial-memory=1310720") # 20 pages
