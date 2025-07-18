//go:build (freebsd || openbsd || netbsd || dragonfly) && !linux && !windows
// +build freebsd openbsd netbsd dragonfly
// +build !linux
// +build !windows

package main

import (
    "bytes"
    "encoding/binary"
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

    "unsafe"

    "golang.org/x/sys/unix"
)

// BSD implementation of system monitoring functions
//  The kvm calls rely more than a little on m.hashimoto's code
//   @ https://github.com/mitchellh/go-ps/blob/master/process_freebsd.go

// BSD process enumeration constants
const (
    CTL_KERN           = 1  // "high kernel": proc, limits
    KERN_PROC          = 14 // struct: process entries
    KERN_PROC_PID      = 1  // by process id
    KERN_PROC_PROC     = 8  // only return procs
    KERN_PROC_PATHNAME = 12 // path to executable
    KERN_PROC_ARGS     = 7  // process arguments
)

// Kinfo_proc represents BSD process information structure
type Kinfo_proc struct {
    Ki_structsize   int32
    Ki_layout       int32
    Ki_args         int64
    Ki_paddr        int64
    Ki_addr         int64
    Ki_tracep       int64
    Ki_textvp       int64
    Ki_fd           int64
    Ki_vmspace      int64
    Ki_wchan        int64
    Ki_pid          int32
    Ki_ppid         int32
    Ki_pgid         int32
    Ki_tpgid        int32
    Ki_sid          int32
    Ki_tsid         int32
    Ki_jobc         [2]byte
    Ki_spare_short1 [2]byte
    Ki_tdev         int32
    Ki_siglist      [16]byte
    Ki_sigmask      [16]byte
    Ki_sigignore    [16]byte
    Ki_sigcatch     [16]byte
    Ki_uid          int32
    Ki_ruid         int32
    Ki_svuid        int32
    Ki_rgid         int32
    Ki_svgid        int32
    Ki_ngroups      [2]byte
    Ki_spare_short2 [2]byte
    Ki_groups       [64]byte
    Ki_size         int64
    Ki_rssize       int64
    Ki_swrss        int64
    Ki_tsize        int64
    Ki_dsize        int64
    Ki_ssize        int64
    Ki_xstat        [2]byte
    Ki_acflag       [2]byte
    Ki_pctcpu       int32
    Ki_estcpu       int32
    Ki_slptime      int32
    Ki_swtime       int32
    Ki_cow          int32
    Ki_runtime      int64
    Ki_start        [16]byte
    Ki_childtime    [16]byte
    Ki_flag         int64
    Ki_kiflag       int64
    Ki_traceflag    int32
    Ki_stat         [1]byte
    Ki_nice         [1]byte
    Ki_lock         [1]byte
    Ki_rqindex      [1]byte
    Ki_oncpu        [1]byte
    Ki_lastcpu      [1]byte
    Ki_ocomm        [17]byte
    Ki_wmesg        [9]byte
    Ki_login        [18]byte
    Ki_lockname     [9]byte
    Ki_comm         [20]byte
    Ki_emul         [17]byte
    Ki_sparestrings [68]byte
    Ki_spareints    [36]byte
    Ki_cr_flags     int32
    Ki_jid          int32
    Ki_numthreads   int32
    Ki_tid          int32
    Ki_pri          int32
    Ki_rusage       [144]byte
    Ki_rusage_ch    [144]byte
    Ki_pcb          int64
    Ki_kstack       int64
    Ki_udata        int64
    Ki_tdaddr       int64
    Ki_spareptrs    [48]byte
    Ki_spareint64s  [96]byte
    Ki_sflag        int64
    Ki_tdflags      int64
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

    // If sysctl failed, try using uptime command as fallback
    if !uptimeSet {
        cmd := exec.Command("uptime")
        if output, err := cmd.Output(); err == nil {
            // Parse uptime output to extract uptime
            outputStr := string(output)
            if strings.Contains(outputStr, "up") {
                // Look for patterns like "up 2 days, 3:45" or "up 3:45"
                parts := strings.Split(outputStr, "up")
                if len(parts) > 1 {
                    uptimePart := strings.Split(parts[1], ",")[0] // Get first part before comma
                    uptimePart = strings.TrimSpace(uptimePart)

                    // Try to parse different uptime formats
                    if strings.Contains(uptimePart, "day") {
                        // Format: "2 days, 3:45"
                        dayParts := strings.Split(uptimePart, "day")
                        if len(dayParts) > 0 {
                            if days, err := strconv.Atoi(strings.TrimSpace(dayParts[0])); err == nil {
                                // Convert days to seconds (simplified)
                                resources.Uptime = float64(days * 24 * 3600)
                                uptimeSet = true
                            }
                        }
                    } else if strings.Contains(uptimePart, ":") {
                        // Format: "3:45" (hours:minutes)
                        timeParts := strings.Split(uptimePart, ":")
                        if len(timeParts) == 2 {
                            if hours, err := strconv.Atoi(timeParts[0]); err == nil {
                                if minutes, err := strconv.Atoi(timeParts[1]); err == nil {
                                    resources.Uptime = float64(hours*3600 + minutes*60)
                                    uptimeSet = true
                                }
                            }
                        }
                    }
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
    // Use vmstat command for load average (primary method for BSD)
    cmd := exec.Command("vmstat")
    if output, err := cmd.Output(); err == nil {
        lines := strings.Split(string(output), "\n")
        for _, line := range lines {
            line = strings.TrimSpace(line)
            if line == "" || strings.HasPrefix(line, "procs") || strings.HasPrefix(line, "r") {
                continue // Skip header lines
            }

            fields := strings.Fields(line)
            if len(fields) >= 15 {
                // Parse vmstat output: r b w avm fre flt re pi po fr sr ada0 cd0 in sy cs us sy id
                // The last three fields are CPU usage percentages: us sy id
                if us, err := strconv.ParseFloat(fields[len(fields)-3], 64); err == nil {
                    if sy, err := strconv.ParseFloat(fields[len(fields)-2], 64); err == nil {
                        if id, err := strconv.ParseFloat(fields[len(fields)-1], 64); err == nil {
                            // Calculate load average based on CPU usage
                            // Use all three values for more accurate calculation
                            totalUsage := us + sy
                            totalCPU := us + sy + id

                            if totalCPU > 0 {
                                // Calculate load as percentage of CPU in use
                                cpuLoad := totalUsage / totalCPU
                                load1 := cpuLoad
                                load5 := cpuLoad  // Simplified - same as 1min for now
                                load15 := cpuLoad // Simplified - same as 1min for now
                                return []float64{load1, load5, load15}, nil
                            }
                        }
                    }
                }
                break
            }
        }
    }

    // Fallback to sysctl if vmstat fails
    loadPaths := []string{
        "vm.loadavg",
        "kern.loadavg",
        "vm.stats.vm.v_loadavg",
        "kern.cp_time",
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
            // Try using uptime command as fallback
            cmd := exec.Command("uptime")
            if output, err := cmd.Output(); err == nil {
                // Parse uptime output like "load average: 0.00, 0.00, 0.00"
                outputStr := string(output)
                if strings.Contains(outputStr, "load average:") {
                    parts := strings.Split(outputStr, "load average:")
                    if len(parts) > 1 {
                        loadPart := strings.TrimSpace(parts[1])
                        loads := strings.Split(loadPart, ",")
                        if len(loads) >= 3 {
                            result := make([]float64, 3)
                            for i, load := range loads {
                                if val, err := strconv.ParseFloat(strings.TrimSpace(load), 64); err == nil {
                                    result[i] = val
                                } else {
                                    result[i] = 0
                                }
                            }
                            return result, nil
                        }
                    }
                }
            }

            // Try using sysctl command directly for load average
            loadPaths := []string{"vm.loadavg", "kern.loadavg"}
            for _, path := range loadPaths {
                cmd := exec.Command("sysctl", "-n", path)
                if output, err := cmd.Output(); err == nil {
                    data := strings.TrimSpace(string(output))
                    fields := strings.Fields(data)
                    if len(fields) >= 3 {
                        result := make([]float64, 3)
                        for i := 0; i < 3; i++ {
                            if i < len(fields) {
                                if val, err := strconv.ParseFloat(fields[i], 64); err == nil {
                                    result[i] = val
                                } else {
                                    result[i] = 0
                                }
                            } else {
                                result[i] = 0
                            }
                        }
                        return result, nil
                    }
                }
            }

            // Return error if all methods fail
            return []float64{0, 0, 0}, fmt.Errorf("failed to get load average from any source")
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
        "hw.physmem64",
        "hw.realmem64",
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

    // Use vmstat command for memory information (primary method for BSD)
    cmd := exec.Command("vmstat")
    if output, err := cmd.Output(); err == nil {
        lines := strings.Split(string(output), "\n")
        for _, line := range lines {
            line = strings.TrimSpace(line)
            if line == "" || strings.HasPrefix(line, "procs") || strings.HasPrefix(line, "r") {
                continue // Skip header lines
            }

            fields := strings.Fields(line)
            if len(fields) >= 5 {
                // Parse vmstat output: r b w avm fre flt re pi po fr sr ada0 cd0 in sy cs us sy id
                // avm = active virtual memory (used)
                // fre = free memory
                if avm, err := strconv.ParseUint(fields[3], 10, 64); err == nil {
                    info.Used = avm
                }
                if fre, err := strconv.ParseUint(fields[4], 10, 64); err == nil {
                    info.Free = fre
                }

                // Calculate total memory as used + free
                if info.Used > 0 && info.Free > 0 {
                    info.Total = info.Used + info.Free
                }
                break
            }
        }
    }

    // Set sentinel values for fields not provided by vmstat
    if info.Cached == 0 {
        info.Cached = 0xFFFFFFFFFFFFFFFF // Sentinel value
    }
    if info.Buffers == 0 {
        info.Buffers = 0xFFFFFFFFFFFFFFFF // Sentinel value
    }

    // Calculate available memory
    info.Available = info.Free + info.Cached + info.Buffers

    // Get swap information using vm.swap_info
    swapData, err := syscall.Sysctl("vm.swap_info")
    if err == nil {
        // Parse swap info from BSD sysctl output
        lines := strings.Split(swapData, "\n")
        for _, line := range lines {
            line = strings.TrimSpace(line)
            if line == "" {
                continue
            }

            // Look for swap device information
            fields := strings.Fields(line)
            if len(fields) >= 4 {
                // Try to parse swap device line
                // Format might be: device total used free
                if total, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
                    info.SwapTotal += total * 1024 // Convert KB to bytes
                }
                if used, err := strconv.ParseUint(fields[2], 10, 64); err == nil {
                    info.SwapUsed += used * 1024 // Convert KB to bytes
                }
                if free, err := strconv.ParseUint(fields[3], 10, 64); err == nil {
                    info.SwapFree += free * 1024 // Convert KB to bytes
                }
            }
        }
    }

    // Set sentinel values for swap if not available
    if info.SwapTotal == 0 {
        info.SwapTotal = 0xFFFFFFFFFFFFFFFF // Sentinel value
    }
    if info.SwapUsed == 0 {
        info.SwapUsed = 0xFFFFFFFFFFFFFFFF // Sentinel value
    }
    if info.SwapFree == 0 {
        info.SwapFree = 0xFFFFFFFFFFFFFFFF // Sentinel value
    }

    // Calculate available memory
    info.Available = info.Free + info.Cached + info.Buffers

    // Initialize pressure and OOM scores maps
    info.Pressure = make(map[string]PressureStats)
    info.OOMScores = make(map[string]int)
    info.Slab = make(map[string]SlabInfo)

    return info, nil
}

// getProcessList returns list of all processes using BSD sysctl
func getProcessList(options map[string]interface{}) ([]ProcessInfo, error) {
    var processes []ProcessInfo

    // Use BSD sysctl to get all processes
    mib := []int32{CTL_KERN, KERN_PROC, KERN_PROC_PROC, 0}
    buf, length, err := call_sysctl(mib)
    if err != nil {
        return processes, fmt.Errorf("failed to get process list: %v", err)
    }

    // Get kinfo_proc size
    k := Kinfo_proc{}
    procinfo_len := int(unsafe.Sizeof(k))
    count := int(length / uint64(procinfo_len))

    // Parse each process
    for i := 0; i < count; i++ {
        b := buf[i*procinfo_len : i*procinfo_len+procinfo_len]
        kinfo, err := parse_kinfo_proc(b)
        if err != nil {
            continue
        }

        process := ProcessInfo{
            PID:         int(kinfo.Ki_pid),
            Name:        getCommString(kinfo.Ki_comm),
            State:       getProcessState(kinfo.Ki_stat[0]),
            PPID:        int(kinfo.Ki_ppid),
            Priority:    int(kinfo.Ki_pri),
            Nice:        int(kinfo.Ki_nice[0]),
            StartTime:   int64(kinfo.Ki_start[0]), // Simplified
            Threads:     int(kinfo.Ki_numthreads),
            UID:         fmt.Sprintf("%d", kinfo.Ki_uid),
            GID:         fmt.Sprintf("%d", kinfo.Ki_rgid),
            UserTime:    float64(kinfo.Ki_runtime) / 1000000.0, // Convert to seconds
            SystemTime:  0.0,                                   // Not directly available in kinfo_proc
            MemoryUsage: uint64(kinfo.Ki_size),
            MemoryRSS:   uint64(kinfo.Ki_rssize * 4096), // Convert pages to bytes
            Command:     getCommString(kinfo.Ki_comm),
        }

        // Get command line arguments
        if args, err := getProcessArgs(kinfo.Ki_pid); err == nil {
            process.Command = args
        }

        processes = append(processes, process)
    }

    return processes, nil
}

// getProcessInfo returns detailed information for a specific process
func getProcessInfo(pid int, options map[string]interface{}) (ProcessInfo, error) {
    var proc ProcessInfo
    proc.PID = pid

    // Use BSD sysctl to get specific process
    mib := []int32{CTL_KERN, KERN_PROC, KERN_PROC_PID, int32(pid)}
    buf, length, err := call_sysctl(mib)
    if err != nil {
        return proc, fmt.Errorf("process %d not found", pid)
    }

    // Check if we got the expected data size
    k := Kinfo_proc{}
    if length != uint64(unsafe.Sizeof(k)) {
        return proc, fmt.Errorf("invalid process data size")
    }

    kinfo, err := parse_kinfo_proc(buf)
    if err != nil {
        return proc, fmt.Errorf("failed to parse process data: %v", err)
    }

    proc = ProcessInfo{
        PID:         int(kinfo.Ki_pid),
        Name:        getCommString(kinfo.Ki_comm),
        State:       getProcessState(kinfo.Ki_stat[0]),
        PPID:        int(kinfo.Ki_ppid),
        Priority:    int(kinfo.Ki_pri),
        Nice:        int(kinfo.Ki_nice[0]),
        StartTime:   int64(kinfo.Ki_start[0]), // Simplified
        Threads:     int(kinfo.Ki_numthreads),
        UID:         fmt.Sprintf("%d", kinfo.Ki_uid),
        GID:         fmt.Sprintf("%d", kinfo.Ki_rgid),
        UserTime:    float64(kinfo.Ki_runtime) / 1000000.0,
        SystemTime:  0.0, // Not directly available in kinfo_proc
        MemoryUsage: uint64(kinfo.Ki_size),
        MemoryRSS:   uint64(kinfo.Ki_rssize * 4096),
        Command:     getCommString(kinfo.Ki_comm),
    }

    // Get command line arguments
    if args, err := getProcessArgs(kinfo.Ki_pid); err == nil {
        proc.Command = args
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

        // Fallback values
        info.Usage["user"] = 0.0
        info.Usage["system"] = 0.0
        info.Usage["interrupt"] = 0.0
        info.Usage["idle"] = 100.0

        // Get CPU usage from sysctl
        cpuTimeData, err := syscall.Sysctl("kern.cp_time")
        if err == nil {
            // Parse CPU time data
            fields := strings.Fields(cpuTimeData)
            if len(fields) >= 4 {
                user, _ := strconv.ParseUint(fields[0], 10, 64)
                nice, _ := strconv.ParseUint(fields[1], 10, 64)
                system, _ := strconv.ParseUint(fields[2], 10, 64)
                interrupt, _ := strconv.ParseUint(fields[3], 10, 64)
                idle, _ := strconv.ParseUint(fields[4], 10, 64)

                // Calculate percentages
                total := user + nice + system + interrupt + idle
                if total > 0 {
                    userPercent := float64(user) / float64(total) * 100.0
                    systemPercent := float64(system) / float64(total) * 100.0
                    interruptPercent := float64(interrupt) / float64(total) * 100.0
                    idlePercent := float64(idle) / float64(total) * 100.0

                    info.Usage["user"] = userPercent
                    info.Usage["interrupt"] = interruptPercent
                    info.Usage["system"] = systemPercent
                    info.Usage["idle"] = idlePercent
                }
            }
        }
    } else {
        // Return data for all cores
        info.Usage = make(map[string]interface{})
        cores := make(map[string]interface{})

        for i := 0; i < info.Cores; i++ {
            coreData := make(map[string]interface{})

            // Fallback values
            coreData["user"] = 0.0
            coreData["system"] = 0.0
            coreData["idle"] = 100.0
            coreData["interrupt"] = 0.0

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
                    interrupt, _ := strconv.ParseUint(fields[3], 10, 64)
                    idle, _ := strconv.ParseUint(fields[4], 10, 64)

                    // Calculate percentages
                    total := user + nice + system + interrupt + idle
                    if total > 0 {
                        userPercent := float64(user) / float64(total) * 100.0
                        systemPercent := float64(system) / float64(total) * 100.0
                        interruptPercent := float64(interrupt) / float64(total) * 100.0
                        idlePercent := float64(idle) / float64(total) * 100.0

                        coreData["user"] = userPercent
                        coreData["system"] = systemPercent
                        coreData["interrupt"] = interruptPercent
                        coreData["idle"] = idlePercent
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

    // Use BSD sysctl to get process resource usage
    mib := []int32{CTL_KERN, KERN_PROC, KERN_PROC_PID, int32(pid)}
    buf, length, err := call_sysctl(mib)
    if err != nil {
        return usage, fmt.Errorf("process %d not found", pid)
    }

    k := Kinfo_proc{}
    if length != uint64(unsafe.Sizeof(k)) {
        return usage, fmt.Errorf("invalid process data size")
    }

    kinfo, err := parse_kinfo_proc(buf)
    if err != nil {
        return usage, fmt.Errorf("failed to parse process data: %v", err)
    }

    // Set CPU times from kinfo_proc
    usage.CPUUser = float64(kinfo.Ki_runtime) / 1000000.0
    usage.CPUSystem = 0.0         // Not directly available
    usage.CPUChildrenUser = 0.0   // Not directly available
    usage.CPUChildrenSystem = 0.0 // Not directly available

    // Set memory usage from kinfo_proc
    usage.MemoryCurrent = uint64(kinfo.Ki_rssize * 4096) // RSS in pages
    usage.MemoryPeak = uint64(kinfo.Ki_size)             // Virtual size

    // Set sentinel values for fields not available on BSD
    usage.IOReadBytes = 0xFFFFFFFFFFFFFFFF
    usage.IOWriteBytes = 0xFFFFFFFFFFFFFFFF
    usage.IOReadOps = 0xFFFFFFFFFFFFFFFF
    usage.IOWriteOps = 0xFFFFFFFFFFFFFFFF
    usage.ContextSwitches = 0xFFFFFFFFFFFFFFFF
    usage.PageFaults = 0xFFFFFFFFFFFFFFFF

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

// getProcessState converts BSD process state to string
func getProcessState(stat byte) string {
    switch stat {
    case 1:
        return "S" // Sleeping
    case 2:
        return "R" // Running
    case 3:
        return "Z" // Zombie
    case 4:
        return "T" // Stopped
    case 5:
        return "D" // Uninterruptible sleep
    case 6:
        return "W" // Wait
    case 7:
        return "L" // Lock wait
    default:
        return "?"
    }
}

// Helper functions
func call_sysctl(mib []int32) ([]byte, uint64, error) {
    miblen := uint64(len(mib))

    // Get required buffer size
    length := uint64(0)
    _, _, err := syscall.RawSyscall6(
        syscall.SYS___SYSCTL,
        uintptr(unsafe.Pointer(&mib[0])),
        uintptr(miblen),
        0,
        uintptr(unsafe.Pointer(&length)),
        0,
        0)
    if err != 0 {
        return make([]byte, 0), length, err
    }
    if length == 0 {
        return make([]byte, 0), length, nil
    }

    // Get proc info itself
    buf := make([]byte, length)
    _, _, err = syscall.RawSyscall6(
        syscall.SYS___SYSCTL,
        uintptr(unsafe.Pointer(&mib[0])),
        uintptr(miblen),
        uintptr(unsafe.Pointer(&buf[0])),
        uintptr(unsafe.Pointer(&length)),
        0,
        0)
    if err != 0 {
        return buf, length, err
    }

    return buf, length, nil
}

func parse_kinfo_proc(buf []byte) (Kinfo_proc, error) {
    var k Kinfo_proc
    br := bytes.NewReader(buf)
    err := binary.Read(br, binary.LittleEndian, &k)
    if err != nil {
        return k, err
    }
    return k, nil
}

func getCommString(comm [20]byte) string {
    n := -1
    for i, b := range comm {
        if b == 0 {
            break
        }
        n = i + 1
    }
    if n == -1 {
        n = len(comm)
    }
    return string(comm[:n])
}

func getProcessArgs(pid int32) (string, error) {
    // Get command line arguments using sysctl
    // Use KERN_PROC_ARGS to get the full command line
    mib := []int32{CTL_KERN, KERN_PROC, KERN_PROC_ARGS, pid}
    buf, length, err := call_sysctl(mib)
    if err != nil {
        return "", err
    }

    if length == 0 {
        return "", nil
    }

    // Parse the command line arguments
    // The buffer contains null-terminated strings
    var args []string
    current := ""
    for i := 0; i < len(buf); i++ {
        if buf[i] == 0 {
            if current != "" {
                args = append(args, current)
                current = ""
            }
        } else {
            current += string(buf[i])
        }
    }

    // Join all arguments with spaces
    return strings.Join(args, " "), nil
}
