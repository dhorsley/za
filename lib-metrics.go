//go:build !test

package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "runtime"
    "sync"
    "sync/atomic"
    "time"

    "github.com/VictoriaMetrics/metrics"
)

var (
    metricsServer *http.Server
    enableMetrics bool
)

// startMetricsServer initializes and starts the Prometheus metrics HTTP server.
// It registers all gauge-with-callback metrics and starts listening on the specified port.
func startMetricsServer(port int) {
    registerSystemGauges()
    registerProcessGauges()
    registerBuildInfo()
    registerFFIInventory()
    registerWebGauges()
    registerErrorChainGauge()
    registerLoggingGauges()

    mux := http.NewServeMux()
    mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        metrics.WritePrometheus(w, false)
    })
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, "/metrics", http.StatusFound)
    })

    metricsServer = &http.Server{
        Addr:    fmt.Sprintf("0.0.0.0:%d", port),
        Handler: mux,
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

func registerSystemGauges() {
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

func registerFFIInventory() {
    metrics.NewGauge(`za_ffi_loaded_libraries`, func() float64 {
        return float64(len(loadedCLibraries))
    })
    metrics.NewGauge(`za_ffi_active_callbacks`, func() float64 {
        callbackMutex.RLock()
        defer callbackMutex.RUnlock()
        return float64(len(callbackHandles))
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
