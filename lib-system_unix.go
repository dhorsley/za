//go:build linux
// +build linux

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
)

// Unix/Linux implementation of system monitoring functions

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
    if data, err := os.ReadFile("/proc/uptime"); err == nil {
        fields := strings.Fields(string(data))
        if len(fields) > 0 {
            resources.Uptime, _ = strconv.ParseFloat(fields[0], 64)
        }
    }

    return resources, nil
}

// getSystemLoad returns system load averages
func getSystemLoad() ([]float64, error) {
    data, err := os.ReadFile("/proc/loadavg")
    if err != nil {
        return nil, fmt.Errorf("failed to read /proc/loadavg: %v", err)
    }

    fields := strings.Fields(string(data))
    if len(fields) < 3 {
        return nil, fmt.Errorf("invalid loadavg format")
    }

    loads := make([]float64, 3)
    for i := 0; i < 3; i++ {
        loads[i], err = strconv.ParseFloat(fields[i], 64)
        if err != nil {
            return nil, fmt.Errorf("failed to parse load average: %v", err)
        }
    }

    return loads, nil
}

// getMemoryInfo returns detailed memory information including pressure and OOM scores
func getMemoryInfo() (MemoryInfo, error) {
    var info MemoryInfo

    // Read /proc/meminfo
    data, err := os.ReadFile("/proc/meminfo")
    if err != nil {
        return info, fmt.Errorf("failed to read /proc/meminfo: %v", err)
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        fields := strings.Fields(line)
        if len(fields) < 2 {
            continue
        }

        value, err := strconv.ParseUint(fields[1], 10, 64)
        if err != nil {
            continue
        }

        // Convert from KB to bytes
        value *= 1024

        switch fields[0] {
        case "MemTotal:":
            info.Total = value
        case "MemAvailable:":
            info.Available = value
        case "MemFree:":
            info.Free = value
        case "Cached:":
            info.Cached = value
        case "Buffers:":
            info.Buffers = value
        case "SwapTotal:":
            info.SwapTotal = value
        case "SwapFree:":
            info.SwapFree = value
        }
    }

    info.Used = info.Total - info.Free - info.Cached - info.Buffers
    info.SwapUsed = info.SwapTotal - info.SwapFree

    // Get memory pressure
    info.Pressure = getMemoryPressure()

    // Get OOM scores
    info.OOMScores = getOOMScores()

    // Get slab info
    info.Slab = getSlabInfo()

    return info, nil
}

// getMemoryPressure reads memory pressure from /proc/pressure/memory
func getMemoryPressure() map[string]PressureStats {
    pressure := make(map[string]PressureStats)

    // Read /proc/pressure/memory
    if data, err := os.ReadFile("/proc/pressure/memory"); err == nil {
        lines := strings.Split(string(data), "\n")
        for _, line := range lines {
            if strings.TrimSpace(line) == "" {
                continue
            }
            fields := strings.Fields(line)
            if len(fields) >= 4 {
                resource := fields[0]
                avg10, _ := strconv.ParseFloat(fields[1], 64)
                avg60, _ := strconv.ParseFloat(fields[2], 64)
                avg300, _ := strconv.ParseFloat(fields[3], 64)
                total, _ := strconv.ParseUint(fields[4], 10, 64)

                pressure[resource] = PressureStats{
                    Avg10:  avg10,
                    Avg60:  avg60,
                    Avg300: avg300,
                    Total:  total,
                }
            }
        }
    }

    return pressure
}

// getOOMScores reads OOM scores from /proc/*/oom_score
func getOOMScores() map[string]int {
    scores := make(map[string]int)

    // Read /proc/*/oom_score for all processes
    entries, err := os.ReadDir("/proc")
    if err != nil {
        return scores
    }

    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }

        // Check if it's a process directory (numeric name)
        if _, err := strconv.Atoi(entry.Name()); err != nil {
            continue
        }

        // Read oom_score
        oomScorePath := fmt.Sprintf("/proc/%s/oom_score", entry.Name())
        if data, err := os.ReadFile(oomScorePath); err == nil {
            if score, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
                scores[entry.Name()] = score
            }
        }
    }

    return scores
}

// getSlabInfo reads slab allocation info from /proc/slabinfo
func getSlabInfo() map[string]SlabInfo {
    slabInfo := make(map[string]SlabInfo)

    // Read /proc/slabinfo
    data, err := os.ReadFile("/proc/slabinfo")
    if err != nil {
        return slabInfo
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 7 {
            continue
        }

        name := fields[0]
        activeObjs, _ := strconv.ParseUint(fields[1], 10, 64)
        numObjs, _ := strconv.ParseUint(fields[2], 10, 64)
        objSize, _ := strconv.ParseUint(fields[3], 10, 64)
        objPerSlab, _ := strconv.ParseUint(fields[4], 10, 64)
        pagesPerSlab, _ := strconv.ParseUint(fields[5], 10, 64)
        limit, _ := strconv.ParseUint(fields[6], 10, 64)
        batchCount, _ := strconv.ParseUint(fields[7], 10, 64)

        slabInfo[name] = SlabInfo{
            ActiveObjs:   activeObjs,
            NumObjs:      numObjs,
            ObjSize:      objSize,
            ObjPerSlab:   objPerSlab,
            PagesPerSlab: pagesPerSlab,
            Limit:        limit,
            BatchCount:   batchCount,
        }
    }

    return slabInfo
}

// getProcessList returns list of all processes
func getProcessList(options map[string]interface{}) ([]ProcessInfo, error) {
    var processes []ProcessInfo

    // Read /proc directory to get process list
    entries, err := os.ReadDir("/proc")
    if err != nil {
        return nil, err
    }

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

// getProcessInfo returns detailed information for a specific process
func getProcessInfo(pid int, options map[string]interface{}) (ProcessInfo, error) {
    var proc ProcessInfo
    proc.PID = pid

    // Read /proc/{pid}/stat
    statPath := fmt.Sprintf("/proc/%d/stat", pid)
    data, err := os.ReadFile(statPath)
    if err != nil {
        return proc, err
    }

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

    info.Cores = runtime.NumCPU()
    info.Threads = runtime.NumCPU()

    // Validate core number if specified
    if coreNumber >= 0 {
        if coreNumber >= info.Cores {
            return info, fmt.Errorf("invalid core number %d: system has %d cores", coreNumber, info.Cores)
        }
    }

    // Check if we should include detailed information
    includeDetails := false
    if options != nil && options["details"] != nil {
        if details, ok := options["details"].(bool); ok {
            includeDetails = details
        }
    }

    // Read CPU model from /proc/cpuinfo
    data, err := os.ReadFile("/proc/cpuinfo")
    if err == nil {
        lines := strings.Split(string(data), "\n")
        for _, line := range lines {
            if strings.HasPrefix(line, "model name") {
                parts := strings.SplitN(line, ":", 2)
                if len(parts) == 2 {
                    info.Model = strings.TrimSpace(parts[1])
                    break
                }
            }
        }
    }

    // Read CPU usage from /proc/stat
    data, err = os.ReadFile("/proc/stat")
    if err == nil {
        lines := strings.Split(string(data), "\n")

        if coreNumber >= 0 {
            // Get specific core information
            coreFound := false
            for _, line := range lines {
                if strings.HasPrefix(line, fmt.Sprintf("cpu%d ", coreNumber)) {
                    fields := strings.Fields(line)
                    if len(fields) >= 5 {
                        info.Usage = make(map[string]interface{})
                        info.Usage["core"] = coreNumber
                        info.Usage["user"] = parseUint64(fields[1])
                        info.Usage["nice"] = parseUint64(fields[2])
                        info.Usage["system"] = parseUint64(fields[3])
                        info.Usage["idle"] = parseUint64(fields[4])
                        if len(fields) > 5 {
                            info.Usage["iowait"] = parseUint64(fields[5])
                        }
                        if len(fields) > 6 {
                            info.Usage["irq"] = parseUint64(fields[6])
                        }
                        if len(fields) > 7 {
                            info.Usage["softirq"] = parseUint64(fields[7])
                        }
                        if len(fields) > 8 {
                            info.Usage["steal"] = parseUint64(fields[8])
                        }
                        if len(fields) > 9 {
                            info.Usage["guest"] = parseUint64(fields[9])
                        }
                        if len(fields) > 10 {
                            info.Usage["guest_nice"] = parseUint64(fields[10])
                        }
                        coreFound = true
                    }
                    break
                }
            }

            if !coreFound {
                return info, fmt.Errorf("core %d not found in /proc/stat", coreNumber)
            }
        } else {
            // Get data for all individual cores
            info.Usage = make(map[string]interface{})
            cores := make(map[string]interface{})

            for core := 0; core < info.Cores; core++ {
                coreFound := false
                for _, line := range lines {
                    if strings.HasPrefix(line, fmt.Sprintf("cpu%d ", core)) {
                        fields := strings.Fields(line)
                        if len(fields) >= 5 {
                            coreData := make(map[string]interface{})
                            coreData["user"] = parseUint64(fields[1])
                            coreData["nice"] = parseUint64(fields[2])
                            coreData["system"] = parseUint64(fields[3])
                            coreData["idle"] = parseUint64(fields[4])
                            if len(fields) > 5 {
                                coreData["iowait"] = parseUint64(fields[5])
                            }
                            if len(fields) > 6 {
                                coreData["irq"] = parseUint64(fields[6])
                            }
                            if len(fields) > 7 {
                                coreData["softirq"] = parseUint64(fields[7])
                            }
                            if len(fields) > 8 {
                                coreData["steal"] = parseUint64(fields[8])
                            }
                            if len(fields) > 9 {
                                coreData["guest"] = parseUint64(fields[9])
                            }
                            if len(fields) > 10 {
                                coreData["guest_nice"] = parseUint64(fields[10])
                            }
                            cores[fmt.Sprintf("core_%d", core)] = coreData
                            coreFound = true
                        }
                        break
                    }
                }
                if !coreFound {
                    // If we can't find individual core data, fall back to overall stats
                    for _, line := range lines {
                        if strings.HasPrefix(line, "cpu ") {
                            fields := strings.Fields(line)
                            if len(fields) >= 5 {
                                coreData := make(map[string]interface{})
                                coreData["user"] = parseUint64(fields[1])
                                coreData["nice"] = parseUint64(fields[2])
                                coreData["system"] = parseUint64(fields[3])
                                coreData["idle"] = parseUint64(fields[4])
                                if len(fields) > 5 {
                                    coreData["iowait"] = parseUint64(fields[5])
                                }
                                if len(fields) > 6 {
                                    coreData["irq"] = parseUint64(fields[6])
                                }
                                if len(fields) > 7 {
                                    coreData["softirq"] = parseUint64(fields[7])
                                }
                                if len(fields) > 8 {
                                    coreData["steal"] = parseUint64(fields[8])
                                }
                                if len(fields) > 9 {
                                    coreData["guest"] = parseUint64(fields[9])
                                }
                                if len(fields) > 10 {
                                    coreData["guest_nice"] = parseUint64(fields[10])
                                }
                                cores[fmt.Sprintf("core_%d", core)] = coreData
                            }
                            break
                        }
                    }
                }
            }
            info.Usage["cores"] = cores
        }
    }

    // If details are requested, try to get additional information
    if includeDetails {
        if coreNumber >= 0 {
            // Try to read CPU frequency information for specific core
            freqPath := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", coreNumber)
            if freqData, err := os.ReadFile(freqPath); err == nil {
                if freq, err := strconv.ParseUint(strings.TrimSpace(string(freqData)), 10, 64); err == nil {
                    info.Usage["frequency_mhz"] = float64(freq) / 1000.0
                }
            } else {
                // Try alternative frequency paths
                altFreqPaths := []string{
                    fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_cur_freq", coreNumber),
                    fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_available_frequencies", coreNumber),
                }
                for _, altPath := range altFreqPaths {
                    if freqData, err := os.ReadFile(altPath); err == nil {
                        if strings.Contains(altPath, "scaling_available_frequencies") {
                            // Parse the first frequency from the list
                            freqs := strings.Fields(string(freqData))
                            if len(freqs) > 0 {
                                if freq, err := strconv.ParseUint(freqs[0], 10, 64); err == nil {
                                    info.Usage["frequency_mhz"] = float64(freq) / 1000.0
                                    break
                                }
                            }
                        } else {
                            if freq, err := strconv.ParseUint(strings.TrimSpace(string(freqData)), 10, 64); err == nil {
                                info.Usage["frequency_mhz"] = float64(freq) / 1000.0
                                break
                            }
                        }
                    }
                }

                // If no cpufreq data available, try /proc/cpuinfo
                if _, exists := info.Usage["frequency_mhz"]; !exists {
                    if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
                        lines := strings.Split(string(data), "\n")
                        currentProcessor := -1
                        for _, line := range lines {
                            if strings.HasPrefix(line, "processor") {
                                parts := strings.SplitN(line, ":", 2)
                                if len(parts) == 2 {
                                    if proc, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
                                        currentProcessor = proc
                                    }
                                }
                            } else if strings.HasPrefix(line, "cpu MHz") && currentProcessor == coreNumber {
                                parts := strings.SplitN(line, ":", 2)
                                if len(parts) == 2 {
                                    if freq, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
                                        info.Usage["frequency_mhz"] = freq
                                        break
                                    }
                                }
                            }
                        }
                    }
                }
            }
        } else {
            // Get frequency data for all cores
            frequencies := make(map[string]interface{})
            for core := 0; core < info.Cores; core++ {
                freqPath := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", core)
                freqFound := false

                if freqData, err := os.ReadFile(freqPath); err == nil {
                    if freq, err := strconv.ParseUint(strings.TrimSpace(string(freqData)), 10, 64); err == nil {
                        frequencies[fmt.Sprintf("core_%d", core)] = float64(freq) / 1000.0
                        freqFound = true
                    }
                }

                if !freqFound {
                    // Try alternative frequency paths
                    altFreqPaths := []string{
                        fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_cur_freq", core),
                        fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_available_frequencies", core),
                    }
                    for _, altPath := range altFreqPaths {
                        if freqData, err := os.ReadFile(altPath); err == nil {
                            if strings.Contains(altPath, "scaling_available_frequencies") {
                                // Parse the first frequency from the list
                                freqs := strings.Fields(string(freqData))
                                if len(freqs) > 0 {
                                    if freq, err := strconv.ParseUint(freqs[0], 10, 64); err == nil {
                                        frequencies[fmt.Sprintf("core_%d", core)] = float64(freq) / 1000.0
                                        freqFound = true
                                        break
                                    }
                                }
                            } else {
                                if freq, err := strconv.ParseUint(strings.TrimSpace(string(freqData)), 10, 64); err == nil {
                                    frequencies[fmt.Sprintf("core_%d", core)] = float64(freq) / 1000.0
                                    freqFound = true
                                    break
                                }
                            }
                        }
                    }
                }

                if !freqFound {
                    frequencies[fmt.Sprintf("core_%d", core)] = -999.0 // Clearly invalid sentinel value
                }
            }

            // If no cpufreq data available, try /proc/cpuinfo for overall frequency
            if len(frequencies) == 0 || allZeroFrequencies(frequencies) {
                if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
                    lines := strings.Split(string(data), "\n")
                    currentProcessor := -1
                    for _, line := range lines {
                        if strings.HasPrefix(line, "processor") {
                            parts := strings.SplitN(line, ":", 2)
                            if len(parts) == 2 {
                                if proc, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
                                    currentProcessor = proc
                                }
                            }
                        } else if strings.HasPrefix(line, "cpu MHz") {
                            parts := strings.SplitN(line, ":", 2)
                            if len(parts) == 2 {
                                if freq, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
                                    if currentProcessor >= 0 && currentProcessor < info.Cores {
                                        frequencies[fmt.Sprintf("core_%d", currentProcessor)] = freq
                                    }
                                }
                            }
                        }
                    }
                }
            }
            info.Usage["frequencies_mhz"] = frequencies
        }

        // Try to read CPU temperature (if available)
        tempPaths := []string{
            "/sys/class/thermal/thermal_zone0/temp",
            "/sys/devices/platform/coretemp.0/temp1_input",
            "/sys/devices/platform/coretemp.0/temp2_input",
            "/sys/devices/platform/coretemp.0/temp3_input",
            "/sys/devices/platform/coretemp.0/temp4_input",
            "/sys/devices/platform/coretemp.0/temp5_input",
            "/sys/devices/platform/coretemp.0/temp6_input",
            "/sys/devices/platform/coretemp.0/temp7_input",
            "/sys/devices/platform/coretemp.0/temp8_input",
            "/sys/class/hwmon/hwmon0/temp1_input",
            "/sys/class/hwmon/hwmon1/temp1_input",
            "/sys/class/hwmon/hwmon2/temp1_input",
            "/sys/class/hwmon/hwmon3/temp1_input",
            "/sys/class/hwmon/hwmon4/temp1_input",
            "/sys/class/hwmon/hwmon5/temp1_input",
            "/sys/class/hwmon/hwmon6/temp1_input",
            "/sys/class/hwmon/hwmon7/temp1_input",
            "/sys/class/hwmon/hwmon8/temp1_input",
            // Additional paths that might work in WSL
            "/sys/devices/virtual/thermal/thermal_zone0/temp",
            "/sys/devices/virtual/thermal/thermal_zone1/temp",
            "/sys/devices/virtual/thermal/thermal_zone2/temp",
            "/sys/devices/virtual/thermal/thermal_zone3/temp",
            "/sys/devices/virtual/thermal/thermal_zone4/temp",
            "/sys/devices/virtual/thermal/thermal_zone5/temp",
            "/sys/devices/virtual/thermal/thermal_zone6/temp",
            "/sys/devices/virtual/thermal/thermal_zone7/temp",
            "/sys/devices/virtual/thermal/thermal_zone8/temp",
            "/sys/devices/virtual/thermal/thermal_zone9/temp",
            "/sys/devices/virtual/thermal/thermal_zone10/temp",
            "/sys/devices/virtual/thermal/thermal_zone11/temp",
            "/sys/devices/virtual/thermal/thermal_zone12/temp",
            "/sys/devices/virtual/thermal/thermal_zone13/temp",
            "/sys/devices/virtual/thermal/thermal_zone14/temp",
            "/sys/devices/virtual/thermal/thermal_zone15/temp",
        }

        tempFound := false
        for _, tempPath := range tempPaths {
            if tempData, err := os.ReadFile(tempPath); err == nil {
                if temp, err := strconv.ParseUint(strings.TrimSpace(string(tempData)), 10, 64); err == nil {
                    info.Usage["temperature_celsius"] = float64(temp) / 1000.0 // Convert millidegrees to degrees
                    tempFound = true
                    break
                }
            }
        }

        if !tempFound {
            // Try to find temperature files dynamically
            hwmonDirs, err := os.ReadDir("/sys/class/hwmon")
            if err == nil {
                for _, hwmon := range hwmonDirs {
                    if hwmon.IsDir() {
                        hwmonPath := fmt.Sprintf("/sys/class/hwmon/%s", hwmon.Name())
                        files, err := os.ReadDir(hwmonPath)
                        if err == nil {
                            for _, file := range files {
                                if strings.HasPrefix(file.Name(), "temp") && strings.HasSuffix(file.Name(), "_input") {
                                    tempPath := fmt.Sprintf("%s/%s", hwmonPath, file.Name())
                                    if tempData, err := os.ReadFile(tempPath); err == nil {
                                        if temp, err := strconv.ParseUint(strings.TrimSpace(string(tempData)), 10, 64); err == nil {
                                            info.Usage["temperature_celsius"] = float64(temp) / 1000.0
                                            tempFound = true
                                            break
                                        }
                                    }
                                }
                            }
                        }
                        if tempFound {
                            break
                        }
                    }
                }
            }
        }

        // If still no temperature found, try reading from /proc/acpi/thermal_zone (if available)
        if !tempFound {
            if thermalEntries, err := os.ReadDir("/proc/acpi/thermal_zone"); err == nil {
                for _, entry := range thermalEntries {
                    if entry.IsDir() {
                        tempPath := fmt.Sprintf("/proc/acpi/thermal_zone/%s/temperature", entry.Name())
                        if tempData, err := os.ReadFile(tempPath); err == nil {
                            // Parse ACPI temperature format (e.g., "temperature:             45 C")
                            lines := strings.Split(string(tempData), "\n")
                            for _, line := range lines {
                                if strings.Contains(line, "temperature:") {
                                    parts := strings.Fields(line)
                                    if len(parts) >= 2 {
                                        if temp, err := strconv.ParseFloat(parts[1], 64); err == nil {
                                            info.Usage["temperature_celsius"] = temp
                                            tempFound = true
                                            break
                                        }
                                    }
                                }
                            }
                        }
                        if tempFound {
                            break
                        }
                    }
                }
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

// Helper function to parse uint64 with error handling
func parseUint64(s string) uint64 {
    val, _ := strconv.ParseUint(s, 10, 64)
    return val
}

// Helper function to check if all frequency values are zero
func allZeroFrequencies(frequencies map[string]interface{}) bool {
    for _, freq := range frequencies {
        if freqFloat, ok := freq.(float64); ok {
            if freqFloat != 0.0 {
                return false
            }
        }
    }
    return true
}

// Debug function to check available frequency and temperature files
func debugCPUFiles() map[string]interface{} {
    debug := make(map[string]interface{})

    // Check frequency files
    freqFiles := []string{}
    for i := 0; i < 16; i++ { // Check first 16 cores
        freqPaths := []string{
            fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_cur_freq", i),
            fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_cur_freq", i),
            fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_available_frequencies", i),
            fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_max_freq", i),
            fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_min_freq", i),
        }
        for _, path := range freqPaths {
            if _, err := os.Stat(path); err == nil {
                freqFiles = append(freqFiles, path)
            }
        }
    }
    debug["frequency_files"] = freqFiles

    // Check if cpufreq directory exists at all
    cpufreqExists := false
    if _, err := os.Stat("/sys/devices/system/cpu/cpu0/cpufreq"); err == nil {
        cpufreqExists = true
    }
    debug["cpufreq_exists"] = cpufreqExists

    // Check temperature files
    tempFiles := []string{}
    tempPaths := []string{
        "/sys/class/thermal/thermal_zone0/temp",
        "/sys/devices/platform/coretemp.0/temp1_input",
        "/sys/class/hwmon/hwmon0/temp1_input",
        "/sys/class/hwmon/hwmon1/temp1_input",
        "/sys/class/hwmon/hwmon2/temp1_input",
        "/sys/class/hwmon/hwmon3/temp1_input",
        "/sys/class/hwmon/hwmon4/temp1_input",
        "/sys/class/hwmon/hwmon5/temp1_input",
        "/sys/class/hwmon/hwmon6/temp1_input",
        "/sys/class/hwmon/hwmon7/temp1_input",
        "/sys/class/hwmon/hwmon8/temp1_input",
    }
    for _, path := range tempPaths {
        if _, err := os.Stat(path); err == nil {
            tempFiles = append(tempFiles, path)
        }
    }
    debug["temperature_files"] = tempFiles

    // Check thermal zones
    thermalZones := []string{}
    if thermalEntries, err := os.ReadDir("/sys/class/thermal"); err == nil {
        for _, entry := range thermalEntries {
            if entry.IsDir() {
                thermalZones = append(thermalZones, entry.Name())
            }
        }
    }
    debug["thermal_zones"] = thermalZones

    // Check hwmon directories
    hwmonDirs := []string{}
    if hwmonEntries, err := os.ReadDir("/sys/class/hwmon"); err == nil {
        for _, entry := range hwmonEntries {
            if entry.IsDir() {
                hwmonDirs = append(hwmonDirs, entry.Name())
            }
        }
    }
    debug["hwmon_dirs"] = hwmonDirs

    // Check if /proc/cpuinfo has frequency info
    cpuinfoFreq := false
    if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
        if strings.Contains(string(data), "cpu MHz") {
            cpuinfoFreq = true
        }
    }
    debug["cpuinfo_has_frequency"] = cpuinfoFreq

    return debug
}

// getNetworkIO returns network I/O statistics
func getNetworkIO(options map[string]interface{}) ([]NetworkIOStats, error) {
    var stats []NetworkIOStats

    // Read /proc/net/dev
    data, err := os.ReadFile("/proc/net/dev")
    if err != nil {
        return nil, fmt.Errorf("failed to read /proc/net/dev: %v", err)
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines[2:] { // Skip header lines
        if strings.TrimSpace(line) == "" {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 16 {
            continue
        }

        interfaceName := strings.TrimSuffix(fields[0], ":")
        rxBytes, _ := strconv.ParseUint(fields[1], 10, 64)
        txBytes, _ := strconv.ParseUint(fields[9], 10, 64)
        rxPackets, _ := strconv.ParseUint(fields[2], 10, 64)
        txPackets, _ := strconv.ParseUint(fields[10], 10, 64)
        rxErrors, _ := strconv.ParseUint(fields[3], 10, 64)
        txErrors, _ := strconv.ParseUint(fields[11], 10, 64)
        rxDropped, _ := strconv.ParseUint(fields[4], 10, 64)
        txDropped, _ := strconv.ParseUint(fields[12], 10, 64)

        // Apply interface filter if specified
        if options != nil && options["interface"] != nil {
            if interfaceName != options["interface"].(string) {
                continue
            }
        }

        stats = append(stats, NetworkIOStats{
            Interface: interfaceName,
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

    // Read /proc/diskstats
    data, err := os.ReadFile("/proc/diskstats")
    if err != nil {
        return stats, err
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 14 {
            continue
        }

        device := fields[2]
        readOps, _ := strconv.ParseUint(fields[3], 10, 64)
        readBytes, _ := strconv.ParseUint(fields[5], 10, 64)
        writeOps, _ := strconv.ParseUint(fields[7], 10, 64)
        writeBytes, _ := strconv.ParseUint(fields[9], 10, 64)
        readTime, _ := strconv.ParseUint(fields[6], 10, 64)
        writeTime, _ := strconv.ParseUint(fields[10], 10, 64)

        // Apply device filter if specified
        if options != nil && options["device"] != nil {
            if device != options["device"].(string) {
                continue
            }
        }

        stats = append(stats, DiskIOStats{
            Device:     device,
            ReadBytes:  readBytes * 512, // Convert sectors to bytes
            WriteBytes: writeBytes * 512,
            ReadOps:    readOps,
            WriteOps:   writeOps,
            ReadTime:   readTime,
            WriteTime:  writeTime,
        })
    }

    return stats, nil
}

// getResourceUsage returns resource usage for a specific process
func getResourceUsage(pid int) (ResourceUsage, error) {
    var usage ResourceUsage
    usage.PID = pid

    // Read /proc/{pid}/stat
    statPath := fmt.Sprintf("/proc/%d/stat", pid)
    data, err := os.ReadFile(statPath)
    if err != nil {
        return usage, err
    }

    fields := strings.Fields(string(data))
    if len(fields) < 24 {
        return usage, fmt.Errorf("invalid stat format")
    }

    // Parse CPU times
    utime, _ := strconv.ParseUint(fields[13], 10, 64)
    stime, _ := strconv.ParseUint(fields[14], 10, 64)
    cutime, _ := strconv.ParseUint(fields[15], 10, 64)
    cstime, _ := strconv.ParseUint(fields[16], 10, 64)
    usage.CPUUser = float64(utime) / 100.0            // Convert to seconds
    usage.CPUSystem = float64(stime) / 100.0          // Convert to seconds
    usage.CPUChildrenUser = float64(cutime) / 100.0   // Convert to seconds
    usage.CPUChildrenSystem = float64(cstime) / 100.0 // Convert to seconds

    // Read memory info from /proc/{pid}/status
    statusPath := fmt.Sprintf("/proc/%d/status", pid)
    if statusData, err := os.ReadFile(statusPath); err == nil {
        lines := strings.Split(string(statusData), "\n")
        for _, line := range lines {
            if strings.HasPrefix(line, "VmRSS:") {
                fields := strings.Fields(line)
                if len(fields) >= 2 {
                    if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
                        usage.MemoryCurrent = val * 1024 // Convert KB to bytes
                    }
                }
            } else if strings.HasPrefix(line, "VmPeak:") {
                fields := strings.Fields(line)
                if len(fields) >= 2 {
                    if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
                        usage.MemoryPeak = val * 1024 // Convert KB to bytes
                    }
                }
            }
        }
    }

    // Read I/O stats from /proc/{pid}/io
    ioPath := fmt.Sprintf("/proc/%d/io", pid)
    if ioData, err := os.ReadFile(ioPath); err == nil {
        lines := strings.Split(string(ioData), "\n")
        for _, line := range lines {
            fields := strings.Fields(line)
            if len(fields) < 2 {
                continue
            }
            switch fields[0] {
            case "rchar:":
                usage.IOReadBytes, _ = strconv.ParseUint(fields[1], 10, 64)
            case "wchar:":
                usage.IOWriteBytes, _ = strconv.ParseUint(fields[1], 10, 64)
            case "syscr:":
                usage.IOReadOps, _ = strconv.ParseUint(fields[1], 10, 64)
            case "syscw:":
                usage.IOWriteOps, _ = strconv.ParseUint(fields[1], 10, 64)
            }
        }
    }

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

// getDiskUsage returns filesystem usage information
func getDiskUsage(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Read /proc/mounts to get mount points
    data, err := os.ReadFile("/proc/mounts")
    if err != nil {
        return result, err
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 6 {
            continue
        }

        device := fields[0]
        mountPoint := fields[1]
        filesystem := fields[2]

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

        // Get filesystem stats
        var stat syscall.Statfs_t
        if err := syscall.Statfs(mountPoint, &stat); err != nil {
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

// getMountInfo returns mount point information
func getMountInfo(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Read /proc/mounts
    data, err := os.ReadFile("/proc/mounts")
    if err != nil {
        return result, err
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 6 {
            continue
        }

        device := fields[0]
        mountPoint := fields[1]
        filesystem := fields[2]
        mountOptions := fields[3]

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

// getNetworkDevices returns network device information
func getNetworkDevices(options map[string]interface{}) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    // Read /sys/class/net to get network interfaces
    netDir := "/sys/class/net"
    entries, err := os.ReadDir(netDir)
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

    for _, entry := range entries {
        deviceName := entry.Name()

        // Check if device is up
        operstatePath := fmt.Sprintf("%s/%s/operstate", netDir, deviceName)
        operstateData, err := os.ReadFile(operstatePath)
        operstate := "unknown"
        if err == nil {
            operstate = strings.TrimSpace(string(operstateData))
        }

        // Skip down interfaces unless include_all is true
        if operstate != "up" && !includeAll {
            continue
        }

        // Get MAC address
        addressPath := fmt.Sprintf("%s/%s/address", netDir, deviceName)
        macAddress := ""
        if addressData, err := os.ReadFile(addressPath); err == nil {
            macAddress = strings.TrimSpace(string(addressData))
        }

        // Get device type
        typePath := fmt.Sprintf("%s/%s/type", netDir, deviceName)
        deviceType := ""
        if typeData, err := os.ReadFile(typePath); err == nil {
            deviceType = strings.TrimSpace(string(typeData))
        }

        // Get speed
        speedPath := fmt.Sprintf("%s/%s/speed", netDir, deviceName)
        speed := ""
        if speedData, err := os.ReadFile(speedPath); err == nil {
            speed = strings.TrimSpace(string(speedData))
        }

        // Get duplex
        duplexPath := fmt.Sprintf("%s/%s/duplex", netDir, deviceName)
        duplex := ""
        if duplexData, err := os.ReadFile(duplexPath); err == nil {
            duplex = strings.TrimSpace(string(duplexData))
        }

        // Get IP addresses using netlink or /proc/net/dev
        ipAddresses := []string{}

        // Get IP addresses from the interface
        if operstate == "up" {
            // Use net.Interfaces to get actual IP addresses
            if netIface, err := net.InterfaceByName(deviceName); err == nil {
                if addrs, err := netIface.Addrs(); err == nil {
                    for _, addr := range addrs {
                        if ipnet, ok := addr.(*net.IPNet); ok {
                            // Only include non-loopback addresses
                            if !ipnet.IP.IsLoopback() {
                                ipAddresses = append(ipAddresses, ipnet.IP.String())
                            }
                        }
                    }
                }
            }
        }

        // Get gateway by parsing routing table
        gateway := ""
        if operstate == "up" {
            // Try to get default gateway from /proc/net/route
            if routeData, err := os.ReadFile("/proc/net/route"); err == nil {
                lines := strings.Split(string(routeData), "\n")
                for _, line := range lines {
                    if strings.HasPrefix(line, deviceName) {
                        fields := strings.Fields(line)
                        if len(fields) >= 4 {
                            // Check if this is the default route (destination 00000000)
                            if fields[1] == "00000000" {
                                // Gateway is in field 2 (hex format)
                                if gatewayHex := fields[2]; gatewayHex != "00000000" {
                                    // Convert hex gateway to IP
                                    if len(gatewayHex) == 8 {
                                        // Parse hex IP (little endian)
                                        ipBytes := make([]byte, 4)
                                        for i := 0; i < 4; i++ {
                                            hexByte := gatewayHex[i*2 : i*2+2]
                                            if val, err := strconv.ParseUint(hexByte, 16, 8); err == nil {
                                                ipBytes[3-i] = byte(val) // Reverse byte order
                                            }
                                        }
                                        gateway = fmt.Sprintf("%d.%d.%d.%d", ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])
                                    }
                                }
                                break
                            }
                        }
                    }
                }
            }
        }

        deviceInfo := map[string]interface{}{
            "name":         deviceName,
            "enabled":      operstate == "up",
            "mac_address":  macAddress,
            "ip_addresses": ipAddresses,
            "gateway":      gateway,
            "link_speed":   speed,
            "duplex":       duplex,
            "device_type":  deviceType,
            "operstate":    operstate,
        }

        result = append(result, deviceInfo)
    }

    return result, nil
}

// getDefaultGatewayInterface returns the name of the default gateway interface
func getDefaultGatewayInterface() (string, error) {
    // Read /proc/net/route to find the default route
    data, err := os.ReadFile("/proc/net/route")
    if err != nil {
        return "", err
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 8 {
            continue
        }

        // Check if this is the default route (destination 00000000)
        if fields[1] == "00000000" {
            // Interface name is in the first field
            return fields[0], nil
        }
    }

    return "", fmt.Errorf("no default route found")
}

// getDefaultGatewayAddress returns the IP address of the default gateway
func getDefaultGatewayAddress() (string, error) {
    // Read /proc/net/route to find the default route
    data, err := os.ReadFile("/proc/net/route")
    if err != nil {
        return "", err
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 8 {
            continue
        }

        // Check if this is the default route (destination 00000000)
        if fields[1] == "00000000" {
            // Gateway is in field 2 (hex format)
            if gatewayHex := fields[2]; gatewayHex != "00000000" {
                // Convert hex gateway to IP
                if len(gatewayHex) == 8 {
                    var ipBytes [4]byte
                    for i := 0; i < 4; i++ {
                        hexByte := gatewayHex[i*2 : i*2+2]
                        if val, err := strconv.ParseUint(hexByte, 16, 8); err == nil {
                            ipBytes[i] = byte(val)
                        }
                    }
                    gateway := fmt.Sprintf("%d.%d.%d.%d", ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])
                    return gateway, nil
                }
            }
        }
    }

    return "", fmt.Errorf("no default gateway found")
}

// getDefaultGatewayInfo returns complete default gateway information
func getDefaultGatewayInfo() (map[string]interface{}, error) {
    // Read /proc/net/route to find the default route
    data, err := os.ReadFile("/proc/net/route")
    if err != nil {
        return nil, err
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) < 8 {
            continue
        }

        // Check if this is the default route (destination 00000000)
        if fields[1] == "00000000" {
            interfaceName := fields[0]
            gatewayHex := fields[2]

            // Convert hex gateway to IP
            var gateway string
            if gatewayHex != "00000000" && len(gatewayHex) == 8 {
                var ipBytes [4]byte
                for i := 0; i < 4; i++ {
                    hexByte := gatewayHex[i*2 : i*2+2]
                    if val, err := strconv.ParseUint(hexByte, 16, 8); err == nil {
                        ipBytes[i] = byte(val)
                    }
                }
                gateway = fmt.Sprintf("%d.%d.%d.%d", ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])
            }

            return map[string]interface{}{
                "interface": interfaceName,
                "gateway":   gateway,
            }, nil
        }
    }

    return nil, fmt.Errorf("no default gateway found")
}
