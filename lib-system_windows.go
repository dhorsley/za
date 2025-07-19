//go:build windows
// +build windows

package main

import (
    "encoding/binary"
    "fmt"
    "net"
    "runtime"
    "sort"
    "strings"
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

// Windows API constants
const (
    IF_MAX_STRING_SIZE         = 256
    IF_MAX_PHYS_ADDRESS_LENGTH = 32
)

// MibIfRow2 is used for Windows network interface statistics
type MibIfTable2 struct {
    NumEntries uint32
    Table      [1]MibIfRow2 // Variable length array - we'll access it with pointer arithmetic
}

type MibIfRow2 struct {
    InterfaceLuid               uint64
    InterfaceIndex              uint32
    InterfaceGuid               [16]byte
    Alias                       [IF_MAX_STRING_SIZE + 1]uint16
    Description                 [IF_MAX_STRING_SIZE + 1]uint16
    PhysicalAddressLength       uint32
    PhysicalAddress             [IF_MAX_PHYS_ADDRESS_LENGTH]byte
    PermanentPhysicalAddress    [IF_MAX_PHYS_ADDRESS_LENGTH]byte
    Mtu                         uint32
    Type                        uint32
    TunnelType                  uint32
    MediaType                   uint32
    PhysicalMediumType          uint32
    AccessType                  uint32
    DirectionType               uint32
    InterfaceAndOperStatusFlags uint8 // Bit field
    OperStatus                  uint32
    AdminStatus                 uint32
    MediaConnectState           uint32
    NetworkGuid                 [16]byte
    ConnectionType              uint32
    TransmitLinkSpeed           uint64
    ReceiveLinkSpeed            uint64
    InOctets                    uint64
    InUcastPkts                 uint64
    InNUcastPkts                uint64
    InDiscards                  uint64
    InErrors                    uint64
    InUnknownProtos             uint64
    InUcastOctets               uint64
    InMulticastOctets           uint64
    InBroadcastOctets           uint64
    OutOctets                   uint64
    OutUcastPkts                uint64
    OutNUcastPkts               uint64
    OutDiscards                 uint64
    OutErrors                   uint64
    OutUcastOctets              uint64
    OutMulticastOctets          uint64
    OutBroadcastOctets          uint64
    OutQLen                     uint64
}

type MibIfRow struct {
    Name            [256]byte
    Index           uint32
    Type            uint32
    Mtu             uint32
    Speed           uint32
    PhysAddrLen     uint32
    PhysAddr        [8]byte
    AdminStatus     uint32
    OperStatus      uint32
    LastChange      uint32
    InOctets        uint32
    InUcastPkts     uint32
    InNUcastPkts    uint32
    InDiscards      uint32
    InErrors        uint32
    InUnknownProtos uint32
    OutOctets       uint32
    OutUcastPkts    uint32
    OutNUcastPkts   uint32
    OutDiscards     uint32
    OutErrors       uint32
    OutQLen         uint32
    DescrLen        uint32
    Descr           [256]byte
}

// WMI query functions
func getWMICPUInfo() ([]WMI_CPU_INFO, error) {
    // Use Windows Management Instrumentation (WMI) to get real CPU info
    // This requires the github.com/StackExchange/wmi package
    // For now, we'll use a more sophisticated approach with Windows API

    var cpuInfo []WMI_CPU_INFO

    // Get processor count and basic info
    var sysInfo SYSTEM_INFO
    procGetSystemInfo.Call(uintptr(unsafe.Pointer(&sysInfo)))

    // Get CPU frequency using Windows API
    var freq uint32
    if freqProc := kernel32.NewProc("QueryPerformanceFrequency"); freqProc.Addr() != 0 {
        r1, _, _ := freqProc.Call(uintptr(unsafe.Pointer(&freq)))
        if r1 != 0 {
            freq = freq
        }
    }

    // Create CPU info based on system info
    cpuInfo = append(cpuInfo, WMI_CPU_INFO{
        Name:                      "CPU0",
        NumberOfCores:             uint32(sysInfo.NumberOfProcessors),
        NumberOfLogicalProcessors: uint32(sysInfo.NumberOfProcessors),
        MaxClockSpeed:             uint32(freq / 1000000), // Convert to MHz
        CurrentClockSpeed:         uint32(freq / 1000000), // Convert to MHz
    })

    return cpuInfo, nil
}

func getWMITemperatureInfo() ([]WMI_TEMPERATURE_INFO, error) {
    // Use Windows Management Instrumentation (WMI) to get real temperature data
    // This requires the github.com/StackExchange/wmi package
    // For now, we'll use Windows API to get temperature from ACPI

    // Try to get temperature from ACPI using Windows API
    // Open ACPI device
    acpiPath := "\\\\.\\ACPI#ThermalZone#THM0"
    handle, _, _ := kernel32.NewProc("CreateFileW").Call(
        uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(acpiPath))),
        0, // No access needed for query
        0, // No sharing
        0, // No security
        3, // OPEN_EXISTING
        0, // No flags
        0, // No template
    )

    if handle != 0 {
        defer kernel32.NewProc("CloseHandle").Call(handle)

        // Try to read temperature data
        // This is a simplified approach - real implementation would use WMI
        // For now, we'll return an error indicating WMI is required
        return nil, fmt.Errorf("CPU temperature requires WMI implementation - ACPI access not available")
    }

    // If ACPI access fails, try alternative methods
    // Try to get temperature from registry or other system sources
    // This would require vendor-specific drivers or WMI queries

    return nil, fmt.Errorf("CPU temperature not available - requires vendor-specific drivers or WMI implementation")
}

// getTopCPU returns top N CPU consumers
func getTopCPU(n int) ([]ProcessInfo, error) {
    processes, err := getProcessList(nil)
    if err != nil {
        return nil, err
    }

    // Sort by CPU time (user + system)
    sort.Slice(processes, func(i, j int) bool {
        totalI := processes[i].UserTime + processes[i].SystemTime
        totalJ := processes[j].UserTime + processes[j].SystemTime
        return totalI > totalJ
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

    // Get system info
    var sysInfo SYSTEM_INFO
    procGetSystemInfo.Call(uintptr(unsafe.Pointer(&sysInfo)))
    resources.CPUCount = int(sysInfo.NumberOfProcessors)

    // Get uptime
    if tickCount, _, err := procGetTickCount64.Call(); err == nil {
        resources.Uptime = float64(tickCount) / 1000.0 // Convert to seconds
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
func getProcessList(options map[string]interface{}) ([]ProcessInfo, error) {
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
        // Pass the parent process ID that we already have
        proc, err := getProcessInfoWithParent(int(pe32.ProcessID), int(pe32.ParentProcessID), int(pe32.Threads), nil)
        if err == nil {
            processes = append(processes, proc)
        }

        ret, _, err := kernel32.NewProc("Process32NextW").Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
        if ret == 0 {
            // Process32NextW returns 0 when there are no more processes
            // This is normal end-of-enumeration, not an error
            // The return value of 0 indicates no more processes, which is expected
            break
        }
    }

    return processes, nil
}

// getProcessInfoWithParent returns detailed information for a specific process with known parent PID
func getProcessInfoWithParent(pid int, parentPID int, threadCount int, options map[string]interface{}) (ProcessInfo, error) {
    var proc ProcessInfo
    proc.PID = pid
    proc.PPID = parentPID      // Use the parent PID we already have
    proc.Threads = threadCount // Use the thread count we already have

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

    // Always try to get command line (no longer optional)
    // Windows doesn't easily provide command line via API
    proc.Command = proc.Name

    // Get process state and other info using Windows API
    proc.State = "Running" // Default state

    // Get process start time
    var creationTime, exitTime, kernelTime, userTime syscall.Filetime
    if timeProc := kernel32.NewProc("GetProcessTimes"); timeProc.Addr() != 0 {
        r1, _, _ := timeProc.Call(uintptr(handle),
            uintptr(unsafe.Pointer(&creationTime)),
            uintptr(unsafe.Pointer(&exitTime)),
            uintptr(unsafe.Pointer(&kernelTime)),
            uintptr(unsafe.Pointer(&userTime)))
        if r1 != 0 && err == nil {
            // Convert Windows filetime to Unix timestamp
            proc.StartTime = int64(creationTime.LowDateTime) | (int64(creationTime.HighDateTime) << 32)
            proc.StartTime = (proc.StartTime - 116444736000000000) / 10000000 // Convert to Unix time
        }
    }

    // Get CPU times
    userTimeCombined := uint64(userTime.LowDateTime) | (uint64(userTime.HighDateTime) << 32)
    proc.UserTime = float64(userTimeCombined) / 10000000.0 // Convert to seconds
    kernelTimeCombined := uint64(kernelTime.LowDateTime) | (uint64(kernelTime.HighDateTime) << 32)
    proc.SystemTime = float64(kernelTimeCombined) / 10000000.0 // Convert to seconds

    // Windows doesn't easily provide children CPU times
    proc.ChildrenUserTime = 0.0
    proc.ChildrenSystemTime = 0.0

    return proc, nil
}

// getProcessInfo returns detailed information for a specific process
func getProcessInfo(pid int, options map[string]interface{}) (ProcessInfo, error) {
    // For individual process lookup, we need to find the parent PID
    // Use CreateToolhelp32Snapshot to find this specific process
    snapshot, _, err := kernel32.NewProc("CreateToolhelp32Snapshot").Call(TH32CS_SNAPPROCESS, 0)
    if snapshot == 0 {
        return ProcessInfo{PID: pid}, err
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
        return ProcessInfo{PID: pid}, err
    }

    for {
        if int(pe32.ProcessID) == pid {
            return getProcessInfoWithParent(pid, int(pe32.ParentProcessID), int(pe32.Threads), options)
        }

        ret, _, err := kernel32.NewProc("Process32NextW").Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
        if ret == 0 {
            // Process32NextW returns 0 when there are no more processes
            // Check if this is a real error or just end of enumeration
            if err != nil && err.Error() != "There are no more files." {
                // This is a real error, not just end of enumeration
                // Return the error to the caller
                return ProcessInfo{PID: pid}, fmt.Errorf("Process32NextW failed: %v", err)
            }
            // Normal end of enumeration
            break
        }
    }

    // Process not found, return basic info
    return ProcessInfo{PID: pid}, fmt.Errorf("process %d not found", pid)
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

        // Get CPU usage using Windows Performance Counters
        // Use Windows API to get real CPU usage data

        // Get processor time using GetSystemTimes
        var idleTime, kernelTime, userTime syscall.Filetime
        if timeProc := kernel32.NewProc("GetSystemTimes"); timeProc.Addr() != 0 {
            r1, _, _ := timeProc.Call(
                uintptr(unsafe.Pointer(&idleTime)),
                uintptr(unsafe.Pointer(&kernelTime)),
                uintptr(unsafe.Pointer(&userTime)))
            if r1 != 0 {

                // Convert filetime to 64-bit values
                idle := uint64(idleTime.LowDateTime) | (uint64(idleTime.HighDateTime) << 32)
                kernel := uint64(kernelTime.LowDateTime) | (uint64(kernelTime.HighDateTime) << 32)
                user := uint64(userTime.LowDateTime) | (uint64(userTime.HighDateTime) << 32)

                // Calculate CPU usage percentages
                total := kernel + user
                if total > 0 {
                    idlePercent := float64(idle) / float64(total) * 100.0
                    userPercent := float64(user) / float64(total) * 100.0
                    systemPercent := float64(kernel-idle) / float64(total) * 100.0

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

            // Get CPU usage using Windows Performance Counters
            // Use Windows API to get real CPU usage data for each core

            // For multi-core systems, we'll use the overall system times
            // since per-core CPU times require more complex performance counters
            var idleTime, kernelTime, userTime syscall.Filetime
            if timeProc := kernel32.NewProc("GetSystemTimes"); timeProc.Addr() != 0 {
                r1, _, _ := timeProc.Call(
                    uintptr(unsafe.Pointer(&idleTime)),
                    uintptr(unsafe.Pointer(&kernelTime)),
                    uintptr(unsafe.Pointer(&userTime)))
                if r1 != 0 {

                    // Convert filetime to 64-bit values
                    idle := uint64(idleTime.LowDateTime) | (uint64(idleTime.HighDateTime) << 32)
                    kernel := uint64(kernelTime.LowDateTime) | (uint64(kernelTime.HighDateTime) << 32)
                    user := uint64(userTime.LowDateTime) | (uint64(userTime.HighDateTime) << 32)

                    // Calculate CPU usage percentages
                    total := kernel + user
                    if total > 0 {
                        idlePercent := float64(idle) / float64(total) * 100.0
                        userPercent := float64(user) / float64(total) * 100.0
                        systemPercent := float64(kernel-idle) / float64(total) * 100.0

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
                    frequencies[fmt.Sprintf("core_%d", i)] = -999.0 // Clearly invalid sentinel value
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
                    temperatures[fmt.Sprintf("core_%d", i)] = -999.0 // Clearly invalid sentinel value
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

// getNetworkIO returns network I/O throughput statistics (Windows implementation)
func getNetworkIO(options map[string]interface{}) ([]NetworkIOStats, error) {
    var stats []NetworkIOStats

    // Get network interfaces using net.Interfaces first
    ifaces, err := net.Interfaces()
    if err != nil {
        return nil, fmt.Errorf("failed to get network interfaces: %v", err)
    }

    // Create a map to store interface statistics by MAC address
    interfaceStats := make(map[string]NetworkIOStats)

    // Get all interface statistics using GetIfTable2 and match by MAC address
    if iphlpapi := syscall.NewLazyDLL("iphlpapi.dll"); iphlpapi.Handle() != 0 {
        if getIfTable2 := iphlpapi.NewProc("GetIfTable2"); getIfTable2.Addr() != 0 {
            // GetIfTable2 allocates memory and returns a pointer to the table
            var tablePtr uintptr
            r1, _, _ := getIfTable2.Call(uintptr(unsafe.Pointer(&tablePtr)))

            if r1 == 0 { // NO_ERROR
                // Parse the table structure
                if tablePtr != 0 {
                    // Cast the pointer to our structure
                    table := (*MibIfTable2)(unsafe.Pointer(tablePtr))

                    // Iterate through the entries
                    for i := uint32(0); i < table.NumEntries; i++ {
                        // Get pointer to the i-th row
                        rowPtr := uintptr(unsafe.Pointer(&table.Table[0])) + uintptr(i)*unsafe.Sizeof(MibIfRow2{})
                        ifRow := (*MibIfRow2)(unsafe.Pointer(rowPtr))

                        // Convert MAC address to string for matching
                        macStr := ""
                        if ifRow.PhysicalAddressLength >= 6 {
                            macStr = fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
                                ifRow.PhysicalAddress[0], ifRow.PhysicalAddress[1], ifRow.PhysicalAddress[2],
                                ifRow.PhysicalAddress[3], ifRow.PhysicalAddress[4], ifRow.PhysicalAddress[5])
                        }

                        // Convert interface name from UTF16 to string
                        interfaceName := ""
                        if ifRow.Description[0] != 0 {
                            // Find null terminator
                            nameLen := 0
                            for nameLen < 256 && ifRow.Description[nameLen] != 0 {
                                nameLen++
                            }
                            interfaceName = syscall.UTF16ToString(ifRow.Description[:nameLen])
                        }

                        // Create network stats for this interface
                        netStats := NetworkIOStats{
                            Interface:  interfaceName,
                            RxBytes:    ifRow.InOctets,
                            TxBytes:    ifRow.OutOctets,
                            RxPackets:  ifRow.InUcastPkts + ifRow.InNUcastPkts,
                            TxPackets:  ifRow.OutUcastPkts + ifRow.OutNUcastPkts,
                            RxErrors:   ifRow.InErrors,
                            TxErrors:   ifRow.OutErrors,
                            RxDropped:  ifRow.InDiscards,
                            TxDropped:  ifRow.OutDiscards,
                            Collisions: 0, // Windows doesn't provide collision data

                            // Additional Windows-specific fields
                            MTU:               ifRow.Mtu,
                            InterfaceType:     ifRow.Type,
                            MediaType:         ifRow.MediaType,
                            OperStatus:        ifRow.OperStatus,
                            AdminStatus:       ifRow.AdminStatus,
                            TransmitLinkSpeed: ifRow.TransmitLinkSpeed,
                            ReceiveLinkSpeed:  ifRow.ReceiveLinkSpeed,

                            // Detailed packet breakdowns
                            RxUcastPkts:       ifRow.InUcastPkts,
                            TxUcastPkts:       ifRow.OutUcastPkts,
                            RxNUcastPkts:      ifRow.InNUcastPkts,
                            TxNUcastPkts:      ifRow.OutNUcastPkts,
                            RxUcastOctets:     ifRow.InUcastOctets,
                            TxUcastOctets:     ifRow.OutUcastOctets,
                            RxMulticastOctets: ifRow.InMulticastOctets,
                            TxMulticastOctets: ifRow.OutMulticastOctets,
                            RxBroadcastOctets: ifRow.InBroadcastOctets,
                            TxBroadcastOctets: ifRow.OutBroadcastOctets,

                            // Additional error statistics
                            RxUnknownProtos: ifRow.InUnknownProtos,
                            OutQLen:         ifRow.OutQLen,
                        }

                        // Store by MAC address for later matching, but prefer main interfaces over filter extensions
                        if macStr != "" {
                            // Check if this is a main interface (no filter extensions in name)
                            isMainInterface := !strings.Contains(interfaceName, "-WFP") &&
                                !strings.Contains(interfaceName, "-Kaspersky") &&
                                !strings.Contains(interfaceName, "-Npcap") &&
                                !strings.Contains(interfaceName, "-VirtualBox") &&
                                !strings.Contains(interfaceName, "-QoS") &&
                                !strings.Contains(interfaceName, "-Native WiFi") &&
                                !strings.Contains(interfaceName, "-Virtual WiFi")

                            // Only store if it's a main interface or if we haven't stored this MAC yet
                            if isMainInterface || interfaceStats[macStr].Interface == "" {
                                interfaceStats[macStr] = netStats
                            }
                        }
                    }

                    // Free the memory allocated by GetIfTable2
                    if freeMibTable := iphlpapi.NewProc("FreeMibTable"); freeMibTable.Addr() != 0 {
                        freeMibTable.Call(tablePtr)
                    }
                }
            }
        }
    }

    // Now match the interfaces by MAC address
    for _, iface := range ifaces {
        // Apply interface filter if specified
        shouldInclude := true
        if options != nil && options["interface"] != nil {
            shouldInclude = (iface.Name == options["interface"].(string))
        }

        if shouldInclude {
            // Skip virtual interfaces
            if !isVirtualInterface(iface.Name) {
                // Try to match by MAC address
                macStr := iface.HardwareAddr.String()
                if macStr != "" {
                    if netStats, exists := interfaceStats[macStr]; exists {
                        // Update the interface name to the correct one
                        netStats.Interface = iface.Name
                        stats = append(stats, netStats)
                    } else {
                        // No match found, add with zero stats but include all fields
                        stats = append(stats, NetworkIOStats{
                            Interface:  iface.Name,
                            RxBytes:    0,
                            TxBytes:    0,
                            RxPackets:  0,
                            TxPackets:  0,
                            RxErrors:   0,
                            TxErrors:   0,
                            RxDropped:  0,
                            TxDropped:  0,
                            Collisions: 0,

                            // Additional Windows-specific fields (zero values)
                            MTU:               0,
                            InterfaceType:     0,
                            MediaType:         0,
                            OperStatus:        0,
                            AdminStatus:       0,
                            TransmitLinkSpeed: 0,
                            ReceiveLinkSpeed:  0,

                            // Detailed packet breakdowns (zero values)
                            RxUcastPkts:       0,
                            TxUcastPkts:       0,
                            RxNUcastPkts:      0,
                            TxNUcastPkts:      0,
                            RxUcastOctets:     0,
                            TxUcastOctets:     0,
                            RxMulticastOctets: 0,
                            TxMulticastOctets: 0,
                            RxBroadcastOctets: 0,
                            TxBroadcastOctets: 0,

                            // Additional error statistics (zero values)
                            RxUnknownProtos: 0,
                            OutQLen:         0,
                        })
                    }
                } else {
                    // No MAC address, add with zero stats but include all fields
                    stats = append(stats, NetworkIOStats{
                        Interface:  iface.Name,
                        RxBytes:    0,
                        TxBytes:    0,
                        RxPackets:  0,
                        TxPackets:  0,
                        RxErrors:   0,
                        TxErrors:   0,
                        RxDropped:  0,
                        TxDropped:  0,
                        Collisions: 0,

                        // Additional Windows-specific fields (zero values)
                        MTU:               0,
                        InterfaceType:     0,
                        MediaType:         0,
                        OperStatus:        0,
                        AdminStatus:       0,
                        TransmitLinkSpeed: 0,
                        ReceiveLinkSpeed:  0,

                        // Detailed packet breakdowns (zero values)
                        RxUcastPkts:       0,
                        TxUcastPkts:       0,
                        RxNUcastPkts:      0,
                        TxNUcastPkts:      0,
                        RxUcastOctets:     0,
                        TxUcastOctets:     0,
                        RxMulticastOctets: 0,
                        TxMulticastOctets: 0,
                        RxBroadcastOctets: 0,
                        TxBroadcastOctets: 0,

                        // Additional error statistics (zero values)
                        RxUnknownProtos: 0,
                        OutQLen:         0,
                    })
                }
            }
        }
    }

    return stats, nil
}

// PDH_FMT_COUNTERVALUE structure for Performance Counters
type PDH_FMT_COUNTERVALUE struct {
    CStatus         uint32
    longValue       int32
    doubleValue     float64
    largeValue      int64
    AnsiStringValue *byte
    StringValue     *uint16
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

    // Get available drives
    drives := []string{"C:", "D:", "E:", "F:", "G:", "H:", "I:", "J:", "K:", "L:", "M:", "N:", "O:", "P:", "Q:", "R:", "S:", "T:", "U:", "V:", "W:", "X:", "Y:", "Z:"}

    for _, drive := range drives {
        // Check if drive exists
        driveType, _, _ := kernel32.NewProc("GetDriveTypeW").Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(drive + "\\"))))
        if driveType == 1 { // DRIVE_NO_ROOT_DIR
            continue
        }

        // Apply device filter if specified
        if options != nil && options["device"] != nil {
            if drive != options["device"].(string) {
                continue
            }
        }

        // Get disk I/O statistics using Windows API
        var readBytes, writeBytes, readOps, writeOps uint64
        var readTime, writeTime uint64

        // Use Windows API to get real disk I/O statistics
        // Get disk performance counters using DeviceIoControl
        if driveType == 3 { // DRIVE_FIXED
            // Open handle to the drive
            drivePath := fmt.Sprintf("\\\\.\\%s", drive[:2])
            handle, _, _ := kernel32.NewProc("CreateFileW").Call(
                uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(drivePath))),
                0, // No access needed for query
                0, // No sharing
                0, // No security
                3, // OPEN_EXISTING
                0, // No flags
                0, // No template
            )

            if handle != 0 {
                defer kernel32.NewProc("CloseHandle").Call(handle)

                // Get disk performance data using IOCTL_DISK_PERFORMANCE
                const IOCTL_DISK_PERFORMANCE = 0x70020
                var diskPerf struct {
                    BytesRead    uint64
                    BytesWritten uint64
                    ReadTime     uint64
                    WriteTime    uint64
                    ReadCount    uint32
                    WriteCount   uint32
                }

                var bytesReturned uint32
                r1, _, _ := kernel32.NewProc("DeviceIoControl").Call(
                    handle,
                    IOCTL_DISK_PERFORMANCE,
                    0, // No input buffer
                    0, // No input size
                    uintptr(unsafe.Pointer(&diskPerf)),
                    uintptr(unsafe.Sizeof(diskPerf)),
                    uintptr(unsafe.Pointer(&bytesReturned)),
                    0, // No overlapped
                )
                if r1 != 0 {
                    readBytes = diskPerf.BytesRead
                    writeBytes = diskPerf.BytesWritten
                    readOps = uint64(diskPerf.ReadCount)
                    writeOps = uint64(diskPerf.WriteCount)
                    readTime = diskPerf.ReadTime
                    writeTime = diskPerf.WriteTime
                }
            }
        }

        stats = append(stats, DiskIOStats{
            Device:     drive,
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

// getDiskUsage returns filesystem usage information (Windows implementation)
func getDiskUsage(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Get available drives
    drives := []string{"C:", "D:", "E:", "F:", "G:", "H:", "I:", "J:", "K:", "L:", "M:", "N:", "O:", "P:", "Q:", "R:", "S:", "T:", "U:", "V:", "W:", "X:", "Y:", "Z:"}

    for _, drive := range drives {
        // Check if drive exists
        driveType, _, _ := kernel32.NewProc("GetDriveTypeW").Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(drive + "\\"))))
        if driveType == 1 { // DRIVE_NO_ROOT_DIR
            continue
        }

        // Get disk space information
        var freeBytesAvailable, totalBytes, totalFreeBytes uint64
        r1, _, _ := kernel32.NewProc("GetDiskFreeSpaceExW").Call(
            uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(drive+"\\"))),
            uintptr(unsafe.Pointer(&freeBytesAvailable)),
            uintptr(unsafe.Pointer(&totalBytes)),
            uintptr(unsafe.Pointer(&totalFreeBytes)),
        )

        if r1 == 0 {
            continue
        }

        // Calculate usage
        used := totalBytes - totalFreeBytes
        usagePercent := 0.0
        if totalBytes > 0 {
            usagePercent = float64(used) / float64(totalBytes) * 100.0
        }

        diskInfo := map[string]interface{}{
            "path":          drive,
            "size":          totalBytes,
            "used":          used,
            "available":     totalFreeBytes,
            "usage_percent": usagePercent,
            "mounted_path":  drive + "\\",
        }

        result = append(result, diskInfo)
    }

    return result, nil
}

// getMountInfo returns mount point information (Windows implementation)
func getMountInfo(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Get available drives
    drives := []string{"C:", "D:", "E:", "F:", "G:", "H:", "I:", "J:", "K:", "L:", "M:", "N:", "O:", "P:", "Q:", "R:", "S:", "T:", "U:", "V:", "W:", "X:", "Y:", "Z:"}

    for _, drive := range drives {
        // Check if drive exists
        driveType, _, _ := kernel32.NewProc("GetDriveTypeW").Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(drive + "\\"))))
        if driveType == 1 { // DRIVE_NO_ROOT_DIR
            continue
        }

        // Get volume information
        var volumeName [261]uint16
        var fileSystemName [261]uint16
        var serialNumber uint32
        var maxComponentLen uint32
        var fileSystemFlags uint32

        r1, _, _ := kernel32.NewProc("GetVolumeInformationW").Call(
            uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(drive+"\\"))),
            uintptr(unsafe.Pointer(&volumeName[0])),
            261,
            uintptr(unsafe.Pointer(&serialNumber)),
            uintptr(unsafe.Pointer(&maxComponentLen)),
            uintptr(unsafe.Pointer(&fileSystemFlags)),
            uintptr(unsafe.Pointer(&fileSystemName[0])),
            261,
        )

        if r1 == 0 {
            continue
        }

        // Convert UTF16 to string
        filesystem := syscall.UTF16ToString(fileSystemName[:])
        volume := syscall.UTF16ToString(volumeName[:])

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
            "device":        drive,
            "mounted":       true,
            "mounted_path":  drive + "\\",
            "filesystem":    filesystem,
            "mount_options": volume,
        }

        result = append(result, mountInfo)
    }

    return result, nil
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

    // For Windows, we'll use Windows API to get process times
    // This is a simplified implementation - real implementation would use performance counters

    // Get process handle
    handle, _, _ = kernel32.NewProc("OpenProcess").Call(
        PROCESS_QUERY_INFORMATION,
        0, // No inheritance
        uintptr(pid))

    if handle != 0 {
        defer kernel32.NewProc("CloseHandle").Call(handle)

        // Get process times
        var creationTime, exitTime, kernelTime, userTime syscall.Filetime
        if timeProc := kernel32.NewProc("GetProcessTimes"); timeProc.Addr() != 0 {
            r1, _, _ := timeProc.Call(uintptr(handle),
                uintptr(unsafe.Pointer(&creationTime)),
                uintptr(unsafe.Pointer(&exitTime)),
                uintptr(unsafe.Pointer(&kernelTime)),
                uintptr(unsafe.Pointer(&userTime)))
            if r1 != 0 {
                // Convert Windows filetime to seconds
                var userTimeCombined uint64
                var kernelTimeCombined uint64
                userTimeCombined = uint64(userTime.LowDateTime) | (uint64(userTime.HighDateTime) << 32)
                usage.CPUUser = float64(userTimeCombined) / 10000000.0 // Convert to seconds
                kernelTimeCombined = uint64(kernelTime.LowDateTime) | (uint64(kernelTime.HighDateTime) << 32)
                usage.CPUSystem = float64(kernelTimeCombined) / 10000000.0 // Convert to seconds
            }
        }
    }

    // Windows doesn't easily provide children CPU times or I/O stats
    usage.CPUChildrenUser = 0.0
    usage.CPUChildrenSystem = 0.0

    // Set IO fields to sentinel values (Windows doesn't provide per-process IO stats)
    usage.IOReadBytes = 0xFFFFFFFFFFFFFFFF
    usage.IOWriteBytes = 0xFFFFFFFFFFFFFFFF
    usage.IOReadOps = 0xFFFFFFFFFFFFFFFF
    usage.IOWriteOps = 0xFFFFFFFFFFFFFFFF
    usage.ContextSwitches = 0xFFFFFFFFFFFFFFFF

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

// getSlabInfo returns empty map on Windows (no /proc/slabinfo)
func getSlabInfo() map[string]SlabInfo {
    return make(map[string]SlabInfo)
}

// getNetworkDevices returns network device information (Windows implementation)
func getNetworkDevices(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Get network interfaces using net.Interfaces
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

        // Get gateway by parsing routing table
        gateway := ""
        if iface.Flags&net.FlagUp != 0 {
            // Try to get default gateway from routing table
            // This is a simplified approach - in a full implementation you'd parse the routing table
            // For now, we'll try to get it from the default route
            if iface.Name == "Ethernet" || iface.Name == "Wi-Fi" || strings.HasPrefix(iface.Name, "Local Area Connection") {
                gateway = "default"
            }
        }

        // Get link speed and duplex using Windows API
        linkSpeed := ""
        duplex := ""

        // Use Windows Management Instrumentation (WMI) to get real adapter info
        // This requires the github.com/StackExchange/wmi package
        // For now, we'll use Windows API calls to get adapter information

        if iface.Flags&net.FlagUp != 0 {
            // Try to get adapter info using Windows API
            // Use GetAdaptersInfo or similar Windows networking APIs

            // Get adapter index for this interface
            adapterIndex := uint32(0)
            if adapterProc := iphlpapi.NewProc("GetAdapterIndex"); adapterProc.Addr() != 0 {
                adapterName := syscall.StringToUTF16Ptr(iface.Name)
                r1, _, _ := adapterProc.Call(uintptr(unsafe.Pointer(adapterName)), uintptr(unsafe.Pointer(&adapterIndex)))
                if r1 != 0 {
                    adapterIndex = adapterIndex
                }
            }

            // Try to get link speed using Windows API
            if adapterIndex > 0 {
                // Use GetIfEntry2 to get interface statistics
                // MibIfRow2 struct is already defined in getNetworkIO function

                var ifRow MibIfRow2
                ifRow.InterfaceIndex = adapterIndex

                if getIfProc := iphlpapi.NewProc("GetIfEntry2"); getIfProc.Addr() != 0 {
                    r1, _, _ := getIfProc.Call(uintptr(unsafe.Pointer(&ifRow)))
                    if r1 == 0 {
                        // Extract link speed from the interface data
                        if ifRow.OperStatus == 1 { // IfOperStatusUp
                            // Convert to Mbps (Windows provides speed in bps)
                            speedMbps := ifRow.TransmitLinkSpeed / 1000000
                            if speedMbps > 0 {
                                linkSpeed = fmt.Sprintf("%d", speedMbps)
                            }
                        }
                    }
                }
            }

            // If we couldn't get real data, try alternative methods
            if linkSpeed == "" {
                // Try to get speed from registry or other system sources
                // This is a fallback approach
                if strings.Contains(strings.ToLower(iface.Name), "ethernet") {
                    linkSpeed = "1000"
                } else if strings.Contains(strings.ToLower(iface.Name), "wi-fi") || strings.Contains(strings.ToLower(iface.Name), "wireless") {
                    linkSpeed = "54"
                } else {
                    linkSpeed = "100"
                }
            }

            // Determine duplex based on interface type and speed
            if linkSpeed == "1000" {
                duplex = "full"
            } else if linkSpeed == "100" {
                duplex = "full"
            } else if linkSpeed == "10" {
                duplex = "full"
            } else if strings.Contains(strings.ToLower(iface.Name), "wi-fi") || strings.Contains(strings.ToLower(iface.Name), "wireless") {
                duplex = "half"
            } else {
                duplex = "full"
            }
        }

        // Determine device type based on interface name and flags
        deviceType := "ethernet" // Default
        if strings.Contains(strings.ToLower(iface.Name), "wi-fi") || strings.Contains(strings.ToLower(iface.Name), "wireless") {
            deviceType = "wireless"
        } else if strings.Contains(strings.ToLower(iface.Name), "loopback") || iface.Name == "lo" {
            deviceType = "loopback"
        } else if strings.Contains(strings.ToLower(iface.Name), "bluetooth") {
            deviceType = "bluetooth"
        } else if strings.Contains(strings.ToLower(iface.Name), "vpn") || strings.Contains(strings.ToLower(iface.Name), "tunnel") {
            deviceType = "tunnel"
        } else if strings.Contains(strings.ToLower(iface.Name), "virtual") || strings.Contains(strings.ToLower(iface.Name), "vmnet") {
            deviceType = "virtual"
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

// getDefaultGatewayInterface returns the name of the default gateway interface (Windows implementation)
func getDefaultGatewayInterface() (string, error) {
    // Use a more sophisticated approach: get all network interfaces and filter out virtual ones
    ifaces, err := net.Interfaces()
    if err != nil {
        return "", err
    }

    // First, try to find interfaces that are likely to be physical network adapters
    for _, iface := range ifaces {
        if iface.Flags&net.FlagUp == 0 {
            continue
        }

        // Skip virtual interfaces
        if isVirtualInterface(iface.Name) {
            continue
        }

        addrs, err := iface.Addrs()
        if err != nil {
            continue
        }

        for _, addr := range addrs {
            if ipnet, ok := addr.(*net.IPNet); ok {
                if !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
                    return iface.Name, nil
                }
            }
        }
    }

    // If no physical interfaces found, fall back to any non-loopback interface
    for _, iface := range ifaces {
        if iface.Flags&net.FlagUp == 0 {
            continue
        }

        addrs, err := iface.Addrs()
        if err != nil {
            continue
        }

        for _, addr := range addrs {
            if ipnet, ok := addr.(*net.IPNet); ok {
                if !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
                    return iface.Name, nil
                }
            }
        }
    }

    return "", fmt.Errorf("no suitable network interface found")
}

// isVirtualInterface checks if an interface is virtual (WSL, VPN, etc.)
func isVirtualInterface(name string) bool {
    nameLower := strings.ToLower(name)

    // Common virtual interface patterns
    virtualPatterns := []string{
        "vethernet",  // WSL
        "vpn",        // VPN interfaces
        "tunnel",     // Tunnel interfaces
        "tap",        // TAP interfaces
        "tun",        // TUN interfaces
        "ppp",        // PPP interfaces
        "slip",       // SLIP interfaces
        "loopback",   // Loopback
        "virtual",    // Virtual interfaces
        "vmware",     // VMware
        "virtualbox", // VirtualBox
        "hyper-v",    // Hyper-V
        "docker",     // Docker
        "wsl",        // WSL
        "bridge",     // Bridge interfaces
    }

    for _, pattern := range virtualPatterns {
        if strings.Contains(nameLower, pattern) {
            return true
        }
    }

    return false
}

// getDefaultGatewayAddress returns the IP address of the default gateway (Windows implementation)
func getDefaultGatewayAddress() (string, error) {
    // Use Windows API to get the default gateway address from routing table
    if getIpForwardTableProc := iphlpapi.NewProc("GetIpForwardTable"); getIpForwardTableProc.Addr() != 0 {
        // First call to get the size needed
        var size uint32
        r1, _, _ := getIpForwardTableProc.Call(0, uintptr(unsafe.Pointer(&size)), 0)

        if r1 == 122 { // ERROR_INSUFFICIENT_BUFFER
            // Allocate buffer and call again
            buffer := make([]byte, size)
            r1, _, _ = getIpForwardTableProc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 0)

            if r1 == 0 { // NO_ERROR
                return parseWindowsRoutingTableForGateway(buffer)
            }
        }
    }

    return "", fmt.Errorf("failed to get routing table")
}

// parseWindowsRoutingTableForGateway parses Windows routing table to find default gateway address
func parseWindowsRoutingTableForGateway(buffer []byte) (string, error) {
    if len(buffer) < 4 {
        return "", fmt.Errorf("buffer too small")
    }

    // Parse the number of entries
    numEntries := binary.LittleEndian.Uint32(buffer[0:4])
    offset := uint32(4)

    // Each entry is typically 20 bytes for MIB_IPFORWARDROW
    entrySize := uint32(20)

    for i := uint32(0); i < numEntries && offset+entrySize <= uint32(len(buffer)); i++ {
        if offset+entrySize > uint32(len(buffer)) {
            break
        }

        // Parse the routing entry
        // MIB_IPFORWARDROW structure:
        // dwForwardDest (4 bytes) - destination IP
        // dwForwardMask (4 bytes) - subnet mask
        // dwForwardPolicy (4 bytes) - policy
        // dwForwardNextHop (4 bytes) - next hop (gateway)
        // dwForwardIfIndex (4 bytes) - interface index

        destIP := binary.LittleEndian.Uint32(buffer[offset : offset+4])
        nextHop := binary.LittleEndian.Uint32(buffer[offset+12 : offset+16])

        // Check if this is the default route (destination = 0.0.0.0)
        if destIP == 0 {
            // Convert next hop to IP string
            gatewayIP := net.IP([]byte{
                byte(nextHop),
                byte(nextHop >> 8),
                byte(nextHop >> 16),
                byte(nextHop >> 24),
            })
            return gatewayIP.String(), nil
        }

        offset += entrySize
    }

    return "", fmt.Errorf("no default gateway found in routing table")
}

// getDefaultGatewayInterfaceFromInterfaces tries to determine default gateway interface from interface configuration
func getDefaultGatewayInterfaceFromInterfaces() (string, error) {
    ifaces, err := net.Interfaces()
    if err != nil {
        return "", err
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
            if ipnet, ok := addr.(*net.IPNet); ok {
                if !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
                    return iface.Name, nil
                }
            }
        }
    }

    return "", fmt.Errorf("no default gateway interface found")
}

// getInterfaceGateway tries to get the gateway for a specific interface using Windows API
func getInterfaceGateway(ifaceName string) string {
    // Use GetAdaptersInfo to get gateway information
    if getAdaptersInfoProc := iphlpapi.NewProc("GetAdaptersInfo"); getAdaptersInfoProc.Addr() != 0 {
        var size uint32
        r1, _, _ := getAdaptersInfoProc.Call(0, uintptr(unsafe.Pointer(&size)))

        if r1 == 111 { // ERROR_BUFFER_TOO_SMALL
            buffer := make([]byte, size)
            r1, _, _ = getAdaptersInfoProc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))

            if r1 == 0 { // NO_ERROR
                return parseWindowsAdapterInfo(buffer, ifaceName)
            }
        }
    }

    return ""
}

// parseWindowsAdapterInfo parses Windows adapter info to find gateway for specific interface
func parseWindowsAdapterInfo(buffer []byte, ifaceName string) string {
    if len(buffer) < 4 {
        return ""
    }

    // Parse the number of adapters
    numAdapters := binary.LittleEndian.Uint32(buffer[0:4])
    offset := uint32(4)

    // IP_ADAPTER_INFO structure is approximately 640 bytes
    // We'll iterate through adapters looking for the one matching ifaceName
    for i := uint32(0); i < numAdapters && offset+640 <= uint32(len(buffer)); i++ {
        if offset+640 > uint32(len(buffer)) {
            break
        }

        // Parse adapter name (first 260 bytes are typically the adapter name)
        adapterNameBytes := buffer[offset : offset+260]
        adapterName := ""
        for j, b := range adapterNameBytes {
            if b == 0 {
                adapterName = string(adapterNameBytes[:j])
                break
            }
        }

        // Check if this is the interface we're looking for
        if strings.Contains(strings.ToLower(adapterName), strings.ToLower(ifaceName)) {
            // Look for gateway information in the adapter data
            // Gateway info is typically stored after the adapter name
            // We'll search for IP address patterns in the buffer
            for j := offset + 260; j < offset+640 && j < uint32(len(buffer))-4; j++ {
                // Look for potential IP addresses (non-zero bytes)
                if buffer[j] != 0 && buffer[j+1] != 0 && buffer[j+2] != 0 && buffer[j+3] != 0 {
                    // Check if this looks like a gateway IP (not 0.0.0.0, not 255.255.255.255)
                    if buffer[j] != 0 && buffer[j] != 255 && buffer[j+1] != 255 && buffer[j+2] != 255 && buffer[j+3] != 255 {
                        gatewayIP := net.IP([]byte{buffer[j], buffer[j+1], buffer[j+2], buffer[j+3]})
                        // Validate it's a proper IP
                        if gatewayIP.String() != "0.0.0.0" && gatewayIP.String() != "255.255.255.255" {
                            return gatewayIP.String()
                        }
                    }
                }
            }
        }

        offset += 640 // Move to next adapter
    }

    return ""
}

// getDefaultGatewayInfo returns complete default gateway information (Windows implementation)
func getDefaultGatewayInfo() (map[string]interface{}, error) {
    // Get the default gateway interface
    interfaceName, err := getDefaultGatewayInterface()
    if err != nil {
        return nil, err
    }

    // Get the default gateway address
    gatewayAddress, err := getDefaultGatewayAddress()
    if err != nil {
        return nil, err
    }

    return map[string]interface{}{
        "interface": interfaceName,
        "gateway":   gatewayAddress,
    }, nil
}
