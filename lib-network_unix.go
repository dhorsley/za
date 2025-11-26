//go:build linux && !freebsd && !openbsd && !netbsd && !dragonfly && !windows && !test

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Linux netlink constants
const (
	NETLINK_INET_DIAG   = 4
	INET_DIAG_GETSOCK   = 1
	SOCK_DIAG_BY_FAMILY = 20
)

// ioctl function for interface operations
func ioctl(fd int, request uintptr, arg uintptr) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, arg)
	if err != 0 {
		return err
	}
	return nil
}

// Linux socket structures
type InetDiagReq struct {
	Family   uint8
	Protocol uint8
	Ext      uint8
	Pad      uint8
	States   uint32
	ID       InetDiagSockID
}

type InetDiagSockID struct {
	IDiagSPort  [2]byte
	IDiagDPort  [2]byte
	IDiagSrc    [16]byte
	IDiagDst    [16]byte
	IDiagIf     uint32
	IDiagCookie [2]uint32
}

type InetDiagMsg struct {
	IDiagFamily  uint8
	IDiagState   uint8
	IDiagTimer   uint8
	IDiagRetrans uint8
	ID           InetDiagSockID
	Expires      uint32
	RQueue       uint32
	WQueue       uint32
	UID          uint32
	Inode        uint32
}

func icmpPing(targetIP string, timeout time.Duration) (map[string]any, error) {

	var sock int
	var err error

	// Unix-like systems (primary implementation)
	sock, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
	if err != nil {
		// If raw socket fails, we can't do ICMP ping without privileges
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
	copy(addr.Addr[:], net.ParseIP(targetIP).To4())

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

func icmpTraceroute(targetIP string, timeout time.Duration, maxHops int) ([]map[string]any, error) {

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, 1) // IPPROTO_ICMP = 1
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	hops := []map[string]any{}

	// Set socket options
	syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_TTL, 1)
	syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, int(timeout.Milliseconds()))

	for ttl := 1; ttl <= maxHops; ttl++ {
		// Set TTL for this hop
		syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_TTL, ttl)

		// Create ICMP echo request
		msg := make([]byte, 8)
		msg[0] = 8 // echo request
		msg[1] = 0 // code
		binary.BigEndian.PutUint16(msg[6:], uint16(time.Now().UnixNano()))
		csum := calculateICMPChecksum(msg)
		msg[2] = byte(csum >> 8)
		msg[3] = byte(csum & 0xff)

		// Send packet
		addr := &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}} // Will be overridden
		copy(addr.Addr[:], net.ParseIP(targetIP).To4())

		start := time.Now()
		err = syscall.Sendto(fd, msg, 0, addr)
		if err != nil {
			hops = append(hops, map[string]any{
				"hop":     ttl,
				"address": "",
				"latency": 0,
				"error":   fmt.Sprintf("send error: %v", err),
			})
			continue
		}

		// Receive response with timeout
		resp := make([]byte, 1024)
		received := make(chan struct{})
		var n int
		var recvErr error

		go func() {
			n, _, recvErr = syscall.Recvfrom(fd, resp, 0)
			close(received)
		}()

		select {
		case <-received:
			latency := time.Since(start).Milliseconds()
			if recvErr != nil {
				hops = append(hops, map[string]any{
					"hop":     ttl,
					"address": "",
					"latency": latency,
					"error":   "timeout",
				})
				continue
			}

			// Parse response
			if n >= 20 {
				respIP := net.IP(resp[12:16]).String()
				hops = append(hops, map[string]any{
					"hop":     ttl,
					"address": respIP,
					"latency": latency,
					"error":   "",
				})

				// Check if we reached the target
				if respIP == targetIP {
					return hops, nil
				}
			}
		case <-time.After(timeout):
			hops = append(hops, map[string]any{
				"hop":     ttl,
				"address": "",
				"latency": timeout.Milliseconds(),
				"error":   "timeout",
			})
		}
	}

	return hops, nil
}

func tcpTraceroute(targetIP string, port int, timeout time.Duration, maxHops int) ([]map[string]any, error) {

	hops := []map[string]any{}

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_TCP)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	// Set socket options
	syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_TTL, 1)
	syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, int(timeout.Milliseconds()))

	for ttl := 1; ttl <= maxHops; ttl++ {
		// Set TTL for this hop
		syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_TTL, ttl)

		// Create TCP SYN packet
		tcpHeader := make([]byte, 20)
		tcpHeader[0] = byte(port >> 8) // Source port
		tcpHeader[1] = byte(port)
		tcpHeader[2] = byte(port >> 8) // Dest port
		tcpHeader[3] = byte(port)
		tcpHeader[4] = 0 // Sequence number
		tcpHeader[5] = 0
		tcpHeader[6] = 0
		tcpHeader[7] = 0
		tcpHeader[8] = 0 // Ack number
		tcpHeader[9] = 0
		tcpHeader[10] = 0
		tcpHeader[11] = 0
		tcpHeader[12] = 0x50 // Data offset
		tcpHeader[13] = 0x02 // SYN flag
		tcpHeader[14] = 0x00 // Window size
		tcpHeader[15] = 0x00
		tcpHeader[16] = 0x00 // Checksum
		tcpHeader[17] = 0x00
		tcpHeader[18] = 0x00 // Urgent pointer
		tcpHeader[19] = 0x00

		// Send packet
		addr := &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}
		copy(addr.Addr[:], net.ParseIP(targetIP).To4())

		start := time.Now()
		err = syscall.Sendto(fd, tcpHeader, 0, addr)
		if err != nil {
			hops = append(hops, map[string]any{
				"hop":     ttl,
				"address": "",
				"latency": 0,
				"error":   fmt.Sprintf("send error: %v", err),
			})
			continue
		}

		// Receive response with timeout
		resp := make([]byte, 1024)
		received := make(chan struct{})
		var n int
		var recvErr error

		go func() {
			n, _, recvErr = syscall.Recvfrom(fd, resp, 0)
			close(received)
		}()

		select {
		case <-received:
			latency := time.Since(start).Milliseconds()
			if recvErr != nil {
				hops = append(hops, map[string]any{
					"hop":     ttl,
					"address": "",
					"latency": latency,
					"error":   "timeout",
				})
				continue
			}

			// Parse response
			if n >= 20 {
				respIP := net.IP(resp[12:16]).String()
				hops = append(hops, map[string]any{
					"hop":     ttl,
					"address": respIP,
					"latency": latency,
					"error":   "",
				})

				// Check if we reached the target
				if respIP == targetIP {
					return hops, nil
				}
			}
		case <-time.After(timeout):
			hops = append(hops, map[string]any{
				"hop":     ttl,
				"address": "",
				"latency": timeout.Milliseconds(),
				"error":   "timeout",
			})
		}
	}

	return hops, nil

}

// Network monitoring functions for Unix-like systems

// Get all network connections via netlink/sysctl
func getNetworkConnections() ([]map[string]any, error) {
	var allConnections []map[string]any

	// Get TCP connections via syscalls
	tcp, err := getTcpConnections()
	if err != nil {
		fmt.Printf("[ERROR] getTcpConnections: %v\n", err)
	} else {
		allConnections = append(allConnections, tcp...)
	}

	// Get UDP connections via syscalls
	udp, err := getUdpConnections()
	if err != nil {
		fmt.Printf("[ERROR] getUdpConnections: %v\n", err)
	} else {
		allConnections = append(allConnections, udp...)
	}

	// Get Unix domain sockets via syscalls
	unix, err := getUnixConnections()
	if err != nil {
		fmt.Printf("[ERROR] getUnixConnections: %v\n", err)
	} else {
		allConnections = append(allConnections, unix...)
	}

	return allConnections, nil
}

// Get TCP connections via netlink sockets
func getTcpConnections() ([]map[string]any, error) {

	// Try netlink first
	netlinkConnections, err := getTcpConnectionsNetlink()
	if err != nil {
		// Fallback to basic syscall approach
		return getTcpConnectionsFallback()
	}

	return netlinkConnections, nil
}

// Get TCP connections via netlink sockets (privileged)
func getTcpConnectionsNetlink() ([]map[string]any, error) {
	// Create netlink socket
	sock, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, NETLINK_INET_DIAG)
	if err != nil {
		return nil, fmt.Errorf("failed to create netlink socket: %v", err)
	}
	defer syscall.Close(sock)

	// Bind socket
	addr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pid:    uint32(os.Getpid()),
	}
	err = syscall.Bind(sock, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind netlink socket: %v", err)
	}

	// Prepare request
	req := InetDiagReq{
		Family:   syscall.AF_INET,
		Protocol: syscall.IPPROTO_TCP,
		States:   0xffffffff, // All states
	}

	// Send request
	err = syscall.Sendmsg(sock, (*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:], nil, addr, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to send netlink request: %v", err)
	}

	// Receive response
	buf := make([]byte, 4096)
	n, _, err := syscall.Recvfrom(sock, buf, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to receive netlink response: %v", err)
	}

	// Parse response
	var connections []map[string]any
	offset := 0
	for offset < n {
		if offset+16 > n {
			break
		}

		// Parse netlink header
		msgLen := binary.LittleEndian.Uint32(buf[offset:])
		msgType := binary.LittleEndian.Uint16(buf[offset+4:])
		_ = binary.LittleEndian.Uint16(buf[offset+6:])  // flags
		_ = binary.LittleEndian.Uint32(buf[offset+8:])  // seq
		_ = binary.LittleEndian.Uint32(buf[offset+12:]) // netlinkPid

		if msgType == syscall.NLMSG_ERROR {
			return nil, fmt.Errorf("netlink error")
		}

		if msgType == syscall.NLMSG_DONE {
			break
		}

		// Parse inet diag message
		if offset+int(msgLen) > n {
			break
		}

		diagMsg := (*InetDiagMsg)(unsafe.Pointer(&buf[offset+16]))

		// Convert addresses and ports
		localAddr := bytesToIP(diagMsg.ID.IDiagSrc[:4])
		remoteAddr := bytesToIP(diagMsg.ID.IDiagDst[:4])
		localPort := binary.BigEndian.Uint16(diagMsg.ID.IDiagSPort[:])
		remotePort := binary.BigEndian.Uint16(diagMsg.ID.IDiagDPort[:])

		// Get process info
		pid, processName := getProcessByInode(diagMsg.Inode)

		// Determine state
		stateStr := getTcpStateString(uint32(diagMsg.IDiagState))

		// Get interface name
		interfaceName := getInterfaceName(diagMsg.ID.IDiagIf)

		// Get username from UID
		username := getUsername(diagMsg.UID)

		connections = append(connections, map[string]any{
			"protocol":      "tcp",
			"local_addr":    localAddr,
			"local_port":    int(localPort),
			"remote_addr":   remoteAddr,
			"remote_port":   int(remotePort),
			"state":         stateStr,
			"pid":           int(pid),
			"process":       processName,
			"interface":     interfaceName,
			"inode":         int(diagMsg.Inode),
			"uid":           int(diagMsg.UID),
			"username":      username,
			"receive_queue": int(diagMsg.RQueue),
			"send_queue":    int(diagMsg.WQueue),
		})

		offset += int(msgLen)
	}

	return connections, nil
}

// Get TCP connections via fallback method (unprivileged)
func getTcpConnectionsFallback() ([]map[string]any, error) {
	var connections []map[string]any

	// Read /proc/net/tcp for TCP connections
	data, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/net/tcp: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Parse local address:port
		localAddrPort := fields[1]
		localParts := strings.Split(localAddrPort, ":")
		if len(localParts) != 2 {
			continue
		}

		localAddrHex := localParts[0]
		localPortHex := localParts[1]

		// Parse remote address:port
		remoteAddrPort := fields[2]
		remoteParts := strings.Split(remoteAddrPort, ":")
		if len(remoteParts) != 2 {
			continue
		}

		remoteAddrHex := remoteParts[0]
		remotePortHex := remoteParts[1]

		// Parse state
		stateHex := fields[3]

		// Convert hex addresses to IP
		localAddr := hexToIP(localAddrHex)
		remoteAddr := hexToIP(remoteAddrHex)

		// Convert hex ports to int
		localPort := hexToInt(localPortHex)
		remotePort := hexToInt(remotePortHex)

		// Convert hex state to string
		state := hexToTcpState(stateHex)

		// Get inode for process lookup
		inode := 0
		if len(fields) > 9 {
			inode = hexToInt(fields[9])
		}

		// Get process info
		pid, processName := getProcessByInode(uint32(inode))

		// Get interface name
		interfaceName := getInterfaceNameFromAddr(localAddr)

		// Parse queue information if available
		receiveQueue := 0
		sendQueue := 0
		if len(fields) > 4 {
			receiveQueue = hexToInt(fields[4])
		}
		if len(fields) > 5 {
			sendQueue = hexToInt(fields[5])
		}

		// Get UID if available (field 7)
		uid := 0
		if len(fields) > 7 {
			uid = hexToInt(fields[7])
		}

		// Get username from UID
		username := getUsername(uint32(uid))

		connections = append(connections, map[string]any{
			"protocol":      "tcp",
			"local_addr":    localAddr,
			"local_port":    localPort,
			"remote_addr":   remoteAddr,
			"remote_port":   remotePort,
			"state":         state,
			"pid":           pid,
			"process":       processName,
			"interface":     interfaceName,
			"inode":         inode,
			"uid":           uid,
			"username":      username,
			"receive_queue": receiveQueue,
			"send_queue":    sendQueue,
		})
	}

	return connections, nil
}

// Get UDP connections via netlink sockets
func getUdpConnections() ([]map[string]any, error) {

	// Try netlink first
	netlinkConnections, err := getUdpConnectionsNetlink()
	if err != nil {
		// Fallback to basic syscall approach
		return getUdpConnectionsFallback()
	}

	return netlinkConnections, nil
}

// Get UDP connections via netlink sockets (privileged)
func getUdpConnectionsNetlink() ([]map[string]any, error) {
	// Create netlink socket
	sock, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, NETLINK_INET_DIAG)
	if err != nil {
		return nil, fmt.Errorf("failed to create netlink socket: %v", err)
	}
	defer syscall.Close(sock)

	// Bind socket
	addr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pid:    uint32(os.Getpid()),
	}
	err = syscall.Bind(sock, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind netlink socket: %v", err)
	}

	// Prepare request for UDP
	req := InetDiagReq{
		Family:   syscall.AF_INET,
		Protocol: syscall.IPPROTO_UDP,
		States:   0xffffffff, // All states
	}

	// Send request
	err = syscall.Sendmsg(sock, (*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:], nil, addr, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to send netlink request: %v", err)
	}

	// Receive response
	buf := make([]byte, 4096)
	n, _, err := syscall.Recvfrom(sock, buf, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to receive netlink response: %v", err)
	}

	// Parse response
	var connections []map[string]any
	offset := 0
	for offset < n {
		if offset+16 > n {
			break
		}

		// Parse netlink header
		msgLen := binary.LittleEndian.Uint32(buf[offset:])
		msgType := binary.LittleEndian.Uint16(buf[offset+4:])

		if msgType == syscall.NLMSG_ERROR {
			return nil, fmt.Errorf("netlink error")
		}

		if msgType == syscall.NLMSG_DONE {
			break
		}

		// Parse inet diag message
		if offset+int(msgLen) > n {
			break
		}

		diagMsg := (*InetDiagMsg)(unsafe.Pointer(&buf[offset+16]))

		// Convert addresses and ports
		localAddr := bytesToIP(diagMsg.ID.IDiagSrc[:4])
		remoteAddr := bytesToIP(diagMsg.ID.IDiagDst[:4])
		localPort := binary.BigEndian.Uint16(diagMsg.ID.IDiagSPort[:])
		remotePort := binary.BigEndian.Uint16(diagMsg.ID.IDiagDPort[:])

		// Get process info
		pid, processName := getProcessByInode(diagMsg.Inode)

		// Get interface name
		interfaceName := getInterfaceName(0) // Fallback to default for fallback

		// Get username from UID
		username := getUsername(diagMsg.UID)

		connections = append(connections, map[string]any{
			"protocol":      "udp",
			"local_addr":    localAddr,
			"local_port":    int(localPort),
			"remote_addr":   remoteAddr,
			"remote_port":   int(remotePort),
			"state":         "LISTEN",
			"pid":           int(pid),
			"process":       processName,
			"interface":     interfaceName,
			"inode":         int(diagMsg.Inode),
			"uid":           int(diagMsg.UID),
			"username":      username,
			"receive_queue": int(diagMsg.RQueue),
			"send_queue":    int(diagMsg.WQueue),
		})

		offset += int(msgLen)
	}

	return connections, nil
}

// Get UDP connections via fallback method (unprivileged)
func getUdpConnectionsFallback() ([]map[string]any, error) {
	var connections []map[string]any

	// Read /proc/net/udp for UDP connections
	data, err := os.ReadFile("/proc/net/udp")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/net/udp: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Parse local address:port
		localAddrPort := fields[1]
		localParts := strings.Split(localAddrPort, ":")
		if len(localParts) != 2 {
			continue
		}

		localAddrHex := localParts[0]
		localPortHex := localParts[1]

		// Parse remote address:port
		remoteAddrPort := fields[2]
		remoteParts := strings.Split(remoteAddrPort, ":")
		if len(remoteParts) != 2 {
			continue
		}

		remoteAddrHex := remoteParts[0]
		remotePortHex := remoteParts[1]

		// Convert hex addresses to IP
		localAddr := hexToIP(localAddrHex)
		remoteAddr := hexToIP(remoteAddrHex)

		// Convert hex ports to int
		localPort := hexToInt(localPortHex)
		remotePort := hexToInt(remotePortHex)

		// Get inode for process lookup
		inode := 0
		if len(fields) > 9 {
			inode = hexToInt(fields[9])
		}

		// Get process info
		pid, processName := getProcessByInode(uint32(inode))

		// Get interface name
		interfaceName := getInterfaceNameFromAddr(localAddr)

		// Parse queue information if available
		receiveQueue := 0
		sendQueue := 0
		if len(fields) > 4 {
			receiveQueue = hexToInt(fields[4])
		}
		if len(fields) > 5 {
			sendQueue = hexToInt(fields[5])
		}

		// Get UID if available (field 7)
		uid := 0
		if len(fields) > 7 {
			uid = hexToInt(fields[7])
		}

		// Get username from UID
		username := getUsername(uint32(uid))

		connections = append(connections, map[string]any{
			"protocol":      "udp",
			"local_addr":    localAddr,
			"local_port":    localPort,
			"remote_addr":   remoteAddr,
			"remote_port":   remotePort,
			"state":         "LISTEN",
			"pid":           pid,
			"process":       processName,
			"interface":     interfaceName,
			"inode":         inode,
			"uid":           uid,
			"username":      username,
			"receive_queue": receiveQueue,
			"send_queue":    sendQueue,
		})
	}

	return connections, nil
}

// Get Unix domain sockets via netlink sockets
func getUnixConnections() ([]map[string]any, error) {

	// Try netlink first
	netlinkConnections, err := getUnixConnectionsNetlink()
	if err != nil {
		// Fallback to basic syscall approach
		return getUnixConnectionsFallback()
	}

	return netlinkConnections, nil
}

// Get Unix domain sockets via netlink sockets (privileged)
func getUnixConnectionsNetlink() ([]map[string]any, error) {
	// Create netlink socket for Unix domain socket diagnostics
	sock, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, NETLINK_INET_DIAG)
	if err != nil {
		return nil, fmt.Errorf("failed to create netlink socket: %v", err)
	}
	defer syscall.Close(sock)

	// Bind socket
	addr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Pid:    uint32(os.Getpid()),
	}
	err = syscall.Bind(sock, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind netlink socket: %v", err)
	}

	// Prepare request for Unix domain sockets
	req := InetDiagReq{
		Family:   syscall.AF_UNIX,
		Protocol: 0,          // Unix sockets don't use protocol field
		States:   0xffffffff, // All states
	}

	// Send request
	err = syscall.Sendmsg(sock, (*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:], nil, addr, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to send netlink request: %v", err)
	}

	// Receive response
	buf := make([]byte, 4096)
	n, _, err := syscall.Recvfrom(sock, buf, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to receive netlink response: %v", err)
	}

	// Parse response
	var connections []map[string]any
	offset := 0
	for offset < n {
		if offset+16 > n {
			break
		}

		// Parse netlink header
		msgLen := binary.LittleEndian.Uint32(buf[offset:])
		msgType := binary.LittleEndian.Uint16(buf[offset+4:])

		if msgType == syscall.NLMSG_ERROR {
			return nil, fmt.Errorf("netlink error")
		}

		if msgType == syscall.NLMSG_DONE {
			break
		}

		// Parse inet diag message
		if offset+int(msgLen) > n {
			break
		}

		diagMsg := (*InetDiagMsg)(unsafe.Pointer(&buf[offset+16]))

		// For Unix sockets, we need to parse the path from the socket ID
		// The path is stored in the IDiagSrc field for Unix sockets
		path := parseUnixPath(diagMsg.ID.IDiagSrc[:])

		// Get process info
		pid, processName := getProcessByInode(diagMsg.Inode)

		// Determine state
		stateStr := getUnixStateString(uint32(diagMsg.IDiagState))

		// Get interface name
		interfaceName := getInterfaceName(0) // Fallback to default for fallback

		// Get username from UID
		username := getUsername(diagMsg.UID)

		connections = append(connections, map[string]any{
			"protocol":      "unix",
			"local_addr":    path,
			"local_port":    0,
			"remote_addr":   "",
			"remote_port":   0,
			"state":         stateStr,
			"pid":           int(pid),
			"process":       processName,
			"interface":     interfaceName,
			"inode":         int(diagMsg.Inode),
			"uid":           int(diagMsg.UID),
			"username":      username,
			"receive_queue": int(diagMsg.RQueue),
			"send_queue":    int(diagMsg.WQueue),
		})

		offset += int(msgLen)
	}

	return connections, nil
}

// Get Unix domain sockets via fallback method (unprivileged)
func getUnixConnectionsFallback() ([]map[string]any, error) {
	var connections []map[string]any

	// Read /proc/net/unix for Unix domain sockets
	data, err := os.ReadFile("/proc/net/unix")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/net/unix: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}

		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		// Parse socket type
		socketTypeHex := fields[4]
		socketType := hexToInt(socketTypeHex)

		// Parse state
		stateHex := fields[5]
		state := hexToUnixState(stateHex)

		// Parse inode
		inode := 0
		if len(fields) > 6 {
			inode = hexToInt(fields[6])
		}

		// Parse path (if available)
		path := ""
		if len(fields) > 7 {
			path = fields[7]
		}

		// Get process info
		pid, processName := getProcessByInode(uint32(inode))

		// Determine socket type string
		socketTypeStr := "unknown"
		switch socketType {
		case 1:
			socketTypeStr = "stream"
		case 2:
			socketTypeStr = "dgram"
		case 3:
			socketTypeStr = "raw"
		case 4:
			socketTypeStr = "rdm"
		case 5:
			socketTypeStr = "seqpacket"
		}

		// Get interface name
		interfaceName := "unix" // Unix sockets don't use network interfaces

		// Get UID if available (field 8)
		uid := 0
		if len(fields) > 8 {
			uid = hexToInt(fields[8])
		}

		// Get username from UID
		username := getUsername(uint32(uid))

		connections = append(connections, map[string]any{
			"protocol":      "unix",
			"local_addr":    path,
			"local_port":    0,
			"remote_addr":   "",
			"remote_port":   0,
			"state":         state,
			"pid":           pid,
			"process":       processName,
			"interface":     interfaceName,
			"socket_type":   socketTypeStr,
			"inode":         inode,
			"uid":           uid,
			"username":      username,
			"receive_queue": 0, // Unix sockets don't have queue info in /proc/net/unix
			"send_queue":    0, // Unix sockets don't have queue info in /proc/net/unix
		})
	}

	return connections, nil
}

// Get available protocols for Unix-like systems
func getAvailableProtocols() []string {
	var protocols []string

	// Always available
	protocols = append(protocols, "tcp", "udp")

	// Unix domain sockets (Unix-like systems)
	protocols = append(protocols, "unix")

	// Raw sockets (test availability)
	if canUseRawSockets() {
		protocols = append(protocols, "raw")
	}

	// IPv6 versions
	protocols = append(protocols, "tcp6", "udp6")

	return protocols
}

// Get protocol information for Unix-like systems
func getProtocolInfo() map[string]any {
	info := map[string]any{
		"platform": "linux",
		"protocols": map[string]any{
			"tcp": map[string]any{
				"available":           true,
				"description":         "Transmission Control Protocol",
				"source":              "netlink",
				"requires_privileges": false,
			},
			"udp": map[string]any{
				"available":           true,
				"description":         "User Datagram Protocol",
				"source":              "netlink",
				"requires_privileges": false,
			},
			"unix": map[string]any{
				"available":           true,
				"description":         "Unix Domain Sockets",
				"source":              "netlink",
				"requires_privileges": false,
			},
			"raw": map[string]any{
				"available":           canUseRawSockets(),
				"description":         "Raw Sockets",
				"source":              "netlink",
				"requires_privileges": true,
				"reason":              getRawSocketReason(),
			},
		},
	}

	return info
}

// Get open files for Unix-like systems
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

// Helper functions for Unix-like systems
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

// Helper functions for Linux network monitoring

// Convert bytes to IP string
func bytesToIP(b []byte) string {
	if len(b) < 4 {
		return "0.0.0.0"
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
}

// Get process name by PID
func getProcessName(pid int) string {
	commFile := fmt.Sprintf("/proc/%d/comm", pid)
	data, err := os.ReadFile(commFile)
	if err != nil {
		return fmt.Sprintf("process-%d", pid)
	}

	name := strings.TrimSpace(string(data))
	if name == "" {
		return fmt.Sprintf("process-%d", pid)
	}

	return name
}

// Convert TCP state to string
func getTcpStateString(state uint32) string {
	switch state {
	case 1:
		return "ESTABLISHED"
	case 2:
		return "SYN_SENT"
	case 3:
		return "SYN_RECV"
	case 4:
		return "FIN_WAIT1"
	case 5:
		return "FIN_WAIT2"
	case 6:
		return "TIME_WAIT"
	case 7:
		return "CLOSE"
	case 8:
		return "CLOSE_WAIT"
	case 9:
		return "LAST_ACK"
	case 10:
		return "LISTEN"
	case 11:
		return "CLOSING"
	default:
		return "UNKNOWN"
	}
}

// Parse Unix socket path from socket ID
func parseUnixPath(idBytes []byte) string {
	// Unix socket paths are stored as null-terminated strings in the socket ID
	// Find the null terminator
	for i, b := range idBytes {
		if b == 0 {
			return string(idBytes[:i])
		}
	}
	return string(idBytes)
}

// Convert Unix socket state to string
func getUnixStateString(state uint32) string {
	switch state {
	case 1:
		return "ESTABLISHED"
	case 2:
		return "LISTEN"
	default:
		return "UNKNOWN"
	}
}

// Helper functions for hex conversion

// Convert hex IP address to string
func hexToIP(hexAddr string) string {
	// Remove leading zeros and convert to int
	addrInt, err := strconv.ParseUint(hexAddr, 16, 32)
	if err != nil {
		return "0.0.0.0"
	}

	// Convert to bytes (little-endian on Linux)
	addrBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(addrBytes, uint32(addrInt))

	return fmt.Sprintf("%d.%d.%d.%d", addrBytes[0], addrBytes[1], addrBytes[2], addrBytes[3])
}

// Convert hex port to int
func hexToInt(hexStr string) int {
	val, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil {
		return 0
	}
	return int(val)
}

// Convert hex TCP state to string
func hexToTcpState(hexState string) string {
	state, err := strconv.ParseUint(hexState, 16, 32)
	if err != nil {
		return "UNKNOWN"
	}

	switch state {
	case 1:
		return "ESTABLISHED"
	case 2:
		return "SYN_SENT"
	case 3:
		return "SYN_RECV"
	case 4:
		return "FIN_WAIT1"
	case 5:
		return "FIN_WAIT2"
	case 6:
		return "TIME_WAIT"
	case 7:
		return "CLOSE"
	case 8:
		return "CLOSE_WAIT"
	case 9:
		return "LAST_ACK"
	case 10:
		return "LISTEN"
	case 11:
		return "CLOSING"
	default:
		return "UNKNOWN"
	}
}

// Convert hex Unix socket state to string
func hexToUnixState(hexState string) string {
	state, err := strconv.ParseUint(hexState, 16, 32)
	if err != nil {
		return "UNKNOWN"
	}

	switch state {
	case 1:
		return "ESTABLISHED"
	case 2:
		return "LISTEN"
	case 3:
		return "CONNECTING"
	case 4:
		return "DISCONNECTING"
	default:
		return "UNKNOWN"
	}
}

// Improved process lookup using /proc/[pid]/net/tcp and /proc/[pid]/net/udp
func getProcessByInode(inode uint32) (int, string) {
	if inode == 0 {
		return 0, "unknown"
	}

	// First try to find the process by scanning /proc/net files
	// This is much faster than scanning all /proc/[pid]/fd directories
	pid, processName := findProcessByInodeFromNetFiles(inode)
	if pid > 0 {
		return pid, processName
	}

	// Fallback: scan /proc for processes (slower but more comprehensive)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, "unknown"
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pidStr := entry.Name()
		if pidStr == "." || pidStr == ".." {
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		count++
		if count > 100 { // Limit scanning to avoid performance issues
			break
		}

		// Check if this process has the socket inode
		if hasInode(pid, inode) {
			processName := getProcessName(pid)
			return pid, processName
		}
	}

	return 0, "unknown"
}

// Find process by inode from /proc/net files (much faster)
func findProcessByInodeFromNetFiles(inode uint32) (int, string) {
	// Scan all processes and check their /proc/[pid]/net/tcp and /proc/[pid]/net/udp
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, "unknown"
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pidStr := entry.Name()
		if pidStr == "." || pidStr == ".." {
			continue
		}

		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Check if this process has the inode in its network files
		if hasInodeInNetFiles(pid, inode) {
			processName := getProcessName(pid)
			return pid, processName
		}
	}

	return 0, "unknown"
}

// Check if process has inode in its network files
func hasInodeInNetFiles(pid int, inode uint32) bool {
	// Check /proc/[pid]/net/tcp
	if hasInodeInNetFile(pid, "tcp", inode) {
		return true
	}

	// Check /proc/[pid]/net/udp
	if hasInodeInNetFile(pid, "udp", inode) {
		return true
	}

	// Check /proc/[pid]/net/unix
	if hasInodeInNetFile(pid, "unix", inode) {
		return true
	}

	return false
}

// Check if process has inode in a specific network file
func hasInodeInNetFile(pid int, netType string, inode uint32) bool {
	filename := fmt.Sprintf("/proc/%d/net/%s", pid, netType)
	data, err := os.ReadFile(filename)
	if err != nil {
		return false
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header and empty lines
		}

		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		// Parse inode (field 9 for tcp/udp, field 6 for unix)
		inodeField := 9
		if netType == "unix" {
			inodeField = 6
		}

		if len(fields) <= inodeField {
			continue
		}

		fileInode := hexToInt(fields[inodeField])
		if uint32(fileInode) == inode {
			return true
		}
	}

	return false
}

// Check if process has socket with given inode (fallback method)
func hasInode(pid int, inode uint32) bool {
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		linkPath := fmt.Sprintf("/proc/%d/fd/%s", pid, entry.Name())
		linkTarget, err := os.Readlink(linkPath)
		if err != nil {
			continue
		}

		// Check if it's a socket with our inode
		if strings.HasPrefix(linkTarget, "socket:[") {
			inodeStr := strings.TrimSuffix(strings.TrimPrefix(linkTarget, "socket:["), "]")
			if inodeStr == fmt.Sprintf("%d", inode) {
				return true
			}
		}
	}

	return false
}

// Get network interface name by index
func getInterfaceName(ifIndex uint32) string {
	if ifIndex == 0 {
		return "lo" // Default to loopback for index 0
	}

	// Read /proc/net/dev to get interface names
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.Contains(line, "Inter-|") || strings.Contains(line, "face |") {
			continue // Skip header lines
		}

		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

		// Interface name is the first field (with colon)
		ifName := strings.TrimSuffix(fields[0], ":")

		// Get interface index using syscall
		ifIndexFromName := getInterfaceIndex(ifName)
		if ifIndexFromName == ifIndex {
			return ifName
		}
	}

	return "unknown"
}

// Get interface name from local address (fallback method)
func getInterfaceNameFromAddr(localAddr string) string {
	if localAddr == "127.0.0.1" || localAddr == "127.0.0.53" || localAddr == "127.0.0.54" {
		return "lo" // Loopback interface
	}

	if localAddr == "0.0.0.0" {
		return "all" // All interfaces
	}

	// Try to determine interface from address
	// This is a simplified approach - in a real implementation you'd use routing tables
	return "unknown"
}

// Get interface index by name using syscall
func getInterfaceIndex(ifName string) uint32 {
	// Try to get interface index using socket ioctl
	sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return 0
	}
	defer syscall.Close(sock)

	// Use SIOCGIFINDEX ioctl to get interface index
	var ifreq struct {
		Name  [16]byte
		Index int32
	}
	copy(ifreq.Name[:], ifName)

	// Use ioctl to get interface index - this may not be available on all systems
	// so we'll use a fallback approach
	var index uint32
	if ioctl(sock, syscall.SIOCGIFINDEX, uintptr(unsafe.Pointer(&ifreq))) == nil {
		index = uint32(ifreq.Index)
	} else {
		// Fallback: try to find interface by scanning /proc/net/dev
		index = findInterfaceIndexByName(ifName)
	}

	return index
}

// Fallback function to find interface index by name
func findInterfaceIndexByName(ifName string) uint32 {
	// Read /proc/net/dev to get interface names and try to match
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0
	}

	lines := strings.Split(string(data), "\n")
	index := uint32(1) // Start with index 1 (lo is usually 1)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.Contains(line, "Inter-|") || strings.Contains(line, "face |") {
			continue // Skip header lines
		}

		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

		// Interface name is the first field (with colon)
		name := strings.TrimSuffix(fields[0], ":")
		if name == ifName {
			return index
		}
		index++
	}

	return 0
}

// Get username from UID
func getUsername(uid uint32) string {
	// Try to get username from /etc/passwd
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return fmt.Sprintf("user_%d", uid)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}

		username := fields[0]
		uidStr := fields[2]

		if parsedUID, err := strconv.ParseUint(uidStr, 10, 32); err == nil {
			if uint32(parsedUID) == uid {
				return username
			}
		}
	}

	return fmt.Sprintf("user_%d", uid)
}
