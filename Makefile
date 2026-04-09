.PHONY: build build-debug clean

CMAKE ?= cmake
WASI_SDK_DIR ?= /opt/wasi-sdk
WASI_TOOLCHAIN_FILE ?= $(WASI_SDK_DIR)/share/cmake/wasi-sdk.cmake

build:
		@echo "Configuring and building qjs..."
		cd qjswasm/quickjs && \
		rm -rf build && \
		$(CMAKE) -B build \
				-DQJS_BUILD_LIBC=ON \
				-DQJS_BUILD_CLI_WITH_MIMALLOC=OFF \
				-DCMAKE_TOOLCHAIN_FILE=$(WASI_TOOLCHAIN_FILE) \
				-DCMAKE_PROJECT_INCLUDE=../qjswasm.cmake
		@echo "Building qjs target..."
		make -C qjswasm/quickjs/build qjswasm -j$(nproc)
		@echo "Copying build/qjswasm to top-level as qjs.wasm..."
		cp qjswasm/quickjs/build/qjswasm qjs.wasm

		wasm-opt -O3 qjs.wasm -o qjs.wasm

build-debug:
		@echo "Configuring and building qjs with runtime address debug..."
		cd qjswasm/quickjs && \
		rm -rf build && \
		$(CMAKE) -B build \
				-DQJS_BUILD_LIBC=ON \
				-DQJS_BUILD_CLI_WITH_MIMALLOC=OFF \
				-DQJS_DEBUG_RUNTIME_ADDRESS=ON \
				-DCMAKE_TOOLCHAIN_FILE=$(WASI_TOOLCHAIN_FILE) \
				-DCMAKE_PROJECT_INCLUDE=../qjswasm.cmake
		@echo "Building qjs target..."
		make -C qjswasm/quickjs/build qjswasm -j$(nproc)
		@echo "Copying build/qjswasm to top-level as qjs.wasm..."
		cp qjswasm/quickjs/build/qjswasm qjs.wasm

		wasm-opt -O3 qjs.wasm -o qjs.wasm

clean:
	@echo "Cleaning build directory..."
	cd quickjs && rm -rf build

test:
	./test.sh

lint:
	golangci-lint run
