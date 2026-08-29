//go:build windows && !noffi && cgo
// +build windows,!noffi,cgo

package main

/*
#include <windows.h>

// Platform dynamic-loader abstraction used by the common libffi
// implementation. These are non-static so they resolve across the
// separately compiled cgo preamble translation units.
void *za_dlopen(const char *path) {
    if (path == NULL) {
        return NULL;
    }
    return (void *)LoadLibraryA(path);
}

void *za_dlsym(void *handle, const char *symbol) {
    if (handle == NULL || symbol == NULL) {
        return NULL;
    }
    return (void *)GetProcAddress((HMODULE)handle, symbol);
}

void za_dlclose(void *handle) {
    if (handle != NULL) {
        FreeLibrary((HMODULE)handle);
    }
}
*/
import "C"

import (
	"path/filepath"
)

// libffiLoadPaths returns the ordered list of candidate DLL names for
// loading the libffi runtime on Windows. The directory the executed source
// file was loaded from is checked FIRST (explicit full path), before the
// OS normal DLL search order (exe dir, PATH, System32).
func libffiLoadPaths(providerDir string) []string {
	names := []string{
		"libffi-8.dll", // libffi 3.4.x/3.8.x soname (MSYS2 MinGW-w64 build)
		"libffi-7.dll",
		"libffi.dll",
	}

	var paths []string
	if providerDir != "" {
		for _, n := range names {
			paths = append(paths, filepath.Join(providerDir, n))
		}
	}
	paths = append(paths, names...)
	return paths
}