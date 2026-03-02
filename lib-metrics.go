//go:build !test

package main

import (
    "context"
    "fmt"
    "log"
    "math"
    "net"
    "net/http"
    "os"
    "regexp"
    "runtime"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    "github.com/VictoriaMetrics/metrics"
)

var (
    metricsServer *http.Server
    enableMetrics bool
    metricsAllowCIDRs []*net.IPNet  // nil slice = no CIDRs configured = block all
)

// User metric registry for custom metrics exposed via stdlib
type userMetricKind int

const (
    userMetricCounter userMetricKind = iota
    userMetricGauge
    userMetricSummary
)

type userMetric struct {
    kind    userMetricKind
    counter *metrics.Counter  // counter type
    summary *metrics.Summary  // summary type
    gauge   int64             // atomic float64 bits (math.Float64bits) for gauge type
}

var (
    userMetricsMu sync.Mutex
    userMetrics   = make(map[string]*userMetric)
)

// Default exclusion lists for metrics filtering
var (
    defaultNetExcludePrefixes = []string{
        "lo", "veth", "docker", "br-", "virbr", "tun", "tap",
        "dummy", "cali", "flannel", "cilium", "lxcbr", "vxlan",
    }

    defaultDiskExcludePrefixes = []string{
        "loop", "ram", "fd", "sr", "scd", "zram",
    }

    // Matches partition devices: sda1, nvme0n1p1, mmcblk0p1, etc. (not whole disks)
    partitionRegex = regexp.MustCompile(
        `^(([hsv]|xv)d[a-z]\d+|nvme\d+n\d+p\d+|mmcblk\d+p\d+)$`,
    )

    defaultFSExcludeTypes = map[string]bool{
        "tmpfs": true, "devtmpfs": true, "sysfs": true, "proc": true,
        "procfs": true, "cgroup": true, "cgroup2": true, "overlay": true,
        "squashfs": true, "devpts": true, "hugetlbfs": true, "mqueue": true,
        "bpf": true, "tracefs": true, "debugfs": true, "securityfs": true,
        "configfs": true, "pstore": true, "iso9660": true, "autofs": true,
        "binfmt_misc": true, "nsfs": true, "rpc_pipefs": true,
        "selinuxfs": true, "fusectl": true, "fuse.gvfsd-fuse": true,
    }
)

// Runtime vars for metrics filtering (populated at startup, nil = use default exclusions)
var (
    metricsNetExcludePrefixes    []string
    metricsDiskExcludePrefixes   []string
    metricsDiskExcludePartitions bool
    metricsFSExcludeTypes        map[string]bool
    metricsNetIncludePatterns    []string        // non-nil → allowlist mode
    metricsDiskIncludePatterns   []string        // non-nil → allowlist mode
    metricsFSIncludeTypes        map[string]bool // non-nil → allowlist mode
)

// splitTrimmed splits a string on commas and trims whitespace from each part.
func splitTrimmed(s string) []string {
    var result []string
    for _, part := range strings.Split(s, ",") {
        if part = strings.TrimSpace(part); part != "" {
            result = append(result, part)
        }
    }
    return result
}

// getDefaultInterfaceIP returns the IP of the interface used for outbound traffic
// by dialing a UDP address (no actual connection is made).
func getDefaultInterfaceIP() string {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        return "0.0.0.0"
    }
    defer conn.Close()
    return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// isNetExcluded returns true if a network interface should be excluded from metrics.
// Explicit includes override exclusions.
func isNetExcluded(iface string) bool {
    // Check if explicitly included (overrides exclusions)
    if metricsNetIncludePatterns != nil {
        for _, pat := range metricsNetIncludePatterns {
            if iface == pat || strings.HasPrefix(iface, pat) {
                return false  // Explicitly included, so not excluded
            }
        }
    }
    // Apply default exclusions
    for _, prefix := range metricsNetExcludePrefixes {
        if iface == prefix || strings.HasPrefix(iface, prefix) {
            return true
        }
    }
    return false
}

// isDiskExcluded returns true if a disk device should be excluded from metrics.
// Explicit includes override exclusions.
func isDiskExcluded(device string) bool {
    // Check if explicitly included (overrides exclusions)
    if metricsDiskIncludePatterns != nil {
        for _, pat := range metricsDiskIncludePatterns {
            if device == pat || strings.HasPrefix(device, pat) {
                return false  // Explicitly included, so not excluded
            }
        }
    }
    // Apply default exclusions
    for _, prefix := range metricsDiskExcludePrefixes {
        if strings.HasPrefix(device, prefix) {
            return true
        }
    }
    return metricsDiskExcludePartitions && partitionRegex.MatchString(device)
}

// isFSExcluded returns true if a filesystem type should be excluded from metrics.
// Explicit includes override exclusions.
func isFSExcluded(fsType string) bool {
    // Check if explicitly included (overrides exclusions)
    if metricsFSIncludeTypes != nil && metricsFSIncludeTypes[fsType] {
        return false  // Explicitly included, so not excluded
    }
    // Apply default exclusions
    return metricsFSExcludeTypes[fsType]
}

// startMetricsServer initializes and starts the Prometheus metrics HTTP server.
// It registers all gauge-with-callback metrics and starts listening on the specified port.
// allowCIDRs is a comma-separated list of CIDR blocks to allow; if empty, all requests are blocked.
// bindAddr is the address to bind to ("" → "0.0.0.0", "auto" → default interface IP, or specific IP/hostname).
func startMetricsServer(port int, allowCIDRs string, bindAddr string) {
    // Initialize filtering variables
    metricsNetExcludePrefixes = defaultNetExcludePrefixes
    if v := os.Getenv("ZA_PROMETHEUS_NET_INCLUDE"); v != "" {
        metricsNetIncludePatterns = splitTrimmed(v)
    }

    metricsDiskExcludePrefixes = defaultDiskExcludePrefixes
    metricsDiskExcludePartitions = true
    if v := os.Getenv("ZA_PROMETHEUS_DISK_INCLUDE"); v != "" {
        metricsDiskIncludePatterns = splitTrimmed(v)
    }

    metricsFSExcludeTypes = defaultFSExcludeTypes
    if v := os.Getenv("ZA_PROMETHEUS_FS_INCLUDE"); v != "" {
        metricsFSIncludeTypes = make(map[string]bool)
        for _, t := range splitTrimmed(v) {
            metricsFSIncludeTypes[t] = true
        }
    }

    // Parse and validate CIDR allowlist
    metricsAllowCIDRs = nil  // reset
    for _, s := range strings.Split(allowCIDRs, ",") {
        s = strings.TrimSpace(s)
        if s == "" {
            continue
        }
        _, cidr, err := net.ParseCIDR(s)
        if err != nil {
            log.Printf("[za] metrics: invalid CIDR %q: %v (skipping)", s, err)
            continue
        }
        metricsAllowCIDRs = append(metricsAllowCIDRs, cidr)

        // Auto-expand IPv4 CIDRs to IPv6 equivalents for dual-stack support
        if cidr.IP.To4() != nil {
            switch s {
            case "0.0.0.0/0":
                // Allow all IPv4 → also allow all IPv6
                if _, ipv6All, _ := net.ParseCIDR("::/0"); ipv6All != nil {
                    metricsAllowCIDRs = append(metricsAllowCIDRs, ipv6All)
                }
            case "127.0.0.1/32":
                // IPv4 loopback → also allow IPv6 loopback
                if _, ipv6Loopback, _ := net.ParseCIDR("::1/128"); ipv6Loopback != nil {
                    metricsAllowCIDRs = append(metricsAllowCIDRs, ipv6Loopback)
                }
            }
        }
    }

    registerRuntimeGauges()
    registerSystemGauges()
    registerProcessGauges()
    registerBuildInfo()
    registerFFIInventory()
    registerWebGauges()
    registerErrorChainGauge()
    registerLoggingGauges()

    // Determine bind address
    switch bindAddr {
    case "":
        bindAddr = "0.0.0.0"
    case "auto":
        bindAddr = getDefaultInterfaceIP()
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        metrics.WritePrometheus(w, false)
    })
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, "/metrics", http.StatusFound)
    })

    metricsServer = &http.Server{
        Addr:    fmt.Sprintf("%s:%d", bindAddr, port),
        Handler: metricsCIDRMiddleware(mux),
    }

    // Log effective configuration
    if len(metricsAllowCIDRs) == 0 {
        log.Printf("[za] metrics server on %s:%d: no CIDR set — all requests blocked", bindAddr, port)
    } else {
        var cidrs []string
        for _, cidr := range metricsAllowCIDRs {
            cidrs = append(cidrs, cidr.String())
        }
        log.Printf("[za] metrics server on %s:%d: allowing: %s", bindAddr, port, strings.Join(cidrs, ", "))
    }

    go func() {
        if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Printf("[za] metrics server error: %v", err)
        }
    }()
}

// stopMetricsServer gracefully shuts down the metrics server.
func stopMetricsServer() {
    if metricsServer != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()
        metricsServer.Shutdown(ctx)
    }
}

// metricsCIDRMiddleware wraps an HTTP handler to enforce CIDR-based IP filtering.
func metricsCIDRMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !metricsIPAllowed(r.RemoteAddr) {
            log.Printf("[za] metrics: rejected %s", r.RemoteAddr)
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// metricsIPAllowed checks if the given remote address is allowed by the CIDR allowlist.
// Returns true if the IP matches any allowed CIDR, or false if not configured / not matched.
func metricsIPAllowed(remoteAddr string) bool {
    host, _, err := net.SplitHostPort(remoteAddr)
    if err != nil {
        host = remoteAddr
    }
    ip := net.ParseIP(host)
    if ip == nil {
        return false
    }
    for _, cidr := range metricsAllowCIDRs {
        if cidr.Contains(ip) {
            return true
        }
    }
    return false
}

// ============================================================================
// SYSTEM GAUGES
// ============================================================================

var (
    memInfoCache struct {
        mu   sync.Mutex
        last time.Time
        info MemoryInfo
    }
    systemLoadCache struct {
        mu   sync.Mutex
        last time.Time
        load []float64
    }
    resourceUsageCache struct {
        mu   sync.Mutex
        last time.Time
        info ResourceUsage
    }
    networkIOCache struct {
        mu   sync.Mutex
        last time.Time
        data []NetworkIOStats
    }
    diskIOCache struct {
        mu   sync.Mutex
        last time.Time
        data []DiskIOStats
    }
    diskUsageCache struct {
        mu   sync.Mutex
        last time.Time
        data []map[string]interface{}
    }
    runtimeMemStatsCache struct {
        mu   sync.Mutex
        last time.Time
        ms   runtime.MemStats
    }
    systemStatsCache struct {
        mu            sync.Mutex
        last          time.Time
        contextSwitches uint64
        interrupts      uint64
        bootTime        int64
        fdAllocated     uint64
        fdMax           uint64
    }
    processStartTime = time.Now()
)

func cachedMemInfo() MemoryInfo {
    memInfoCache.mu.Lock()
    defer memInfoCache.mu.Unlock()
    if time.Since(memInfoCache.last) > time.Second {
        memInfoCache.info, _ = getMemoryInfo()
        memInfoCache.last = time.Now()
    }
    return memInfoCache.info
}

func cachedSystemLoad() []float64 {
    systemLoadCache.mu.Lock()
    defer systemLoadCache.mu.Unlock()
    if time.Since(systemLoadCache.last) > time.Second {
        systemLoadCache.load, _ = getSystemLoad()
        systemLoadCache.last = time.Now()
    }
    return systemLoadCache.load
}

func cachedResourceUsage() ResourceUsage {
    resourceUsageCache.mu.Lock()
    defer resourceUsageCache.mu.Unlock()
    if time.Since(resourceUsageCache.last) > time.Second {
        resourceUsageCache.info, _ = getResourceUsage(os.Getpid())
        resourceUsageCache.last = time.Now()
    }
    return resourceUsageCache.info
}

func cachedNetworkIO() []NetworkIOStats {
    networkIOCache.mu.Lock()
    defer networkIOCache.mu.Unlock()
    if time.Since(networkIOCache.last) > time.Second {
        networkIOCache.data, _ = getNetworkIO(nil)
        networkIOCache.last = time.Now()
    }
    return networkIOCache.data
}

func cachedDiskIO() []DiskIOStats {
    diskIOCache.mu.Lock()
    defer diskIOCache.mu.Unlock()
    if time.Since(diskIOCache.last) > time.Second {
        diskIOCache.data, _ = getDiskIO(nil)
        diskIOCache.last = time.Now()
    }
    return diskIOCache.data
}

func cachedDiskUsage() []map[string]interface{} {
    diskUsageCache.mu.Lock()
    defer diskUsageCache.mu.Unlock()
    if time.Since(diskUsageCache.last) > time.Second {
        diskUsageCache.data, _ = getDiskUsage(nil)
        diskUsageCache.last = time.Now()
    }
    return diskUsageCache.data
}

func cachedMemStats() runtime.MemStats {
    runtimeMemStatsCache.mu.Lock()
    defer runtimeMemStatsCache.mu.Unlock()
    if time.Since(runtimeMemStatsCache.last) > time.Second {
        runtime.ReadMemStats(&runtimeMemStatsCache.ms)
        runtimeMemStatsCache.last = time.Now()
    }
    return runtimeMemStatsCache.ms
}

func cachedSystemStats() (contextSwitches, interrupts uint64, bootTime int64, fdAllocated, fdMax uint64) {
    systemStatsCache.mu.Lock()
    defer systemStatsCache.mu.Unlock()
    if time.Since(systemStatsCache.last) > time.Second {
        systemStatsCache.contextSwitches = getContextSwitches()
        systemStatsCache.interrupts = getInterrupts()
        systemStatsCache.bootTime = getSystemBootTime()
        systemStatsCache.fdAllocated, systemStatsCache.fdMax = getSystemFileDescriptorStats()
        systemStatsCache.last = time.Now()
    }
    return systemStatsCache.contextSwitches, systemStatsCache.interrupts, systemStatsCache.bootTime, systemStatsCache.fdAllocated, systemStatsCache.fdMax
}

func registerRuntimeGauges() {
    // Go runtime heap metrics
    metrics.NewGauge(`za_runtime_heap_alloc_bytes`, func() float64 {
        return float64(cachedMemStats().HeapAlloc)
    })
    metrics.NewGauge(`za_runtime_heap_sys_bytes`, func() float64 {
        return float64(cachedMemStats().HeapSys)
    })
    metrics.NewGauge(`za_runtime_heap_idle_bytes`, func() float64 {
        return float64(cachedMemStats().HeapIdle)
    })
    metrics.NewGauge(`za_runtime_heap_inuse_bytes`, func() float64 {
        return float64(cachedMemStats().HeapInuse)
    })
    metrics.NewGauge(`za_runtime_heap_released_bytes`, func() float64 {
        return float64(cachedMemStats().HeapReleased)
    })
    metrics.NewGauge(`za_runtime_heap_objects`, func() float64 {
        return float64(cachedMemStats().HeapObjects)
    })
    metrics.NewGauge(`za_runtime_sys_bytes`, func() float64 {
        return float64(cachedMemStats().Sys)
    })
    metrics.NewGauge(`za_runtime_next_gc_bytes`, func() float64 {
        return float64(cachedMemStats().NextGC)
    })
    metrics.NewGauge(`za_runtime_gc_cpu_fraction`, func() float64 {
        return cachedMemStats().GCCPUFraction
    })
    metrics.NewGauge(`za_runtime_mallocs_total`, func() float64 {
        return float64(cachedMemStats().Mallocs)
    })
    metrics.NewGauge(`za_runtime_frees_total`, func() float64 {
        return float64(cachedMemStats().Frees)
    })

    // Go runtime GC metrics
    metrics.NewGauge(`za_runtime_gc_cycles_total`, func() float64 {
        return float64(cachedMemStats().NumGC)
    })
    metrics.NewGauge(`za_runtime_gc_pause_last_ns`, func() float64 {
        ms := cachedMemStats()
        if ms.NumGC > 0 {
            return float64(ms.PauseNs[(ms.NumGC+255)%256])
        }
        return 0
    })
    metrics.NewGauge(`za_runtime_gc_pause_total_ns`, func() float64 {
        return float64(cachedMemStats().PauseTotalNs)
    })
}

func registerSystemGauges() {
    registerNetworkGauges()
    registerDiskIOGauges()
    registerDiskUsageGauges()

    // System CPU count
    metrics.NewGauge(`za_system_cpu_count`, func() float64 {
        return float64(runtime.NumCPU())
    })

    // System boot time (Priority 4)
    metrics.NewGauge(`za_system_boot_time_seconds`, func() float64 {
        _, _, bootTime, _, _ := cachedSystemStats()
        return float64(bootTime)
    })

    // System context switches (Priority 4)
    metrics.NewGauge(`za_system_context_switches_total`, func() float64 {
        ctxt, _, _, _, _ := cachedSystemStats()
        return float64(ctxt)
    })

    // System interrupts (Priority 4)
    metrics.NewGauge(`za_system_interrupts_total`, func() float64 {
        _, intr, _, _, _ := cachedSystemStats()
        return float64(intr)
    })

    // System file descriptor limits (Priority 4)
    metrics.NewGauge(`za_system_filefd_allocated`, func() float64 {
        _, _, _, allocated, _ := cachedSystemStats()
        return float64(allocated)
    })
    metrics.NewGauge(`za_system_filefd_maximum`, func() float64 {
        _, _, _, _, max := cachedSystemStats()
        return float64(max)
    })

    // Load average
    metrics.NewGauge(`za_system_load_average{interval="1m"}`, func() float64 {
        load := cachedSystemLoad()
        if len(load) > 0 {
            return load[0]
        }
        return 0
    })
    metrics.NewGauge(`za_system_load_average{interval="5m"}`, func() float64 {
        load := cachedSystemLoad()
        if len(load) > 1 {
            return load[1]
        }
        return 0
    })
    metrics.NewGauge(`za_system_load_average{interval="15m"}`, func() float64 {
        load := cachedSystemLoad()
        if len(load) > 2 {
            return load[2]
        }
        return 0
    })

    // Memory
    metrics.NewGauge(`za_system_memory_bytes{type="total"}`, func() float64 {
        return float64(cachedMemInfo().Total)
    })
    metrics.NewGauge(`za_system_memory_bytes{type="used"}`, func() float64 {
        return float64(cachedMemInfo().Used)
    })
    metrics.NewGauge(`za_system_memory_bytes{type="free"}`, func() float64 {
        return float64(cachedMemInfo().Free)
    })
    metrics.NewGauge(`za_system_memory_bytes{type="cached"}`, func() float64 {
        return float64(cachedMemInfo().Cached)
    })
    metrics.NewGauge(`za_system_memory_bytes{type="buffers"}`, func() float64 {
        return float64(cachedMemInfo().Buffers)
    })

    // Swap
    metrics.NewGauge(`za_system_swap_bytes{type="total"}`, func() float64 {
        return float64(cachedMemInfo().SwapTotal)
    })
    metrics.NewGauge(`za_system_swap_bytes{type="used"}`, func() float64 {
        return float64(cachedMemInfo().SwapUsed)
    })
    metrics.NewGauge(`za_system_swap_bytes{type="free"}`, func() float64 {
        return float64(cachedMemInfo().SwapFree)
    })
}

// ============================================================================
// PROCESS GAUGES
// ============================================================================

func registerProcessGauges() {
    metrics.NewGauge(`za_process_cpu_seconds_total{mode="user"}`, func() float64 {
        return cachedResourceUsage().CPUUser
    })
    metrics.NewGauge(`za_process_cpu_seconds_total{mode="system"}`, func() float64 {
        return cachedResourceUsage().CPUSystem
    })
    metrics.NewGauge(`za_process_memory_bytes{type="rss"}`, func() float64 {
        return float64(cachedResourceUsage().MemoryCurrent)
    })
    metrics.NewGauge(`za_process_memory_bytes{type="peak"}`, func() float64 {
        return float64(cachedResourceUsage().MemoryPeak)
    })
    metrics.NewGauge(`za_process_io_bytes_total{direction="read"}`, func() float64 {
        return float64(cachedResourceUsage().IOReadBytes)
    })
    metrics.NewGauge(`za_process_io_bytes_total{direction="write"}`, func() float64 {
        return float64(cachedResourceUsage().IOWriteBytes)
    })
    metrics.NewGauge(`za_process_threads`, func() float64 {
        info, _ := getProcessInfo(os.Getpid(), nil)
        return float64(info.Threads)
    })
    metrics.NewGauge(`za_runtime_goroutines`, func() float64 {
        return float64(runtime.NumGoroutine())
    })
    metrics.NewGauge(`za_concurrent_funcs`, func() float64 {
        return float64(atomic.LoadInt32(&concurrent_funcs))
    })

    // Process lifecycle metrics
    metrics.NewGauge(`za_process_start_time_seconds`, func() float64 {
        return float64(processStartTime.Unix())
    })
    metrics.NewGauge(`za_process_uptime_seconds`, func() float64 {
        return time.Since(processStartTime).Seconds()
    })

    // Process file descriptor metrics
    metrics.NewGauge(`za_process_open_fds`, func() float64 {
        return float64(getOpenFDs())
    })
    metrics.NewGauge(`za_process_max_fds`, func() float64 {
        return float64(getMaxFDs())
    })
}

// ============================================================================
// BUILD INFO GAUGE
// ============================================================================

func registerBuildInfo() {
    metrics.NewGauge(
        fmt.Sprintf(`za_build_info{version=%q,build_date=%q,comment=%q}`,
            BuildVersion, BuildDate, BuildComment),
        func() float64 { return 1 },
    )
}

// ============================================================================
// FFI INVENTORY GAUGES
// ============================================================================

var ffiDeclaredGaugesRegistered = &sync.Map{} // tracks which library aliases have been registered

// ============================================================================
// NETWORK I/O GAUGES
// ============================================================================

var networkGaugesRegistered = &sync.Map{} // tracks which network interfaces have been registered

func registerNetworkGauges() {
    const sentinel uint64 = 0xFFFFFFFFFFFFFFFF
    metrics.NewGauge(`za_system_network_bytes_total{interface="aggregated",direction="rx"}`, func() float64 {
        var total uint64
        for _, stats := range cachedNetworkIO() {
            if isNetExcluded(stats.Interface) || stats.RxBytes == sentinel || stats.TxBytes == sentinel {
                continue
            }
            total += stats.RxBytes
            // Lazy register per-interface gauges
            if _, exists := networkGaugesRegistered.LoadOrStore(stats.Interface+"_rx_bytes", true); !exists {
                iface := stats.Interface
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_network_bytes_total{interface=%q,direction="rx"}`, iface),
                    func() float64 {
                        for _, s := range cachedNetworkIO() {
                            if s.Interface == iface && s.RxBytes != sentinel && s.TxBytes != sentinel {
                                return float64(s.RxBytes)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_network_bytes_total{interface="aggregated",direction="tx"}`, func() float64 {
        var total uint64
        for _, stats := range cachedNetworkIO() {
            if isNetExcluded(stats.Interface) || stats.RxBytes == sentinel || stats.TxBytes == sentinel {
                continue
            }
            total += stats.TxBytes
            // Lazy register per-interface gauges
            if _, exists := networkGaugesRegistered.LoadOrStore(stats.Interface+"_tx_bytes", true); !exists {
                iface := stats.Interface
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_network_bytes_total{interface=%q,direction="tx"}`, iface),
                    func() float64 {
                        for _, s := range cachedNetworkIO() {
                            if s.Interface == iface && s.RxBytes != sentinel && s.TxBytes != sentinel {
                                return float64(s.TxBytes)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_network_packets_total{interface="aggregated",direction="rx"}`, func() float64 {
        var total uint64
        for _, stats := range cachedNetworkIO() {
            if isNetExcluded(stats.Interface) {
                continue
            }
            total += stats.RxPackets
            // Lazy register per-interface gauges
            if _, exists := networkGaugesRegistered.LoadOrStore(stats.Interface+"_rx_packets", true); !exists {
                iface := stats.Interface
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_network_packets_total{interface=%q,direction="rx"}`, iface),
                    func() float64 {
                        for _, s := range cachedNetworkIO() {
                            if s.Interface == iface && !isNetExcluded(iface) {
                                return float64(s.RxPackets)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_network_packets_total{interface="aggregated",direction="tx"}`, func() float64 {
        var total uint64
        for _, stats := range cachedNetworkIO() {
            if isNetExcluded(stats.Interface) {
                continue
            }
            total += stats.TxPackets
            // Lazy register per-interface gauges
            if _, exists := networkGaugesRegistered.LoadOrStore(stats.Interface+"_tx_packets", true); !exists {
                iface := stats.Interface
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_network_packets_total{interface=%q,direction="tx"}`, iface),
                    func() float64 {
                        for _, s := range cachedNetworkIO() {
                            if s.Interface == iface && !isNetExcluded(iface) {
                                return float64(s.TxPackets)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_network_errors_total{interface="aggregated",direction="rx"}`, func() float64 {
        var total uint64
        for _, stats := range cachedNetworkIO() {
            if isNetExcluded(stats.Interface) {
                continue
            }
            total += stats.RxErrors
            // Lazy register per-interface gauges
            if _, exists := networkGaugesRegistered.LoadOrStore(stats.Interface+"_rx_errors", true); !exists {
                iface := stats.Interface
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_network_errors_total{interface=%q,direction="rx"}`, iface),
                    func() float64 {
                        for _, s := range cachedNetworkIO() {
                            if s.Interface == iface && !isNetExcluded(iface) {
                                return float64(s.RxErrors)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_network_errors_total{interface="aggregated",direction="tx"}`, func() float64 {
        var total uint64
        for _, stats := range cachedNetworkIO() {
            if isNetExcluded(stats.Interface) {
                continue
            }
            total += stats.TxErrors
            // Lazy register per-interface gauges
            if _, exists := networkGaugesRegistered.LoadOrStore(stats.Interface+"_tx_errors", true); !exists {
                iface := stats.Interface
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_network_errors_total{interface=%q,direction="tx"}`, iface),
                    func() float64 {
                        for _, s := range cachedNetworkIO() {
                            if s.Interface == iface && !isNetExcluded(iface) {
                                return float64(s.TxErrors)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_network_dropped_total{interface="aggregated",direction="rx"}`, func() float64 {
        var total uint64
        for _, stats := range cachedNetworkIO() {
            if isNetExcluded(stats.Interface) {
                continue
            }
            total += stats.RxDropped
            // Lazy register per-interface gauges
            if _, exists := networkGaugesRegistered.LoadOrStore(stats.Interface+"_rx_dropped", true); !exists {
                iface := stats.Interface
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_network_dropped_total{interface=%q,direction="rx"}`, iface),
                    func() float64 {
                        for _, s := range cachedNetworkIO() {
                            if s.Interface == iface && !isNetExcluded(iface) {
                                return float64(s.RxDropped)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_network_dropped_total{interface="aggregated",direction="tx"}`, func() float64 {
        var total uint64
        for _, stats := range cachedNetworkIO() {
            if isNetExcluded(stats.Interface) {
                continue
            }
            total += stats.TxDropped
            // Lazy register per-interface gauges
            if _, exists := networkGaugesRegistered.LoadOrStore(stats.Interface+"_tx_dropped", true); !exists {
                iface := stats.Interface
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_network_dropped_total{interface=%q,direction="tx"}`, iface),
                    func() float64 {
                        for _, s := range cachedNetworkIO() {
                            if s.Interface == iface && !isNetExcluded(iface) {
                                return float64(s.TxDropped)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })
}

// ============================================================================
// DISK I/O GAUGES
// ============================================================================

var diskIOGaugesRegistered = &sync.Map{} // tracks which disk devices have been registered

func registerDiskIOGauges() {
    metrics.NewGauge(`za_system_disk_bytes_total{device="aggregated",direction="read"}`, func() float64 {
        var total uint64
        for _, stats := range cachedDiskIO() {
            if isDiskExcluded(stats.Device) {
                continue
            }
            total += stats.ReadBytes
            // Lazy register per-device gauges
            if _, exists := diskIOGaugesRegistered.LoadOrStore(stats.Device+"_read_bytes", true); !exists {
                dev := stats.Device
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_disk_bytes_total{device=%q,direction="read"}`, dev),
                    func() float64 {
                        for _, s := range cachedDiskIO() {
                            if s.Device == dev && !isDiskExcluded(dev) {
                                return float64(s.ReadBytes)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_disk_bytes_total{device="aggregated",direction="write"}`, func() float64 {
        var total uint64
        for _, stats := range cachedDiskIO() {
            if isDiskExcluded(stats.Device) {
                continue
            }
            total += stats.WriteBytes
            // Lazy register per-device gauges
            if _, exists := diskIOGaugesRegistered.LoadOrStore(stats.Device+"_write_bytes", true); !exists {
                dev := stats.Device
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_disk_bytes_total{device=%q,direction="write"}`, dev),
                    func() float64 {
                        for _, s := range cachedDiskIO() {
                            if s.Device == dev && !isDiskExcluded(dev) {
                                return float64(s.WriteBytes)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_disk_ops_total{device="aggregated",direction="read"}`, func() float64 {
        var total uint64
        for _, stats := range cachedDiskIO() {
            if isDiskExcluded(stats.Device) {
                continue
            }
            total += stats.ReadOps
            // Lazy register per-device gauges
            if _, exists := diskIOGaugesRegistered.LoadOrStore(stats.Device+"_read_ops", true); !exists {
                dev := stats.Device
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_disk_ops_total{device=%q,direction="read"}`, dev),
                    func() float64 {
                        for _, s := range cachedDiskIO() {
                            if s.Device == dev && !isDiskExcluded(dev) {
                                return float64(s.ReadOps)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_disk_ops_total{device="aggregated",direction="write"}`, func() float64 {
        var total uint64
        for _, stats := range cachedDiskIO() {
            if isDiskExcluded(stats.Device) {
                continue
            }
            total += stats.WriteOps
            // Lazy register per-device gauges
            if _, exists := diskIOGaugesRegistered.LoadOrStore(stats.Device+"_write_ops", true); !exists {
                dev := stats.Device
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_disk_ops_total{device=%q,direction="write"}`, dev),
                    func() float64 {
                        for _, s := range cachedDiskIO() {
                            if s.Device == dev && !isDiskExcluded(dev) {
                                return float64(s.WriteOps)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_disk_time_ms{device="aggregated",direction="read"}`, func() float64 {
        var total uint64
        for _, stats := range cachedDiskIO() {
            if isDiskExcluded(stats.Device) {
                continue
            }
            total += stats.ReadTime
            // Lazy register per-device gauges
            if _, exists := diskIOGaugesRegistered.LoadOrStore(stats.Device+"_read_time", true); !exists {
                dev := stats.Device
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_disk_time_ms{device=%q,direction="read"}`, dev),
                    func() float64 {
                        for _, s := range cachedDiskIO() {
                            if s.Device == dev && !isDiskExcluded(dev) {
                                return float64(s.ReadTime)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })

    metrics.NewGauge(`za_system_disk_time_ms{device="aggregated",direction="write"}`, func() float64 {
        var total uint64
        for _, stats := range cachedDiskIO() {
            if isDiskExcluded(stats.Device) {
                continue
            }
            total += stats.WriteTime
            // Lazy register per-device gauges
            if _, exists := diskIOGaugesRegistered.LoadOrStore(stats.Device+"_write_time", true); !exists {
                dev := stats.Device
                metrics.NewGauge(
                    fmt.Sprintf(`za_system_disk_time_ms{device=%q,direction="write"}`, dev),
                    func() float64 {
                        for _, s := range cachedDiskIO() {
                            if s.Device == dev && !isDiskExcluded(dev) {
                                return float64(s.WriteTime)
                            }
                        }
                        return 0
                    },
                )
            }
        }
        return float64(total)
    })
}

// ============================================================================
// DISK USAGE GAUGES
// ============================================================================

var diskUsageGaugesRegistered = &sync.Map{} // tracks which mount points have been registered

func registerDiskUsageGauges() {
    metrics.NewGauge(`za_system_disk_usage_bytes{mount_point="total",type="total"}`, func() float64 {
        var total uint64
        for _, usage := range cachedDiskUsage() {
            fsType, _ := usage["fstype"].(string)
            if isFSExcluded(fsType) {
                continue
            }
            if size, ok := usage["size"].(uint64); ok {
                total += size
            } else if size, ok := usage["size"].(float64); ok {
                total += uint64(size)
            }
            // Lazy register per-mount-point gauges
            if mp, ok := usage["mounted_path"].(string); ok {
                if _, exists := diskUsageGaugesRegistered.LoadOrStore(mp+"_total", true); !exists {
                    mountPoint := mp
                    // Total bytes gauge
                    metrics.NewGauge(
                        fmt.Sprintf(`za_system_disk_usage_bytes{mount_point=%q,type="total"}`, mountPoint),
                        func() float64 {
                            for _, u := range cachedDiskUsage() {
                                if mp2, ok := u["mounted_path"].(string); ok && mp2 == mountPoint {
                                    fsType2, _ := u["fstype"].(string)
                                    if !isFSExcluded(fsType2) {
                                        if size, ok := u["size"].(uint64); ok {
                                            return float64(size)
                                        } else if size, ok := u["size"].(float64); ok {
                                            return size
                                        }
                                    }
                                }
                            }
                            return 0
                        },
                    )
                    // Used bytes gauge
                    metrics.NewGauge(
                        fmt.Sprintf(`za_system_disk_usage_bytes{mount_point=%q,type="used"}`, mountPoint),
                        func() float64 {
                            for _, u := range cachedDiskUsage() {
                                if mp2, ok := u["mounted_path"].(string); ok && mp2 == mountPoint {
                                    fsType2, _ := u["fstype"].(string)
                                    if !isFSExcluded(fsType2) {
                                        if used, ok := u["used"].(uint64); ok {
                                            return float64(used)
                                        } else if used, ok := u["used"].(float64); ok {
                                            return used
                                        }
                                    }
                                }
                            }
                            return 0
                        },
                    )
                    // Available bytes gauge
                    metrics.NewGauge(
                        fmt.Sprintf(`za_system_disk_usage_bytes{mount_point=%q,type="available"}`, mountPoint),
                        func() float64 {
                            for _, u := range cachedDiskUsage() {
                                if mp2, ok := u["mounted_path"].(string); ok && mp2 == mountPoint {
                                    fsType2, _ := u["fstype"].(string)
                                    if !isFSExcluded(fsType2) {
                                        if avail, ok := u["available"].(uint64); ok {
                                            return float64(avail)
                                        } else if avail, ok := u["available"].(float64); ok {
                                            return avail
                                        }
                                    }
                                }
                            }
                            return 0
                        },
                    )
                    // Usage percent gauge
                    metrics.NewGauge(
                        fmt.Sprintf(`za_system_disk_usage_percent{mount_point=%q}`, mountPoint),
                        func() float64 {
                            for _, u := range cachedDiskUsage() {
                                if mp2, ok := u["mounted_path"].(string); ok && mp2 == mountPoint {
                                    fsType2, _ := u["fstype"].(string)
                                    if !isFSExcluded(fsType2) {
                                        if pct, ok := u["usage_percent"].(float64); ok {
                                            return pct
                                        }
                                    }
                                }
                            }
                            return 0
                        },
                    )
                }
            }
        }
        return float64(total)
    })
}

func registerFFIInventory() {
    metrics.NewGauge(`za_ffi_loaded_libraries`, func() float64 {
        return float64(len(loadedCLibraries))
    })
    metrics.NewGauge(`za_ffi_active_callbacks`, func() float64 {
        return float64(getActiveCallbackCount())
    })
}

// registerFFIDeclaredFunctions registers a gauge for declared functions in a library.
// Should be called dynamically when libraries are loaded.
func registerFFIDeclaredFunctions(alias string, countFn func() int) {
    if _, exists := ffiDeclaredGaugesRegistered.LoadOrStore(alias, true); !exists {
        metrics.NewGauge(
            fmt.Sprintf(`za_ffi_declared_functions{library=%q}`, alias),
            func() float64 {
                return float64(countFn())
            },
        )
    }
}

// ============================================================================
// WEB GAUGES
// ============================================================================

func registerWebGauges() {
    metrics.NewGauge(`za_web_active_servers`, func() float64 {
        weblock.RLock()
        defer weblock.RUnlock()
        return float64(len(web_handles))
    })
    metrics.NewGauge(`za_web_active_requests`, func() float64 {
        return float64(atomic.LoadInt32(&web_active_requests))
    })
}

// ============================================================================
// ERROR CHAIN GAUGE
// ============================================================================

func registerErrorChainGauge() {
    metrics.NewGauge(`za_error_chain_depth`, func() float64 {
        calllock.RLock()
        defer calllock.RUnlock()
        return float64(len(errorChain))
    })
}

// ============================================================================
// LOGGING GAUGES
// ============================================================================

var (
    logDropCount      int64
    logLevelCount     [8]int64 // index = RFC 5424 level (0=EMERG…7=DEBUG)
)

func registerLoggingGauges() {
    levelNames := []string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}
    for i, name := range levelNames {
        i, name := i, name // capture loop vars
        metrics.NewGauge(
            fmt.Sprintf(`za_log_messages_by_level_total{level=%q}`, name),
            func() float64 { return float64(atomic.LoadInt64(&logLevelCount[i])) },
        )
    }
    metrics.NewGauge(`za_log_queue_depth`, func() float64 {
        used, _, _, _, _ := getLogQueueStats()
        return float64(used)
    })
    metrics.NewGauge(`za_log_queue_capacity`, func() float64 {
        _, total, _, _, _ := getLogQueueStats()
        return float64(total)
    })
    metrics.NewGauge(`za_log_worker_running`, func() float64 {
        _, _, running, _, _ := getLogQueueStats()
        if running {
            return 1
        }
        return 0
    })
    metrics.NewGauge(`za_log_messages_total{dest="main"}`, func() float64 {
        _, _, _, _, mainMessages := getLogQueueStats()
        return float64(mainMessages)
    })
    metrics.NewGauge(`za_log_messages_total{dest="web"}`, func() float64 {
        _, _, _, webMessages, _ := getLogQueueStats()
        return float64(webMessages)
    })
    metrics.NewGauge(`za_log_drops_total`, func() float64 {
        return float64(atomic.LoadInt64(&logDropCount))
    })
}

// ============================================================================
// USER METRIC MANAGEMENT
// ============================================================================

// userMetricRegister registers a new user-defined metric.
// Returns true on success, false if already registered or on error.
func userMetricRegister(name, kind string) bool {
    if !enableMetrics {
        return false
    }

    var k userMetricKind
    switch kind {
    case "counter":
        k = userMetricCounter
    case "gauge":
        k = userMetricGauge
    case "summary":
        k = userMetricSummary
    default:
        return false
    }

    userMetricsMu.Lock()
    defer userMetricsMu.Unlock()

    if _, exists := userMetrics[name]; exists {
        return false // already registered
    }

    entry := &userMetric{kind: k}

    switch k {
    case userMetricCounter:
        entry.counter = metrics.GetOrCreateCounter(name)
    case userMetricGauge:
        entry.gauge = 0
        metrics.NewGauge(name, func() float64 {
            return math.Float64frombits(uint64(atomic.LoadInt64(&entry.gauge)))
        })
    case userMetricSummary:
        entry.summary = metrics.GetOrCreateSummary(name)
    }

    userMetrics[name] = entry
    return true
}

// userMetricDeregister deregisters and removes a user-defined metric.
// Returns true on success, false if not found.
func userMetricDeregister(name string) bool {
    if !enableMetrics {
        return false
    }

    userMetricsMu.Lock()
    defer userMetricsMu.Unlock()

    if _, exists := userMetrics[name]; !exists {
        return false
    }

    delete(userMetrics, name)
    metrics.UnregisterMetric(name)
    return true
}

// userMetricInc increments a counter metric by 1.
// Returns false if metric not found or wrong type.
func userMetricInc(name string) bool {
    if !enableMetrics {
        return false
    }

    userMetricsMu.Lock()
    entry, exists := userMetrics[name]
    userMetricsMu.Unlock()

    if !exists || entry.kind != userMetricCounter {
        return false
    }

    entry.counter.Inc()
    return true
}

// userMetricAdd adds n to a counter metric.
// Returns false if metric not found or wrong type.
func userMetricAdd(name string, n int) bool {
    if !enableMetrics {
        return false
    }

    userMetricsMu.Lock()
    entry, exists := userMetrics[name]
    userMetricsMu.Unlock()

    if !exists || entry.kind != userMetricCounter {
        return false
    }

    entry.counter.Add(n)
    return true
}

// userMetricSet sets a gauge metric to a value.
// Returns false if metric not found or wrong type.
func userMetricSet(name string, value float64) bool {
    if !enableMetrics {
        return false
    }

    userMetricsMu.Lock()
    entry, exists := userMetrics[name]
    userMetricsMu.Unlock()

    if !exists || entry.kind != userMetricGauge {
        return false
    }

    bits := math.Float64bits(value)
    atomic.StoreInt64(&entry.gauge, int64(bits))
    return true
}

// userMetricObserve records an observation in a summary metric.
// Returns false if metric not found or wrong type.
func userMetricObserve(name string, value float64) bool {
    if !enableMetrics {
        return false
    }

    userMetricsMu.Lock()
    entry, exists := userMetrics[name]
    userMetricsMu.Unlock()

    if !exists || entry.kind != userMetricSummary {
        return false
    }

    entry.summary.Update(value)
    return true
}

// buildMetricsLib registers the metrics stdlib functions.
func buildMetricsLib() {
    features["metrics"] = Feature{version: 1, category: "metrics"}
    categories["metrics"] = []string{
        "metric_enabled",
        "metric_register",
        "metric_deregister",
        "metric_inc",
        "metric_add",
        "metric_set",
        "metric_observe",
    }

    slhelp["metric_enabled"] = LibHelp{in: "", out: "bool", action: "Returns true if the metrics server is enabled (started with -M or ZA_PROMETHEUS env var)."}
    stdlib["metric_enabled"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        return enableMetrics, nil
    }

    slhelp["metric_register"] = LibHelp{in: "name,type", out: "bool", action: "Register a custom metric with type 'counter', 'gauge', or 'summary'. Returns false if already registered or metrics not enabled."}
    stdlib["metric_register"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, _ := expect_args("metric_register", args, 1, "2", "string", "string"); !ok {
            return false, nil
        }
        name := args[0].(string)
        kind := args[1].(string)
        return userMetricRegister(name, kind), nil
    }

    slhelp["metric_deregister"] = LibHelp{in: "name", out: "bool", action: "Deregister and remove a custom metric. Returns false if not found or metrics not enabled."}
    stdlib["metric_deregister"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, _ := expect_args("metric_deregister", args, 1, "1", "string"); !ok {
            return false, nil
        }
        name := args[0].(string)
        return userMetricDeregister(name), nil
    }

    slhelp["metric_inc"] = LibHelp{in: "name", out: "bool", action: "Increment a counter metric by 1. Returns false if not found, wrong type, or metrics not enabled."}
    stdlib["metric_inc"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, _ := expect_args("metric_inc", args, 1, "1", "string"); !ok {
            return false, nil
        }
        name := args[0].(string)
        return userMetricInc(name), nil
    }

    slhelp["metric_add"] = LibHelp{in: "name,n", out: "bool", action: "Add n to a counter metric. Returns false if not found, wrong type, or metrics not enabled."}
    stdlib["metric_add"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, _ := expect_args("metric_add", args, 1, "2", "string", "int"); !ok {
            return false, nil
        }
        name := args[0].(string)
        n := args[1].(int)
        return userMetricAdd(name, n), nil
    }

    slhelp["metric_set"] = LibHelp{in: "name,value", out: "bool", action: "Set a gauge metric to a value. Returns false if not found, wrong type, or metrics not enabled."}
    stdlib["metric_set"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, _ := expect_args("metric_set", args, 1, "2", "string", "float"); !ok {
            return false, nil
        }
        name := args[0].(string)
        value := args[1].(float64)
        return userMetricSet(name, value), nil
    }

    slhelp["metric_observe"] = LibHelp{in: "name,value", out: "bool", action: "Record an observation in a summary metric. Returns false if not found, wrong type, or metrics not enabled."}
    stdlib["metric_observe"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, _ := expect_args("metric_observe", args, 1, "2", "string", "float"); !ok {
            return false, nil
        }
        name := args[0].(string)
        value := args[1].(float64)
        return userMetricObserve(name, value), nil
    }
}
