//go:build (freebsd || openbsd || netbsd || dragonfly) && !linux && !windows
// +build freebsd openbsd netbsd dragonfly
// +build !linux
// +build !windows

package main

import (
    "encoding/binary"
    "fmt"
    "net"
    "os"
    "runtime"
    "strings"
    "syscall"
    "time"
    "unsafe"
)

// FreeBSD sysctl implementation for network connections
func getNetworkConnections() ([]map[string]any, error) {
    var allConnections []map[string]any

    // Get TCP connections via sysctl
    if tcp, err := getTcpConnections(); err == nil {
        allConnections = append(allConnections, tcp...)
    }

    // Get UDP connections via sysctl
    if udp, err := getUdpConnections(); err == nil {
        allConnections = append(allConnections, udp...)
    }

    // Get Unix domain sockets
    if unix, err := getUnixConnections(); err == nil {
        allConnections = append(allConnections, unix...)
    }

    // Get raw sockets
    if raw, err := getRawConnections(); err == nil {
        allConnections = append(allConnections, raw...)
    }

    return allConnections, nil
}

// FreeBSD TCP connection structure
type xtcpcb struct {
    Xtc_len  uint32
    Xtc_addr syscall.SockaddrInet4
    Xtc_pcb  uint64
    Xtc_inc  struct {
        Ini_pport uint16
        Ini_lport uint16
        Ini_ip    [4]byte
        Ini_laddr [4]byte
    }
    Xtc_2e         uint32
    Xtc_state      uint32
    Xtc_timer      [4]uint32
    Xtc_so_options uint32
    Xtc_so_linger  uint32
    Xtc_so_pcb     uint64
    Xtc_so_type    uint32
    Xtc_so_timeo   uint32
    Xtc_so_error   uint32
    Xtc_so_proto   uint64
    Xtc_so_head    uint64
    Xtc_so_next    uint64
    Xtc_so_prev    uint64
    Xtc_so_q0      uint64
    Xtc_so_q0len   uint32
    Xtc_so_q0limit uint32
    Xtc_so_q1      uint64
    Xtc_so_q1len   uint32
    Xtc_so_q1limit uint32
    Xtc_so_q2      uint64
    Xtc_so_q2len   uint32
    Xtc_so_q2limit uint32
    Xtc_so_q3      uint64
    Xtc_so_q3len   uint32
    Xtc_so_q3limit uint32
    Xtc_so_owner   uint32
    Xtc_so_uid     uint32
    Xtc_so_euid    uint32
    Xtc_so_gid     uint32
    Xtc_so_egid    uint32
    Xtc_so_pid     uint32
    Xtc_so_rcv     struct {
        Sb_cc         uint32
        Sb_hiwat      uint32
        Sb_mbcnt      uint32
        Sb_mbmax      uint32
        Sb_mb         uint64
        Sb_mbtail     uint64
        Sb_mbhead     uint64
        Sb_lastrecord uint64
        Sb_sel        uint64
        Sb_flags      uint32
        Sb_timeo      uint32
        Sb_wait       uint64
    }
    Xtc_so_snd struct {
        Sb_cc         uint32
        Sb_hiwat      uint32
        Sb_mbcnt      uint32
        Sb_mbmax      uint32
        Sb_mb         uint64
        Sb_mbtail     uint64
        Sb_mbhead     uint64
        Sb_lastrecord uint64
        Sb_sel        uint64
        Sb_flags      uint32
        Sb_timeo      uint32
        Sb_wait       uint64
    }
    Xtc_t_inpcb      uint64
    Xtc_t_inp        uint64
    Xtc_t_state      uint32
    Xtc_t_timer      [4]uint32
    Xtc_t_flags      uint32
    Xtc_t_so_options uint32
    Xtc_t_so_linger  uint32
    Xtc_t_so_pcb     uint64
    Xtc_t_so_type    uint32
    Xtc_t_so_timeo   uint32
    Xtc_t_so_error   uint32
    Xtc_t_so_proto   uint64
    Xtc_t_so_head    uint64
    Xtc_t_so_next    uint64
    Xtc_t_so_prev    uint64
    Xtc_t_so_q0      uint64
    Xtc_t_so_q0len   uint32
    Xtc_t_so_q0limit uint32
    Xtc_t_so_q1      uint64
    Xtc_t_so_q1len   uint32
    Xtc_t_so_q1limit uint32
    Xtc_t_so_q2      uint64
    Xtc_t_so_q2len   uint32
    Xtc_t_so_q2limit uint32
    Xtc_t_so_q3      uint64
    Xtc_t_so_q3len   uint32
    Xtc_t_so_q3limit uint32
    Xtc_t_so_owner   uint32
    Xtc_t_so_uid     uint32
    Xtc_t_so_euid    uint32
    Xtc_t_so_gid     uint32
    Xtc_t_so_egid    uint32
    Xtc_t_so_pid     uint32
    Xtc_t_so_rcv     struct {
        Sb_cc         uint32
        Sb_hiwat      uint32
        Sb_mbcnt      uint32
        Sb_mbmax      uint32
        Sb_mb         uint64
        Sb_mbtail     uint64
        Sb_mbhead     uint64
        Sb_lastrecord uint64
        Sb_sel        uint64
        Sb_flags      uint32
        Sb_timeo      uint32
        Sb_wait       uint64
    }
    Xtc_t_so_snd struct {
        Sb_cc         uint32
        Sb_hiwat      uint32
        Sb_mbcnt      uint32
        Sb_mbmax      uint32
        Sb_mb         uint64
        Sb_mbtail     uint64
        Sb_mbhead     uint64
        Sb_lastrecord uint64
        Sb_sel        uint64
        Sb_flags      uint32
        Sb_timeo      uint32
        Sb_wait       uint64
    }
}

// FreeBSD UDP connection structure
type xudpcb struct {
    Xud_len  uint32
    Xud_addr syscall.SockaddrInet4
    Xud_pcb  uint64
    Xud_inc  struct {
        Ini_pport uint16
        Ini_lport uint16
        Ini_ip    [4]byte
        Ini_laddr [4]byte
    }
    Xud_so_options uint32
    Xud_so_linger  uint32
    Xud_so_pcb     uint64
    Xud_so_type    uint32
    Xud_so_timeo   uint32
    Xud_so_error   uint32
    Xud_so_proto   uint64
    Xud_so_head    uint64
    Xud_so_next    uint64
    Xud_so_prev    uint64
    Xud_so_q0      uint64
    Xud_so_q0len   uint32
    Xud_so_q0limit uint32
    Xud_so_q1      uint64
    Xud_so_q1len   uint32
    Xud_so_q1limit uint32
    Xud_so_q2      uint64
    Xud_so_q2len   uint32
    Xud_so_q2limit uint32
    Xud_so_q3      uint64
    Xud_so_q3len   uint32
    Xud_so_q3limit uint32
    Xud_so_owner   uint32
    Xud_so_uid     uint32
    Xud_so_euid    uint32
    Xud_so_gid     uint32
    Xud_so_egid    uint32
    Xud_so_pid     uint32
}

// FreeBSD Unix socket structure (basic)
type xunpcb struct {
    Xun_len        uint32
    Xun_addr       syscall.SockaddrUnix
    Xun_pcb        uint64
    Xun_so_options uint32
    Xun_so_linger  uint32
    Xun_so_pcb     uint64
    Xun_so_type    uint32
    Xun_so_timeo   uint32
    Xun_so_error   uint32
    Xun_so_proto   uint64
    Xun_so_head    uint64
    Xun_so_next    uint64
    Xun_so_prev    uint64
    Xun_so_q0      uint64
    Xun_so_q0len   uint32
    Xun_so_q0limit uint32
    Xun_so_q1      uint64
    Xun_so_q1len   uint32
    Xun_so_q1limit uint32
    Xun_so_q2      uint64
    Xun_so_q2len   uint32
    Xun_so_q2limit uint32
    Xun_so_q3      uint64
    Xun_so_q3len   uint32
    Xun_so_q3limit uint32
    Xun_so_owner   uint32
    Xun_so_uid     uint32
    Xun_so_euid    uint32
    Xun_so_gid     uint32
    Xun_so_egid    uint32
    Xun_so_pid     uint32
}

// Get TCP connections via sysctl
func getTcpConnections() ([]map[string]any, error) {
    var connections []map[string]any

    // Get TCP table via sysctl
    data, err := syscall.Sysctl("net.inet.tcp.pcblist")
    if err != nil {
        return nil, fmt.Errorf("failed to get TCP table: %v", err)
    }

    // Convert string to []byte for binary operations
    dataBytes := []byte(data)

    // Parse the data
    offset := 0
    for offset < len(dataBytes) {
        if offset+4 > len(dataBytes) {
            break
        }

        // Get length of this entry
        entryLen := binary.LittleEndian.Uint32(dataBytes[offset : offset+4])
        if entryLen == 0 || offset+int(entryLen) > len(dataBytes) {
            break
        }

        // Parse the xtcpcb structure
        if offset+int(unsafe.Sizeof(xtcpcb{})) > len(dataBytes) {
            break
        }

        tcp := (*xtcpcb)(unsafe.Pointer(&dataBytes[offset]))

        // Extract connection info
        localAddr := net.IP(tcp.Xtc_inc.Ini_laddr[:])
        remoteAddr := net.IP(tcp.Xtc_inc.Ini_ip[:])
        localPort := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&tcp.Xtc_inc.Ini_lport))[:])
        remotePort := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&tcp.Xtc_inc.Ini_pport))[:])

        // Determine state
        state := "UNKNOWN"
        switch tcp.Xtc_t_state {
        case 1: // TCPS_CLOSED
            state = "CLOSED"
        case 2: // TCPS_LISTEN
            state = "LISTEN"
        case 3: // TCPS_SYN_SENT
            state = "SYN_SENT"
        case 4: // TCPS_SYN_RECEIVED
            state = "SYN_RECEIVED"
        case 5: // TCPS_ESTABLISHED
            state = "ESTABLISHED"
        case 6: // TCPS_CLOSE_WAIT
            state = "CLOSE_WAIT"
        case 7: // TCPS_FIN_WAIT_1
            state = "FIN_WAIT_1"
        case 8: // TCPS_CLOSING
            state = "CLOSING"
        case 9: // TCPS_LAST_ACK
            state = "LAST_ACK"
        case 10: // TCPS_FIN_WAIT_2
            state = "FIN_WAIT_2"
        case 11: // TCPS_TIME_WAIT
            state = "TIME_WAIT"
        }

        connections = append(connections, map[string]any{
            "protocol":    "tcp",
            "local_addr":  localAddr.String(),
            "local_port":  int(localPort),
            "remote_addr": remoteAddr.String(),
            "remote_port": int(remotePort),
            "state":       state,
            "pid":         int(tcp.Xtc_so_pid),
            "process":     getProcessName(int(tcp.Xtc_so_pid)),
            "interface":   "FreeBSD",
        })

        offset += int(entryLen)
    }

    return connections, nil
}

// Get UDP connections via sysctl
func getUdpConnections() ([]map[string]any, error) {
    var connections []map[string]any

    // Get UDP table via sysctl
    data, err := syscall.Sysctl("net.inet.udp.pcblist")
    if err != nil {
        return nil, fmt.Errorf("failed to get UDP table: %v", err)
    }

    // Convert string to []byte for binary operations
    dataBytes := []byte(data)

    // Parse the data
    offset := 0
    for offset < len(dataBytes) {
        if offset+4 > len(dataBytes) {
            break
        }

        // Get length of this entry
        entryLen := binary.LittleEndian.Uint32(dataBytes[offset : offset+4])
        if entryLen == 0 || offset+int(entryLen) > len(dataBytes) {
            break
        }

        // Parse the xudpcb structure
        if offset+int(unsafe.Sizeof(xudpcb{})) > len(dataBytes) {
            break
        }

        udp := (*xudpcb)(unsafe.Pointer(&dataBytes[offset]))

        // Extract connection info
        localAddr := net.IP(udp.Xud_inc.Ini_laddr[:])
        localPort := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&udp.Xud_inc.Ini_lport))[:])

        connections = append(connections, map[string]any{
            "protocol":    "udp",
            "local_addr":  localAddr.String(),
            "local_port":  int(localPort),
            "remote_addr": "0.0.0.0",
            "remote_port": 0,
            "state":       "LISTEN",
            "pid":         int(udp.Xud_so_pid),
            "process":     getProcessName(int(udp.Xud_so_pid)),
            "interface":   "FreeBSD",
        })

        offset += int(entryLen)
    }

    return connections, nil
}

// Get Unix domain sockets
func getUnixConnections() ([]map[string]any, error) {
    var connections []map[string]any

    // Get Unix domain socket table via sysctl
    data, err := syscall.Sysctl("net.local.pcblist")
    if err != nil {
        // Unix sockets might not be available, return empty list
        return connections, nil
    }

    // Convert string to []byte for binary operations
    dataBytes := []byte(data)

    // Parse Unix domain socket data
    offset := 0
    for offset < len(dataBytes) {
        if offset+4 > len(dataBytes) {
            break
        }

        // Get length of this entry
        entryLen := binary.LittleEndian.Uint32(dataBytes[offset : offset+4])
        if entryLen == 0 || offset+int(entryLen) > len(dataBytes) {
            break
        }

        // Parse the xunpcb structure
        if offset+int(unsafe.Sizeof(xunpcb{})) > len(dataBytes) {
            break
        }

        unix := (*xunpcb)(unsafe.Pointer(&dataBytes[offset]))

        // Extract socket path from sockaddr_un structure
        // This is a simplified approach - the actual path is in the sockaddr structure
        socketPath := "/tmp/socket" // Default path

        // Try to extract actual path from the data
        if offset+int(entryLen) <= len(dataBytes) {
            // Look for null-terminated string in the data
            for i := offset + int(unsafe.Sizeof(xunpcb{})); i < offset+int(entryLen) && i < len(dataBytes); i++ {
                if dataBytes[i] == 0 {
                    if i > offset+int(unsafe.Sizeof(xunpcb{})) {
                        path := string(dataBytes[offset+int(unsafe.Sizeof(xunpcb{})) : i])
                        if path != "" {
                            socketPath = path
                        }
                    }
                    break
                }
            }
        }

        connections = append(connections, map[string]any{
            "protocol":    "unix",
            "local_addr":  socketPath,
            "local_port":  0,
            "remote_addr": "",
            "remote_port": 0,
            "state":       "LISTEN",
            "pid":         int(unix.Xun_so_pid),
            "process":     getProcessName(int(unix.Xun_so_pid)),
            "interface":   "unix",
        })

        offset += int(entryLen)
    }

    return connections, nil
}

// Get raw sockets
func getRawConnections() ([]map[string]any, error) {
    var connections []map[string]any

    // Raw sockets are typically not enumerated by sysctl
    // They are created on-demand and don't maintain persistent state
    // We can check for raw socket capabilities but can't enumerate active ones

    // Raw sockets cannot be enumerated, so return empty list
    // The capability is checked by canUseRawSockets() for protocol availability
    return connections, nil
}

// Get available protocols for FreeBSD
func getAvailableProtocols() []string {
    var protocols []string

    // Always available
    protocols = append(protocols, "tcp", "udp")

    // Unix domain sockets
    protocols = append(protocols, "unix")

    // Raw sockets (test availability)
    if canUseRawSockets() {
        protocols = append(protocols, "raw")
    }

    // IPv6 versions
    protocols = append(protocols, "tcp6", "udp6")

    // ICMP (via raw sockets)
    if canUseRawSockets() {
        protocols = append(protocols, "icmp")
    }

    return protocols
}

// Get protocol information for FreeBSD
func getProtocolInfo() map[string]any {
    info := map[string]any{
        "platform": runtime.GOOS,
        "protocols": map[string]any{
            "tcp": map[string]any{
                "available":           true,
                "description":         "Transmission Control Protocol",
                "source":              "sysctl",
                "requires_privileges": false,
            },
            "udp": map[string]any{
                "available":           true,
                "description":         "User Datagram Protocol",
                "source":              "sysctl",
                "requires_privileges": false,
            },
            "unix": map[string]any{
                "available":           true,
                "description":         "Unix Domain Sockets",
                "source":              "sysctl",
                "requires_privileges": false,
            },
            "raw": map[string]any{
                "available":           canUseRawSockets(),
                "description":         "Raw Sockets",
                "source":              "sysctl",
                "requires_privileges": true,
                "reason":              getRawSocketReason(),
            },
            "icmp": map[string]any{
                "available":           canUseRawSockets(),
                "description":         "Internet Control Message Protocol",
                "source":              "sysctl",
                "requires_privileges": true,
                "reason":              getRawSocketReason(),
            },
        },
    }

    return info
}

// Get open files for FreeBSD
func getOpenFiles() ([]map[string]any, error) {
    var files []map[string]any

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

// Helper functions
func canUseRawSockets() bool {
    sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
    if err != nil {
        return false
    }
    syscall.Close(sock)
    return true
}

func getRawSocketReason() string {
    if !canUseRawSockets() {
        return "Requires elevated privileges"
    }
    return "Available"
}

// Get process name by PID for FreeBSD
func getProcessName(pid int) string {
    if pid == 0 {
        return "System"
    }

    // Try to read process name from /proc
    procPath := fmt.Sprintf("/proc/%d/comm", pid)
    if data, err := os.ReadFile(procPath); err == nil {
        // Remove trailing newline
        name := string(data)
        if len(name) > 0 && name[len(name)-1] == '\n' {
            name = name[:len(name)-1]
        }
        if name != "" {
            return name
        }
    }

    // Fallback: try to read from /proc/pid/status
    statusPath := fmt.Sprintf("/proc/%d/status", pid)
    if data, err := os.ReadFile(statusPath); err == nil {
        lines := strings.Split(string(data), "\n")
        for _, line := range lines {
            if strings.HasPrefix(line, "Name:") {
                fields := strings.Fields(line)
                if len(fields) >= 2 {
                    return fields[1]
                }
            }
        }
    }

    // Final fallback
    return fmt.Sprintf("process-%d", pid)
}

// icmpPing performs ICMP ping on FreeBSD
func icmpPing(targetIP string, timeout time.Duration) (map[string]any, error) {
    // FreeBSD implementation of ICMP ping
    // This is a simplified implementation that may require root privileges

    // Try to resolve the target IP
    ip := net.ParseIP(targetIP)
    if ip == nil {
        return map[string]any{"success": false, "latency": 0, "error": "Invalid IP address"}, nil
    }

    // Create raw socket for ICMP
    sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
    if err != nil {
        return map[string]any{"success": false, "latency": 0, "error": "ICMP ping requires root privileges: " + err.Error()}, nil
    }
    defer syscall.Close(sock)

    // Set socket timeout
    tv := syscall.NsecToTimeval(int64(timeout))
    syscall.SetsockoptTimeval(sock, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
    syscall.SetsockoptTimeval(sock, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &tv)

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

    // Parse target IP
    addr := syscall.SockaddrInet4{Port: 0}
    copy(addr.Addr[:], ip.To4())

    start := time.Now()

    // Send ICMP echo request
    err = syscall.Sendto(sock, msg, 0, &addr)
    if err != nil {
        return map[string]any{"success": false, "latency": 0, "error": "Failed to send ICMP packet: " + err.Error()}, nil
    }

    // Receive ICMP echo reply
    resp := make([]byte, 1024)
    _, _, err = syscall.Recvfrom(sock, resp, 0)
    if err != nil {
        return map[string]any{"success": false, "latency": 0, "error": "Failed to receive ICMP response: " + err.Error()}, nil
    }

    latency := time.Since(start).Milliseconds()
    return map[string]any{"success": true, "latency": latency, "error": ""}, nil
}

// icmpTraceroute performs ICMP traceroute on FreeBSD
func icmpTraceroute(targetIP string, timeout time.Duration, maxHops int) ([]map[string]any, error) {
    var hops []map[string]any

    // FreeBSD implementation of ICMP traceroute
    // This is a simplified implementation that may require root privileges

    ip := net.ParseIP(targetIP)
    if ip == nil {
        return nil, fmt.Errorf("invalid IP address")
    }

    // Create raw socket for ICMP
    sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
    if err != nil {
        return nil, fmt.Errorf("ICMP traceroute requires root privileges: %v", err)
    }
    defer syscall.Close(sock)

    // Set socket timeout
    tv := syscall.NsecToTimeval(int64(timeout))
    syscall.SetsockoptTimeval(sock, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
    syscall.SetsockoptTimeval(sock, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &tv)

    // Set TTL for each hop
    for ttl := 1; ttl <= maxHops; ttl++ {
        syscall.SetsockoptInt(sock, syscall.IPPROTO_IP, syscall.IP_TTL, ttl)

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

        // Parse target IP
        addr := syscall.SockaddrInet4{Port: 0}
        copy(addr.Addr[:], ip.To4())

        start := time.Now()

        // Send ICMP echo request
        err = syscall.Sendto(sock, msg, 0, &addr)
        if err != nil {
            hops = append(hops, map[string]any{
                "hop":     ttl,
                "address": "*",
                "latency": 0,
                "error":   "Failed to send packet",
            })
            continue
        }

        // Receive ICMP echo reply or time exceeded
        resp := make([]byte, 1024)
        _, from, err := syscall.Recvfrom(sock, resp, 0)
        if err != nil {
            hops = append(hops, map[string]any{
                "hop":     ttl,
                "address": "*",
                "latency": 0,
                "error":   "No response",
            })
            continue
        }

        latency := time.Since(start).Milliseconds()

        // Extract source address
        var sourceAddr string
        if fromAddr, ok := from.(*syscall.SockaddrInet4); ok {
            sourceAddr = net.IP(fromAddr.Addr[:]).String()
        } else {
            sourceAddr = "unknown"
        }

        hops = append(hops, map[string]any{
            "hop":     ttl,
            "address": sourceAddr,
            "latency": latency,
            "error":   "",
        })

        // Check if we reached the target
        if sourceAddr == targetIP {
            break
        }
    }

    return hops, nil
}

// tcpTraceroute performs TCP traceroute on FreeBSD
func tcpTraceroute(targetIP string, port int, timeout time.Duration, maxHops int) ([]map[string]any, error) {
    var hops []map[string]any

    // FreeBSD implementation of TCP traceroute
    // This is a simplified implementation that may require root privileges

    ip := net.ParseIP(targetIP)
    if ip == nil {
        return nil, fmt.Errorf("invalid IP address")
    }

    // Create raw socket for TCP
    sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_TCP)
    if err != nil {
        return nil, fmt.Errorf("TCP traceroute requires root privileges: %v", err)
    }
    defer syscall.Close(sock)

    // Set socket timeout
    tv := syscall.NsecToTimeval(int64(timeout))
    syscall.SetsockoptTimeval(sock, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
    syscall.SetsockoptTimeval(sock, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &tv)

    // Set TTL for each hop
    for ttl := 1; ttl <= maxHops; ttl++ {
        syscall.SetsockoptInt(sock, syscall.IPPROTO_IP, syscall.IP_TTL, ttl)

        // Build TCP SYN packet
        tcpHeader := make([]byte, 20)
        tcpHeader[0] = 0 // Source port (will be set by kernel)
        tcpHeader[1] = 0
        binary.BigEndian.PutUint16(tcpHeader[2:], uint16(port))                  // Destination port
        binary.BigEndian.PutUint32(tcpHeader[4:], uint32(time.Now().UnixNano())) // Sequence number
        tcpHeader[8] = 0                                                         // Acknowledgment number
        tcpHeader[9] = 0
        tcpHeader[10] = 0
        tcpHeader[11] = 0
        tcpHeader[12] = 0x50                               // Data offset (5 words)
        tcpHeader[13] = 0x02                               // SYN flag
        binary.BigEndian.PutUint16(tcpHeader[14:], 0x4000) // Window size
        tcpHeader[16] = 0                                  // Checksum (will be calculated by kernel)
        tcpHeader[17] = 0
        tcpHeader[18] = 0 // Urgent pointer
        tcpHeader[19] = 0

        // Parse target IP
        addr := syscall.SockaddrInet4{Port: port}
        copy(addr.Addr[:], ip.To4())

        start := time.Now()

        // Send TCP SYN packet
        err = syscall.Sendto(sock, tcpHeader, 0, &addr)
        if err != nil {
            hops = append(hops, map[string]any{
                "hop":     ttl,
                "address": "*",
                "latency": 0,
                "error":   "Failed to send packet",
            })
            continue
        }

        // Receive response
        resp := make([]byte, 1024)
        _, from, err := syscall.Recvfrom(sock, resp, 0)
        if err != nil {
            hops = append(hops, map[string]any{
                "hop":     ttl,
                "address": "*",
                "latency": 0,
                "error":   "No response",
            })
            continue
        }

        latency := time.Since(start).Milliseconds()

        // Extract source address
        var sourceAddr string
        if fromAddr, ok := from.(*syscall.SockaddrInet4); ok {
            sourceAddr = net.IP(fromAddr.Addr[:]).String()
        } else {
            sourceAddr = "unknown"
        }

        hops = append(hops, map[string]any{
            "hop":     ttl,
            "address": sourceAddr,
            "latency": latency,
            "error":   "",
        })

        // Check if we reached the target
        if sourceAddr == targetIP {
            break
        }
    }

    return hops, nil
}

// calculateICMPChecksum calculates ICMP checksum
func calculateICMPChecksum(data []byte) uint16 {
    var sum uint32
    for i := 0; i < len(data)-1; i += 2 {
        sum += uint32(data[i])<<8 + uint32(data[i+1])
    }
    if len(data)%2 == 1 {
        sum += uint32(data[len(data)-1]) << 8
    }
    for sum > 0xffff {
        sum = (sum & 0xffff) + (sum >> 16)
    }
    return uint16(^sum)
}
