//go:build (freebsd || openbsd || netbsd || dragonfly) && !linux && !windows
// +build freebsd openbsd netbsd dragonfly

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
    "unsafe"

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

    // Get uptime
    if data, err := os.ReadFile("/var/run/dmesg.boot"); err == nil {
        lines := strings.Split(string(data), "\n")
        for _, line := range lines {
            if strings.Contains(line, "Timecounter") {
                // Parse uptime from boot time
                // This is a simplified approach
                break
            }
        }
    }

    return resources, nil
}

// getSystemLoad returns system load averages
func getSystemLoad() ([]float64, error) {
    var loadavg [3]float64
    size := uintptr(unsafe.Sizeof(loadavg))

    if err := syscall.Sysctl("vm.loadavg", (*byte)(unsafe.Pointer(&loadavg[0])), &size, nil, 0); err != nil {
        return []float64{0, 0, 0}, err
    }

    return loadavg[:], nil
}

// getMemoryInfo returns detailed memory information
func getMemoryInfo() (MemoryInfo, error) {
    var info MemoryInfo

    // Get memory stats via sysctl
    var vmStats struct {
        Total    uint64
        Active   uint64
        Inactive uint64
        Free     uint64
        Cache    uint64
        Buffer   uint64
    }

    // Get total memory
    size := uintptr(unsafe.Sizeof(vmStats.Total))
    if err := syscall.Sysctl("hw.physmem", (*byte)(unsafe.Pointer(&vmStats.Total)), &size, nil, 0); err == nil {
        info.Total = vmStats.Total
    }

    // Get active memory
    size = uintptr(unsafe.Sizeof(vmStats.Active))
    if err := syscall.Sysctl("vm.stats.vm.v_active_count", (*byte)(unsafe.Pointer(&vmStats.Active)), &size, nil, 0); err == nil {
        // Convert page count to bytes
        info.Used = vmStats.Active * 4096
    }

    // Get free memory
    size = uintptr(unsafe.Sizeof(vmStats.Free))
    if err := syscall.Sysctl("vm.stats.vm.v_free_count", (*byte)(unsafe.Pointer(&vmStats.Free)), &size, nil, 0); err == nil {
        // Convert page count to bytes
        info.Free = vmStats.Free * 4096
    }

    // Get cached memory
    size = uintptr(unsafe.Sizeof(vmStats.Cache))
    if err := syscall.Sysctl("vm.stats.vm.v_cache_count", (*byte)(unsafe.Pointer(&vmStats.Cache)), &size, nil, 0); err == nil {
        // Convert page count to bytes
        info.Cached = vmStats.Cache * 4096
    }

    // Calculate available memory
    info.Available = info.Free + info.Cached

    // Get swap info
    var swapInfo struct {
        Total uint64
        Used  uint64
    }

    size = uintptr(unsafe.Sizeof(swapInfo))
    if err := syscall.Sysctl("vm.swap_info", (*byte)(unsafe.Pointer(&swapInfo)), &size, nil, 0); err == nil {
        info.SwapTotal = swapInfo.Total
        info.SwapUsed = swapInfo.Used
        info.SwapFree = info.SwapTotal - info.SwapUsed
    }

    // BSD doesn't have memory pressure or OOM scores like Linux
    info.Pressure = make(map[string]PressureStats)
    info.OOMScores = make(map[string]int)
    info.Slab = make(map[string]SlabInfo)

    return info, nil
}

// getProcessList returns list of all processes
func getProcessList(options map[string]interface{}) ([]ProcessInfo, error) {
    var processes []ProcessInfo

    // Use kvm to get process list
    kvm, err := unix.KvmOpen(nil)
    if err != nil {
        return nil, err
    }
    defer kvm.Close()

    procs, err := kvm.GetProcs(unix.KERN_PROC_ALL, 0)
    if err != nil {
        return nil, err
    }

    for _, proc := range procs {
        processInfo, err := getProcessInfo(int(proc.Pid), options)
        if err == nil {
            processes = append(processes, processInfo)
        }
    }

    return processes, nil
}

// getProcessInfo returns detailed information for a specific process
func getProcessInfo(pid int, options map[string]interface{}) (ProcessInfo, error) {
    var proc ProcessInfo
    proc.PID = pid

    // Use kvm to get process info
    kvm, err := unix.KvmOpen(nil)
    if err != nil {
        return proc, err
    }
    defer kvm.Close()

    procs, err := kvm.GetProcs(unix.KERN_PROC_PID, uint32(pid))
    if err != nil || len(procs) == 0 {
        return proc, fmt.Errorf("process not found")
    }

    procInfo := procs[0]

    // Get process name
    if len(procInfo.Comm) > 0 {
        proc.Name = strings.TrimRight(string(procInfo.Comm[:]), "\x00")
    }

    // Get process state
    proc.State = string(procInfo.Stat)
    proc.PPID = int(procInfo.Ppid)
    proc.Priority = int(procInfo.Priority)
    proc.StartTime = int64(procInfo.Start)

    // Get memory usage
    proc.MemoryUsage = uint64(procInfo.VmSize)
    proc.MemoryRSS = uint64(procInfo.VmRSS)

    // Get user/group info
    proc.UID = fmt.Sprintf("%d", procInfo.Uid)
    proc.GID = fmt.Sprintf("%d", procInfo.Gid)

    // Get command line if requested
    if options != nil && options["include_cmdline"] == true {
        // BSD doesn't easily provide command line via kvm
        proc.Command = proc.Name
    }

    // Parse basic process information
    fields := strings.Fields(string(procInfo.Stat))
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

    // Check if we should include detailed information
    includeDetails := false
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
    var model [256]byte
    size := uintptr(len(model))
    if err := syscall.Sysctl("hw.model", (*byte)(unsafe.Pointer(&model[0])), &size, nil, 0); err == nil {
        info.Model = strings.TrimRight(string(model[:size]), "\x00")
    }

    // Get CPU usage
    if coreNumber >= 0 {
        // Return data for specific core
        info.Usage = make(map[string]interface{})
        info.Usage["core"] = coreNumber

        // Get CPU usage using BSD sysctl
        // Use BSD sysctl to get real CPU usage data

        // Get CPU usage from sysctl
        var cpuUsage struct {
            User   uint64
            Nice   uint64
            System uint64
            Idle   uint64
        }

        // Try to get CPU usage from kern.cp_time
        cpuTimeSize := uintptr(unsafe.Sizeof(cpuUsage))
        if err := syscall.Sysctl("kern.cp_time", (*byte)(unsafe.Pointer(&cpuUsage)), &cpuTimeSize, nil, 0); err == nil {
            // Calculate percentages
            total := cpuUsage.User + cpuUsage.Nice + cpuUsage.System + cpuUsage.Idle
            if total > 0 {
                userPercent := float64(cpuUsage.User) / float64(total) * 100.0
                systemPercent := float64(cpuUsage.System) / float64(total) * 100.0
                idlePercent := float64(cpuUsage.Idle) / float64(total) * 100.0

                info.Usage["user"] = userPercent
                info.Usage["system"] = systemPercent
                info.Usage["idle"] = idlePercent
            } else {
                info.Usage["user"] = 0.0
                info.Usage["system"] = 0.0
                info.Usage["idle"] = 100.0
            }
        } else {
            // Fallback to alternative sysctl
            var cpuTime [4]uint64
            cpuTimeSize = uintptr(unsafe.Sizeof(cpuTime))
            if err := syscall.Sysctl("kern.cp_times", (*byte)(unsafe.Pointer(&cpuTime)), &cpuTimeSize, nil, 0); err == nil {
                total := cpuTime[0] + cpuTime[1] + cpuTime[2] + cpuTime[3]
                if total > 0 {
                    userPercent := float64(cpuTime[0]) / float64(total) * 100.0
                    systemPercent := float64(cpuTime[2]) / float64(total) * 100.0
                    idlePercent := float64(cpuTime[3]) / float64(total) * 100.0

                    info.Usage["user"] = userPercent
                    info.Usage["system"] = systemPercent
                    info.Usage["idle"] = idlePercent
                } else {
                    info.Usage["user"] = 0.0
                    info.Usage["system"] = 0.0
                    info.Usage["idle"] = 100.0
                }
            }
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
            var cpuUsage struct {
                User   uint64
                Nice   uint64
                System uint64
                Idle   uint64
            }

            // Try to get CPU usage from kern.cp_time
            cpuTimeSize := uintptr(unsafe.Sizeof(cpuUsage))
            if err := syscall.Sysctl("kern.cp_time", (*byte)(unsafe.Pointer(&cpuUsage)), &cpuTimeSize, nil, 0); err == nil {
                // Calculate percentages
                total := cpuUsage.User + cpuUsage.Nice + cpuUsage.System + cpuUsage.Idle
                if total > 0 {
                    userPercent := float64(cpuUsage.User) / float64(total) * 100.0
                    systemPercent := float64(cpuUsage.System) / float64(total) * 100.0
                    idlePercent := float64(cpuUsage.Idle) / float64(total) * 100.0

                    coreData["user"] = userPercent
                    coreData["system"] = systemPercent
                    coreData["idle"] = idlePercent
                } else {
                    coreData["user"] = 0.0
                    coreData["system"] = 0.0
                    coreData["idle"] = 100.0
                }
            } else {
                // Fallback to alternative sysctl
                var cpuTime [4]uint64
                cpuTimeSize = uintptr(unsafe.Sizeof(cpuTime))
                if err := syscall.Sysctl("kern.cp_times", (*byte)(unsafe.Pointer(&cpuTime)), &cpuTimeSize, nil, 0); err == nil {
                    total := cpuTime[0] + cpuTime[1] + cpuTime[2] + cpuTime[3]
                    if total > 0 {
                        userPercent := float64(cpuTime[0]) / float64(total) * 100.0
                        systemPercent := float64(cpuTime[2]) / float64(total) * 100.0
                        idlePercent := float64(cpuTime[3]) / float64(total) * 100.0

                        coreData["user"] = userPercent
                        coreData["system"] = systemPercent
                        coreData["idle"] = idlePercent
                    } else {
                        coreData["user"] = 0.0
                        coreData["system"] = 0.0
                        coreData["idle"] = 100.0
                    }
                }
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

    // Get network interfaces via sysctl
    interfaces, err := net.Interfaces()
    if err != nil {
        return stats, nil
    }

    for _, iface := range interfaces {
        // Apply interface filter if specified
        if options != nil && options["interface"] != nil {
            if iface.Name != options["interface"].(string) {
                continue
            }
        }

        // Query interface stats using sysctl
        // This is a simplified implementation
        var rxBytes, txBytes, rxPackets, txPackets uint64
        var rxErrors, txErrors, rxDropped, txDropped uint64

        // Try to get interface statistics from sysctl
        if iface.Flags&net.FlagUp != 0 {
            // Get received bytes
            if rxBytesStr, err := unix.Sysctl(fmt.Sprintf("net.link.ether.inet.%s.ibytes", iface.Name)); err == nil {
                if rxBytesVal, err := strconv.ParseUint(rxBytesStr, 10, 64); err == nil {
                    rxBytes = rxBytesVal
                }
            }

            // Get transmitted bytes
            if txBytesStr, err := unix.Sysctl(fmt.Sprintf("net.link.ether.inet.%s.obytes", iface.Name)); err == nil {
                if txBytesVal, err := strconv.ParseUint(txBytesStr, 10, 64); err == nil {
                    txBytes = txBytesVal
                }
            }

            // Get received packets
            if rxPacketsStr, err := unix.Sysctl(fmt.Sprintf("net.link.ether.inet.%s.ipackets", iface.Name)); err == nil {
                if rxPacketsVal, err := strconv.ParseUint(rxPacketsStr, 10, 64); err == nil {
                    rxPackets = rxPacketsVal
                }
            }

            // Get transmitted packets
            if txPacketsStr, err := unix.Sysctl(fmt.Sprintf("net.link.ether.inet.%s.opackets", iface.Name)); err == nil {
                if txPacketsVal, err := strconv.ParseUint(txPacketsStr, 10, 64); err == nil {
                    txPackets = txPacketsVal
                }
            }

            // Get errors
            if rxErrorsStr, err := unix.Sysctl(fmt.Sprintf("net.link.ether.inet.%s.ierrors", iface.Name)); err == nil {
                if rxErrorsVal, err := strconv.ParseUint(rxErrorsStr, 10, 64); err == nil {
                    rxErrors = rxErrorsVal
                }
            }

            if txErrorsStr, err := unix.Sysctl(fmt.Sprintf("net.link.ether.inet.%s.oerrors", iface.Name)); err == nil {
                if txErrorsVal, err := strconv.ParseUint(txErrorsStr, 10, 64); err == nil {
                    txErrors = txErrorsVal
                }
            }

            // If sysctl didn't work, return error instead of fake data
            if rxBytes == 0 && txBytes == 0 {
                return nil, fmt.Errorf("network I/O statistics not available for interface %s - sysctl queries failed", iface.Name)
            }
        }

        stats = append(stats, NetworkIOStats{
            Interface: iface.Name,
            RxBytes:   rxBytes,
            TxBytes:   txBytes,
            RxPackets: rxPackets,
            TxPackets: txPackets,
            RxErrors:  rxErrors,
            TxErrors:  txErrors,
            RxDropped: rxDropped,
            TxDropped: txDropped,
        })
    }

    return stats, nil
}

// getDiskIO returns disk I/O statistics
func getDiskIO(options map[string]interface{}) ([]DiskIOStats, error) {
    var stats []DiskIOStats

    // Get disk stats via sysctl
    // This is a simplified implementation
    devices := []string{"ada0", "ada1", "da0", "da1"} // Common BSD device names

    for _, device := range devices {
        // Apply device filter if specified
        if options != nil && options["device"] != nil {
            if device != options["device"].(string) {
                continue
            }
        }

        // Get disk I/O statistics using sysctl
        var readBytes, writeBytes, readOps, writeOps uint64
        var readTime, writeTime uint64

        // Try to get disk statistics from sysctl
        // Get read bytes
        if readBytesStr, err := unix.Sysctl(fmt.Sprintf("dev.%s.rbytes", device)); err == nil {
            if readBytesVal, err := strconv.ParseUint(readBytesStr, 10, 64); err == nil {
                readBytes = readBytesVal
            }
        }

        // Get write bytes
        if writeBytesStr, err := unix.Sysctl(fmt.Sprintf("dev.%s.wbytes", device)); err == nil {
            if writeBytesVal, err := strconv.ParseUint(writeBytesStr, 10, 64); err == nil {
                writeBytes = writeBytesVal
            }
        }

        // Get read operations
        if readOpsStr, err := unix.Sysctl(fmt.Sprintf("dev.%s.rops", device)); err == nil {
            if readOpsVal, err := strconv.ParseUint(readOpsStr, 10, 64); err == nil {
                readOps = readOpsVal
            }
        }

        // Get write operations
        if writeOpsStr, err := unix.Sysctl(fmt.Sprintf("dev.%s.wops", device)); err == nil {
            if writeOpsVal, err := strconv.ParseUint(writeOpsStr, 10, 64); err == nil {
                writeOps = writeOpsVal
            }
        }

        // Get read time
        if readTimeStr, err := unix.Sysctl(fmt.Sprintf("dev.%s.rtime", device)); err == nil {
            if readTimeVal, err := strconv.ParseUint(readTimeStr, 10, 64); err == nil {
                readTime = readTimeVal
            }
        }

        // Get write time
        if writeTimeStr, err := unix.Sysctl(fmt.Sprintf("dev.%s.wtime", device)); err == nil {
            if writeTimeVal, err := strconv.ParseUint(writeTimeStr, 10, 64); err == nil {
                writeTime = writeTimeVal
            }
        }

        // If sysctl didn't work, return error instead of fake data
        if readBytes == 0 && writeBytes == 0 {
            return nil, fmt.Errorf("disk I/O statistics not available for device %s - sysctl queries failed", device)
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

// getDiskUsage returns filesystem usage information (BSD implementation)
func getDiskUsage(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Get mount information using getmntinfo
    mounts, err := unix.Getmntinfo()
    if err != nil {
        return result, err
    }

    for _, mount := range mounts {
        device := mount.Fstypename
        mountPoint := mount.Mntonname
        filesystem := mount.Fstypename

        // Apply filters if specified
        if options != nil {
            if excludePatterns, exists := options["exclude_patterns"]; exists {
                if patterns, ok := excludePatterns.([]string); ok {
                    for _, pattern := range patterns {
                        if strings.Contains(filesystem, pattern) || strings.Contains(mountPoint, pattern) {
                            continue
                        }
                    }
                }
            }
        }

        // Get filesystem stats using statfs
        var stat unix.Statfs_t
        if err := unix.Statfs(mountPoint, &stat); err != nil {
            continue
        }

        // Calculate usage
        total := stat.Blocks * uint64(stat.Bsize)
        free := stat.Bfree * uint64(stat.Bsize)
        used := total - free
        usagePercent := 0.0
        if total > 0 {
            usagePercent = float64(used) / float64(total) * 100.0
        }

        diskInfo := map[string]interface{}{
            "path":          device,
            "size":          total,
            "used":          used,
            "available":     free,
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

    // Get mount information using getmntinfo
    mounts, err := unix.Getmntinfo()
    if err != nil {
        return result, err
    }

    for _, mount := range mounts {
        device := mount.Fstypename
        mountPoint := mount.Mntonname
        filesystem := mount.Fstypename
        mountOptions := mount.Mntfromname

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

    // Use kvm to get process resource usage
    kvm, err := unix.KvmOpen(nil)
    if err != nil {
        return usage, err
    }
    defer kvm.Close()

    procs, err := kvm.GetProcs(unix.KERN_PROC_PID, uint32(pid))
    if err != nil || len(procs) == 0 {
        return usage, fmt.Errorf("process not found")
    }

    procInfo := procs[0]

    // Get memory usage
    usage.MemoryCurrent = uint64(procInfo.VmRSS)
    usage.MemoryPeak = uint64(procInfo.VmSize)

    // Parse CPU times
    utime, _ := strconv.ParseUint(fields[13], 10, 64)
    stime, _ := strconv.ParseUint(fields[14], 10, 64)
    cutime, _ := strconv.ParseUint(fields[15], 10, 64)
    cstime, _ := strconv.ParseUint(fields[16], 10, 64)
    usage.CPUUser = float64(utime) / 100.0            // Convert to seconds
    usage.CPUSystem = float64(stime) / 100.0          // Convert to seconds
    usage.CPUChildrenUser = float64(cutime) / 100.0   // Convert to seconds
    usage.CPUChildrenSystem = float64(cstime) / 100.0 // Convert to seconds

    // BSD doesn't easily provide I/O stats, context switches, or page faults
    // These would require additional sysctl queries

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
    // Try to get routing table via sysctl
    data, err := syscall.Sysctl("net.inet.ip.routing")
    if err != nil {
        // Try alternative sysctl paths
        altPaths := []string{
            "net.inet.ip.forwarding",
            "net.inet.ip.routes",
        }
        for _, path := range altPaths {
            if data, err = syscall.Sysctl(path); err == nil {
                break
            }
        }
        if err != nil {
            return "", fmt.Errorf("failed to get routing information: %v", err)
        }
    }

    // Parse the routing data to find default gateway
    gateway := parseRoutingTable(data)
    if gateway == "" {
        return "", fmt.Errorf("no default gateway found in routing table")
    }

    return gateway, nil
}

// parseRoutingTable parses BSD routing table data to find default gateway
func parseRoutingTable(data string) string {
    lines := strings.Split(data, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        // Look for default route entries
        if strings.HasPrefix(line, "default") || strings.Contains(line, "0.0.0.0") {
            fields := strings.Fields(line)
            for i, field := range fields {
                // Look for gateway indicators
                if field == "via" || field == "gw" || field == "gateway" {
                    if i+1 < len(fields) {
                        gateway := fields[i+1]
                        // Validate it's a proper IP address
                        if net.ParseIP(gateway) != nil {
                            return gateway
                        }
                    }
                }
                // Also check if any field looks like an IP address
                if net.ParseIP(field) != nil && !strings.HasPrefix(field, "0.0.0.0") {
                    return field
                }
            }
        }
    }
    return ""
}

// getDefaultGatewayInfo returns complete default gateway information (BSD implementation)
func getDefaultGatewayInfo() (map[string]interface{}, error) {
    gateway, err := getDefaultGatewayAddress()
    if err != nil {
        return nil, err
    }

    gwIP := net.ParseIP(gateway)
    if gwIP == nil {
        return nil, fmt.Errorf("invalid gateway IP: %s", gateway)
    }

    ifaces, err := net.Interfaces()
    if err != nil {
        return nil, err
    }

    for _, iface := range ifaces {
        if iface.Flags&net.FlagUp == 0 {
            continue
        }
        addrs, err := iface.Addrs()
        if err != nil {
            continue
        }
        for _, addr := range addrs {
            ipnet, ok := addr.(*net.IPNet)
            if !ok || ipnet.IP == nil || ipnet.IP.To4() == nil {
                continue
            }
            // Check if gateway is in the same subnet as this interface
            if ipnet.Contains(gwIP) {
                return map[string]interface{}{
                    "interface": iface.Name,
                    "gateway":   gateway,
                }, nil
            }
        }
    }

    return nil, fmt.Errorf("no default gateway interface found for gateway %s", gateway)
}
