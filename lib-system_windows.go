//go:build windows
// +build windows

package main

/*
#include <windows.h>

// GetTickCount64-based uptime (ms) - available on Vista+.
static unsigned long long za_uptime_ms(void) {
    return (unsigned long long)GetTickCount64();
}
*/
import "C"

import (
    "os"
    "os/user"
    "time"
)

// zaSyncAll flushes filesystem buffers. Windows has no global sync call; it
// is a no-op here (individual files are flushed via FlushFileBuffers).
func zaSyncAll() {}

// getSystemBootTime returns an estimate of the system boot time as a Unix
// timestamp, derived from GetTickCount64 uptime.
func getSystemBootTime() int64 {
    uptimeSec := int64(C.za_uptime_ms()) / 1000
    return time.Now().Unix() - uptimeSec
}

// getContextSwitches is not exposed by Windows; report 0.
func getContextSwitches() uint64 {
    return 0
}

// getInterrupts is not exposed by Windows; report 0.
func getInterrupts() uint64 {
    return 0
}

// getSystemFileDescriptorStats is not exposed by Windows; report (0, 0).
func getSystemFileDescriptorStats() (allocated, maximum uint64) {
    return 0, 0
}

// ============================================================================
// host/system metric stubs. The Windows build does not emulate the Unix /proc
// metrics layer; these satisfy the common call sites with zero/empty results.
// ============================================================================

// getMemoryInfo reports no usable Windows memory breakdown.
func getMemoryInfo() (MemoryInfo, error) {
    return MemoryInfo{}, nil
}

// getSystemLoad has no Windows equivalent; report an empty load sample.
func getSystemLoad() ([]float64, error) {
    return []float64{0, 0, 0}, nil
}

// getResourceUsage has no Windows equivalent.
func getResourceUsage(pid int) (ResourceUsage, error) {
    return ResourceUsage{}, nil
}

// getNetworkIO has no Windows equivalent.
func getNetworkIO(options map[string]interface{}) ([]NetworkIOStats, error) {
    return nil, nil
}

// getDiskIO has no Windows equivalent.
func getDiskIO(options map[string]interface{}) ([]DiskIOStats, error) {
    return nil, nil
}

// getDiskUsage has no Windows equivalent.
func getDiskUsage(options map[string]interface{}) ([]map[string]interface{}, error) {
    return nil, nil
}

// getProcessInfo has no Windows equivalent.
func getProcessInfo(pid int, options map[string]interface{}) (ProcessInfo, error) {
    return ProcessInfo{}, nil
}

// getProcessList has no Windows equivalent.
func getProcessList(options map[string]interface{}) ([]ProcessInfo, error) {
    return nil, nil
}

// getProcessMap has no Windows equivalent.
func getProcessMap(startPID int) (ProcessMap, error) {
    return ProcessMap{}, nil
}

// getProcessTree has no Windows equivalent.
func getProcessTree(startPID int) (ProcessTree, error) {
    return ProcessTree{}, nil
}

// getOpenFDs has no Windows equivalent; report 0.
func getOpenFDs() int {
    return 0
}

// getMaxFDs has no Windows equivalent; report 0.
func getMaxFDs() int {
    return 0
}

// getSystemResources has no Windows equivalent.
func getSystemResources() (SystemResources, error) {
    return SystemResources{}, nil
}

// getTopCPU has no Windows equivalent.
func getTopCPU(n int) ([]ProcessInfo, error) {
    return nil, nil
}

// getTopMemory has no Windows equivalent.
func getTopMemory(n int) ([]ProcessInfo, error) {
    return nil, nil
}

// getTopDiskIO has no Windows equivalent.
func getTopDiskIO(n int) ([]DiskIOStats, error) {
    return nil, nil
}

// getTopNetwork has no Windows equivalent.
func getTopNetwork(n int) ([]NetworkIOStats, error) {
    return nil, nil
}

// getMountInfo has no Windows equivalent.
func getMountInfo(options map[string]interface{}) ([]map[string]interface{}, error) {
    return nil, nil
}

// getSlabInfo has no Windows equivalent.
func getSlabInfo() map[string]SlabInfo {
    return map[string]SlabInfo{}
}

// getNetworkDevices has no Windows equivalent.
func getNetworkDevices(options map[string]interface{}) ([]map[string]interface{}, error) {
    return nil, nil
}

// getDefaultGatewayAddress has no Windows equivalent.
func getDefaultGatewayAddress() (string, error) {
    return "", nil
}

// getDefaultGatewayInfo has no Windows equivalent.
func getDefaultGatewayInfo() (map[string]interface{}, error) {
    return map[string]interface{}{}, nil
}

// getDefaultGatewayInterface has no Windows equivalent.
func getDefaultGatewayInterface() (string, error) {
    return "", nil
}

// getCPUInfo has no Windows equivalent.
func getCPUInfo(coreNumber int, options map[string]interface{}) (CPUInfo, error) {
    return CPUInfo{}, nil
}

// debugCPUFiles has no Windows equivalent.
func debugCPUFiles() map[string]interface{} {
    return map[string]interface{}{}
}

// calculateIODiff has no Windows equivalent.
func calculateIODiff(snapshot1, snapshot2 ResourceSnapshot, duration time.Duration) map[string]interface{} {
    return map[string]interface{}{}
}

// getCMDVersion has no Windows equivalent.
func getCMDVersion() (string, error) {
    return "", nil
}

// getPowerShellVersion has no Windows equivalent.
func getPowerShellVersion() (string, error) {
    return "", nil
}

// getWindowsReleaseInfo has no Windows equivalent.
func getWindowsReleaseInfo() (string, string, string, error) {
    return "", "", "", nil
}

// getCurrentHomeDir returns the user's home directory.
func getCurrentHomeDir() (string, error) {
    return os.UserHomeDir()
}

// getCurrentLocale has no Windows equivalent.
func getCurrentLocale() (string, error) {
    return "", nil
}

// getCurrentUsername returns the current user's name.
func getCurrentUsername() (string, error) {
    u, err := user.Current()
    if err != nil {
        return "", err
    }
    return u.Username, nil
}

// sendSignal has no Windows equivalent.
func sendSignal(pid int, sig any) (bool, error) {
    return false, nil
}