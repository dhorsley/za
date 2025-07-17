package main

import (
    "errors"
    "time"
)

// System monitoring library - Za library function definitions only
// Implementation is in platform-specific files: lib-system_unix.go, lib-system_windows.go, lib-system_bsd.go

// Data structures for system monitoring

// ProcessInfo represents detailed process information
type ProcessInfo struct {
    PID                int
    Name               string
    State              string
    PPID               int
    Priority           int
    Nice               int
    StartTime          int64
    UID                string
    GID                string
    UserTime           float64 // CPU time spent in user mode
    SystemTime         float64 // CPU time spent in system mode
    ChildrenUserTime   float64 // CPU time spent by children in user mode
    ChildrenSystemTime float64 // CPU time spent by children in system mode
    MemoryUsage        uint64
    MemoryRSS          uint64
    Threads            int
    Command            string
}

// SystemResources represents overall system resource usage
type SystemResources struct {
    CPUCount     int
    LoadAverage  []float64
    MemoryTotal  uint64
    MemoryUsed   uint64
    MemoryFree   uint64
    MemoryCached uint64
    SwapTotal    uint64
    SwapUsed     uint64
    SwapFree     uint64
    Uptime       float64
}

// MemoryInfo represents detailed memory information including pressure and OOM scores
type MemoryInfo struct {
    Total     uint64
    Available uint64
    Used      uint64
    Free      uint64
    Cached    uint64
    Buffers   uint64
    SwapTotal uint64
    SwapUsed  uint64
    SwapFree  uint64
    Pressure  map[string]PressureStats
    OOMScores map[string]int
    Slab      map[string]SlabInfo
}

// PressureStats represents memory pressure statistics
type PressureStats struct {
    Avg10  float64
    Avg60  float64
    Avg300 float64
    Total  uint64
}

// SlabInfo represents kernel slab allocation information
type SlabInfo struct {
    ActiveObjs   uint64
    NumObjs      uint64
    ObjSize      uint64
    ObjPerSlab   uint64
    PagesPerSlab uint64
    Limit        uint64
    BatchCount   uint64
}

// CPUInfo represents CPU information and statistics
type CPUInfo struct {
    Model       string
    Cores       int
    Threads     int
    Usage       map[string]interface{} // Changed from []float64 to map for named keys
    LoadAverage []float64
}

// NetworkIOStats represents network I/O statistics
type NetworkIOStats struct {
    Interface  string
    RxBytes    uint64
    TxBytes    uint64
    RxPackets  uint64
    TxPackets  uint64
    RxErrors   uint64
    TxErrors   uint64
    RxDropped  uint64
    TxDropped  uint64
    Collisions uint64 // Added for BSD and any platform that provides it
}

// DiskIOStats represents disk I/O statistics
type DiskIOStats struct {
    Device     string
    ReadBytes  uint64
    WriteBytes uint64
    ReadOps    uint64
    WriteOps   uint64
    ReadTime   uint64
    WriteTime  uint64
}

// ProcessTree represents process hierarchy
type ProcessTree struct {
    PID      int
    Name     string
    Children []ProcessTree
}

// ProcessMap represents process relationships
type ProcessMap struct {
    PID       int
    Name      string
    Relations map[string][]ProcessMap
}

// ResourceUsage represents resource usage for a specific process
type ResourceUsage struct {
    PID               int
    CPUUser           float64
    CPUSystem         float64
    CPUChildrenUser   float64 // Children CPU time in user mode
    CPUChildrenSystem float64 // Children CPU time in system mode
    MemoryCurrent     uint64
    MemoryPeak        uint64
    IOReadBytes       uint64
    IOWriteBytes      uint64
    IOReadOps         uint64
    IOWriteOps        uint64
    ContextSwitches   uint64
    PageFaults        uint64
}

// ResourceSnapshot represents a point-in-time snapshot of system resources
type ResourceSnapshot struct {
    Timestamp time.Time
    Processes []ProcessInfo
    System    SystemResources
    Memory    MemoryInfo
    CPU       CPUInfo
    Network   []NetworkIOStats
    Disk      []DiskIOStats
}

func buildSystemLib() {
    features["system"] = Feature{version: 1, category: "monitoring"}
    categories["system"] = []string{
        "top_cpu", "top_mem", "top_nio", "top_dio",
        "sys_resources", "sys_load",
        "mem_info",
        "ps_info", "ps_tree", "ps_map",
        "cpu_info",
        "nio", "dio",
        "resource_usage", "iodiff",
        "disk_usage", "mount_info", "net_devices",
    }

    // Top N resource consumers (with ALL option where n=-1)
    slhelp["top_cpu"] = LibHelp{in: "n", out: "[]ProcessInfo", action: "Returns top N CPU consumers (processes). Use n=-1 for ALL processes."}
    stdlib["top_cpu"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("top_cpu", args, 1, "1", "int"); !ok {
            return nil, err
        }
        n := args[0].(int)
        return getTopCPU(n)
    }

    slhelp["top_mem"] = LibHelp{in: "n", out: "[]ProcessInfo", action: "Returns top N memory consumers (processes). Use n=-1 for ALL processes."}
    stdlib["top_mem"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("top_mem", args, 1, "1", "int"); !ok {
            return nil, err
        }
        n := args[0].(int)
        return getTopMemory(n)
    }

    slhelp["top_nio"] = LibHelp{in: "n", out: "[]NetworkIOStats", action: "Returns top N network consumers (interfaces). Use n=-1 for ALL interfaces."}
    stdlib["top_nio"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("top_nio", args, 1, "1", "int"); !ok {
            return nil, err
        }
        n := args[0].(int)
        return getTopNetwork(n)
    }

    slhelp["top_dio"] = LibHelp{in: "n", out: "[]DiskIOStats", action: "Returns top N disk I/O consumers (devices). Use n=-1 for ALL devices."}
    stdlib["top_dio"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("top_dio", args, 1, "1", "int"); !ok {
            return nil, err
        }
        n := args[0].(int)
        return getTopDiskIO(n)
    }

    // System information functions
    slhelp["sys_resources"] = LibHelp{in: "", out: "SystemResources", action: "Returns overall system resource usage."}
    stdlib["sys_resources"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("sys_resources", args, 0); !ok {
            return nil, err
        }
        return getSystemResources()
    }

    slhelp["sys_load"] = LibHelp{in: "", out: "map", action: "Returns system load averages (1, 5, 15 minute) with named keys."}
    stdlib["sys_load"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("sys_load", args, 0); !ok {
            return nil, err
        }
        load, err := getSystemLoad()
        if err != nil {
            return nil, err
        }

        // Convert array to map with named keys
        result := make(map[string]interface{})
        if len(load) >= 1 {
            result["load_1min"] = load[0]
        }
        if len(load) >= 2 {
            result["load_5min"] = load[1]
        }
        if len(load) >= 3 {
            result["load_15min"] = load[2]
        }

        return result, nil
    }

    // Memory information with pressure and OOM scores
    slhelp["mem_info"] = LibHelp{in: "", out: "MemoryInfo", action: "Returns detailed memory information including pressure indicators, OOM scores, and slab allocation."}
    stdlib["mem_info"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("mem_info", args, 0); !ok {
            return nil, err
        }
        return getMemoryInfo()
    }

    // Process information functions
    slhelp["ps_info"] = LibHelp{in: "pid[,options]", out: "ProcessInfo", action: "Returns detailed process information. Options: map(.include_cmdline true, .include_environ false)"}
    stdlib["ps_info"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("ps_info", args, 2, "1", "int", "1", "int", "map[string]interface{}"); !ok {
            return nil, err
        }
        pid := args[0].(int)
        var options map[string]interface{}
        if len(args) > 1 {
            options = args[1].(map[string]interface{})
        }
        return getProcessInfo(pid, options)
    }

    slhelp["ps_tree"] = LibHelp{in: "[pid]", out: "ProcessTree", action: "Returns process hierarchy. Optional starting PID, defaults to root."}
    stdlib["ps_tree"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("ps_tree", args, 2, "1", "int", "0"); !ok {
            return nil, err
        }
        var pid int = -1
        if len(args) > 0 {
            pid = args[0].(int)
        }
        return getProcessTree(pid)
    }

    slhelp["ps_map"] = LibHelp{in: "[pid]", out: "ProcessMap", action: "Returns process relationships. Optional starting PID, defaults to root."}
    stdlib["ps_map"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("ps_map", args, 2, "1", "int", "0"); !ok {
            return nil, err
        }
        var pid int = -1
        if len(args) > 0 {
            pid = args[0].(int)
        }
        return getProcessMap(pid)
    }

    slhelp["ps_list"] = LibHelp{in: "[options]", out: "[]ProcessInfo", action: "Returns list of all processes. Options: map(.include_cmdline true, .include_environ false)"}
    stdlib["ps_list"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        var options map[string]interface{}
        if ok, err := expect_args("ps_list", args, 2, "1", "map", "0"); !ok {
            return nil, err
        }
        if len(args) > 0 {
            options = args[0].(map[string]interface{})
        }
        return getProcessList(options)
    }

    // CPU information functions
    slhelp["cpu_info"] = LibHelp{in: "[core_number|options]", out: "CPUInfo", action: "Returns CPU information. Optional core number or options map(.core 0, .details true)"}
    stdlib["cpu_info"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) > 1 {
            return nil, errors.New("cpu_info expects 0 or 1 argument")
        }
        var coreNumber int = -1
        var options map[string]interface{}
        if len(args) > 0 {
            switch v := args[0].(type) {
            case int:
                coreNumber = v
            case map[string]interface{}:
                options = v
                // Extract core number from options if present
                if coreVal, exists := options["core"]; exists {
                    if coreInt, ok := coreVal.(int); ok {
                        coreNumber = coreInt
                    } else if coreFloat, ok := coreVal.(float64); ok {
                        coreNumber = int(coreFloat)
                    }
                }
            default:
                return nil, errors.New("cpu_info argument must be int (core number) or map (options)")
            }
        }
        return getCPUInfo(coreNumber, options)
    }

    // I/O functions
    slhelp["nio"] = LibHelp{in: "[interface_name|options]", out: "[]NetworkIOStats", action: "Returns network I/O throughput statistics. Options: map(.interface \"eth0\", .include_errors true) or simple string for interface name."}
    stdlib["nio"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) > 1 {
            return nil, errors.New("nio expects 0 or 1 argument (interface name string or options map)")
        }
        var options map[string]interface{}
        if len(args) > 0 {
            switch v := args[0].(type) {
            case string:
                // Simple string argument - treat as interface name
                options = map[string]interface{}{"interface": v}
            case map[string]interface{}:
                options = v
            case map[any]any:
                options = make(map[string]interface{})
                for key, val := range v {
                    if ks, ok := key.(string); ok {
                        options[ks] = val
                    }
                }
            default:
                return nil, errors.New("nio argument must be string (interface name) or map (options)")
            }
        }
        return getNetworkIO(options)
    }

    slhelp["dio"] = LibHelp{in: "[device_name|options]", out: "[]DiskIOStats", action: "Returns disk I/O throughput statistics. Options: map(.device \"sda\") or simple string for device name."}
    stdlib["dio"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if len(args) > 1 {
            return nil, errors.New("dio expects 0 or 1 argument (device name string or options map)")
        }
        var options map[string]interface{}
        if len(args) > 0 {
            switch v := args[0].(type) {
            case string:
                // Simple string argument - treat as device name
                options = map[string]interface{}{"device": v}
            case map[string]interface{}:
                options = v
            case map[any]any:
                options = make(map[string]interface{})
                for key, val := range v {
                    if ks, ok := key.(string); ok {
                        options[ks] = val
                    }
                }
            default:
                return nil, errors.New("dio argument must be string (device name) or map (options)")
            }
        }
        return getDiskIO(options)
    }

    // Disk usage and mount information functions
    slhelp["disk_usage"] = LibHelp{in: "[options]", out: "[]map", action: "Returns filesystem usage information. Options: map(.exclude_patterns [\"tmpfs\", \"proc\"])"}
    stdlib["disk_usage"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        var options map[string]interface{}
        if ok, err := expect_args("disk_usage", args, 2, "1", "map", "0"); !ok {
            return nil, err
        }
        if len(args) > 0 {
            options = args[0].(map[string]interface{})
        }
        return getDiskUsage(options)
    }

    slhelp["mount_info"] = LibHelp{in: "[options]", out: "[]map", action: "Returns mount point information. Options: map(.filesystem \"ext4\")"}
    stdlib["mount_info"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        var options map[string]interface{}
        if ok, err := expect_args("mount_info", args, 2, "1", "map", "0"); !ok {
            return nil, err
        }
        if len(args) > 0 {
            options = args[0].(map[string]interface{})
        }
        return getMountInfo(options)
    }

    // Network device information function
    slhelp["net_devices"] = LibHelp{in: "[options]", out: "[]map", action: "Returns network device information. Options: map(.all true)"}
    stdlib["net_devices"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        var options map[string]interface{}
        if ok, err := expect_args("net_devices", args, 2, "1", "map", "0"); !ok {
            return nil, err
        }
        if len(args) > 0 {
            options = args[0].(map[string]interface{})
        }
        return getNetworkDevices(options)
    }

    // Resource usage and throughput calculation
    slhelp["resource_usage"] = LibHelp{in: "pid", out: "ResourceUsage", action: "Returns resource usage for specific process."}
    stdlib["resource_usage"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("resource_usage", args, 1, "1", "int"); !ok {
            return nil, err
        }
        pid := args[0].(int)
        return getResourceUsage(pid)
    }

    slhelp["iodiff"] = LibHelp{in: "snapshot1,snapshot2,duration", out: "map", action: "Calculates throughput rates between two snapshots."}
    stdlib["iodiff"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("iodiff", args, 3, "3", "ResourceSnapshot", "ResourceSnapshot", "time.Duration"); !ok {
            return nil, err
        }
        snapshot1 := args[0].(ResourceSnapshot)
        snapshot2 := args[1].(ResourceSnapshot)
        duration := args[2].(time.Duration)
        result := calculateIODiff(snapshot1, snapshot2, duration)
        return result, nil
    }

    // Debug function to check available CPU files
    slhelp["debug_cpu_files"] = LibHelp{in: "", out: "map", action: "Debug function to check available frequency and temperature files on the system."}
    stdlib["debug_cpu_files"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("debug_cpu_files", args, 0); !ok {
            return nil, err
        }
        result := debugCPUFiles()
        return result, nil
    }

    // Gateway interface function
    slhelp["gw_interface"] = LibHelp{in: "", out: "string", action: "Returns the name of the default gateway interface."}
    stdlib["gw_interface"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("gw_interface", args, 0); !ok {
            return nil, err
        }
        return getDefaultGatewayInterface()
    }

    // Gateway address function
    slhelp["gw_address"] = LibHelp{in: "", out: "string", action: "Returns the IP address of the default gateway."}
    stdlib["gw_address"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("gw_address", args, 0); !ok {
            return nil, err
        }
        return getDefaultGatewayAddress()
    }

    // Gateway info function
    slhelp["gw_info"] = LibHelp{in: "", out: "map", action: "Returns complete default gateway information including interface name and IP address."}
    stdlib["gw_info"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("gw_info", args, 0); !ok {
            return nil, err
        }
        return getDefaultGatewayInfo()
    }
}
