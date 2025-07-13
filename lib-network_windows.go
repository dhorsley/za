//go:build windows
// +build windows

package main

import (
    "encoding/binary"
    "fmt"
    "net"
    "syscall"
    "time"
    "unsafe"
)

// Windows-specific constants that may not be defined in syscall
const (
    PROCESS_VM_READ = 0x0010
    IPPROTO_ICMP    = 1
)

// Windows API constants
const (
    AF_INET  = 2
    AF_INET6 = 23

    TCP_TABLE_OWNER_PID_ALL = 5
    UDP_TABLE_OWNER_PID     = 1

    MIB_TCP_STATE_LISTEN = 2
    MIB_TCP_STATE_ESTAB  = 5
)

// Windows API structures
type MIB_TCPROW_OWNER_PID struct {
    State      uint32
    LocalAddr  uint32
    LocalPort  uint32
    RemoteAddr uint32
    RemotePort uint32
    ProcessId  uint32
}

type MIB_UDPROW_OWNER_PID struct {
    LocalAddr uint32
    LocalPort uint32
    ProcessId uint32
}

// Windows API function declarations
var (
    kernel32 = syscall.NewLazyDLL("kernel32.dll")
    iphlpapi = syscall.NewLazyDLL("iphlpapi.dll")

    procGetExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")
    procGetExtendedUdpTable = iphlpapi.NewProc("GetExtendedUdpTable")
    procGetModuleFileNameW  = kernel32.NewProc("GetModuleFileNameW")
)

func icmpPing(targetIP string, timeout time.Duration) (map[string]any, error) {

    // Try to create raw socket for ICMP echo request
    var err error

    conn, err := net.DialTimeout("ip4:icmp", targetIP, timeout)
    if err != nil {
        return map[string]any{"success": false, "latency": 0, "error": "Failed to connect: " + err.Error()}, nil
    }
    defer conn.Close()

    // Build ICMP echo request packet
    msg := make([]byte, 8)
    msg[0] = 8 // echo request
    msg[1] = 0 // code 0
    binary.BigEndian.PutUint16(msg[6:], uint16(time.Now().UnixNano()))
    msg[2] = 0
    msg[3] = 0
    csum := calculateICMPChecksum(msg)
    msg[2] = byte(csum >> 8)
    msg[3] = byte(csum & 0xff)

    start := time.Now()
    conn.SetDeadline(time.Now().Add(timeout))
    _, err = conn.Write(msg)
    if err != nil {
        return map[string]any{"success": false, "latency": 0, "error": "Failed to send ICMP packet: " + err.Error()}, nil
    }

    // Receive ICMP echo reply
    resp := make([]byte, 1024)
    _, err = conn.Read(resp)
    if err != nil {
        return map[string]any{"success": false, "latency": 0, "error": "Failed to receive ICMP response: " + err.Error()}, nil
    }

    latency := time.Since(start).Milliseconds()
    return map[string]any{"success": true, "latency": latency, "error": ""}, nil

}

func tcpTraceroute(targetIP string, port int, timeout time.Duration, maxHops int) ([]map[string]any, error) {

    // Windows doesn't support raw sockets without admin privileges
    // So we'll use regular TCP connections to simulate traceroute

    hops := []map[string]any{}

    for ttl := 1; ttl <= maxHops; ttl++ {
        // Try to connect to the target host:port
        start := time.Now()
        conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", targetIP, port), timeout)
        latency := time.Since(start).Milliseconds()

        if err != nil {
            hops = append(hops, map[string]any{
                "hop":     ttl,
                "address": "",
                "latency": latency,
                "error":   "timeout",
            })
            continue
        }
        defer conn.Close()

        // If we got a connection, we've reached the target
        hops = append(hops, map[string]any{
            "hop":     ttl,
            "address": targetIP,
            "latency": latency,
            "error":   "",
        })

        // We've reached the target, so we're done
        break
    }

    return hops, nil

}

func icmpTraceroute(targetIP string, timeout time.Duration, maxHops int) ([]map[string]any, error) {
    return tcpTraceroute(targetIP, 80, timeout, maxHops)
}

// Network monitoring functions for Windows

// Get all network connections via Windows API
func getNetworkConnections() ([]map[string]any, error) {
    var allConnections []map[string]any

    // Get TCP connections via Windows API
    tcp, err := getTcpConnections()
    if err != nil {
        return nil, fmt.Errorf("getTcpConnections: %v", err)
    }
    allConnections = append(allConnections, tcp...)

    // Get UDP connections via Windows API
    udp, err := getUdpConnections()
    if err != nil {
        return nil, fmt.Errorf("getUdpConnections: %v", err)
    }
    allConnections = append(allConnections, udp...)

    return allConnections, nil
}

// Get TCP connections via Windows API
func getTcpConnections() ([]map[string]any, error) {
    var size uint32
    var connections []map[string]any

    // First call to get required size
    ret, _, err := procGetExtendedTcpTable.Call(
        uintptr(unsafe.Pointer(&size)),
        uintptr(unsafe.Pointer(&size)),
        0,
        AF_INET,
        TCP_TABLE_OWNER_PID_ALL,
        0,
    )

    if ret != 0 && ret != 122 { // 122 = ERROR_INSUFFICIENT_BUFFER (expected)
        return nil, fmt.Errorf("failed to get TCP table size: ret=%d, err=%v", ret, err)
    }

    // Allocate buffer
    buffer := make([]byte, size)

    // Get TCP table
    ret, _, err = procGetExtendedTcpTable.Call(
        uintptr(unsafe.Pointer(&buffer[0])),
        uintptr(unsafe.Pointer(&size)),
        0,
        AF_INET,
        TCP_TABLE_OWNER_PID_ALL,
        0,
    )

    if ret != 0 {
        return nil, fmt.Errorf("failed to get TCP table: ret=%d, err=%v", ret, err)
    }

    // Parse results
    rowCount := binary.LittleEndian.Uint32(buffer[0:4])
    offset := uint32(4)

    if len(buffer) < 4 {
        return nil, fmt.Errorf("buffer too small: %d bytes", len(buffer))
    }

    for i := uint32(0); i < rowCount; i++ {
        if offset+uint32(unsafe.Sizeof(MIB_TCPROW_OWNER_PID{})) > size {
            break
        }

        row := (*MIB_TCPROW_OWNER_PID)(unsafe.Pointer(&buffer[offset]))

        // Convert addresses
        localAddr := net.IP([]byte{
            byte(row.LocalAddr),
            byte(row.LocalAddr >> 8),
            byte(row.LocalAddr >> 16),
            byte(row.LocalAddr >> 24),
        })

        remoteAddr := net.IP([]byte{
            byte(row.RemoteAddr),
            byte(row.RemoteAddr >> 8),
            byte(row.RemoteAddr >> 16),
            byte(row.RemoteAddr >> 24),
        })

        // Convert ports (network byte order)
        localPort := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&row.LocalPort))[:])
        remotePort := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&row.RemotePort))[:])

        // Determine state
        state := "UNKNOWN"
        switch row.State {
        case MIB_TCP_STATE_LISTEN:
            state = "LISTEN"
        case MIB_TCP_STATE_ESTAB:
            state = "ESTABLISHED"
        }

        // Get process name
        processName := getProcessName(int(row.ProcessId))

        connections = append(connections, map[string]any{
            "protocol":    "tcp",
            "local_addr":  localAddr.String(),
            "local_port":  int(localPort),
            "remote_addr": remoteAddr.String(),
            "remote_port": int(remotePort),
            "state":       state,
            "pid":         int(row.ProcessId),
            "process":     processName,
            "interface":   "Windows",
        })

        offset += uint32(unsafe.Sizeof(MIB_TCPROW_OWNER_PID{}))
    }

    return connections, nil
}

// Get UDP connections via Windows API
func getUdpConnections() ([]map[string]any, error) {
    var size uint32
    var connections []map[string]any

    // First call to get required size
    ret, _, err := procGetExtendedUdpTable.Call(
        uintptr(unsafe.Pointer(&size)),
        uintptr(unsafe.Pointer(&size)),
        0,
        AF_INET,
        UDP_TABLE_OWNER_PID,
        0,
    )

    if ret != 0 && ret != 122 { // 122 = ERROR_INSUFFICIENT_BUFFER (expected)
        return nil, fmt.Errorf("failed to get UDP table size: ret=%d, err=%v", ret, err)
    }

    // Allocate buffer
    buffer := make([]byte, size)

    // Get UDP table
    ret, _, err = procGetExtendedUdpTable.Call(
        uintptr(unsafe.Pointer(&buffer[0])),
        uintptr(unsafe.Pointer(&size)),
        0,
        AF_INET,
        UDP_TABLE_OWNER_PID,
        0,
    )

    if ret != 0 {
        return nil, fmt.Errorf("failed to get UDP table: ret=%d, err=%v", ret, err)
    }

    // Parse results
    rowCount := binary.LittleEndian.Uint32(buffer[0:4])
    offset := uint32(4)

    if len(buffer) < 4 {
        return nil, fmt.Errorf("buffer too small: %d bytes", len(buffer))
    }

    for i := uint32(0); i < rowCount; i++ {
        if offset+uint32(unsafe.Sizeof(MIB_UDPROW_OWNER_PID{})) > size {
            break
        }

        row := (*MIB_UDPROW_OWNER_PID)(unsafe.Pointer(&buffer[offset]))

        // Convert address
        localAddr := net.IP([]byte{
            byte(row.LocalAddr),
            byte(row.LocalAddr >> 8),
            byte(row.LocalAddr >> 16),
            byte(row.LocalAddr >> 24),
        })

        // Convert port (network byte order)
        localPort := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&row.LocalPort))[:])

        // Get process name
        processName := getProcessName(int(row.ProcessId))

        connections = append(connections, map[string]any{
            "protocol":    "udp",
            "local_addr":  localAddr.String(),
            "local_port":  int(localPort),
            "remote_addr": "0.0.0.0",
            "remote_port": 0,
            "state":       "LISTEN",
            "pid":         int(row.ProcessId),
            "process":     processName,
            "interface":   "Windows",
        })

        offset += uint32(unsafe.Sizeof(MIB_UDPROW_OWNER_PID{}))
    }

    return connections, nil
}

// Get process name by PID
func getProcessName(pid int) string {
    if pid == 0 {
        return "System"
    }

    // Open process handle
    handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, false, uint32(pid))
    if err != nil {
        return fmt.Sprintf("process-%d", pid)
    }
    defer syscall.CloseHandle(handle)

    // Get module filename
    var size uint32 = 260 // MAX_PATH
    filename := make([]uint16, size)

    ret, _, err := procGetModuleFileNameW.Call(
        uintptr(handle),
        uintptr(unsafe.Pointer(&filename[0])),
        uintptr(size),
    )

    if ret == 0 {
        return fmt.Sprintf("process-%d", pid)
    }

    // Convert to string and extract basename
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

    if path == "" {
        return fmt.Sprintf("process-%d", pid)
    }

    return path
}

// Get available protocols for Windows
func getAvailableProtocols() []string {
    var protocols []string

    // Always available
    protocols = append(protocols, "tcp", "udp")

    // IPv6 versions
    protocols = append(protocols, "tcp6", "udp6")

    // Raw sockets (limited on Windows)
    if canUseRawSockets() {
        protocols = append(protocols, "raw")
    }

    return protocols
}

// Get protocol information for Windows
func getProtocolInfo() map[string]any {
    info := map[string]any{
        "platform": "windows",
        "protocols": map[string]any{
            "tcp": map[string]any{
                "available":           true,
                "description":         "Transmission Control Protocol",
                "source":              "Windows API",
                "requires_privileges": false,
            },
            "udp": map[string]any{
                "available":           true,
                "description":         "User Datagram Protocol",
                "source":              "Windows API",
                "requires_privileges": false,
            },
            "raw": map[string]any{
                "available":           canUseRawSockets(),
                "description":         "Raw Sockets (Limited)",
                "source":              "Windows API",
                "requires_privileges": true,
                "reason":              getRawSocketReason(),
            },
        },
        "notes": map[string]any{
            "unix_sockets": "Not available on Windows",
            "raw_sockets":  "Limited functionality compared to Unix systems",
        },
    }

    return info
}

// Get open files for Windows
func getOpenFiles() ([]map[string]any, error) {
    var files []map[string]any

    // Use NtQuerySystemInformation to get handle information
    // This requires elevated privileges, so we'll implement a basic version
    // that tries to get socket information from the network tables we already have

    // Get network connections and convert them to file format
    connections, err := getNetworkConnections()
    if err != nil {
        return nil, err
    }

    // Convert network connections to file descriptor format
    for i, conn := range connections {
        protocol := conn["protocol"].(string)
        localAddr := conn["local_addr"].(string)
        localPort := conn["local_port"].(int)
        remoteAddr := conn["remote_addr"].(string)
        remotePort := conn["remote_port"].(int)
        pid := conn["pid"].(int)
        process := conn["process"].(string)

        files = append(files, map[string]any{
            "pid":      pid,
            "process":  process,
            "fd":       i + 3, // Start from 3 (0,1,2 are stdin,stdout,stderr)
            "type":     "socket",
            "protocol": protocol,
            "local":    fmt.Sprintf("%s:%d", localAddr, localPort),
            "remote":   fmt.Sprintf("%s:%d", remoteAddr, remotePort),
        })
    }

    return files, nil
}

// Helper functions for Windows
func canUseRawSockets() bool {
    // Try to create a raw socket to test availability
    sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, IPPROTO_ICMP)
    if err != nil {
        return false
    }
    syscall.Close(sock)
    return true
}

func getRawSocketReason() string {
    if !canUseRawSockets() {
        return "Limited on Windows, requires elevated privileges"
    }
    return "Available (limited)"
}
