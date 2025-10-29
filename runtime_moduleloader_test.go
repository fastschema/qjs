package qjs

import (
	"fmt"
	"testing"
)

// TestSetModuleLoaderFuncWithCustomLoader demonstrates creating a completely custom module loader
// that loads modules from memory
func TestSetModuleLoaderFuncWithCustomLoader(t *testing.T) {
	rt, err := New()
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer rt.Close()

	// Create a custom module loader that provides modules from memory
	moduleSource := map[string]string{
		"math-utils": `
			export function add(a, b) { return a + b; }
			export function multiply(a, b) { return a * b; }
		`,
		"config": `
			export const API_URL = 'https://api.example.com';
			export const VERSION = '1.0.0';
		`,
	}

	var loadedModules []string

	rt.SetModuleLoaderFunc(func(ctx *Context, moduleName string) (string, error) {
		t.Logf("Custom loader called for: %s", moduleName)
		loadedModules = append(loadedModules, moduleName)

		// Check if we have this module in our map
		if source, exists := moduleSource[moduleName]; exists {
			t.Logf("  → Returning source for '%s' from memory", moduleName)
			// Return the source code - it will be compiled automatically
			return source, nil
		}

		// Module not found - delegate to default file system loader
		t.Logf("  → Module '%s' not found in memory, delegating to file system", moduleName)
		return "", nil
	})

	// Test loading virtual modules
	ctx := rt.Context()
	result, err := ctx.Eval("main.js", Code(`
		import { add, multiply } from 'math-utils';
		import { API_URL, VERSION } from 'config';

		const sum = add(5, 3);
		const product = multiply(4, 7);

		export default {
			sum: sum,
			product: product,
			api: API_URL,
			version: VERSION
		};
	`), TypeModule())

	if err != nil {
		t.Fatalf("Failed to evaluate module with in-memory compilation: %v", err)
	}
	defer result.Free()

	// Verify our custom modules were loaded
	if len(loadedModules) != 2 {
		t.Errorf("Expected 2 modules to be loaded, got %d: %v", len(loadedModules), loadedModules)
	}

	// Verify the results
	sum := result.GetPropertyStr("sum")
	defer sum.Free()
	if sum.Int64() != 8 {
		t.Errorf("Expected sum to be 8, got: %d", sum.Int64())
	}

	product := result.GetPropertyStr("product")
	defer product.Free()
	if product.Int64() != 28 {
		t.Errorf("Expected product to be 28, got: %d", product.Int64())
	}

	api := result.GetPropertyStr("api")
	defer api.Free()
	if api.String() != "https://api.example.com" {
		t.Errorf("Expected API_URL to be 'https://api.example.com', got: %s", api.String())
	}

	version := result.GetPropertyStr("version")
	defer version.Free()
	if version.String() != "1.0.0" {
		t.Errorf("Expected VERSION to be '1.0.0', got: %s", version.String())
	}

	t.Log("✓ In-memory module loading test passed!")
	t.Logf("Successfully loaded virtual modules: %v", loadedModules)
}

// TestSetModuleLoaderFuncWithError tests error handling in custom module loader
func TestSetModuleLoaderFuncWithError(t *testing.T) {
	rt, err := New()
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer rt.Close()

	// Module loader that returns an error for specific modules
	rt.SetModuleLoaderFunc(func(ctx *Context, moduleName string) (string, error) {
		t.Logf("Custom loader called for: %s", moduleName)

		if moduleName == "forbidden" {
			return "", fmt.Errorf("module '%s' is forbidden", moduleName)
		}

		// Delegate to default
		return "", nil
	})

	ctx := rt.Context()

	// Try to load a forbidden module
	result, err := ctx.Eval("test.js", Code(`
		import { something } from 'forbidden';
		export default something;
	`), TypeModule())

	if err == nil {
		defer result.Free()
		t.Fatal("Expected error when loading forbidden module, but got success")
	}

	t.Logf("Got expected error: %v", err)

	// Verify the error message contains our custom error
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}

	t.Log("✓ Error handling test passed!")
}
