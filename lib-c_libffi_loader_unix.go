//go:build !windows && !noffi && cgo
// +build !windows,!noffi,cgo

package main

/*
#include <dlfcn.h>

// Platform dynamic-loader abstraction used by the common libffi
// implementation. These are non-static so they resolve across the
// separately compiled cgo preamble translation units.
void *za_dlopen(const char *path) {
    if (path == NULL) {
        return NULL;
    }
    return dlopen(path, RTLD_LAZY | RTLD_LOCAL);
}

void *za_dlsym(void *handle, const char *symbol) {
    if (handle == NULL || symbol == NULL) {
        return NULL;
    }
    return dlsym(handle, symbol);
}

void za_dlclose(void *handle) {
    if (handle != NULL) {
        dlclose(handle);
    }
}
*/
import "C"

import (
    "path/filepath"
)

// libffiLoadPaths returns the ordered list of candidate shared-library
// names/paths for loading the libffi runtime on Unix-like systems.
// The provider directory (the executed source file's directory) is checked
// first so -xx bundles can ship libffi next to the extracted script, matching
// the Windows script-dir-first discovery.
func libffiLoadPaths(providerDir string) []string {
    var paths []string

    if providerDir != "" {
        paths = append(paths,
            filepath.Join(providerDir, "libffi.so.8"),
            filepath.Join(providerDir, "libffi.so.7"),
            filepath.Join(providerDir, "libffi.so"),
        )
    }

    paths = append(paths,
        // Generic names (system ld.so will search standard paths)
        "libffi.so.8",                            // Generic, version 8
        "libffi.so.7",                            // Generic, version 7
        "libffi.so.6",                            // Generic, version 6
        "libffi.so",                              // Generic, unversioned (BSD)

        // Linux-specific paths
        "/usr/lib/x86_64-linux-gnu/libffi.so.8",  // Debian/Ubuntu x86_64
        "/usr/lib/aarch64-linux-gnu/libffi.so.8", // Debian/Ubuntu ARM64
        "/usr/lib64/libffi.so.8",                 // RHEL/Fedora/CentOS
        "/usr/lib/libffi.so.8",                   // Arch/Alpine/Gentoo
        "/usr/lib/libffi.so",                     // Arch/Alpine unversioned

        // FreeBSD paths
        "/usr/local/lib/libffi.so.8",             // FreeBSD ports (versioned)
        "/usr/local/lib/libffi.so.7",             // FreeBSD ports (older)
        "/usr/local/lib/libffi.so",               // FreeBSD ports (unversioned)
        "/usr/lib/libffi.so",                     // FreeBSD base (if exists)

        // OpenBSD paths
        "/usr/local/lib/libffi.so",               // OpenBSD ports (unversioned)

        // NetBSD paths
        "/usr/pkg/lib/libffi.so.8",               // NetBSD pkgsrc (versioned)
        "/usr/pkg/lib/libffi.so",                 // NetBSD pkgsrc (unversioned)

        // Additional fallback paths
        "/lib/libffi.so.8",                       // Some minimal systems
        "/lib64/libffi.so.8",                     // Some minimal systems
    )
    return paths
}