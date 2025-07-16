//go:build (freebsd || openbsd || netbsd || dragonfly) && !linux && !windows
// +build freebsd openbsd netbsd dragonfly

package main

import (
    "encoding/binary"
    "fmt"
    "net"
    "os"
    "runtime"
    "strings"
    "syscall"
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

    // Parse the data
    offset := 0
    for offset < len(data) {
        if offset+4 > len(data) {
            break
        }

        // Get length of this entry
        entryLen := binary.LittleEndian.Uint32(data[offset : offset+4])
        if entryLen == 0 || offset+int(entryLen) > len(data) {
            break
        }

        // Parse the xtcpcb structure
        if offset+int(unsafe.Sizeof(xtcpcb{})) > len(data) {
            break
        }

        tcp := (*xtcpcb)(unsafe.Pointer(&data[offset]))

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

    // Parse the data
    offset := 0
    for offset < len(data) {
        if offset+4 > len(data) {
            break
        }

        // Get length of this entry
        entryLen := binary.LittleEndian.Uint32(data[offset : offset+4])
        if entryLen == 0 || offset+int(entryLen) > len(data) {
            break
        }

        // Parse the xudpcb structure
        if offset+int(unsafe.Sizeof(xudpcb{})) > len(data) {
            break
        }

        udp := (*xudpcb)(unsafe.Pointer(&data[offset]))

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

    // Parse Unix domain socket data
    offset := 0
    for offset < len(data) {
        if offset+4 > len(data) {
            break
        }

        // Get length of this entry
        entryLen := binary.LittleEndian.Uint32(data[offset : offset+4])
        if entryLen == 0 || offset+int(entryLen) > len(data) {
            break
        }

        // Parse the xunpcb structure
        if offset+int(unsafe.Sizeof(xunpcb{})) > len(data) {
            break
        }

        unix := (*xunpcb)(unsafe.Pointer(&data[offset]))

        // Extract socket path from sockaddr_un structure
        // This is a simplified approach - the actual path is in the sockaddr structure
        socketPath := "/tmp/socket" // Default path

        // Try to extract actual path from the data
        if offset+int(entryLen) <= len(data) {
            // Look for null-terminated string in the data
            for i := offset + int(unsafe.Sizeof(xunpcb{})); i < offset+int(entryLen) && i < len(data); i++ {
                if data[i] == 0 {
                    if i > offset+int(unsafe.Sizeof(xunpcb{})) {
                        path := string(data[offset+int(unsafe.Sizeof(xunpcb{})) : i])
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
