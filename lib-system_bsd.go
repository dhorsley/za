//go:build freebsd
// +build freebsd

package main

import (
    "fmt"
    "net"
    "os"
    "runtime"
    "sort"
    "strings"
    "syscall"
    "time"
    "unsafe"

    "golang.org/x/sys/unix"
)

// BSD implementation of system monitoring functions

// getTopCPU returns top N CPU consumers
func getTopCPU(n int) ([]ProcessInfo, error) {
    processes, err := getProcessList()
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
    processes, err := getProcessList()
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

    // Get load average
    load, err := getSystemLoad()
    if err == nil {
        resources.LoadAverage = load
    }

    // Get memory info
    mem, err := getMemoryInfo()
    if err == nil {
        resources.MemoryTotal = mem.Total
        resources.MemoryUsed = mem.Used
        resources.MemoryFree = mem.Free
        resources.MemoryCached = mem.Cached
        resources.SwapTotal = mem.SwapTotal
        resources.SwapUsed = mem.SwapUsed
        resources.SwapFree = mem.SwapFree
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
func getProcessList() ([]ProcessInfo, error) {
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
        processInfo, err := getProcessInfo(int(proc.Pid), nil)
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
    processes, err := getProcessList()
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
    processes, err := getProcessList()
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
        info.Usage["user"] = 0.0 // Would need performance counters for real values
        info.Usage["system"] = 0.0
        info.Usage["idle"] = 0.0

        if includeDetails {
            // Try to get CPU frequency for the specific core
            var freq uint64
            freqSize := uintptr(unsafe.Sizeof(freq))
            freqName := fmt.Sprintf("dev.cpu.%d.freq", coreNumber)
            if err := syscall.Sysctl(freqName, (*byte)(unsafe.Pointer(&freq)), &freqSize, nil, 0); err == nil {
                info.Usage["frequency_mhz"] = float64(freq)
            } else {
                // Try alternative frequency sysctls
                altFreqNames := []string{
                    fmt.Sprintf("dev.cpu.%d.cx_lowest", coreNumber),
                    "hw.cpu.frequency",
                    "hw.clockrate",
                }
                for _, altName := range altFreqNames {
                    if err := syscall.Sysctl(altName, (*byte)(unsafe.Pointer(&freq)), &freqSize, nil, 0); err == nil {
                        info.Usage["frequency_mhz"] = float64(freq)
                        break
                    }
                }
                if _, exists := info.Usage["frequency_mhz"]; !exists {
                    info.Usage["frequency_mhz"] = 0.0
                }
            }

            // Try to get CPU temperature (if available)
            var temp uint64
            tempSize := uintptr(unsafe.Sizeof(temp))
            tempNames := []string{
                "hw.acpi.thermal.tz0.temperature",
                "hw.sensors.cpu0.temp0",
                "hw.sensors.cpu1.temp0",
            }
            tempFound := false
            for _, tempName := range tempNames {
                if err := syscall.Sysctl(tempName, (*byte)(unsafe.Pointer(&temp)), &tempSize, nil, 0); err == nil {
                    info.Usage["temperature_celsius"] = float64(temp) / 10.0 // Convert to Celsius
                    tempFound = true
                    break
                }
            }
            if !tempFound {
                info.Usage["temperature_celsius"] = 0.0
            }
        }
    } else {
        // Return data for all cores
        info.Usage = make(map[string]interface{})
        cores := make(map[string]interface{})

        for i := 0; i < info.Cores; i++ {
            coreData := make(map[string]interface{})
            coreData["user"] = 0.0 // Would need performance counters for real values
            coreData["system"] = 0.0
            coreData["idle"] = 0.0
            cores[fmt.Sprintf("core_%d", i)] = coreData
        }
        info.Usage["cores"] = cores

        if includeDetails {
            // Get frequency data for all cores
            frequencies := make(map[string]interface{})
            for i := 0; i < info.Cores; i++ {
                var freq uint64
                freqSize := uintptr(unsafe.Sizeof(freq))
                freqName := fmt.Sprintf("dev.cpu.%d.freq", i)
                if err := syscall.Sysctl(freqName, (*byte)(unsafe.Pointer(&freq)), &freqSize, nil, 0); err == nil {
                    frequencies[fmt.Sprintf("core_%d", i)] = float64(freq)
                } else {
                    // Try alternative frequency sysctls
                    altFreqNames := []string{
                        fmt.Sprintf("dev.cpu.%d.cx_lowest", i),
                        "hw.cpu.frequency",
                        "hw.clockrate",
                    }
                    freqFound := false
                    for _, altName := range altFreqNames {
                        if err := syscall.Sysctl(altName, (*byte)(unsafe.Pointer(&freq)), &freqSize, nil, 0); err == nil {
                            frequencies[fmt.Sprintf("core_%d", i)] = float64(freq)
                            freqFound = true
                            break
                        }
                    }
                    if !freqFound {
                        frequencies[fmt.Sprintf("core_%d", i)] = 0.0
                    }
                }
            }
            info.Usage["frequencies_mhz"] = frequencies

            // Try to get CPU temperature (if available)
            var temp uint64
            tempSize := uintptr(unsafe.Sizeof(temp))
            tempNames := []string{
                "hw.acpi.thermal.tz0.temperature",
                "hw.sensors.cpu0.temp0",
                "hw.sensors.cpu1.temp0",
            }
            tempFound := false
            for _, tempName := range tempNames {
                if err := syscall.Sysctl(tempName, (*byte)(unsafe.Pointer(&temp)), &tempSize, nil, 0); err == nil {
                    info.Usage["temperature_celsius"] = float64(temp) / 10.0 // Convert to Celsius
                    tempFound = true
                    break
                }
            }
            if !tempFound {
                info.Usage["temperature_celsius"] = 0.0
            }
        }
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

        // Get interface stats via sysctl
        var stats struct {
            RxBytes   uint64
            TxBytes   uint64
            RxPackets uint64
            TxPackets uint64
            RxErrors  uint64
            TxErrors  uint64
        }

        // Query interface stats
        // This is a simplified implementation
        stats = append(stats, NetworkIOStats{
            Interface: iface.Name,
            RxBytes:   0, // Would need sysctl queries for real values
            TxBytes:   0,
            RxPackets: 0,
            TxPackets: 0,
            RxErrors:  0,
            TxErrors:  0,
            RxDropped: 0,
            TxDropped: 0,
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

        stats = append(stats, DiskIOStats{
            Device:     device,
            ReadBytes:  0, // Would need sysctl queries for real values
            WriteBytes: 0,
            ReadOps:    0,
            WriteOps:   0,
            ReadTime:   0,
            WriteTime:  0,
        })
    }

    return stats, nil
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

    // Get CPU time
    usage.CPUUser = float64(procInfo.Utime) / 100.0   // Convert to seconds
    usage.CPUSystem = float64(procInfo.Stime) / 100.0 // Convert to seconds

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
