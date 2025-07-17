//go:build (freebsd || openbsd || netbsd || dragonfly) && !linux && !windows
// +build freebsd openbsd netbsd dragonfly
// +build !linux
// +build !windows

package main

import (
    "fmt"
    "net"
    "os"
    "runtime"
    "sort"
    "strconv"
    "strings"
    "syscall"
    "time"

    "os/exec"

    "golang.org/x/sys/unix"
)

// BSD implementation of system monitoring functions

// getTopCPU returns top N CPU consumers
func getTopCPU(n int) ([]ProcessInfo, error) {
    processes, err := getProcessList(nil)
    if err != nil {
        return nil, err
    }

    // Sort by CPU percentage
    sort.Slice(processes, func(i, j int) bool {
        return processes[i].CPUPercent > processes[j].CPUPercent
    })

    if n == -1 {
        return processes, nil
    }
    if n > len(processes) {
        n = len(processes)
    }
    return processes[:n], nil
}

// getTopMemory returns top N memory consumers
func getTopMemory(n int) ([]ProcessInfo, error) {
    processes, err := getProcessList(nil)
    if err != nil {
        return nil, err
    }

    // Sort by memory usage
    sort.Slice(processes, func(i, j int) bool {
        return processes[i].MemoryUsage > processes[j].MemoryUsage
    })

    if n == -1 {
        return processes, nil
    }
    if n > len(processes) {
        n = len(processes)
    }
    return processes[:n], nil
}

// getTopNetwork returns top N network consumers
func getTopNetwork(n int) ([]NetworkIOStats, error) {
    stats, err := getNetworkIO(nil)
    if err != nil {
        return nil, err
    }

    // Sort by total bytes (rx + tx)
    sort.Slice(stats, func(i, j int) bool {
        total1 := stats[i].RxBytes + stats[i].TxBytes
        total2 := stats[j].RxBytes + stats[j].TxBytes
        return total1 > total2
    })

    if n == -1 {
        return stats, nil
    }
    if n > len(stats) {
        n = len(stats)
    }
    return stats[:n], nil
}

// getTopDiskIO returns top N disk I/O consumers
func getTopDiskIO(n int) ([]DiskIOStats, error) {
    stats, err := getDiskIO(nil)
    if err != nil {
        return nil, err
    }

    // Sort by total bytes (read + write)
    sort.Slice(stats, func(i, j int) bool {
        total1 := stats[i].ReadBytes + stats[i].WriteBytes
        total2 := stats[j].ReadBytes + stats[j].WriteBytes
        return total1 > total2
    })

    if n == -1 {
        return stats, nil
    }
    if n > len(stats) {
        n = len(stats)
    }
    return stats[:n], nil
}

// getSystemResources returns overall system resource usage
func getSystemResources() (SystemResources, error) {
    var resources SystemResources

    // Get CPU count
    resources.CPUCount = runtime.NumCPU()

    // Get load average - handle gracefully if unavailable
    load, err := getSystemLoad()
    if err == nil {
        resources.LoadAverage = load
    } else {
        // Return clearly invalid sentinel values for load average
        resources.LoadAverage = []float64{-1, -1, -1}
    }

    // Get memory info - handle gracefully if unavailable
    mem, err := getMemoryInfo()
    if err == nil {
        resources.MemoryTotal = mem.Total
        resources.MemoryUsed = mem.Used
        resources.MemoryFree = mem.Free
        resources.MemoryCached = mem.Cached
        resources.SwapTotal = mem.SwapTotal
        resources.SwapUsed = mem.SwapUsed
        resources.SwapFree = mem.SwapFree
    } else {
        // Return clearly invalid sentinel values for memory info
        resources.MemoryTotal = 0xFFFFFFFFFFFFFFFF // -1 as uint64
        resources.MemoryUsed = 0xFFFFFFFFFFFFFFFF
        resources.MemoryFree = 0xFFFFFFFFFFFFFFFF
        resources.MemoryCached = 0xFFFFFFFFFFFFFFFF
        resources.SwapTotal = 0xFFFFFFFFFFFFFFFF
        resources.SwapUsed = 0xFFFFFFFFFFFFFFFF
        resources.SwapFree = 0xFFFFFFFFFFFFFFFF
    }

    // Get uptime using kern.boottime sysctl
    boottimePaths := []string{
        "kern.boottime",
        "kern.boottime.sec",
    }

    var uptimeSet bool
    for _, path := range boottimePaths {
        boottimeData, err := syscall.Sysctl(path)
        if err == nil {
            // Parse boottime data: { sec = 1752685192, usec = 878755 }
            // Extract the timestamp from the boottime structure
            if strings.Contains(boottimeData, "sec =") {
                // Find the sec value
                secIndex := strings.Index(boottimeData, "sec =")
                if secIndex != -1 {
                    // Extract the number after "sec ="
                    secPart := boottimeData[secIndex+5:]
                    endIndex := strings.Index(secPart, ",")
                    if endIndex != -1 {
                        secPart = secPart[:endIndex]
                    }
                    if secStr := strings.TrimSpace(secPart); secStr != "" {
                        if bootSec, err := strconv.ParseInt(secStr, 10, 64); err == nil {
                            // Calculate uptime in seconds
                            currentTime := time.Now().Unix()
                            uptimeSeconds := float64(currentTime - bootSec)
                            resources.Uptime = uptimeSeconds
                            uptimeSet = true
                            break
                        }
                    }
                }
            } else {
                // Try to parse as a simple integer
                if bootSec, err := strconv.ParseInt(boottimeData, 10, 64); err == nil {
                    currentTime := time.Now().Unix()
                    uptimeSeconds := float64(currentTime - bootSec)
                    resources.Uptime = uptimeSeconds
                    uptimeSet = true
                    break
                }
            }
        }
    }

    // If no uptime could be determined, set to 0
    if !uptimeSet {
        resources.Uptime = 0
    }

    return resources, nil
}

// getSystemLoad returns system load averages
func getSystemLoad() ([]float64, error) {
    // Try multiple sysctl paths for load average
    loadPaths := []string{
        "vm.loadavg",
        "kern.loadavg",
        "vm.stats.vm.v_loadavg",
        "kern.cp_time", // Alternative approach
    }

    var data string
    var err error

    for _, path := range loadPaths {
        if data, err = syscall.Sysctl(path); err == nil {
            break
        }
    }

    if err != nil {
        // Try reading from /proc/loadavg as fallback
        if loadData, err := os.ReadFile("/proc/loadavg"); err == nil {
            data = string(loadData)
        } else {
            // Return zeros if all methods fail
            return []float64{0, 0, 0}, nil
        }
    }

    // Parse the load average string
    // Format is typically "1.23 2.34 3.45" or similar
    fields := strings.Fields(data)
    if len(fields) < 3 {
        // If we got unexpected data, try to parse what we can
        if len(fields) > 0 {
            loads := make([]float64, 3)
            for i := 0; i < 3; i++ {
                if i < len(fields) {
                    if val, err := strconv.ParseFloat(fields[i], 64); err == nil {
                        loads[i] = val
                    } else {
                        loads[i] = 0
                    }
                } else {
                    loads[i] = 0
                }
            }
            return loads, nil
        }
        return []float64{0, 0, 0}, nil
    }

    loads := make([]float64, 3)
    for i := 0; i < 3; i++ {
        if i < len(fields) {
            if val, err := strconv.ParseFloat(fields[i], 64); err == nil {
                loads[i] = val
            } else {
                loads[i] = 0
            }
        } else {
            loads[i] = 0
        }
    }

    return loads, nil
}

// getMemoryInfo returns detailed memory information
func getMemoryInfo() (MemoryInfo, error) {
    var info MemoryInfo

    // Initialize maps
    info.Pressure = make(map[string]PressureStats)
    info.OOMScores = make(map[string]int)
    info.Slab = make(map[string]SlabInfo)

    // Try multiple sysctl paths for total memory
    totalMemoryPaths := []string{
        "hw.physmem",
        "hw.realmem",
        "vm.stats.vm.v_page_count",
    }

    var totalMemory uint64
    for _, path := range totalMemoryPaths {
        if data, err := syscall.Sysctl(path); err == nil {
            if val, err := strconv.ParseUint(data, 10, 64); err == nil {
                if path == "vm.stats.vm.v_page_count" {
                    totalMemory = val * 4096 // Convert pages to bytes
                } else {
                    totalMemory = val
                }
                info.Total = totalMemory
                break
            }
        }
    }

    // Try different sysctl paths for memory statistics
    memoryPaths := []string{
        "vm.stats.vm.v_active_count",
        "vm.stats.vm.v_active",
        "vm.stats.vm.v_inactive",
        "vm.stats.vm.v_wire_count",
    }

    // Get used memory
    for _, path := range memoryPaths {
        if data, err := syscall.Sysctl(path); err == nil {
            if val, err := strconv.ParseUint(data, 10, 64); err == nil {
                info.Used = val * 4096 // Convert page count to bytes
                break
            }
        }
    }

    // Get free memory
    freePaths := []string{
        "vm.stats.vm.v_free_count",
        "vm.stats.vm.v_free",
    }
    for _, path := range freePaths {
        if data, err := syscall.Sysctl(path); err == nil {
            if val, err := strconv.ParseUint(data, 10, 64); err == nil {
                info.Free = val * 4096 // Convert page count to bytes
                break
            }
        }
    }

    // Get cached memory
    cachePaths := []string{
        "vm.stats.vm.v_cache_count",
        "vm.stats.vm.v_cache",
    }
    for _, path := range cachePaths {
        if data, err := syscall.Sysctl(path); err == nil {
            if val, err := strconv.ParseUint(data, 10, 64); err == nil {
                info.Cached = val * 4096 // Convert page count to bytes
                break
            }
        }
    }

    // Get buffer memory
    bufferPaths := []string{
        "vm.stats.vm.v_buf_count",
        "vm.stats.vm.v_buf",
    }
    for _, path := range bufferPaths {
        if data, err = syscall.Sysctl(path); err == nil {
            if val, err := strconv.ParseUint(data, 10, 64); err == nil {
                info.Buffers = val * 4096 // Convert page count to bytes
                break
            }
        }
    }

    // If still no data, try reading from /proc/meminfo
    if info.Total == 0 {
        if meminfo, err := os.ReadFile("/proc/meminfo"); err == nil {
            lines := strings.Split(string(meminfo), "\n")
            for _, line := range lines {
                if strings.HasPrefix(line, "MemTotal:") {
                    fields := strings.Fields(line)
                    if len(fields) >= 2 {
                        if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
                            info.Total = val * 1024 // Convert KB to bytes
                        }
                    }
                } else if strings.HasPrefix(line, "MemFree:") {
                    fields := strings.Fields(line)
                    if len(fields) >= 2 {
                        if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
                            info.Free = val * 1024 // Convert KB to bytes
                        }
                    }
                } else if strings.HasPrefix(line, "Cached:") {
                    fields := strings.Fields(line)
                    if len(fields) >= 2 {
                        if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
                            info.Cached = val * 1024 // Convert KB to bytes
                        }
                    }
                } else if strings.HasPrefix(line, "Buffers:") {
                    fields := strings.Fields(line)
                    if len(fields) >= 2 {
                        if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
                            info.Buffers = val * 1024 // Convert KB to bytes
                        }
                    }
                }
            }
        }
    }

    // Calculate available memory
    info.Available = info.Free + info.Cached + info.Buffers

    // Get swap information using vm.swap_info
    data, err = syscall.Sysctl("vm.swap_info")
    if err == nil {
        // Parse swap info (simplified)
        // This is a complex structure, so we'll use a simplified approach
        info.SwapTotal = 0
        info.SwapUsed = 0
        info.SwapFree = 0
    }

    // Initialize pressure and OOM scores maps
    info.Pressure = make(map[string]PressureStats)
    info.OOMScores = make(map[string]int)
    info.Slab = make(map[string]SlabInfo)

    return info, nil
}

// getProcessList returns list of all processes
func getProcessList(options map[string]interface{}) ([]ProcessInfo, error) {
    var processes []ProcessInfo

    // BSD implementation using /proc (if available) or sysctl
    // Try /proc first (available on some BSD systems)
    entries, err := os.ReadDir("/proc")
    if err == nil {
        // Use /proc approach (similar to Linux)
        for _, entry := range entries {
            if !entry.IsDir() {
                continue
            }

            // Check if it's a process directory (numeric name)
            pid, err := strconv.Atoi(entry.Name())
            if err != nil {
                continue
            }

            // Get process info
            process, err := getProcessInfo(pid, options)
            if err == nil {
                processes = append(processes, process)
            }
        }
        return processes, nil
    }

    // Fallback: use sysctl to get process list
    // This is a simplified approach
    data, err := syscall.Sysctl("kern.proc.pid")
    if err != nil {
        return processes, nil
    }

    // Parse process list from sysctl output
    lines := strings.Split(data, "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

        // Try to extract PID from the line
        fields := strings.Fields(line)
        if len(fields) > 0 {
            if pid, err := strconv.Atoi(fields[0]); err == nil {
                process, err := getProcessInfo(pid, options)
                if err == nil {
                    processes = append(processes, process)
                }
            }
        }
    }

    return processes, nil
}

// getProcessInfo returns detailed information for a specific process
func getProcessInfo(pid int, options map[string]interface{}) (ProcessInfo, error) {
    var proc ProcessInfo
    proc.PID = pid

    // Try /proc approach first
    statPath := fmt.Sprintf("/proc/%d/stat", pid)
    data, err := os.ReadFile(statPath)
    if err == nil {
        // Parse /proc/{pid}/stat (similar to Linux)
        fields := strings.Fields(string(data))
        if len(fields) < 24 {
            return proc, fmt.Errorf("invalid stat format")
        }

        // Parse basic process information
        proc.Name = strings.Trim(fields[1], "()")
        proc.State = fields[2]
        proc.PPID, _ = strconv.Atoi(fields[3])
        proc.Priority, _ = strconv.Atoi(fields[17])
        proc.Nice, _ = strconv.Atoi(fields[18])
        proc.StartTime, _ = strconv.ParseInt(fields[21], 10, 64)
        proc.Threads, _ = strconv.Atoi(fields[19])

        // Parse CPU timing information
        if len(fields) >= 22 {
            proc.UserTime, _ = strconv.ParseFloat(fields[13], 64)
            proc.SystemTime, _ = strconv.ParseFloat(fields[14], 64)
            proc.ChildrenUserTime, _ = strconv.ParseFloat(fields[15], 64)
            proc.ChildrenSystemTime, _ = strconv.ParseFloat(fields[16], 64)
        }

        // Read command line if requested
        if options != nil && options["include_cmdline"] == true {
            cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
            if cmdline, err := os.ReadFile(cmdlinePath); err == nil {
                proc.Command = strings.ReplaceAll(string(cmdline), "\x00", " ")
            }
        }

        // Read memory information from /proc/{pid}/statm
        statmPath := fmt.Sprintf("/proc/%d/statm", pid)
        if statmData, err := os.ReadFile(statmPath); err == nil {
            statmFields := strings.Fields(string(statmData))
            if len(statmFields) >= 2 {
                size, _ := strconv.ParseUint(statmFields[0], 10, 64)
                rss, _ := strconv.ParseUint(statmFields[1], 10, 64)
                proc.MemoryUsage = size * 4096 // Convert pages to bytes
                proc.MemoryRSS = rss * 4096
            }
        }

        // Get user/group info
        if stat, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
            if sysStat, ok := stat.Sys().(*syscall.Stat_t); ok {
                proc.UID = fmt.Sprintf("%d", sysStat.Uid)
                proc.GID = fmt.Sprintf("%d", sysStat.Gid)
            }
        }

        return proc, nil
    }

    // Fallback: use sysctl for process info
    // This is a simplified approach since BSD sysctl process info is complex
    proc.Name = fmt.Sprintf("process-%d", pid)
    proc.State = "unknown"
    proc.PPID = 0
    proc.Priority = 0
    proc.Nice = 0
    proc.StartTime = 0
    proc.Threads = 1
    proc.UserTime = 0
    proc.SystemTime = 0
    proc.ChildrenUserTime = 0
    proc.ChildrenSystemTime = 0
    proc.MemoryUsage = 0
    proc.MemoryRSS = 0
    proc.UID = "0"
    proc.GID = "0"
    proc.Command = proc.Name

    return proc, nil
}

// getProcessTree returns process hierarchy
func getProcessTree(startPID int) (ProcessTree, error) {
    var tree ProcessTree

    if startPID == -1 {
        startPID = 1 // Start from init
    }

    // Get process info
    proc, err := getProcessInfo(startPID, nil)
    if err != nil {
        return tree, err
    }

    tree.PID = proc.PID
    tree.Name = proc.Name

    // Find children
    processes, err := getProcessList(nil)
    if err != nil {
        return tree, err
    }

    for _, p := range processes {
        if p.PPID == startPID {
            child, err := getProcessTree(p.PID)
            if err == nil {
                tree.Children = append(tree.Children, child)
            }
        }
    }

    return tree, nil
}

// getProcessMap returns process relationships
func getProcessMap(startPID int) (ProcessMap, error) {
    var pmap ProcessMap

    if startPID == -1 {
        startPID = 1 // Start from init
    }

    // Get process info
    proc, err := getProcessInfo(startPID, nil)
    if err != nil {
        return pmap, err
    }

    pmap.PID = proc.PID
    pmap.Name = proc.Name
    pmap.Relations = make(map[string][]ProcessMap)

    // Find relationships
    processes, err := getProcessList(nil)
    if err != nil {
        return pmap, err
    }

    // Find parent
    if proc.PPID != 0 {
        if parent, err := getProcessInfo(proc.PPID, nil); err == nil {
            pmap.Relations["parent"] = []ProcessMap{{
                PID:  parent.PID,
                Name: parent.Name,
            }}
        }
    }

    // Find children
    var children []ProcessMap
    for _, p := range processes {
        if p.PPID == startPID {
            child, err := getProcessMap(p.PID)
            if err == nil {
                children = append(children, child)
            }
        }
    }
    if len(children) > 0 {
        pmap.Relations["children"] = children
    }

    return pmap, nil
}

// getCPUInfo returns CPU information
func getCPUInfo(coreNumber int, options map[string]interface{}) (CPUInfo, error) {
    var info CPUInfo
    includeDetails := false

    // Check if we should include detailed information
    if options != nil && options["details"] != nil {
        if details, ok := options["details"].(bool); ok {
            includeDetails = details
        }
    }

    // Get CPU count
    info.Cores = runtime.NumCPU()
    info.Threads = runtime.NumCPU()

    // Validate core number if specified
    if coreNumber >= 0 {
        if coreNumber >= info.Cores {
            return info, fmt.Errorf("invalid core number %d: system has %d cores", coreNumber, info.Cores)
        }
    }

    // Get CPU model via sysctl
    data, err := syscall.Sysctl("hw.model")
    if err == nil {
        info.Model = strings.TrimSpace(data)
    }

    // Get detailed CPU information if requested
    if includeDetails {
        // Initialize detailed info map if not already present
        if info.Usage == nil {
            info.Usage = make(map[string]interface{})
        }

        // Get CPU architecture
        if arch, err := syscall.Sysctl("hw.machine"); err == nil {
            info.Usage["architecture"] = strings.TrimSpace(arch)
        }

        // Get CPU frequency
        if freq, err := syscall.Sysctl("dev.cpu.0.freq"); err == nil {
            if freqVal, err := strconv.ParseUint(freq, 10, 64); err == nil {
                info.Usage["frequency"] = freqVal
            }
        }

        // Get CPU cache information
        if l1d, err := syscall.Sysctl("hw.cache.l1d.size"); err == nil {
            if l1dSize, err := strconv.ParseUint(l1d, 10, 64); err == nil {
                info.Usage["l1d_cache"] = l1dSize
            }
        }

        if l1i, err := syscall.Sysctl("hw.cache.l1i.size"); err == nil {
            if l1iSize, err := strconv.ParseUint(l1i, 10, 64); err == nil {
                info.Usage["l1i_cache"] = l1iSize
            }
        }

        if l2, err := syscall.Sysctl("hw.cache.l2.size"); err == nil {
            if l2Size, err := strconv.ParseUint(l2, 10, 64); err == nil {
                info.Usage["l2_cache"] = l2Size
            }
        }

        if l3, err := syscall.Sysctl("hw.cache.l3.size"); err == nil {
            if l3Size, err := strconv.ParseUint(l3, 10, 64); err == nil {
                info.Usage["l3_cache"] = l3Size
            }
        }

        // Get CPU temperature if available
        if temp, err := syscall.Sysctl("dev.cpu.0.temperature"); err == nil {
            if tempVal, err := strconv.ParseFloat(temp, 64); err == nil {
                info.Usage["temperature"] = tempVal
            }
        }

        // Get CPU vendor
        if vendor, err := syscall.Sysctl("hw.vendor"); err == nil {
            info.Usage["vendor"] = strings.TrimSpace(vendor)
        }

        // Get CPU stepping and revision
        if stepping, err := syscall.Sysctl("hw.cpu.stepping"); err == nil {
            info.Usage["stepping"] = strings.TrimSpace(stepping)
        }

        if revision, err := syscall.Sysctl("hw.cpu.revision"); err == nil {
            info.Usage["revision"] = strings.TrimSpace(revision)
        }
    }

    // Get CPU usage
    if coreNumber >= 0 {
        // Return data for specific core
        info.Usage = make(map[string]interface{})
        info.Usage["core"] = coreNumber

        // Get CPU usage using BSD sysctl
        // Use BSD sysctl to get real CPU usage data

        // Get CPU usage from sysctl
        cpuTimeData, err := syscall.Sysctl("kern.cp_time")
        if err == nil {
            // Parse CPU time data
            fields := strings.Fields(cpuTimeData)
            if len(fields) >= 4 {
                user, _ := strconv.ParseUint(fields[0], 10, 64)
                nice, _ := strconv.ParseUint(fields[1], 10, 64)
                system, _ := strconv.ParseUint(fields[2], 10, 64)
                idle, _ := strconv.ParseUint(fields[3], 10, 64)

                // Calculate percentages
                total := user + nice + system + idle
                if total > 0 {
                    userPercent := float64(user) / float64(total) * 100.0
                    systemPercent := float64(system) / float64(total) * 100.0
                    idlePercent := float64(idle) / float64(total) * 100.0

                    info.Usage["user"] = userPercent
                    info.Usage["system"] = systemPercent
                    info.Usage["idle"] = idlePercent
                } else {
                    info.Usage["user"] = 0.0
                    info.Usage["system"] = 0.0
                    info.Usage["idle"] = 100.0
                }
            }
        } else {
            // Fallback values
            info.Usage["user"] = 0.0
            info.Usage["system"] = 0.0
            info.Usage["idle"] = 100.0
        }
    } else {
        // Return data for all cores
        info.Usage = make(map[string]interface{})
        cores := make(map[string]interface{})

        for i := 0; i < info.Cores; i++ {
            coreData := make(map[string]interface{})

            // Get CPU usage using BSD sysctl
            // Use BSD sysctl to get real CPU usage data for each core

            // For multi-core systems, we'll use the overall system times
            // since per-core CPU times require more complex sysctl queries
            cpuTimeData, err := syscall.Sysctl("kern.cp_time")
            if err == nil {
                // Parse CPU time data
                fields := strings.Fields(cpuTimeData)
                if len(fields) >= 4 {
                    user, _ := strconv.ParseUint(fields[0], 10, 64)
                    nice, _ := strconv.ParseUint(fields[1], 10, 64)
                    system, _ := strconv.ParseUint(fields[2], 10, 64)
                    idle, _ := strconv.ParseUint(fields[3], 10, 64)

                    // Calculate percentages
                    total := user + nice + system + idle
                    if total > 0 {
                        userPercent := float64(user) / float64(total) * 100.0
                        systemPercent := float64(system) / float64(total) * 100.0
                        idlePercent := float64(idle) / float64(total) * 100.0

                        coreData["user"] = userPercent
                        coreData["system"] = systemPercent
                        coreData["idle"] = idlePercent
                    } else {
                        coreData["user"] = 0.0
                        coreData["system"] = 0.0
                        coreData["idle"] = 100.0
                    }
                }
            } else {
                // Fallback values
                coreData["user"] = 0.0
                coreData["system"] = 0.0
                coreData["idle"] = 100.0
            }
            cores[fmt.Sprintf("core_%d", i)] = coreData
        }
        info.Usage["cores"] = cores
    }

    // Get load average
    load, err := getSystemLoad()
    if err == nil {
        info.LoadAverage = load
    }

    return info, nil
}

// getNetworkIO returns network I/O statistics
func getNetworkIO(options map[string]interface{}) ([]NetworkIOStats, error) {
    var stats []NetworkIOStats

    // Use netstat -i -b -n for reliable network statistics on BSD
    cmd := exec.Command("netstat", "-i", "-b", "-n")
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("netstat command failed: %v", err)
    }

    // Parse netstat output to extract statistics
    // Format: Name Mtu Network Address Ipkts Ierrs Idrop Ibytes Opkts Oerrs Obytes Coll
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "Name") {
            continue // Skip header line
        }

        fields := strings.Fields(line)
        if len(fields) < 12 {
            continue // Need at least 12 fields for complete stats
        }

        interfaceName := fields[0]

        // Apply interface filter if specified
        if options != nil && options["interface"] != nil {
            if interfaceName != options["interface"].(string) {
                continue
            }
        }

        // Parse statistics from netstat output
        var rxPackets, txPackets, rxBytes, txBytes uint64
        var rxErrors, txErrors, rxDropped, collisions uint64

        // Parse input packets (Ipkts)
        if val, err := strconv.ParseUint(fields[4], 10, 64); err == nil {
            rxPackets = val
        }

        // Parse input errors (Ierrs)
        if val, err := strconv.ParseUint(fields[5], 10, 64); err == nil {
            rxErrors = val
        }

        // Parse input drops (Idrop)
        if val, err := strconv.ParseUint(fields[6], 10, 64); err == nil {
            rxDropped = val
        }

        // Parse input bytes (Ibytes)
        if val, err := strconv.ParseUint(fields[7], 10, 64); err == nil {
            rxBytes = val
        }

        // Parse output packets (Opkts)
        if val, err := strconv.ParseUint(fields[8], 10, 64); err == nil {
            txPackets = val
        }

        // Parse output errors (Oerrs)
        if val, err := strconv.ParseUint(fields[9], 10, 64); err == nil {
            txErrors = val
        }

        // Parse output bytes (Obytes)
        if val, err := strconv.ParseUint(fields[10], 10, 64); err == nil {
            txBytes = val
        }

        // Parse collisions (Coll)
        if val, err := strconv.ParseUint(fields[11], 10, 64); err == nil {
            collisions = val
        }

        // Only include interfaces with actual data
        if rxBytes > 0 || txBytes > 0 || rxPackets > 0 || txPackets > 0 {
            stats = append(stats, NetworkIOStats{
                Interface:  interfaceName,
                RxBytes:    rxBytes,
                TxBytes:    txBytes,
                RxPackets:  rxPackets,
                TxPackets:  txPackets,
                RxErrors:   rxErrors,
                TxErrors:   txErrors,
                RxDropped:  rxDropped,
                TxDropped:  0,          // netstat doesn't provide tx_dropped, set to 0
                Collisions: collisions, // set from parsed value
            })
        }
    }

    return stats, nil
}

// getDiskIO returns disk I/O statistics
func getDiskIO(options map[string]interface{}) ([]DiskIOStats, error) {
    var stats []DiskIOStats

    // Get disk stats via iostat command
    // This is a more reliable approach on FreeBSD
    cmd := exec.Command("iostat", "-x")
    output, err := cmd.Output()
    if err != nil {
        return stats, fmt.Errorf("iostat command failed")
    }

    // Parse iostat output to extract statistics for all devices
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "extended device statistics") {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 6 {
            continue
        }

        device := fields[0]

        // Apply device filter if specified
        if options != nil && options["device"] != nil {
            if device != options["device"].(string) {
                continue
            }
        }

        // Parse read/write statistics from iostat output
        // Format: device r/s w/s kr/s kw/s ms/r ms/w ms/o ms/t qlen %b
        var readBytes, writeBytes, readOps, writeOps uint64
        var readTime, writeTime uint64

        if len(fields) >= 5 {
            if val, err := strconv.ParseFloat(fields[1], 64); err == nil {
                readOps = uint64(val)
            }
            if val, err := strconv.ParseFloat(fields[2], 64); err == nil {
                writeOps = uint64(val)
            }
            if val, err := strconv.ParseFloat(fields[3], 64); err == nil {
                readBytes = uint64(val * 1024) // Convert KB to bytes
            }
            if val, err := strconv.ParseFloat(fields[4], 64); err == nil {
                writeBytes = uint64(val * 1024) // Convert KB to bytes
            }
        }

        // If no data found, skip this device
        if readBytes == 0 && writeBytes == 0 {
            continue
        }

        stats = append(stats, DiskIOStats{
            Device:     device,
            ReadBytes:  readBytes,
            WriteBytes: writeBytes,
            ReadOps:    readOps,
            WriteOps:   writeOps,
            ReadTime:   readTime,
            WriteTime:  writeTime,
        })
    }

    return stats, nil
}

// getSlabInfo returns empty map on BSD (no /proc/slabinfo)
func getSlabInfo() map[string]SlabInfo {
    return make(map[string]SlabInfo)
}

// getDiskUsage returns disk usage information (BSD implementation)
func getDiskUsage(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Use df command for reliable disk usage information on BSD
    cmd := exec.Command("df", "-k")
    output, err := cmd.Output()
    if err != nil {
        return result, fmt.Errorf("no such file or directory")
    }

    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "Filesystem") {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 6 {
            continue
        }

        device := fields[0]
        totalKB, _ := strconv.ParseUint(fields[1], 10, 64)
        usedKB, _ := strconv.ParseUint(fields[2], 10, 64)
        availableKB, _ := strconv.ParseUint(fields[3], 10, 64)
        usagePercentStr := strings.TrimSuffix(fields[4], "%")
        usagePercent, _ := strconv.ParseFloat(usagePercentStr, 64)
        mountPoint := fields[5]

        // Convert KB to bytes
        total := totalKB * 1024
        used := usedKB * 1024
        available := availableKB * 1024

        // Apply filters if specified
        if options != nil {
            if excludePatterns, exists := options["exclude_patterns"]; exists {
                if patterns, ok := excludePatterns.([]string); ok {
                    for _, pattern := range patterns {
                        if strings.Contains(device, pattern) || strings.Contains(mountPoint, pattern) {
                            continue
                        }
                    }
                }
            }
        }

        diskInfo := map[string]interface{}{
            "path":          device,
            "size":          total,
            "used":          used,
            "available":     available,
            "usage_percent": usagePercent,
            "mounted_path":  mountPoint,
        }

        result = append(result, diskInfo)
    }

    return result, nil
}

// getMountInfo returns mount point information (BSD implementation)
func getMountInfo(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Use mount command for reliable mount information on BSD
    cmd := exec.Command("mount")
    output, err := cmd.Output()
    if err != nil {
        return result, fmt.Errorf("no such file or directory")
    }

    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        // Parse mount output format: device on mountpoint type filesystem (options)
        // Example: /dev/ada0p2 on / (ufs, local, soft-updates)
        fields := strings.Fields(line)
        if len(fields) < 4 {
            continue
        }

        device := fields[0]
        mountPoint := fields[2]
        filesystem := strings.TrimSuffix(fields[4], ",") // Remove trailing comma
        mountOptions := ""
        if len(fields) > 5 {
            // Extract options from parentheses
            optionsStart := strings.Index(line, "(")
            optionsEnd := strings.Index(line, ")")
            if optionsStart > 0 && optionsEnd > optionsStart {
                mountOptions = line[optionsStart+1 : optionsEnd]
            }
        }

        // Apply filters if specified
        if options != nil {
            if filesystemFilter, exists := options["filesystem"]; exists {
                if fs, ok := filesystemFilter.(string); ok {
                    if filesystem != fs {
                        continue
                    }
                }
            }
        }

        mountInfo := map[string]interface{}{
            "device":        device,
            "mounted":       true,
            "mounted_path":  mountPoint,
            "filesystem":    filesystem,
            "mount_options": mountOptions,
        }

        result = append(result, mountInfo)
    }

    return result, nil
}

// getResourceUsage returns resource usage for a specific process
func getResourceUsage(pid int) (ResourceUsage, error) {
    var usage ResourceUsage
    usage.PID = pid

    // Try to get process information using /proc first
    statPath := fmt.Sprintf("/proc/%d/stat", pid)
    data, err := os.ReadFile(statPath)
    if err == nil {
        // Parse /proc/{pid}/stat (similar to Linux)
        fields := strings.Fields(string(data))
        if len(fields) >= 24 {
            // Parse CPU timing information
            if len(fields) >= 22 {
                utime, _ := strconv.ParseFloat(fields[13], 64)
                stime, _ := strconv.ParseFloat(fields[14], 64)
                cutime, _ := strconv.ParseFloat(fields[15], 64)
                cstime, _ := strconv.ParseFloat(fields[16], 64)
                usage.CPUUser = utime / 100.0            // Convert to seconds
                usage.CPUSystem = stime / 100.0          // Convert to seconds
                usage.CPUChildrenUser = cutime / 100.0   // Convert to seconds
                usage.CPUChildrenSystem = cstime / 100.0 // Convert to seconds
            }

            // Read memory information from /proc/{pid}/statm
            statmPath := fmt.Sprintf("/proc/%d/statm", pid)
            if statmData, err := os.ReadFile(statmPath); err == nil {
                statmFields := strings.Fields(string(statmData))
                if len(statmFields) >= 2 {
                    size, _ := strconv.ParseUint(statmFields[0], 10, 64)
                    rss, _ := strconv.ParseUint(statmFields[1], 10, 64)
                    usage.MemoryCurrent = rss * 4096 // Convert pages to bytes
                    usage.MemoryPeak = size * 4096
                }
            }

            return usage, nil
        }
    }

    // Fallback: use sysctl for process info
    // Try different sysctl paths for process information
    procPaths := []string{
        fmt.Sprintf("kern.proc.pid.%d", pid),
        fmt.Sprintf("kern.proc.%d", pid),
        fmt.Sprintf("vm.proc.%d", pid),
    }

    for _, path := range procPaths {
        if procData, err := syscall.Sysctl(path); err == nil {
            // Parse process data
            lines := strings.Split(procData, "\n")
            if len(lines) > 0 {
                // Parse the first line which contains process info
                fields := strings.Fields(lines[0])
                if len(fields) >= 17 {
                    // Get memory usage from process data
                    if len(fields) >= 6 {
                        if rss, err := strconv.ParseUint(fields[5], 10, 64); err == nil {
                            usage.MemoryCurrent = rss * 1024 // Convert from KB to bytes
                        }
                    }

                    // Parse CPU times if available
                    if len(fields) >= 17 {
                        utime, _ := strconv.ParseUint(fields[13], 10, 64)
                        stime, _ := strconv.ParseUint(fields[14], 10, 64)
                        cutime, _ := strconv.ParseUint(fields[15], 10, 64)
                        cstime, _ := strconv.ParseUint(fields[16], 10, 64)
                        usage.CPUUser = float64(utime) / 100.0            // Convert to seconds
                        usage.CPUSystem = float64(stime) / 100.0          // Convert to seconds
                        usage.CPUChildrenUser = float64(cutime) / 100.0   // Convert to seconds
                        usage.CPUChildrenSystem = float64(cstime) / 100.0 // Convert to seconds
                    }

                    return usage, nil
                }
            }
        }
    }

    // If all else fails, return basic info
    usage.CPUUser = 0
    usage.CPUSystem = 0
    usage.CPUChildrenUser = 0
    usage.CPUChildrenSystem = 0
    usage.MemoryCurrent = 0
    usage.MemoryPeak = 0

    return usage, nil
}

// calculateIODiff calculates throughput rates between two snapshots
func calculateIODiff(snapshot1, snapshot2 ResourceSnapshot, duration time.Duration) map[string]interface{} {
    result := make(map[string]interface{})

    // Calculate network throughput
    if len(snapshot1.Network) > 0 && len(snapshot2.Network) > 0 {
        networkThroughput := make(map[string]interface{})
        for _, net1 := range snapshot1.Network {
            for _, net2 := range snapshot2.Network {
                if net1.Interface == net2.Interface {
                    seconds := duration.Seconds()
                    if seconds > 0 {
                        rxRate := float64(net2.RxBytes-net1.RxBytes) / seconds
                        txRate := float64(net2.TxBytes-net1.TxBytes) / seconds
                        networkThroughput[net1.Interface] = map[string]interface{}{
                            "rx_bytes_per_sec": rxRate,
                            "tx_bytes_per_sec": txRate,
                        }
                    }
                    break
                }
            }
        }
        result["network_throughput"] = networkThroughput
    }

    // Calculate disk throughput
    if len(snapshot1.Disk) > 0 && len(snapshot2.Disk) > 0 {
        diskThroughput := make(map[string]interface{})
        for _, disk1 := range snapshot1.Disk {
            for _, disk2 := range snapshot2.Disk {
                if disk1.Device == disk2.Device {
                    seconds := duration.Seconds()
                    if seconds > 0 {
                        readRate := float64(disk2.ReadBytes-disk1.ReadBytes) / seconds
                        writeRate := float64(disk2.WriteBytes-disk1.WriteBytes) / seconds
                        diskThroughput[disk1.Device] = map[string]interface{}{
                            "read_bytes_per_sec":  readRate,
                            "write_bytes_per_sec": writeRate,
                        }
                    }
                    break
                }
            }
        }
        result["disk_throughput"] = diskThroughput
    }

    return result
}

// getNetworkDevices returns network device information (BSD implementation)
func getNetworkDevices(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Get network interfaces using getifaddrs
    ifaces, err := net.Interfaces()
    if err != nil {
        return result, err
    }

    includeAll := false
    if options != nil {
        if all, exists := options["all"]; exists {
            if include, ok := all.(bool); ok {
                includeAll = include
            }
        }
    }

    for _, iface := range ifaces {
        // Skip down interfaces unless include_all is true
        if iface.Flags&net.FlagUp == 0 && !includeAll {
            continue
        }

        // Get IP addresses
        ipAddresses := []string{}
        addrs, err := iface.Addrs()
        if err == nil {
            for _, addr := range addrs {
                if ipnet, ok := addr.(*net.IPNet); ok {
                    ipAddresses = append(ipAddresses, ipnet.IP.String())
                }
            }
        }

        // Get gateway (simplified - would need routing table parsing)
        gateway := ""
        if iface.Flags&net.FlagUp != 0 {
            gateway = "default"
        }

        // Get link speed and duplex using sysctl
        linkSpeed := ""
        duplex := ""

        // Try to get link speed and duplex from sysctl
        if iface.Flags&net.FlagUp != 0 {
            // Get link speed from sysctl
            speedPath := fmt.Sprintf("dev.%s.%s.speed", iface.Name, "media")
            if speedData, err := unix.Sysctl(speedPath); err == nil {
                if speed, err := strconv.Atoi(speedData); err == nil {
                    linkSpeed = fmt.Sprintf("%d", speed)
                }
            }

            // Get duplex from sysctl
            duplexPath := fmt.Sprintf("dev.%s.%s.duplex", iface.Name, "media")
            if duplexData, err := unix.Sysctl(duplexPath); err == nil {
                duplex = duplexData
            }

            // If sysctl didn't work, try alternative paths
            if linkSpeed == "" {
                // Try different sysctl paths for speed
                speedPaths := []string{
                    fmt.Sprintf("dev.%s.speed", iface.Name),
                    fmt.Sprintf("hw.%s.speed", iface.Name),
                }
                for _, path := range speedPaths {
                    if speedData, err := unix.Sysctl(path); err == nil {
                        if speed, err := strconv.Atoi(speedData); err == nil {
                            linkSpeed = fmt.Sprintf("%d", speed)
                            break
                        }
                    }
                }
            }

            if duplex == "" {
                // Try different sysctl paths for duplex
                duplexPaths := []string{
                    fmt.Sprintf("dev.%s.duplex", iface.Name),
                    fmt.Sprintf("hw.%s.duplex", iface.Name),
                }
                for _, path := range duplexPaths {
                    if duplexData, err := unix.Sysctl(path); err == nil {
                        duplex = duplexData
                        break
                    }
                }
            }

            // If still no data, provide reasonable defaults
            if linkSpeed == "" {
                linkSpeed = "100"
            }
            if duplex == "" {
                duplex = "full"
            }
        }

        // Determine device type based on interface name and flags
        deviceType := "ethernet" // Default
        if strings.Contains(strings.ToLower(iface.Name), "wlan") || strings.Contains(strings.ToLower(iface.Name), "wireless") {
            deviceType = "wireless"
        } else if strings.Contains(strings.ToLower(iface.Name), "lo") || strings.Contains(strings.ToLower(iface.Name), "loopback") {
            deviceType = "loopback"
        } else if strings.Contains(strings.ToLower(iface.Name), "bridge") {
            deviceType = "bridge"
        } else if strings.Contains(strings.ToLower(iface.Name), "vlan") {
            deviceType = "vlan"
        } else if strings.Contains(strings.ToLower(iface.Name), "tun") || strings.Contains(strings.ToLower(iface.Name), "tap") {
            deviceType = "tunnel"
        } else if strings.Contains(strings.ToLower(iface.Name), "lagg") {
            deviceType = "bond"
        }

        // Determine operstate based on interface flags
        operstate := "down"
        if iface.Flags&net.FlagUp != 0 {
            operstate = "up"
        }

        deviceInfo := map[string]interface{}{
            "name":         iface.Name,
            "enabled":      iface.Flags&net.FlagUp != 0,
            "mac_address":  iface.HardwareAddr.String(),
            "ip_addresses": ipAddresses,
            "gateway":      gateway,
            "link_speed":   linkSpeed,
            "duplex":       duplex,
            "device_type":  deviceType,
            "operstate":    operstate,
        }

        result = append(result, deviceInfo)
    }

    return result, nil
}

// getDefaultGatewayInterface returns the name of the default gateway interface (BSD implementation)
func getDefaultGatewayInterface() (string, error) {
    // Use BSD routing table to get the default gateway interface
    // This is a simplified implementation

    // Get all network interfaces
    ifaces, err := net.Interfaces()
    if err != nil {
        return "", err
    }

    // Look for interfaces that are up and have an IP address
    for _, iface := range ifaces {
        if iface.Flags&net.FlagUp == 0 {
            continue
        }

        addrs, err := iface.Addrs()
        if err != nil {
            continue
        }

        // Check if this interface has an IP address
        for _, addr := range addrs {
            if ipnet, ok := addr.(*net.IPNet); ok {
                if !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
                    // This is a good candidate for the default gateway interface
                    // For now, return the first non-loopback interface with an IPv4 address
                    return iface.Name, nil
                }
            }
        }
    }

    return "", fmt.Errorf("no suitable default gateway interface found")
}

// getDefaultGatewayAddress returns the IP address of the default gateway (BSD implementation)
func getDefaultGatewayAddress() (string, error) {
    // Use netstat command to get routing table on BSD
    cmd := exec.Command("netstat", "-rn")
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("netstat command failed: %v", err)
    }

    // Parse netstat output to find default gateway
    gateway := parseNetstatRoutingTable(string(output))
    if gateway == "" {
        return "", fmt.Errorf("no default gateway found in routing table")
    }

    return gateway, nil
}

// parseNetstatRoutingTable parses BSD netstat routing table output to find default gateway
func parseNetstatRoutingTable(output string) string {
    lines := strings.Split(output, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "Routing tables") || strings.HasPrefix(line, "Destination") {
            continue // Skip header lines
        }

        fields := strings.Fields(line)
        if len(fields) < 4 {
            continue
        }

        // Look for default route (destination is "default" or "0.0.0.0")
        destination := fields[0]
        if destination == "default" || destination == "0.0.0.0" {
            // The gateway is typically in the 2nd or 3rd field
            // Format: Destination Gateway Flags Refs Use Netif
            if len(fields) >= 2 {
                gateway := fields[1]
                // Validate it's a proper IP address
                if net.ParseIP(gateway) != nil && gateway != "0.0.0.0" {
                    return gateway
                }
            }
        }
    }
    return ""
}

// parseNetstatRoutingTableWithInterface parses BSD netstat routing table output to find default gateway and interface
func parseNetstatRoutingTableWithInterface(output string) (string, string) {
    lines := strings.Split(output, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "Routing tables") || strings.HasPrefix(line, "Destination") {
            continue // Skip header lines
        }

        fields := strings.Fields(line)
        if len(fields) < 4 {
            continue
        }

        // Look for default route (destination is "default" or "0.0.0.0")
        destination := fields[0]
        if destination == "default" || destination == "0.0.0.0" {
            // The gateway is typically in the 2nd field, interface in the last field
            // Format: Destination Gateway Flags Refs Use Netif
            if len(fields) >= 2 {
                gateway := fields[1]
                // Validate it's a proper IP address
                if net.ParseIP(gateway) != nil && gateway != "0.0.0.0" {
                    // Get the interface name (last field)
                    interfaceName := ""
                    if len(fields) >= 6 {
                        interfaceName = fields[5] // Netif field
                    }
                    return gateway, interfaceName
                }
            }
        }
    }
    return "", ""
}

// getDefaultGatewayInfo returns complete default gateway information (BSD implementation)
func getDefaultGatewayInfo() (map[string]interface{}, error) {
    // Get gateway address using netstat
    gateway, err := getDefaultGatewayAddress()
    if err != nil {
        return nil, err
    }

    // Get interface name using the same approach as getDefaultGatewayInterface
    interfaceName, err := getDefaultGatewayInterface()
    if err != nil {
        // If we can't get the interface name, return just the gateway
        return map[string]interface{}{
            "interface": "",
            "gateway":   gateway,
        }, nil
    }

    return map[string]interface{}{
        "interface": interfaceName,
        "gateway":   gateway,
    }, nil
}

// debugCPUFiles returns debug information about available CPU files (BSD implementation)
func debugCPUFiles() map[string]interface{} {
    result := map[string]interface{}{
        "platform":    "BSD",
        "cpu_files":   []string{},
        "available":   true,
        "description": "CPU debugging using BSD sysctl",
        "sysctl_paths": []string{
            "hw.ncpu",
            "hw.model",
            "hw.machine",
            "hw.physmem",
            "kern.cp_time",
        },
    }

    // Try to get actual CPU information
    cpuCount, err := syscall.Sysctl("hw.ncpu")
    if err == nil {
        if count, err := strconv.Atoi(cpuCount); err == nil {
            result["cpu_count"] = count
        }
    }

    // Get CPU model
    cpuModel, err := syscall.Sysctl("hw.model")
    if err == nil {
        result["cpu_model"] = strings.TrimSpace(cpuModel)
    }

    // Get machine architecture
    machine, err := syscall.Sysctl("hw.machine")
    if err == nil {
        result["machine"] = strings.TrimSpace(machine)
    }

    // Get physical memory
    physmem, err := syscall.Sysctl("hw.physmem")
    if err == nil {
        if mem, err := strconv.ParseUint(physmem, 10, 64); err == nil {
            result["physical_memory"] = mem
        }
    }

    // Get CPU time
    cpuTime, err := syscall.Sysctl("kern.cp_time")
    if err == nil {
        result["cpu_time"] = cpuTime
    }

    return result
}
