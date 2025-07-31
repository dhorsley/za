//go:build !test
// +build !test

package main

import (
    "crypto/tls"
    "fmt"
    "io"
    "math/rand"
    "net"
    "net/http"
    "os"
    "os/exec"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "syscall"
    "time"
)

// Network connection handles
type tcpClientHandle struct {
    conn      net.Conn
    host      string
    port      int
    timeout   time.Duration
    handleID  string
    connected bool
}

type tcpServerHandle struct {
    listener  net.Listener
    port      int
    mode      string // "blocking" or "non_blocking"
    running   bool
    handleID  string
    clients   map[string]net.Conn
    clientMux sync.RWMutex
}

// Global handle storage
var (
    tcpClients = make(map[string]*tcpClientHandle)
    tcpServers = make(map[string]*tcpServerHandle)
    handleMux  sync.RWMutex
    handleID   int64
)

// Generate unique handle ID (following ZA pattern)
func generateHandleID() string {
    b := make([]byte, 16)
    _, err := rand.Read(b)
    if err != nil {
        // Fallback to simple counter if random fails
        handleID++
        return fmt.Sprintf("tcp_%d", handleID)
    }
    return fmt.Sprintf("tcp_%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Check if an error is a permission-related error
func isPermissionError(err error) bool {
    if err == nil {
        return false
    }

    // Check for syscall errors that indicate permission issues
    if syscallErr, ok := err.(*os.SyscallError); ok {
        switch syscallErr.Err {
        case syscall.EACCES, syscall.EPERM, syscall.EINVAL:
            return true
        }
    }

    // Check for direct syscall errors
    switch err {
    case syscall.EACCES, syscall.EPERM, syscall.EINVAL:
        return true
    }

    return false
}

// Check if running with sufficient privileges
func hasPrivileges() bool {
    if runtime.GOOS == "windows" {
        // Windows: try to open admin-only resource
        _, err := os.Open("\\.\\PHYSICALDRIVE0")
        return err == nil
    }

    // Unix-like: check UID and sudo environment
    if os.Geteuid() == 0 {
        return true
    }

    // Check for sudo environment variables
    if os.Getenv("SUDO_UID") != "" || os.Getenv("SUDO_USER") != "" {
        return true
    }

    // Check for OpenBSD doas
    if os.Getenv("DOAS_USER") != "" {
        return true
    }

    return false
}

// Helper function to get platform-specific certificate installation instructions
func getCertificateInstallInstructions(host string, port int) map[string]any {
    instructions := map[string]any{
        "platform": runtime.GOOS,
        "host":     host,
        "port":     port,
    }

    switch runtime.GOOS {
    case "linux":
        instructions["steps"] = []string{
            "1. Obtain the CA certificate file (.crt or .pem format) from your administrator",
            "2. Copy the certificate to the system CA directory:",
            "   sudo cp your-ca.crt /usr/local/share/ca-certificates/",
            "3. Update the CA certificate store:",
            "   sudo update-ca-certificates",
            "4. Verify the certificate is installed:",
            "   openssl verify -CAfile /etc/ssl/certs/ca-certificates.crt your-ca.crt",
        }
        instructions["alternative_methods"] = []string{
            "Alternative 1 - User-specific certificates:",
            "   mkdir -p ~/.local/share/ca-certificates/",
            "   cp your-ca.crt ~/.local/share/ca-certificates/",
            "   update-ca-certificates --fresh",
            "",
            "Alternative 2 - Environment variable:",
            "   export SSL_CERT_FILE=/path/to/your/ca-bundle.crt",
            "   # Add this to your ~/.bashrc or ~/.zshrc",
        }
        instructions["common_locations"] = []string{
            "/etc/ssl/certs/ca-certificates.crt",
            "/etc/pki/tls/certs/ca-bundle.crt",
            "/usr/share/ca-certificates/",
        }

    case "windows":
        instructions["steps"] = []string{
            "1. Obtain the CA certificate file (.crt or .cer format) from your administrator",
            "2. Open Certificate Manager as Administrator:",
            "   certmgr.msc",
            "3. Navigate to 'Trusted Root Certification Authorities' > 'Certificates'",
            "4. Right-click and select 'All Tasks' > 'Import'",
            "5. Follow the Certificate Import Wizard",
            "6. Select 'Place all certificates in the following store'",
            "7. Browse to 'Trusted Root Certification Authorities'",
            "8. Complete the import",
        }
        instructions["alternative_methods"] = []string{
            "Alternative 1 - PowerShell (run as Administrator):",
            "   Import-Certificate -FilePath 'your-ca.crt' -CertStoreLocation 'Cert:\\LocalMachine\\Root'",
            "",
            "Alternative 2 - Environment variable:",
            "   set SSL_CERT_FILE=C:\\path\\to\\your\\ca-bundle.crt",
            "   # Add this to your system environment variables",
        }
        instructions["common_locations"] = []string{
            "Cert:\\LocalMachine\\Root",
            "Cert:\\CurrentUser\\Root",
            "Cert:\\LocalMachine\\CA",
        }

    case "darwin": // macOS
        instructions["steps"] = []string{
            "1. Obtain the CA certificate file (.crt or .pem format) from your administrator",
            "2. Double-click the certificate file to open it in Keychain Access",
            "3. In Keychain Access, select 'System' from the left sidebar",
            "4. Click the '+' button to add the certificate",
            "5. Browse and select your CA certificate file",
            "6. Set 'Trust' to 'Always Trust' for SSL",
            "7. Close Keychain Access",
        }
        instructions["alternative_methods"] = []string{
            "Alternative 1 - Command line (requires admin):",
            "   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain your-ca.crt",
            "",
            "Alternative 2 - User keychain:",
            "   security add-trusted-cert -d -r trustRoot -k ~/Library/Keychains/login.keychain-db your-ca.crt",
            "",
            "Alternative 3 - Environment variable:",
            "   export SSL_CERT_FILE=/path/to/your/ca-bundle.crt",
            "   # Add this to your ~/.zshrc or ~/.bash_profile",
        }
        instructions["common_locations"] = []string{
            "/System/Library/Keychains/SystemRootCertificates.keychain",
            "~/Library/Keychains/login.keychain-db",
            "/Library/Keychains/System.keychain",
        }

    default:
        instructions["steps"] = []string{
            "1. Obtain the CA certificate from your administrator",
            "2. Add the certificate to your system's trusted certificate store",
            "3. The exact method depends on your operating system",
        }
        instructions["note"] = "Platform-specific instructions not available for " + runtime.GOOS
    }

    instructions["troubleshooting"] = []string{
        "If you continue to have issues:",
        "1. Verify the certificate file format is correct (.crt, .pem, .cer)",
        "2. Ensure you have administrator/root privileges",
        "3. Check that the certificate is not expired",
        "4. Verify the certificate chain is complete",
        "5. Try restarting your application after installing the certificate",
    }

    return instructions
}

// Helper function to implement the actual certificate validation
func ssl_cert_validate_impl(host string, port int) map[string]any {
    // First, try with proper verification
    conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", host, port), &tls.Config{
        InsecureSkipVerify: false, // Use system CA store for verification
    })

    if err != nil {
        // If verification fails, try again with insecure to get certificate info
        conn, err = tls.Dial("tcp", fmt.Sprintf("%s:%d", host, port), &tls.Config{
            InsecureSkipVerify: true,
        })

        if err != nil {
            return map[string]any{
                "host":  host,
                "port":  port,
                "valid": false,
                "error": "Cannot connect: " + err.Error(),
            }
        }

        // Connection succeeded but verification failed
        state := conn.ConnectionState()
        if len(state.PeerCertificates) == 0 {
            return map[string]any{
                "host":  host,
                "port":  port,
                "valid": false,
                "error": "No certificate found",
            }
        }

        cert := state.PeerCertificates[0]
        now := time.Now()

        // Analyze certificate chain
        var chainInfo []map[string]any
        var chainError string = ""

        // Check each certificate in the chain
        for i, cert := range state.PeerCertificates {
            certInfo := map[string]any{
                "position":   i,
                "subject":    cert.Subject.CommonName,
                "issuer":     cert.Issuer.CommonName,
                "not_before": cert.NotBefore.Unix(),
                "not_after":  cert.NotAfter.Unix(),
                "serial":     cert.SerialNumber.String(),
                "signature":  cert.SignatureAlgorithm.String(),
                "is_leaf":    i == 0,
                "is_root":    i == len(state.PeerCertificates)-1,
            }

            // Check if certificate is expired
            if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
                certInfo["valid"] = false
                certInfo["error"] = "Certificate expired or not yet valid"
            } else {
                certInfo["valid"] = true
            }

            chainInfo = append(chainInfo, certInfo)
        }

        // Determine why verification failed
        if len(state.PeerCertificates) < 2 {
            chainError = "Incomplete certificate chain - missing intermediate or root certificates"
        } else {
            chainError = "Certificate chain verification failed - unknown root or intermediate certificates"
        }

        return map[string]any{
            "host":         host,
            "port":         port,
            "valid":        false, // Always false when verification fails
            "verified":     false,
            "subject":      cert.Subject.CommonName,
            "issuer":       cert.Issuer.CommonName,
            "not_before":   cert.NotBefore.Unix(),
            "not_after":    cert.NotAfter.Unix(),
            "expires_in":   int(cert.NotAfter.Sub(now).Hours() / 24),
            "serial":       cert.SerialNumber.String(),
            "signature":    cert.SignatureAlgorithm.String(),
            "chain_length": len(state.PeerCertificates),
            "chain_info":   chainInfo,
            "chain_error":  chainError,
            "verification": "failed",
        }
    }
    defer conn.Close()

    // Certificate verification succeeded
    state := conn.ConnectionState()
    cert := state.PeerCertificates[0]
    now := time.Now()

    // Analyze verified certificate chain
    var chainInfo []map[string]any
    for i, cert := range state.PeerCertificates {
        certInfo := map[string]any{
            "position":   i,
            "subject":    cert.Subject.CommonName,
            "issuer":     cert.Issuer.CommonName,
            "not_before": cert.NotBefore.Unix(),
            "not_after":  cert.NotAfter.Unix(),
            "serial":     cert.SerialNumber.String(),
            "signature":  cert.SignatureAlgorithm.String(),
            "is_leaf":    i == 0,
            "is_root":    i == len(state.PeerCertificates)-1,
            "valid":      true, // All certificates in verified chain are valid
        }
        chainInfo = append(chainInfo, certInfo)
    }

    return map[string]any{
        "host":         host,
        "port":         port,
        "valid":        true,
        "verified":     true,
        "subject":      cert.Subject.CommonName,
        "issuer":       cert.Issuer.CommonName,
        "not_before":   cert.NotBefore.Unix(),
        "not_after":    cert.NotAfter.Unix(),
        "expires_in":   int(cert.NotAfter.Sub(now).Hours() / 24),
        "serial":       cert.SerialNumber.String(),
        "signature":    cert.SignatureAlgorithm.String(),
        "chain_length": len(state.PeerCertificates),
        "chain_info":   chainInfo,
        "chain_error":  "",
        "verification": "success",
    }
}

// Helper function to calculate ICMP checksum
func calculateICMPChecksum(data []byte) uint16 {
    var sum uint32
    for i := 0; i < len(data)-1; i += 2 {
        sum += uint32(data[i])<<8 + uint32(data[i+1])
    }
    if len(data)%2 == 1 {
        sum += uint32(data[len(data)-1]) << 8
    }
    sum = (sum >> 16) + (sum & 0xffff)
    sum += sum >> 16
    return uint16(^sum)
}

func buildNetworkLib() {
    features["network"] = Feature{version: 1, category: "network"}
    categories["network"] = []string{
        "tcp_client", "tcp_server", "tcp_close", "tcp_send", "tcp_receive", "tcp_available",
        "icmp_ping", "tcp_ping", "traceroute", "tcp_traceroute", "icmp_traceroute", "dns_resolve", "port_scan",
        "net_interfaces_detailed", "ssl_cert_validate", "ssl_cert_install_help", "http_headers", "http_benchmark",
        "network_stats", "tcp_server_accept", "tcp_server_stop",
        // Network monitoring functions
        "netstat", "netstat_protocols", "netstat_protocol_info", "netstat_protocol",
        "netstat_listen", "netstat_established", "netstat_process", "netstat_interface", "open_files",
    }

    // TCP Client functions
    slhelp["tcp_client"] = LibHelp{
        in:     "host, port, [timeout_seconds]",
        out:    "handle",
        action: "Creates TCP connection to host:port. Returns handle for subsequent operations.",
    }
    stdlib["tcp_client"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_client", args, 2,
            "2", "string", "int",
            "3", "string", "int", "int"); !ok {
            return "", err
        }

        host := args[0].(string)
        port := args[1].(int)
        timeout := 30 * time.Second

        if len(args) == 3 {
            timeout = time.Duration(args[2].(int)) * time.Second
        }

        // Validate port
        if port <= 0 || port > 65535 {
            return "", fmt.Errorf("port must be between 1 and 65535")
        }

        // Create connection
        conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
        if err != nil {
            return "", err
        }

        // Create handle
        handle := &tcpClientHandle{
            conn:      conn,
            host:      host,
            port:      port,
            timeout:   timeout,
            handleID:  generateHandleID(),
            connected: true,
        }

        // Store handle
        handleMux.Lock()
        tcpClients[handle.handleID] = handle
        handleMux.Unlock()

        return handle.handleID, nil
    }

    slhelp["tcp_close"] = LibHelp{
        in:     "handle",
        out:    "bool",
        action: "Closes TCP connection and frees handle.",
    }
    stdlib["tcp_close"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_close", args, 1, "1", "string"); !ok {
            return false, err
        }

        handleID := args[0].(string)

        handleMux.Lock()
        handle, exists := tcpClients[handleID]
        if !exists {
            handleMux.Unlock()
            return false, fmt.Errorf("invalid TCP client handle")
        }

        // Close connection
        handle.conn.Close()
        handle.connected = false

        // Remove from storage
        delete(tcpClients, handleID)
        handleMux.Unlock()

        return true, nil
    }

    slhelp["tcp_send"] = LibHelp{
        in:     "handle, data",
        out:    "bool",
        action: "Sends data over TCP connection.",
    }
    stdlib["tcp_send"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_send", args, 2, "2", "string", "string"); !ok {
            return false, err
        }

        handleID := args[0].(string)
        data := args[1].(string)

        handleMux.RLock()
        handle, exists := tcpClients[handleID]
        handleMux.RUnlock()

        if !exists || !handle.connected {
            return false, fmt.Errorf("invalid or disconnected TCP client handle")
        }

        // Send data
        _, err = handle.conn.Write([]byte(data))
        if err != nil {
            return false, err
        }

        return true, nil
    }

    slhelp["tcp_receive"] = LibHelp{
        in:     "handle, [timeout_seconds]",
        out:    "map",
        action: "Receives data from TCP connection. Returns map with content, available, and error fields.",
    }
    stdlib["tcp_receive"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_receive", args, 2,
            "1", "string",
            "2", "string", "int"); !ok {
            return nil, err
        }

        handleID := args[0].(string)
        timeout := 5 * time.Second

        if len(args) == 2 {
            timeout = time.Duration(args[1].(int)) * time.Second
        }

        handleMux.RLock()
        handle, exists := tcpClients[handleID]
        handleMux.RUnlock()

        if !exists || !handle.connected {
            return map[string]any{
                "content":   "",
                "available": false,
                "error":     "invalid or disconnected TCP client handle",
            }, nil
        }

        // Set read timeout
        handle.conn.SetReadDeadline(time.Now().Add(timeout))

        // Read data
        buffer := make([]byte, 4096)
        n, err := handle.conn.Read(buffer)

        if err != nil {
            if err == io.EOF {
                return map[string]any{
                    "content":   "",
                    "available": false,
                    "error":     "eof",
                }, nil
            }
            return map[string]any{
                "content":   "",
                "available": false,
                "error":     err.Error(),
            }, nil
        }

        return map[string]any{
            "content":   string(buffer[:n]),
            "available": true,
            "error":     "",
        }, nil
    }

    // TCP Server functions
    slhelp["tcp_server"] = LibHelp{
        in:     "port, [mode]",
        out:    "handle",
        action: "Starts a TCP server on the given port. Mode can be 'blocking' or 'non_blocking'. Returns handle.",
    }
    stdlib["tcp_server"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_server", args, 2,
            "1", "int",
            "2", "int", "string"); !ok {
            return "", err
        }
        port := args[0].(int)
        mode := "blocking"
        if len(args) == 2 {
            mode = args[1].(string)
        }
        if port <= 0 || port > 65535 {
            return "", fmt.Errorf("port must be between 1 and 65535")
        }
        ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
        if err != nil {
            return "", err
        }
        handle := &tcpServerHandle{
            listener: ln,
            port:     port,
            mode:     mode,
            running:  true,
            handleID: generateHandleID(),
            clients:  make(map[string]net.Conn),
        }
        handleMux.Lock()
        tcpServers[handle.handleID] = handle
        handleMux.Unlock()
        return handle.handleID, nil
    }

    slhelp["tcp_server_accept"] = LibHelp{
        in:     "handle, [timeout_seconds]",
        out:    "map",
        action: "Accepts a new client connection on the TCP server. Returns map with client handle and address.",
    }
    stdlib["tcp_server_accept"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_server_accept", args, 2,
            "1", "string",
            "2", "string", "int"); !ok {
            return nil, err
        }
        handleID := args[0].(string)
        timeout := 30 * time.Second
        if len(args) == 2 {
            timeout = time.Duration(args[1].(int)) * time.Second
        }
        handleMux.RLock()
        server, exists := tcpServers[handleID]
        handleMux.RUnlock()
        if !exists || !server.running {
            return map[string]any{"error": "invalid or stopped TCP server handle"}, nil
        }
        server.listener.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))
        conn, err := server.listener.Accept()
        if err != nil {
            return map[string]any{"error": err.Error()}, nil
        }
        clientHandleID := generateHandleID()
        handleMux.Lock()
        server.clients[clientHandleID] = conn
        handleMux.Unlock()
        return map[string]any{
            "client_handle": clientHandleID,
            "remote_addr":   conn.RemoteAddr().String(),
            "error":         "",
        }, nil
    }

    slhelp["tcp_server_stop"] = LibHelp{
        in:     "handle",
        out:    "bool",
        action: "Stops the TCP server and closes all client connections.",
    }
    stdlib["tcp_server_stop"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_server_stop", args, 1, "1", "string"); !ok {
            return false, err
        }
        handleID := args[0].(string)
        handleMux.Lock()
        server, exists := tcpServers[handleID]
        if !exists {
            handleMux.Unlock()
            return false, fmt.Errorf("invalid TCP server handle")
        }
        server.running = false
        server.listener.Close()
        for _, conn := range server.clients {
            conn.Close()
        }
        delete(tcpServers, handleID)
        handleMux.Unlock()
        return true, nil
    }

    slhelp["tcp_available"] = LibHelp{
        in:     "handle",
        out:    "bool",
        action: "Checks if TCP client handle is still connected.",
    }
    stdlib["tcp_available"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_available", args, 1, "1", "string"); !ok {
            return false, err
        }
        handleID := args[0].(string)
        handleMux.RLock()
        handle, exists := tcpClients[handleID]
        handleMux.RUnlock()
        if !exists {
            return false, nil
        }
        return handle.connected, nil
    }

    // ICMP Ping
    slhelp["icmp_ping"] = LibHelp{
        in:     "host, [timeout_seconds]",
        out:    "map",
        action: "Performs ICMP ping to host. Returns map with latency, success, and error fields.",
    }
    stdlib["icmp_ping"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("icmp_ping", args, 2,
            "1", "string",
            "2", "string", "int"); !ok {
            return nil, err
        }
        host := args[0].(string)
        timeout := 3 * time.Second
        if len(args) == 2 {
            timeout = time.Duration(args[1].(int)) * time.Second
        }
        // Resolve host to IP first
        ips, err := net.LookupHost(host)
        if err != nil {
            return map[string]any{"success": false, "latency": 0, "error": "DNS lookup failed: " + err.Error()}, nil
        }
        if len(ips) == 0 {
            return map[string]any{"success": false, "latency": 0, "error": "No IP addresses found for " + host}, nil
        }
        targetIP := ips[0]

        return icmpPing(targetIP, timeout)

        /*
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

        */

    }

    // TCP Ping
    slhelp["tcp_ping"] = LibHelp{
        in:     "host, port, [timeout_seconds]",
        out:    "map",
        action: "Performs TCP connect to host:port. Returns map with latency, success, and error fields.",
    }
    stdlib["tcp_ping"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_ping", args, 3,
            "2", "string", "int",
            "3", "string", "int", "int"); !ok {
            return nil, err
        }
        host := args[0].(string)
        port := args[1].(int)
        timeout := 3 * time.Second
        if len(args) == 3 {
            timeout = time.Duration(args[2].(int)) * time.Second
        }
        start := time.Now()
        conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
        if err != nil {
            return map[string]any{"success": false, "latency": 0, "error": err.Error()}, nil
        }
        conn.Close()
        latency := time.Since(start).Milliseconds()
        return map[string]any{"success": true, "latency": latency, "error": ""}, nil
    }

    // Universal Traceroute (calls ICMP or TCP based on protocol argument)
    slhelp["traceroute"] = LibHelp{
        in:     "protocol, host, [port], [max_hops], [timeout_seconds]",
        out:    "slice",
        action: "Performs traceroute to host. Protocol: 'icmp' or 'tcp'. For TCP, port is required. Returns slice of hop info.",
    }
    stdlib["traceroute"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("traceroute", args, 4,
            "2", "string", "string",
            "3", "string", "string", "int",
            "4", "string", "string", "int", "int",
            "5", "string", "string", "int", "int", "int"); !ok {
            return nil, err
        }

        protocol := args[0].(string)

        // Route to appropriate traceroute function based on protocol
        switch strings.ToLower(protocol) {
        case "icmp":
            // Check if we have privileges for ICMP traceroute
            if hasPrivileges() {
                return stdlib["icmp_traceroute"](ns, evalfs, ident, args[1:]...)
            } else {
                // No privileges, use simple TCP fallback
                // simple_tcp_traceroute expects: host, port, [max_hops], [timeout_seconds]
                // args[1:] contains: host, max_hops, timeout
                fallbackArgs := []any{args[1], 80} // host, port 80
                if len(args) >= 3 {
                    fallbackArgs = append(fallbackArgs, args[2]) // max_hops
                }
                if len(args) >= 4 {
                    fallbackArgs = append(fallbackArgs, args[3]) // timeout
                }
                return simple_tcp_traceroute(ns, evalfs, ident, fallbackArgs...)
            }
        case "tcp":
            // Check if we have privileges for TCP traceroute
            if hasPrivileges() {
                return stdlib["tcp_traceroute"](ns, evalfs, ident, args[1:]...)
            } else {
                // No privileges, use simple TCP fallback
                // args[1:] already contains: host, port, [max_hops], [timeout_seconds]
                return simple_tcp_traceroute(ns, evalfs, ident, args[1:]...)
            }
        default:
            return nil, fmt.Errorf("unsupported protocol '%s'. Use 'icmp' or 'tcp'", protocol)
        }
    }

    // ICMP Traceroute (RFC compliant, working implementation)
    slhelp["icmp_traceroute"] = LibHelp{
        in:     "host, [max_hops], [timeout_seconds]",
        out:    "slice",
        action: "Performs ICMP traceroute to host. Returns slice of hop info. Requires root privileges.",
    }
    stdlib["icmp_traceroute"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("icmp_traceroute", args, 3,
            "1", "string",
            "2", "string", "int",
            "3", "string", "int", "int"); !ok {
            return nil, err
        }
        host := args[0].(string)
        maxHops := 30
        timeout := 3 * time.Second
        if len(args) >= 2 {
            maxHops = args[1].(int)
        }
        if len(args) == 3 {
            timeout = time.Duration(args[2].(int)) * time.Second
        }

        // Resolve host to IP
        ips, err := net.LookupHost(host)
        if err != nil {
            return nil, err
        }
        if len(ips) == 0 {
            return nil, fmt.Errorf("no IP addresses found for %s", host)
        }

        targetIP := ips[0]

        return icmpTraceroute(targetIP, timeout, maxHops)

    }

    // TCP Traceroute (RFC compliant, working implementation)
    slhelp["tcp_traceroute"] = LibHelp{
        in:     "host, port, [max_hops], [timeout_seconds]",
        out:    "slice",
        action: "Performs a TCP-based traceroute to host:port. Returns slice of hop info.",
    }
    stdlib["tcp_traceroute"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tcp_traceroute", args, 4,
            "2", "string", "int",
            "3", "string", "int", "int",
            "4", "string", "int", "int", "int"); !ok {
            return nil, err
        }
        host := args[0].(string)
        port := args[1].(int)
        maxHops := 30
        timeout := 3 * time.Second
        if len(args) >= 3 {
            maxHops = args[2].(int)
        }
        if len(args) == 4 {
            timeout = time.Duration(args[3].(int)) * time.Second
        }

        // Resolve host to IP
        ips, err := net.LookupHost(host)
        if err != nil {
            return nil, err
        }
        if len(ips) == 0 {
            return nil, fmt.Errorf("no IP addresses found for %s", host)
        }
        targetIP := ips[0]

        return tcpTraceroute(targetIP, port, timeout, maxHops)

    }

    // DNS Resolve
    slhelp["dns_resolve"] = LibHelp{
        in:     "host, [record_type]",
        out:    "map",
        action: "Resolves host using specified DNS record type. Record types: A, AAAA, CNAME, MX, TXT, NS, PTR, SRV, ANY. Returns map with results.",
    }
    stdlib["dns_resolve"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("dns_resolve", args, 2,
            "1", "string",
            "2", "string", "string"); !ok {
            return nil, err
        }
        host := args[0].(string)
        recordType := "A"
        if len(args) == 2 {
            recordType = strings.ToUpper(args[1].(string))
        }

        result := make(map[string]any)
        result["host"] = host
        result["record_type"] = recordType

        switch recordType {
        case "A":
            ips, err := net.LookupHost(host)
            if err != nil {
                result["error"] = err.Error()
                result["records"] = []string{}
            } else {
                result["records"] = ips
                result["error"] = ""
            }

        case "AAAA":
            ips, err := net.LookupIP(host)
            if err != nil {
                result["error"] = err.Error()
                result["records"] = []string{}
            } else {
                var aaaaIPs []string
                for _, ip := range ips {
                    if ip.To4() == nil { // IPv6 only
                        aaaaIPs = append(aaaaIPs, ip.String())
                    }
                }
                result["records"] = aaaaIPs
                result["error"] = ""
            }

        case "CNAME":
            cname, err := net.LookupCNAME(host)
            if err != nil {
                result["error"] = err.Error()
                result["records"] = []string{}
            } else {
                result["records"] = []string{cname}
                result["error"] = ""
            }

        case "MX":
            mxs, err := net.LookupMX(host)
            if err != nil {
                result["error"] = err.Error()
                result["records"] = []string{}
            } else {
                var mxRecords []string
                for _, mx := range mxs {
                    mxRecords = append(mxRecords, fmt.Sprintf("%s %d", mx.Host, mx.Pref))
                }
                result["records"] = mxRecords
                result["error"] = ""
            }

        case "TXT":
            txts, err := net.LookupTXT(host)
            if err != nil {
                result["error"] = err.Error()
                result["records"] = []string{}
            } else {
                result["records"] = txts
                result["error"] = ""
            }

        case "NS":
            nss, err := net.LookupNS(host)
            if err != nil {
                result["error"] = err.Error()
                result["records"] = []string{}
            } else {
                var nsRecords []string
                for _, ns := range nss {
                    nsRecords = append(nsRecords, ns.Host)
                }
                result["records"] = nsRecords
                result["error"] = ""
            }

        case "PTR":
            // For PTR records, host should be an IP address
            names, err := net.LookupAddr(host)
            if err != nil {
                result["error"] = err.Error()
                result["records"] = []string{}
            } else {
                result["records"] = names
                result["error"] = ""
            }

        case "SRV":
            // SRV records require a specific format: _service._proto.name
            // If host doesn't start with _, we'll try common SRV patterns
            srvHost := host
            if !strings.HasPrefix(host, "_") {
                // Try common SRV patterns
                srvHost = "_sip._tcp." + host
            }

            // Extract service and protocol from the hostname
            // Format should be _service._proto.name
            parts := strings.Split(srvHost, ".")
            if len(parts) < 3 {
                result["error"] = "SRV hostname must be in format _service._proto.name"
                result["records"] = []string{}
                return result, nil
            }

            service := strings.TrimPrefix(parts[0], "_")
            proto := strings.TrimPrefix(parts[1], "_")
            name := strings.Join(parts[2:], ".")

            // Use net.LookupSRV for SRV records
            cname, srvs, err := net.LookupSRV(service, proto, name)
            if err != nil {
                result["error"] = err.Error()
                result["records"] = []string{}
            } else {
                var srvRecords []string
                for _, srv := range srvs {
                    srvRecords = append(srvRecords, fmt.Sprintf("%s:%d (priority: %d, weight: %d)", srv.Target, srv.Port, srv.Priority, srv.Weight))
                }
                if cname != "" {
                    srvRecords = append(srvRecords, fmt.Sprintf("CNAME: %s", cname))
                }
                result["records"] = srvRecords
                result["error"] = ""
            }

        case "ANY":
            // For ANY, we'll return all available record types
            anyResult := make(map[string]any)

            // A records
            if ips, err := net.LookupHost(host); err == nil {
                anyResult["A"] = ips
            }

            // AAAA records
            if ips, err := net.LookupIP(host); err == nil {
                var aaaaIPs []string
                for _, ip := range ips {
                    if ip.To4() == nil {
                        aaaaIPs = append(aaaaIPs, ip.String())
                    }
                }
                if len(aaaaIPs) > 0 {
                    anyResult["AAAA"] = aaaaIPs
                }
            }

            // CNAME
            if cnameRecord, err := net.LookupCNAME(host); err == nil && cnameRecord != "" {
                anyResult["CNAME"] = []string{cnameRecord}
            }

            // MX
            if mxs, err := net.LookupMX(host); err == nil {
                var mxRecords []string
                for _, mx := range mxs {
                    mxRecords = append(mxRecords, fmt.Sprintf("%s %d", mx.Host, mx.Pref))
                }
                anyResult["MX"] = mxRecords
            }

            // TXT
            if txts, err := net.LookupTXT(host); err == nil {
                anyResult["TXT"] = txts
            }

            // NS
            if nss, err := net.LookupNS(host); err == nil {
                var nsRecords []string
                for _, ns := range nss {
                    nsRecords = append(nsRecords, ns.Host)
                }
                anyResult["NS"] = nsRecords
            }

            // SRV (try common SRV patterns)
            if strings.HasPrefix(host, "_") {
                // Extract service and protocol from the hostname
                parts := strings.Split(host, ".")
                if len(parts) >= 3 {
                    service := strings.TrimPrefix(parts[0], "_")
                    proto := strings.TrimPrefix(parts[1], "_")
                    name := strings.Join(parts[2:], ".")

                    if srvCname, srvs, err := net.LookupSRV(service, proto, name); err == nil {
                        var srvRecords []string
                        for _, srv := range srvs {
                            srvRecords = append(srvRecords, fmt.Sprintf("%s:%d (priority: %d, weight: %d)", srv.Target, srv.Port, srv.Priority, srv.Weight))
                        }
                        if srvCname != "" {
                            srvRecords = append(srvRecords, fmt.Sprintf("CNAME: %s", srvCname))
                        }
                        anyResult["SRV"] = srvRecords
                    }
                }
            }

            result["records"] = anyResult
            result["error"] = ""

        default:
            result["error"] = fmt.Sprintf("unsupported record type '%s'. Supported types: A, AAAA, CNAME, MX, TXT, NS, PTR, SRV, ANY", recordType)
            result["records"] = []string{}
        }

        return result, nil
    }

    // Port Scan
    slhelp["port_scan"] = LibHelp{
        in:     "host, ports, [timeout_seconds]",
        out:    "map",
        action: "Scans ports on host. Returns map of port to open/closed.",
    }
    stdlib["port_scan"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("port_scan", args, 3,
            "2", "string", "[]any",
            "3", "string", "[]any", "int"); !ok {
            return nil, err
        }
        host := args[0].(string)
        ports := args[1].([]any)
        timeout := 1 * time.Second
        if len(args) == 3 {
            timeout = time.Duration(args[2].(int)) * time.Second
        }
        results := make(map[string]bool)
        for _, p := range ports {
            port, e := GetAsInt(p)
            if e {
                return nil, fmt.Errorf("invalid port specified '%v'",e)
            }
            conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
            if err == nil {
                results[strconv.Itoa(port)] = true
                conn.Close()
            } else {
                results[strconv.Itoa(port)] = false
            }
        }
        return results, nil
    }

    // Network Interfaces Detailed
    slhelp["net_interfaces_detailed"] = LibHelp{
        in:     "",
        out:    "slice",
        action: "Returns detailed info for all network interfaces.",
    }
    stdlib["net_interfaces_detailed"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("net_interfaces_detailed", args, 0); !ok {
            return nil, err
        }
        interfaces, err := net.Interfaces()
        if err != nil {
            return nil, err
        }
        var result []map[string]any
        for _, iface := range interfaces {
            addrs, _ := iface.Addrs()
            var ips []string
            for _, addr := range addrs {
                if ipnet, ok := addr.(*net.IPNet); ok {
                    ips = append(ips, ipnet.IP.String())
                }
            }
            result = append(result, map[string]any{
                "name":     iface.Name,
                "index":    iface.Index,
                "mtu":      iface.MTU,
                "hardware": iface.HardwareAddr.String(),
                "ips":      ips,
                "up":       iface.Flags&net.FlagUp != 0,
            })
        }
        return result, nil
    }

    // SSL Certificate Validation
    slhelp["ssl_cert_validate"] = LibHelp{
        in:     "host, [port]",
        out:    "map",
        action: "Validates SSL certificate for host:port. Returns certificate information.",
    }
    stdlib["ssl_cert_validate"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("ssl_cert_validate", args, 2,
            "1", "string",
            "2", "string", "int"); !ok {
            return nil, err
        }
        host := args[0].(string)
        port := 443
        if len(args) == 2 {
            port = args[1].(int)
        }
        return ssl_cert_validate_impl(host, port), nil
    }

    // SSL Certificate Validation with Install Help
    slhelp["ssl_cert_install_help"] = LibHelp{
        in:     "host, [port]",
        out:    "map",
        action: "Validates SSL certificate and provides platform-specific installation instructions if verification fails.",
    }
    stdlib["ssl_cert_install_help"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("ssl_cert_install_help", args, 2,
            "1", "string",
            "2", "string", "int"); !ok {
            return nil, err
        }
        host := args[0].(string)
        port := 443
        if len(args) == 2 {
            port = args[1].(int)
        }
        validationResult := ssl_cert_validate_impl(host, port)
        if validationResult["verified"] == false {
            instructions := getCertificateInstallInstructions(host, port)
            validationResult["install_instructions"] = instructions
        }
        return validationResult, nil
    }

    // HTTP Header Inspection
    slhelp["http_headers"] = LibHelp{
        in:     "url, [headers]",
        out:    "map",
        action: "Inspects HTTP headers for URL. Returns response headers and status.",
    }
    stdlib["http_headers"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("http_headers", args, 2,
            "1", "string",
            "2", "string", "[]any"); !ok {
            return nil, err
        }
        url := args[0].(string)
        headerFilter := make(map[string]bool)
        if len(args) == 2 {
            headersList := args[1].([]any)
            for _, header := range headersList {
                if headerStr, ok := header.(string); ok {
                    headerFilter[strings.ToLower(strings.TrimSpace(headerStr))] = true
                }
            }
        }
        req, err := http.NewRequest("HEAD", url, nil)
        if err != nil {
            return map[string]any{"url": url, "error": err.Error(), "status": 0, "headers": map[string]string{}}, nil
        }
        client := &http.Client{Timeout: 10 * time.Second}
        resp, err := client.Do(req)
        if err != nil {
            return map[string]any{"url": url, "error": err.Error(), "status": 0, "headers": map[string]string{}}, nil
        }
        defer resp.Body.Close()
        headers := make(map[string]string)
        for k, v := range resp.Header {
            // If no filter specified, include all headers
            // If filter specified, only include headers in the filter list
            if len(headerFilter) == 0 || headerFilter[strings.ToLower(k)] {
                headers[k] = v[0]
            }
        }
        return map[string]any{"url": url, "status": resp.StatusCode, "headers": headers, "error": ""}, nil
    }

    // HTTP Benchmarking
    slhelp["http_benchmark"] = LibHelp{
        in:     "url, requests, [concurrent], [keep_alive], [skip_verify]",
        out:    "map",
        action: "Benchmarks HTTP requests to URL. Returns performance statistics with error breakdown.",
    }
    stdlib["http_benchmark"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("http_benchmark", args, 5,
            "2", "string", "int",
            "3", "string", "int", "int",
            "4", "string", "int", "int", "bool",
            "5", "string", "int", "int", "bool", "bool"); !ok {
            return nil, err
        }
        url := args[0].(string)
        requests := args[1].(int)
        concurrent := 1
        keepAlive := true
        skipVerify := true
        if len(args) >= 3 {
            concurrent = args[2].(int)
        }
        if len(args) >= 4 {
            keepAlive = args[3].(bool)
        }
        if len(args) >= 5 {
            skipVerify = args[4].(bool)
        }
        if requests <= 0 || concurrent <= 0 {
            return nil, fmt.Errorf("requests and concurrent must be positive")
        }
        transport := &http.Transport{
            DisableKeepAlives: !keepAlive,
            TLSClientConfig: &tls.Config{
                InsecureSkipVerify: skipVerify,
            },
        }
        client := &http.Client{
            Transport: transport,
            Timeout:   30 * time.Second,
        }
        var latencies []int64
        var errors int
        var success int
        var errorTypes map[string]int
        var statusCodes map[int]int
        errorTypes = make(map[string]int)
        statusCodes = make(map[int]int)
        sem := make(chan struct{}, concurrent)
        var wg sync.WaitGroup
        var mux sync.Mutex
        start := time.Now()
        for i := 0; i < requests; i++ {
            wg.Add(1)
            go func() {
                defer wg.Done()
                sem <- struct{}{}
                defer func() { <-sem }()
                reqStart := time.Now()
                resp, err := client.Get(url)
                latency := time.Since(reqStart)
                mux.Lock()
                if err != nil {
                    errors++
                    errStr := err.Error()
                    if strings.Contains(errStr, "timeout") {
                        errorTypes["timeout"]++
                    } else if strings.Contains(errStr, "connection refused") {
                        errorTypes["connection_refused"]++
                    } else if strings.Contains(errStr, "certificate") {
                        errorTypes["certificate_error"]++
                    } else if strings.Contains(errStr, "no route to host") {
                        errorTypes["no_route"]++
                    } else {
                        errorTypes["other"]++
                    }
                } else {
                    success++
                    statusCodes[resp.StatusCode]++
                    resp.Body.Close()
                }
                latencies = append(latencies, latency.Milliseconds())
                mux.Unlock()
            }()
        }
        wg.Wait()
        totalTime := time.Since(start)
        var totalLatency int64
        var minLatency int64 = 999999
        var maxLatency int64
        for _, lat := range latencies {
            totalLatency += lat
            if lat < minLatency {
                minLatency = lat
            }
            if lat > maxLatency {
                maxLatency = lat
            }
        }
        avgLatency := int64(0)
        if len(latencies) > 0 {
            avgLatency = totalLatency / int64(len(latencies))
        }
        errorRates := make(map[string]float64)
        for errType, count := range errorTypes {
            errorRates[errType] = float64(count) / float64(requests) * 100.0
        }
        statusCodeRates := make(map[int]float64)
        for statusCode, count := range statusCodes {
            statusCodeRates[statusCode] = float64(count) / float64(requests) * 100.0
        }
        return map[string]any{
            "url":               url,
            "total_requests":    requests,
            "successful":        success,
            "errors":            errors,
            "total_time":        totalTime.Milliseconds(),
            "avg_latency":       avgLatency,
            "min_latency":       minLatency,
            "max_latency":       maxLatency,
            "requests_per_sec":  float64(requests) / float64(totalTime.Seconds()),
            "error_types":       errorTypes,
            "error_rates":       errorRates,
            "status_codes":      statusCodes,
            "status_code_rates": statusCodeRates,
            "keep_alive":        keepAlive,
            "skip_verify":       skipVerify,
            "concurrent":        concurrent,
        }, nil
    }

    // Network Statistics
    slhelp["network_stats"] = LibHelp{
        in:     "",
        out:    "map",
        action: "Returns network interface statistics (if available).",
    }
    stdlib["network_stats"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("network_stats", args, 0); !ok {
            return nil, err
        }
        stats := make(map[string]any)
        if runtime.GOOS == "linux" {
            data, err := os.ReadFile("/proc/net/dev")
            if err != nil {
                return map[string]any{"error": "Cannot read /proc/net/dev: " + err.Error()}, nil
            }
            lines := strings.Split(string(data), "\n")
            for _, line := range lines[2:] {
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
                stats[interfaceName] = map[string]any{
                    "rx_bytes":   rxBytes,
                    "tx_bytes":   txBytes,
                    "rx_packets": rxPackets,
                    "tx_packets": txPackets,
                    "rx_errors":  rxErrors,
                    "tx_errors":  txErrors,
                    "platform":   "linux",
                }
            }
        } else if runtime.GOOS == "windows" {
            interfaces, err := net.Interfaces()
            if err != nil {
                return map[string]any{"error": "Cannot get network interfaces: " + err.Error()}, nil
            }
            for _, iface := range interfaces {
                addrs, err := iface.Addrs()
                if err != nil {
                    continue
                }
                var ips []string
                for _, addr := range addrs {
                    if ipnet, ok := addr.(*net.IPNet); ok {
                        ips = append(ips, ipnet.IP.String())
                    }
                }
                stats[iface.Name] = map[string]any{
                    "name":     iface.Name,
                    "index":    iface.Index,
                    "mtu":      iface.MTU,
                    "hardware": iface.HardwareAddr.String(),
                    "ips":      ips,
                    "up":       iface.Flags&net.FlagUp != 0,
                    "platform": "windows",
                    "note":     "Detailed stats require Windows API calls",
                }
            }
        } else {
            interfaces, err := net.Interfaces()
            if err != nil {
                return map[string]any{"error": "Cannot get network interfaces: " + err.Error()}, nil
            }
            for _, iface := range interfaces {
                addrs, err := iface.Addrs()
                if err != nil {
                    continue
                }
                var ips []string
                for _, addr := range addrs {
                    if ipnet, ok := addr.(*net.IPNet); ok {
                        ips = append(ips, ipnet.IP.String())
                    }
                }
                stats[iface.Name] = map[string]any{
                    "name":     iface.Name,
                    "index":    iface.Index,
                    "mtu":      iface.MTU,
                    "hardware": iface.HardwareAddr.String(),
                    "ips":      ips,
                    "up":       iface.Flags&net.FlagUp != 0,
                    "platform": runtime.GOOS,
                    "note":     "Detailed stats require platform-specific syscalls",
                }
            }
        }
        return stats, nil
    }

    // Network monitoring functions
    slhelp["netstat"] = LibHelp{
        in:     "",
        out:    "[]map",
        action: "Returns all network connections across all available protocols.",
    }
    stdlib["netstat"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("netstat", args, 0, "0"); !ok {
            return nil, err
        }

        connections, err := getNetworkConnections()
        if err != nil {
            return nil, err
        }

        return connections, nil
    }

    slhelp["netstat_protocols"] = LibHelp{
        in:     "",
        out:    "[]string",
        action: "Returns list of available protocols for network monitoring.",
    }
    stdlib["netstat_protocols"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("netstat_protocols", args, 0, "0"); !ok {
            return nil, err
        }

        return getAvailableProtocols(), nil
    }

    slhelp["netstat_protocol_info"] = LibHelp{
        in:     "",
        out:    "map",
        action: "Returns detailed information about available protocols and their capabilities.",
    }
    stdlib["netstat_protocol_info"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("netstat_protocol_info", args, 0, "0"); !ok {
            return nil, err
        }

        return getProtocolInfo(), nil
    }

    slhelp["netstat_protocol"] = LibHelp{
        in:     "protocol",
        out:    "[]map",
        action: "Returns network connections filtered by specific protocol.",
    }
    stdlib["netstat_protocol"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("netstat_protocol", args, 1, "1", "string"); !ok {
            return nil, err
        }

        protocol := args[0].(string)
        connections, err := getNetworkConnections()
        if err != nil {
            return nil, err
        }

        var filtered []map[string]any
        for _, conn := range connections {
            if conn["protocol"] == strings.ToLower(protocol) {
                filtered = append(filtered, conn)
            }
        }

        return filtered, nil
    }

    slhelp["netstat_listen"] = LibHelp{
        in:     "",
        out:    "[]map",
        action: "Returns network connections in LISTEN state.",
    }
    stdlib["netstat_listen"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("netstat_listen", args, 0, "0"); !ok {
            return nil, err
        }

        return netstatFilter(map[string]any{"state": "LISTEN"})
    }

    slhelp["netstat_established"] = LibHelp{
        in:     "",
        out:    "[]map",
        action: "Returns network connections in ESTABLISHED state.",
    }
    stdlib["netstat_established"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("netstat_established", args, 0, "0"); !ok {
            return nil, err
        }

        return netstatFilter(map[string]any{"state": "ESTABLISHED"})
    }

    slhelp["netstat_process"] = LibHelp{
        in:     "pid",
        out:    "[]map",
        action: "Returns network connections for specific process ID.",
    }
    stdlib["netstat_process"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("netstat_process", args, 1, "1", "int"); !ok {
            return nil, err
        }

        pid := args[0].(int)
        return netstatFilter(map[string]any{"pid": pid})
    }

    slhelp["netstat_interface"] = LibHelp{
        in:     "interface",
        out:    "[]map",
        action: "Returns network connections on specific interface.",
    }
    stdlib["netstat_interface"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("netstat_interface", args, 1, "1", "string"); !ok {
            return nil, err
        }

        iface := args[0].(string)
        return netstatFilter(map[string]any{"interface": iface})
    }

    slhelp["open_files"] = LibHelp{
        in:     "",
        out:    "[]map",
        action: "Returns file descriptors and network connections (lsof equivalent).",
    }
    stdlib["open_files"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("open_files", args, 0, "0"); !ok {
            return nil, err
        }

        files, err := getOpenFiles()
        if err != nil {
            return nil, err
        }
        return files, nil
    }

    slhelp["has_privileges"] = LibHelp{
        in:     "",
        out:    "bool",
        action: "Returns true if the current process has elevated privileges (root/sudo/admin).",
    }
    stdlib["has_privileges"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("has_privileges", args, 0); !ok {
            return nil, err
        }

        return hasPrivileges(), nil
    }

}

// Internal helper function for simple TCP traceroute (no raw sockets required)
func simple_tcp_traceroute(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
    if ok, err := expect_args("simple_tcp_traceroute", args, 4,
        "2", "string", "int",
        "3", "string", "int", "int",
        "4", "string", "int", "int", "int"); !ok {
        return nil, err
    }
    host := args[0].(string)
    port := args[1].(int)
    maxHops := 30
    timeout := 3 * time.Second
    if len(args) >= 3 {
        maxHops = args[2].(int)
    }
    if len(args) == 4 {
        timeout = time.Duration(args[3].(int)) * time.Second
    }

    hops := []map[string]any{}

    // Try to connect directly first
    start := time.Now()
    conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
    if err == nil {
        latency := time.Since(start).Milliseconds()
        hops = append(hops, map[string]any{
            "hop":     1,
            "address": host,
            "latency": latency,
            "error":   "",
        })
        conn.Close()
        return hops, nil
    }

    // If direct connection fails, try to find intermediate hops
    // This is a simplified approach that tries common gateway patterns
    commonGateways := []string{
        "192.168.1.1",
        "192.168.0.1",
        "10.0.0.1",
        "172.16.0.1",
    }

    // Try to get default gateway from routing table
    if runtime.GOOS != "windows" {
        // On Unix-like systems, try to get default gateway
        cmd := exec.Command("ip", "route", "show", "default")
        if output, err := cmd.Output(); err == nil {
            lines := strings.Split(string(output), "\n")
            for _, line := range lines {
                if strings.Contains(line, "default via") {
                    fields := strings.Fields(line)
                    for i, field := range fields {
                        if field == "via" && i+1 < len(fields) {
                            gateway := fields[i+1]
                            // Add this gateway to the beginning of our list
                            commonGateways = append([]string{gateway}, commonGateways...)
                            break
                        }
                    }
                    break
                }
            }
        }
    }

    for i, gateway := range commonGateways {
        if i >= maxHops {
            break
        }

        start := time.Now()
        conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", gateway, port), timeout)
        if err == nil {
            latency := time.Since(start).Milliseconds()
            hops = append(hops, map[string]any{
                "hop":     i + 1,
                "address": gateway,
                "latency": latency,
                "error":   "",
            })
            conn.Close()
        } else {
            latency := time.Since(start).Milliseconds()
            hops = append(hops, map[string]any{
                "hop":     i + 1,
                "address": gateway,
                "latency": latency,
                "error":   "timeout",
            })
        }
    }

    return hops, nil
}

// Network monitoring helper functions

// Filter network connections
func netstatFilter(filters map[string]any) ([]map[string]any, error) {
    connections, err := getNetworkConnections()
    if err != nil {
        return nil, err
    }

    var filtered []map[string]any
    for _, conn := range connections {
        if matchesFilters(conn, filters) {
            filtered = append(filtered, conn)
        }
    }

    return filtered, nil
}

// Check if connection matches filters
func matchesFilters(conn map[string]any, filters map[string]any) bool {
    for key, value := range filters {
        if connValue, exists := conn[key]; !exists || connValue != value {
            return false
        }
    }
    return true
}
