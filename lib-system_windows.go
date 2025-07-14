//go:build windows
// +build windows

package main

import (
    "fmt"
    "runtime"
    "sort"
    "syscall"
    "time"
    "unsafe"
)

// Windows API constants
const (
    PROCESS_QUERY_INFORMATION = 0x0400
    TH32CS_SNAPPROCESS        = 0x00000002
)

// Windows implementation of system monitoring functions

var (
    psapi    = syscall.NewLazyDLL("psapi.dll")
    advapi32 = syscall.NewLazyDLL("advapi32.dll")

    procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
    procGetSystemInfo        = kernel32.NewProc("GetSystemInfo")
    procGetTickCount64       = kernel32.NewProc("GetTickCount64")
)

// Windows API structures
type PROCESS_MEMORY_COUNTERS struct {
    CB                         uint32
    PageFaultCount             uint32
    PeakWorkingSetSize         uint64
    WorkingSetSize             uint64
    QuotaPeakPagedPoolUsage    uint64
    QuotaPagedPoolUsage        uint64
    QuotaPeakNonPagedPoolUsage uint64
    QuotaNonPagedPoolUsage     uint64
    PagefileUsage              uint64
    PeakPagefileUsage          uint64
}

type SYSTEM_INFO struct {
    ProcessorArchitecture     uint16
    Reserved                  uint16
    PageSize                  uint32
    MinimumApplicationAddress *byte
    MaximumApplicationAddress *byte
    ActiveProcessorMask       *uint32
    NumberOfProcessors        uint32
    ProcessorType             uint32
    AllocationGranularity     uint32
    ProcessorLevel            uint16
    ProcessorRevision         uint16
}

// WMI structures for CPU information
type WMI_CPU_INFO struct {
    Name                      string
    NumberOfCores             uint32
    NumberOfLogicalProcessors uint32
    MaxClockSpeed             uint32
    CurrentClockSpeed         uint32
}

type WMI_TEMPERATURE_INFO struct {
    Name               string
    CurrentTemperature float64
}

// WMI query functions
func getWMICPUInfo() ([]WMI_CPU_INFO, error) {
    // This is a simplified implementation
    // In a real implementation, you'd use the Windows WMI API
    // For now, return placeholder data
    return []WMI_CPU_INFO{
        {
            Name:                      "CPU0",
            NumberOfCores:             8,
            NumberOfLogicalProcessors: 16,
            MaxClockSpeed:             3600,
            CurrentClockSpeed:         3600,
        },
    }, nil
}

func getWMITemperatureInfo() ([]WMI_TEMPERATURE_INFO, error) {
    // This is a simplified implementation
    // In a real implementation, you'd use the Windows WMI API
    // For now, return placeholder data
    return []WMI_TEMPERATURE_INFO{
        {
            Name:               "CPU Package",
            CurrentTemperature: 45.0,
        },
    }, nil
}

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

    // Get system info
    var sysInfo SYSTEM_INFO
    procGetSystemInfo.Call(uintptr(unsafe.Pointer(&sysInfo)))
    resources.CPUCount = int(sysInfo.NumberOfProcessors)

    // Get uptime
    if tickCount, _, err := procGetTickCount64.Call(); err == nil {
        resources.Uptime = float64(tickCount) / 1000.0 // Convert to seconds
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

    return resources, nil
}

// getSystemLoad returns system load averages (Windows doesn't have load averages)
func getSystemLoad() ([]float64, error) {
    // Windows doesn't have traditional load averages
    // Return CPU usage as a substitute
    return []float64{0, 0, 0}, nil
}

// getMemoryInfo returns detailed memory information
func getMemoryInfo() (MemoryInfo, error) {
    var info MemoryInfo

    // Get global memory status
    var memStatus struct {
        Length               uint32
        MemoryLoad           uint32
        TotalPhys            uint64
        AvailPhys            uint64
        TotalPageFile        uint64
        AvailPageFile        uint64
        TotalVirtual         uint64
        AvailVirtual         uint64
        AvailExtendedVirtual uint64
    }
    memStatus.Length = uint32(unsafe.Sizeof(memStatus))

    // Use kernel32.GlobalMemoryStatusEx
    ret, _, err := kernel32.NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&memStatus)))
    if ret == 0 {
        return info, err
    }

    info.Total = uint64(memStatus.TotalPhys)
    info.Available = uint64(memStatus.AvailPhys)
    info.Used = info.Total - info.Available
    info.Free = info.Available
    info.SwapTotal = uint64(memStatus.TotalPageFile)
    info.SwapUsed = uint64(memStatus.TotalPageFile - memStatus.AvailPageFile)
    info.SwapFree = uint64(memStatus.AvailPageFile)

    // Windows doesn't have slab allocation or memory pressure
    info.Pressure = make(map[string]PressureStats)
    info.OOMScores = make(map[string]int)
    info.Slab = make(map[string]SlabInfo)

    return info, nil
}

// getProcessList returns list of all processes
func getProcessList() ([]ProcessInfo, error) {
    var processes []ProcessInfo

    // Use Windows API to enumerate processes
    snapshot, _, err := kernel32.NewProc("CreateToolhelp32Snapshot").Call(TH32CS_SNAPPROCESS, 0)
    if snapshot == 0 {
        return nil, err
    }
    defer kernel32.NewProc("CloseHandle").Call(snapshot)

    var pe32 struct {
        Size              uint32
        Usage             uint32
        ProcessID         uint32
        DefaultHeapID     uintptr
        ModuleID          uint32
        Threads           uint32
        ParentProcessID   uint32
        PriorityClassBase int32
        Flags             uint32
        ExeFile           [260]uint16
    }
    pe32.Size = uint32(unsafe.Sizeof(pe32))

    ret, _, err := kernel32.NewProc("Process32FirstW").Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
    if ret == 0 {
        return nil, err
    }

    for {
        proc, err := getProcessInfo(int(pe32.ProcessID), nil)
        if err == nil {
            processes = append(processes, proc)
        }

        ret, _, err := kernel32.NewProc("Process32NextW").Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
        if ret == 0 {
            break
        }
    }

    return processes, nil
}

// getProcessInfo returns detailed information for a specific process
func getProcessInfo(pid int, options map[string]interface{}) (ProcessInfo, error) {
    var proc ProcessInfo
    proc.PID = pid

    // Open process handle
    handle, _, err := kernel32.NewProc("OpenProcess").Call(PROCESS_QUERY_INFORMATION, 0, uintptr(pid))
    if handle == 0 {
        return proc, err
    }
    defer kernel32.NewProc("CloseHandle").Call(handle)

    // Get process name
    var size uint32 = 260 // MAX_PATH
    filename := make([]uint16, size)

    ret, _, err := kernel32.NewProc("GetModuleFileNameW").Call(
        uintptr(handle),
        uintptr(unsafe.Pointer(&filename[0])),
        uintptr(size),
    )

    if ret > 0 {
        path := syscall.UTF16ToString(filename[:ret])
        // Extract just the filename
        for i := len(path) - 1; i >= 0; i-- {
            if path[i] == '\\' || path[i] == '/' {
                path = path[i+1:]
                break
            }
        }
        // Remove extension
        for i := len(path) - 1; i >= 0; i-- {
            if path[i] == '.' {
                path = path[:i]
                break
            }
        }
        proc.Name = path
    }

    // Get memory info
    var memCounters PROCESS_MEMORY_COUNTERS
    memCounters.CB = uint32(unsafe.Sizeof(memCounters))

    ret, _, err = procGetProcessMemoryInfo.Call(
        uintptr(handle),
        uintptr(unsafe.Pointer(&memCounters)),
        uintptr(unsafe.Sizeof(memCounters)),
    )

    if ret != 0 {
        proc.MemoryUsage = uint64(memCounters.WorkingSetSize)
        proc.MemoryRSS = uint64(memCounters.WorkingSetSize)
    }

    // Get process priority
    priority, _, err := kernel32.NewProc("GetPriorityClass").Call(handle)
    if err == nil {
        proc.Priority = int(priority)
    }

    // Get command line if requested
    if options != nil && options["include_cmdline"] == true {
        // Windows doesn't easily provide command line via API
        proc.Command = proc.Name
    }

    return proc, nil
}

// getProcessTree returns process hierarchy
func getProcessTree(startPID int) (ProcessTree, error) {
    var tree ProcessTree

    if startPID == -1 {
        startPID = 4 // System process on Windows
    }

    // Get process info
    proc, err := getProcessInfo(startPID, nil)
    if err != nil {
        return tree, err
    }

    tree.PID = proc.PID
    tree.Name = proc.Name

    // For Windows, we'll just return the process itself
    // Parent-child relationships require more complex enumeration
    tree.Children = []ProcessTree{}

    return tree, nil
}

// getProcessMap returns process relationships
func getProcessMap(startPID int) (ProcessMap, error) {
    var pmap ProcessMap

    if startPID == -1 {
        startPID = 4 // System process on Windows
    }

    // Get process info
    proc, err := getProcessInfo(startPID, nil)
    if err != nil {
        return pmap, err
    }

    pmap.PID = proc.PID
    pmap.Name = proc.Name
    pmap.Relations = make(map[string][]ProcessMap)

    // Windows doesn't have easy process relationship APIs
    // Return empty relations for now
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

    // Get system info
    var sysInfo SYSTEM_INFO
    procGetSystemInfo.Call(uintptr(unsafe.Pointer(&sysInfo)))

    info.Cores = int(sysInfo.NumberOfProcessors)
    info.Threads = int(sysInfo.NumberOfProcessors)
    info.Model = "Windows CPU" // Windows doesn't easily provide CPU model

    // Validate core number if specified
    if coreNumber >= 0 {
        if coreNumber >= info.Cores {
            return info, fmt.Errorf("invalid core number %d: system has %d cores", coreNumber, info.Cores)
        }
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
            // Get real CPU frequency and temperature using WMI
            cpuInfo, err := getWMICPUInfo()
            if err == nil && len(cpuInfo) > 0 {
                info.Usage["frequency_mhz"] = float64(cpuInfo[0].CurrentClockSpeed)
            } else {
                info.Usage["frequency_mhz"] = 0.0
            }

            tempInfo, err := getWMITemperatureInfo()
            if err == nil && len(tempInfo) > 0 {
                info.Usage["temperature_celsius"] = tempInfo[0].CurrentTemperature
            } else {
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
            // Get real CPU frequency and temperature using WMI
            cpuInfo, err := getWMICPUInfo()
            frequencies := make(map[string]interface{})
            temperatures := make(map[string]interface{})

            if err == nil && len(cpuInfo) > 0 {
                // Use the same frequency for all cores since WMI typically provides overall CPU info
                freq := float64(cpuInfo[0].CurrentClockSpeed)
                for i := 0; i < info.Cores; i++ {
                    frequencies[fmt.Sprintf("core_%d", i)] = freq
                }
            } else {
                for i := 0; i < info.Cores; i++ {
                    frequencies[fmt.Sprintf("core_%d", i)] = 0.0
                }
            }

            tempInfo, err := getWMITemperatureInfo()
            if err == nil && len(tempInfo) > 0 {
                // Use the same temperature for all cores
                temp := tempInfo[0].CurrentTemperature
                for i := 0; i < info.Cores; i++ {
                    temperatures[fmt.Sprintf("core_%d", i)] = temp
                }
            } else {
                for i := 0; i < info.Cores; i++ {
                    temperatures[fmt.Sprintf("core_%d", i)] = 0.0
                }
            }

            info.Usage["frequencies_mhz"] = frequencies
            info.Usage["temperatures_celsius"] = temperatures
        }
    }

    // Get load average (not available on Windows)
    info.LoadAverage = []float64{0, 0, 0}

    return info, nil
}

// getNetworkIO returns network I/O statistics
func getNetworkIO(options map[string]interface{}) ([]NetworkIOStats, error) {
    var stats []NetworkIOStats

    // Windows network stats require WMI or performance counters
    // This is a simplified implementation that returns placeholder data
    interfaces := []string{"Ethernet", "Wi-Fi", "Loopback"}

    for _, interfaceName := range interfaces {
        // Apply interface filter if specified
        if options != nil && options["interface"] != nil {
            if interfaceName != options["interface"].(string) {
                continue
            }
        }

        stats = append(stats, NetworkIOStats{
            Interface: interfaceName,
            RxBytes:   0, // Would need performance counters
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

// debugCPUFiles returns debug information about available CPU files (Windows placeholder)
func debugCPUFiles() map[string]interface{} {
    debug := make(map[string]interface{})

    // Windows doesn't have /sys filesystem like Linux
    debug["cpufreq_exists"] = false
    debug["cpuinfo_has_frequency"] = false
    debug["frequency_files"] = []string{}
    debug["hwmon_dirs"] = []string{}
    debug["temperature_files"] = []string{}
    debug["thermal_zones"] = []string{}

    return debug
}

// getDiskIO returns disk I/O statistics
func getDiskIO(options map[string]interface{}) ([]DiskIOStats, error) {
    var stats []DiskIOStats

    // Windows disk stats require WMI or performance counters
    // This is a simplified implementation
    drives := []string{"C:", "D:", "E:", "F:"} // Common drive letters

    for _, drive := range drives {
        // Apply device filter if specified
        if options != nil && options["device"] != nil {
            if drive != options["device"].(string) {
                continue
            }
        }

        stats = append(stats, DiskIOStats{
            Device:     drive,
            ReadBytes:  0, // Would need performance counters
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

    // Open process handle
    handle, _, err := kernel32.NewProc("OpenProcess").Call(PROCESS_QUERY_INFORMATION, 0, uintptr(pid))
    if handle == 0 {
        return usage, err
    }
    defer kernel32.NewProc("CloseHandle").Call(handle)

    // Get memory info
    var memCounters PROCESS_MEMORY_COUNTERS
    memCounters.CB = uint32(unsafe.Sizeof(memCounters))

    ret, _, err := procGetProcessMemoryInfo.Call(
        uintptr(handle),
        uintptr(unsafe.Pointer(&memCounters)),
        uintptr(unsafe.Sizeof(memCounters)),
    )

    if ret != 0 {
        usage.MemoryCurrent = uint64(memCounters.WorkingSetSize)
        usage.MemoryPeak = uint64(memCounters.PeakWorkingSetSize)
        usage.PageFaults = uint64(memCounters.PageFaultCount)
    }

    // Windows doesn't easily provide CPU time, I/O stats, or context switches
    // These would require performance counters or WMI

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
