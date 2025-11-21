.PHONY: build build-debug clean apply-patches clean-patches

# Apply all patches from qjswasm/patches/ to quickjs submodule
apply-patches:
	@echo "Applying QuickJS patches..."
	@cd qjswasm/quickjs && git checkout quickjs.c
	@for patch in qjswasm/patches/*.patch; do \
		if [ -f "$$patch" ]; then \
			echo "  Applying $$(basename $$patch)..."; \
			cd qjswasm/quickjs && git apply "../patches/$$(basename $$patch)" || exit 1; \
			cd ../..; \
		fi \
	done
	@echo "All patches applied successfully"

# Clean up applied patches (restore original files)
clean-patches:
	@echo "Cleaning up applied patches..."
	@cd qjswasm/quickjs && git checkout quickjs.c
	@echo "Patches cleaned up"

# Run QuickJS API tests before building WASM binary
# This verifies that our patches compile correctly and basic functionality works
test-quickjs: apply-patches
	@echo "Running QuickJS API tests..."
	@cd qjswasm/quickjs && \
	rm -rf build && \
	cmake -B build \
		-DQJS_BUILD_LIBC=ON \
		-DQJS_BUILD_CLI_WITH_MIMALLOC=OFF \
		-DCMAKE_TOOLCHAIN_FILE=/opt/wasi-sdk/share/cmake/wasi-sdk.cmake >/dev/null 2>&1 && \
	make -C build api-test -j$(shell nproc 2>/dev/null || sysctl -n hw.ncpu || echo 4) >/dev/null 2>&1 && \
	build/api-test
	@echo "✅ QuickJS API tests passed!"
	@echo ""

# Run full QuickJS test262 suite (slow, for comprehensive testing)
test-quickjs-full: apply-patches
	@echo "Running full QuickJS test262 suite (this may take several minutes)..."
	cd qjswasm/quickjs && \
	rm -rf build && \
	cmake -B build \
		-DQJS_BUILD_LIBC=ON \
		-DQJS_BUILD_CLI_WITH_MIMALLOC=OFF \
		-DCMAKE_TOOLCHAIN_FILE=/opt/wasi-sdk/share/cmake/wasi-sdk.cmake && \
	make -C build run-test262 -j$(shell nproc 2>/dev/null || sysctl -n hw.ncpu || echo 4) && \
	build/run-test262 -c tests.conf
	@echo "✅ Full test suite passed!"

build: test-quickjs
	@echo "Configuring and building qjs..."
	cd qjswasm/quickjs && \
	rm -rf build && \
	cmake -B build \
			-DQJS_BUILD_LIBC=ON \
			-DQJS_BUILD_CLI_WITH_MIMALLOC=OFF \
			-DCMAKE_TOOLCHAIN_FILE=/opt/wasi-sdk/share/cmake/wasi-sdk.cmake \
			-DCMAKE_PROJECT_INCLUDE=../qjswasm.cmake
	@echo "Building qjs target..."
	make -C qjswasm/quickjs/build qjswasm -j$(nproc)
	@echo "Copying build/qjswasm to top-level as qjs.wasm..."
	cp qjswasm/quickjs/build/qjswasm qjs.wasm

	wasm-opt -O3 qjs.wasm -o qjs.wasm
	$(MAKE) clean-patches

build-debug: apply-patches
	@echo "Configuring and building qjs with runtime address debug..."
	cd qjswasm/quickjs && \
	rm -rf build && \
	cmake -B build \
			-DQJS_BUILD_LIBC=ON \
			-DQJS_BUILD_CLI_WITH_MIMALLOC=OFF \
			-DQJS_DEBUG_RUNTIME_ADDRESS=ON \
			-DCMAKE_TOOLCHAIN_FILE=/opt/wasi-sdk/share/cmake/wasi-sdk.cmake \
			-DCMAKE_PROJECT_INCLUDE=../qjswasm.cmake
	@echo "Building qjs target..."
	make -C qjswasm/quickjs/build qjswasm -j$(nproc)
	@echo "Copying build/qjswasm to top-level as qjs.wasm..."
	cp qjswasm/quickjs/build/qjswasm qjs.wasm

	wasm-opt -O3 qjs.wasm -o qjs.wasm
	$(MAKE) clean-patches

clean:
	@echo "Cleaning build directory..."
	cd quickjs && rm -rf build

test:
	./test.sh

lint:
	golangci-lint run
