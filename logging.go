package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// LogRequest represents a single logging request
type LogRequest struct {
	Message     string
	Fields      map[string]any // Snapshot of current fields
	IsJSON      bool           // Format at time of request
	IsError     bool           // Error vs normal log
	IsWebAccess bool           // Web access log vs main log
	SourceLine  int16          // For error logs
	Timestamp   time.Time      // When request was made
	DestFile    string         // Specific destination file (empty = use default)
	HTTPStatus  int            // For web access logs (0 = not HTTP)
	Level       int            // RFC 5424 log level (0-7)
}

// Global logging queue system
var logQueue chan LogRequest
var logWorkerRunning bool
var queueFullWarned bool

// logLevelToString converts log level number to string name
func logLevelToString(level int) string {
	switch level {
	case LOG_EMERG:
		return "emerg"
	case LOG_ALERT:
		return "alert"
	case LOG_CRIT:
		return "crit"
	case LOG_ERR:
		return "error"
	case LOG_WARNING:
		return "warn"
	case LOG_NOTICE:
		return "notice"
	case LOG_INFO:
		return "info"
	case LOG_DEBUG:
		return "debug"
	default:
		return "unknown"
	}
} // Track if we've already warned about full queue

// getLoggingFormatString returns the current logging format as a string
// This centralizes format detection to make adding new formats easier
func getLoggingFormatString() string {
	if jsonLoggingEnabled {
		return "JSON"
	}
	// Future formats can be added here:
	// if csvLoggingEnabled { return "CSV" }
	// if networkLoggingEnabled { return "Network" }
	// if xmlLoggingEnabled { return "XML" }
	return "Plain text"
}

// getLoggingStateString returns the current logging state with formatting
func getLoggingStateString() string {
	if loggingEnabled {
		return sparkle("[#4]ENABLED[#-]")
	}
	return sparkle("[#2]DISABLED[#-]")
}

// getErrorLoggingStateString returns the error logging state with formatting
func getErrorLoggingStateString() string {
	if errorLoggingEnabled {
		return sparkle("[#4]ENABLED[#-]")
	}
	return sparkle("[#2]DISABLED[#-]")
}

// getQueueWorkerStateString returns the queue worker state with formatting
func getQueueWorkerStateString() string {
	if logWorkerRunning {
		return sparkle("[#4]RUNNING[#-]")
	}
	return sparkle("[#2]STOPPED[#-]")
}

// getMemoryReserveStateString returns the memory reserve state with formatting
func getMemoryReserveStateString() string {
	if enhancedErrorsEnabled && emergencyMemoryReserve != nil {
		return sparkle("[#4]active[#-]")
	}
	return sparkle("[#2]inactive[#-]")
}

// startLogWorker starts the background logging worker
func startLogWorker() {
	fmt.Fprintf(os.Stderr, "DEBUG: startLogWorker called - logWorkerRunning=%v\n", logWorkerRunning)
	if logWorkerRunning {
		fmt.Fprintf(os.Stderr, "DEBUG: startLogWorker EARLY_RETURN - worker already running\n")
		return
	}
	fmt.Fprintf(os.Stderr, "DEBUG: startLogWorker creating queue with size %d\n", logQueueSize)
	logQueue = make(chan LogRequest, logQueueSize) // Use configurable size
	logWorkerRunning = true
	queueFullWarned = false
	fmt.Fprintf(os.Stderr, "DEBUG: startLogWorker about to launch goroutine\n")

	go func() {
		fmt.Fprintf(os.Stderr, "DEBUG: Log worker goroutine started\n")
		for request := range logQueue {
			fmt.Fprintf(os.Stderr, "DEBUG: Log worker received request from queue: msg='%s'\n", request.Message)
			processLogRequest(request)
			fmt.Fprintf(os.Stderr, "DEBUG: Log worker completed processing request\n")
		}
		fmt.Fprintf(os.Stderr, "DEBUG: Log worker goroutine exiting\n")
		logWorkerRunning = false
	}()
	fmt.Fprintf(os.Stderr, "DEBUG: startLogWorker goroutine launched, returning\n")
}

// stopLogWorker stops the background logging worker
func stopLogWorker() {
	if logQueue != nil {
		close(logQueue)
		logQueue = nil
	}
}

// queueLogRequest sends a log request to the queue with full detection
func queueLogRequest(request LogRequest) {
	fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest START - msg='%s' IsError=%v IsWebAccess=%v\n", request.Message, request.IsError, request.IsWebAccess)

	// Skip queuing if logging is disabled (unless it's web access or error logging)
	if !loggingEnabled && !request.IsWebAccess && !request.IsError {
		fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest EARLY_RETURN - logging disabled\n")
		return
	}

	if !logWorkerRunning {
		fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest starting log worker\n")
		startLogWorker()
	}

	// Check memory reserve status for critical operations
	memoryConstrained := false
	if enhancedErrorsEnabled && emergencyMemoryReserve != nil && emergencyReserveSize > 0 {
		// We have memory monitoring enabled
		// For web access logs, be more aggressive about dropping under memory pressure
		queueUsage := len(logQueue)
		if request.IsWebAccess && queueUsage > logQueueSize*3/4 {
			memoryConstrained = true
		}
	}

	fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest about to try select - queue len=%d size=%d\n", len(logQueue), logQueueSize)

	select {
	case logQueue <- request:
		// Sent successfully
		fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest SUCCESS - request sent to queue\n")
		queueFullWarned = false // Reset warning flag when queue flows again

	case <-time.After(100 * time.Millisecond):
		fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest TIMEOUT - queue is full\n")
		// Queue is full - apply memory-aware handling
		if memoryConstrained && request.IsWebAccess {
			// Drop this web access log request to preserve memory
			fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest dropping web access request due to memory constraint\n")
			return
		}

		// Queue is full, log a warning (but only once per episode)
		if !queueFullWarned {
			queueFullWarned = true

			// Send warning directly to log file (bypass queue to avoid recursion)
			warningMessage := fmt.Sprintf("WARNING: Logging queue full (size: %d). Consider increasing queue size with LOGGING QUEUE SIZE if this persists.", logQueueSize)
			fmt.Fprintf(os.Stderr, "DEBUG: Writing queue full warning directly via plog_direct: %s\n", warningMessage)
			plog_direct(warningMessage) // Write directly to file
			fmt.Fprintf(os.Stderr, "DEBUG: plog_direct call completed\n")
		}

		// For critical logs (errors, main logs), still try to queue
		// For non-critical web access logs, try once more then drop
		if request.IsError || !request.IsWebAccess {
			fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest blocking on queue for critical log - IsError=%v IsWebAccess=%v\n", request.IsError, request.IsWebAccess)
			logQueue <- request // Block until space available for critical logs
			fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest critical log successfully queued after blocking\n")
		} else {
			// For web access logs, try once more without blocking
			fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest trying non-blocking retry for web access log\n")
			select {
			case logQueue <- request:
				// Sent successfully
				fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest web access log sent on retry\n")
				return
			default:
				// Drop the web access request to prevent blocking
				fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest dropping web access log\n")
				return
			}
		}
	}
	fmt.Fprintf(os.Stderr, "DEBUG: queueLogRequest COMPLETE\n")
}

// processLogRequest handles a single log request
func processLogRequest(request LogRequest) {
	fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest START - msg='%s' level=%d\n", request.Message, request.Level)

	if !loggingEnabled && !request.IsWebAccess {
		fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest EARLY_RETURN - logging disabled\n")
		return
	}

	// Apply log level filtering (lower numbers = higher priority)
	if request.Level > logMinLevel {
		fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest EARLY_RETURN - level filtered\n")
		return // Skip this log entry - level too low
	}

	fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest AFTER_FILTERING\n")

	// Update statistics
	if request.IsWebAccess {
		webLogRequestCount++
	} else {
		mainLogRequestCount++
	}

	// Determine destination file
	destFile := logFile // Default to main log file
	if request.IsWebAccess && request.DestFile != "" {
		destFile = request.DestFile
	} else if request.DestFile != "" {
		destFile = request.DestFile
	}

	fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest AFTER_DEST_FILE - destFile='%s'\n", destFile)

	// Check rotation for the appropriate file
	if request.IsWebAccess && request.DestFile != "" {
		checkAndRotateWebLog(request.DestFile)
	} else {
		checkAndRotateLog()
	}

	fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest AFTER_ROTATION\n")

	// Handle enhanced error logging for HTTP errors (3xx/4xx/5xx)
	if request.IsWebAccess && request.HTTPStatus >= 300 {
		request.IsError = true
		if request.Fields == nil {
			request.Fields = make(map[string]any)
		}
		request.Fields["http_status"] = request.HTTPStatus
		if request.HTTPStatus >= 400 {
			request.Fields["level"] = "ERROR"
		} else {
			request.Fields["level"] = "WARNING" // 3xx redirects
		}
	}

	fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest BEFORE_WRITE - isJSON=%v\n", request.IsJSON)

	if request.IsJSON {
		// Always include level field in JSON output
		if request.Fields == nil {
			request.Fields = make(map[string]any)
		}
		request.Fields["level"] = logLevelToString(request.Level)

		if request.IsError && !request.IsWebAccess {
			// Add error-specific fields for non-web errors
			if request.SourceLine > 0 {
				request.Fields["source_line"] = request.SourceLine
			}
		}

		// Write to appropriate destination
		if request.IsWebAccess && destFile != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest CALLING plog_json_direct_to_file\n")
			plog_json_direct_to_file(destFile, request.Message, request.Fields)
		} else {
			fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest CALLING plog_json_direct\n")
			plog_json_direct(request.Message, request.Fields)
		}
	} else {
		// Plain text logging
		var message string
		if request.IsError && request.SourceLine >= 0 {
			message = fmt.Sprintf("ERROR (line %d): %s", request.SourceLine, request.Message)
		} else if request.IsError {
			message = fmt.Sprintf("ERROR: %s", request.Message)
		} else {
			message = request.Message
		}

		// Write to appropriate destination
		if request.IsWebAccess && destFile != "" {
			fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest CALLING plog_direct_to_file\n")
			plog_direct_to_file(destFile, message)
		} else {
			fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest CALLING plog_direct\n")
			plog_direct(message)
		}
	}

	fmt.Fprintf(os.Stderr, "DEBUG: processLogRequest END\n")
}

// validateLogFilePath checks if a log file path is safe and writable
// Returns the expanded/resolved path and any error
func validateLogFilePath(path string) (string, error) {
	// Handle tilde expansion for ~/path
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~ in path: %v", err)
		}
		path = filepath.Join(homeDir, path[2:]) // Remove "~/" and join with home
	}

	// Resolve to absolute path for security checks (allows relative input)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	// Note: We used to block working directory logging, but that was overly restrictive.
	// Developers should be free to organize their logs as they see fit.

	// OS-specific forbidden paths
	var forbiddenPrefixes []string
	var rootCheck string

	switch runtime.GOOS {
	case "windows":
		forbiddenPrefixes = []string{
			"C:\\Temp\\",
			"C:\\Windows\\Temp\\",
			"C:\\Windows\\",
			"C:\\Windows\\System32\\",
			"C:\\Program Files\\",
			"C:\\Program Files (x86)\\",
		}
		// Add environment temp directories
		if tempDir := os.Getenv("TEMP"); tempDir != "" {
			forbiddenPrefixes = append(forbiddenPrefixes, tempDir+"\\")
		}
		if tempDir := os.Getenv("TMP"); tempDir != "" {
			forbiddenPrefixes = append(forbiddenPrefixes, tempDir+"\\")
		}

		// Prevent root drive files (C:\file.log)
		if matched, _ := regexp.MatchString(`^[A-Za-z]:\\[^\\]+$`, absPath); matched {
			return "", fmt.Errorf("cannot log directly to drive root")
		}

	case "freebsd", "openbsd", "netbsd":
		// BSD variants
		forbiddenPrefixes = []string{
			"/tmp/",
			"/var/tmp/",
			"/dev/",
			"/proc/",
			"/sys/",
			"/boot/",
			"/lib/",
			"/libexec/",
			"/bin/",
			"/sbin/",
			"/usr/bin/",
			"/usr/sbin/",
			"/usr/libexec/",
			"/kernel/", // FreeBSD specific
			"/compat/", // FreeBSD compatibility layer
		}
		rootCheck = "/"

	default: // Linux (including Alpine), and other Unix-like
		forbiddenPrefixes = []string{
			"/tmp/",
			"/var/tmp/",
			"/dev/",
			"/proc/",
			"/sys/",
			"/boot/",
			"/lib/",
			"/lib64/",
			"/bin/",
			"/sbin/",
			"/usr/bin/",
			"/usr/sbin/",
		}
		rootCheck = "/"
	}

	// Check forbidden prefixes
	for _, prefix := range forbiddenPrefixes {
		if strings.HasPrefix(absPath, prefix) {
			return "", fmt.Errorf("cannot log to %s (forbidden location)", prefix)
		}
	}

	// Check root directory files (Unix-like systems)
	if rootCheck != "" && filepath.Dir(absPath) == rootCheck {
		return "", fmt.Errorf("cannot log directly to root directory")
	}

	// Check write capability
	if err := checkWriteCapability(absPath); err != nil {
		return "", fmt.Errorf("cannot write to specified log path: %v", err)
	}

	// Return the expanded/resolved path
	return absPath, nil
}

// checkWriteCapability verifies that we can write to the specified log path
func checkWriteCapability(path string) error {
	dir := filepath.Dir(path)

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Directory doesn't exist - create it for logging use
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create directory %s: %v", dir, err)
		}
		// Directory created successfully - leave it for actual logging use
	}

	// Check if we can write to the directory
	testFile := filepath.Join(dir, ".za_log_test")
	f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("no write permission for directory %s: %v", dir, err)
	}
	f.Close()
	os.Remove(testFile) // Clean up test file

	// If the log file already exists, check if it's writable
	if _, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("existing log file %s is not writable: %v", path, err)
		}
		f.Close()
	}

	return nil
}

// isDirEmpty checks if a directory is empty
func isDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// checkAndRotateLog checks if log rotation is needed and performs it
func checkAndRotateLog() {
	if logRotateSize == 0 || !loggingEnabled || logFile == "" {
		return
	}

	info, err := os.Stat(logFile)
	if err != nil || info.Size() < logRotateSize {
		return
	}

	// Rotate existing files: log.3 -> log.4, log.2 -> log.3, log.1 -> log.2
	for i := logRotateCount; i > 1; i-- {
		oldName := fmt.Sprintf("%s.%d", logFile, i-1)
		newName := fmt.Sprintf("%s.%d", logFile, i)
		if _, err := os.Stat(oldName); err == nil {
			os.Rename(oldName, newName)
		}
	}

	// Move current log to .1
	if logRotateCount > 0 {
		rotatedName := fmt.Sprintf("%s.1", logFile)
		if err := os.Rename(logFile, rotatedName); err != nil {
			// If rename fails, just continue - don't break logging
			return
		}
	}

	// Clean up old files beyond the count limit
	for i := logRotateCount + 1; i <= logRotateCount+10; i++ {
		oldFile := fmt.Sprintf("%s.%d", logFile, i)
		if _, err := os.Stat(oldFile); err == nil {
			os.Remove(oldFile)
		}
	}
}

// checkAndRotateWebLog checks if web access log rotation is needed and performs it
func checkAndRotateWebLog(webLogFile string) {
	if logRotateSize == 0 || webLogFile == "" {
		return
	}

	info, err := os.Stat(webLogFile)
	if err != nil || info.Size() < logRotateSize {
		return
	}

	// Rotate existing files: weblog.3 -> weblog.4, weblog.2 -> weblog.3, weblog.1 -> weblog.2
	for i := logRotateCount; i > 1; i-- {
		oldName := fmt.Sprintf("%s.%d", webLogFile, i-1)
		newName := fmt.Sprintf("%s.%d", webLogFile, i)
		if _, err := os.Stat(oldName); err == nil {
			os.Rename(oldName, newName)
		}
	}

	// Move current web log to .1
	if logRotateCount > 0 {
		rotatedName := fmt.Sprintf("%s.1", webLogFile)
		if err := os.Rename(webLogFile, rotatedName); err != nil {
			// If rename fails, just continue - don't break logging
			return
		}
	}

	// Clean up old web log files beyond the count limit
	for i := logRotateCount + 1; i <= logRotateCount+10; i++ {
		oldFile := fmt.Sprintf("%s.%d", webLogFile, i)
		if _, err := os.Stat(oldFile); err == nil {
			os.Remove(oldFile)
		}
	}
}

// getCallChainForLogging returns call chain information for error logging
func getCallChainForLogging() []string {
	if !enhancedErrorsEnabled {
		return nil
	}

	var callChain []string
	// This would integrate with the existing enhanced error system
	// For now, return a placeholder that can be filled in during integration
	return callChain
}

// getLogQueueUsage returns current queue usage for library function
func getLogQueueUsage() (used int, total int, running bool) {
	if logQueue == nil {
		return 0, logQueueSize, false
	}
	return len(logQueue), logQueueSize, logWorkerRunning
}

// getLogQueueStats returns detailed queue statistics including web access logs
func getLogQueueStats() (used int, total int, running bool, webRequests int64, mainRequests int64) {
	if logQueue == nil {
		return 0, logQueueSize, false, webLogRequestCount, mainLogRequestCount
	}
	return len(logQueue), logQueueSize, logWorkerRunning, webLogRequestCount, mainLogRequestCount
}

// logError logs an error to the current log destination respecting format settings
func logError(line int16, message string, parser *leparser) {
	if !errorLoggingEnabled || !loggingEnabled {
		return
	}

	// Create snapshot of current fields for JSON logging
	var fieldsCopy map[string]any
	if jsonLoggingEnabled {
		fieldsCopy = make(map[string]any)
		for k, v := range logFields {
			fieldsCopy[k] = v
		}
		fieldsCopy["source_line"] = line

		if enhancedErrorsEnabled {
			// Add enhanced error context if available
			if callChain := getCallChainForLogging(); callChain != nil {
				fieldsCopy["call_chain"] = callChain
			}
		}
	}

	// Queue the error log request
	request := LogRequest{
		Message:    message,
		Fields:     fieldsCopy,
		IsJSON:     jsonLoggingEnabled,
		IsError:    true,
		SourceLine: line,
		Level:      LOG_ERR, // Error logs use ERROR level
		Timestamp:  time.Now(),
	}

	queueLogRequest(request)
}

// plog_direct writes directly to log file (used by queue processor)
func plog_direct(message string) {
	if !loggingEnabled {
		return
	}

	subj, _ := gvget("@logsubject")
	subjStr := ""
	if subj != nil {
		if s, ok := subj.(string); ok {
			subjStr = s
		}
	}

	// Use atomic write
	if err := writeFileAtomic(logFile, []byte(message), subjStr); err != nil {
		log.Println(err)
	}
}

// plog_json_direct writes JSON directly to log file (used by queue processor)
func plog_json_direct(message string, fields map[string]any) {
	if !loggingEnabled {
		return
	}

	// Build JSON log entry
	logEntry := make(map[string]any)
	logEntry["message"] = message
	logEntry["timestamp"] = time.Now().Format(time.RFC3339)

	// Add subject if set
	if subj, exists := gvget("@logsubject"); exists && subj != nil {
		if subjStr, ok := subj.(string); ok && subjStr != "" {
			logEntry["subject"] = subjStr
		}
	}

	// Add custom fields
	for k, v := range fields {
		logEntry[k] = v
	}

	// Convert to JSON
	jsonBytes, err := json.Marshal(logEntry)
	if err != nil {
		// Fallback to regular logging if JSON fails
		plog_direct(message)
		return
	}

	// Use atomic write with no prefix for JSON logs
	if err := writeFileAtomic(logFile, jsonBytes, ""); err != nil {
		log.Println(err)
	}
}

// plog_direct_to_file logs a plain text message directly to the specified file
func plog_direct_to_file(filename, message string) {
	if filename == "" {
		return
	}

	// Get subject prefix if set
	subjStr := ""
	if subj, exists := gvget("@logsubject"); exists && subj != nil {
		if s, ok := subj.(string); ok && s != "" {
			subjStr = s
		}
	}

	// Use atomic write
	writeFileAtomic(filename, []byte(message), subjStr)
}

// plog_json_direct_to_file logs a JSON message directly to the specified file
func plog_json_direct_to_file(filename, message string, fields map[string]any) {
	if filename == "" {
		return
	}

	// Create JSON log entry
	logEntry := make(map[string]any)
	logEntry["message"] = message
	logEntry["timestamp"] = time.Now().Format(time.RFC3339)

	// Add provided fields
	for k, v := range fields {
		logEntry[k] = v
	}

	// Add subject if set
	if subj, exists := gvget("@logsubject"); exists && subj != nil {
		if subjStr, ok := subj.(string); ok && subjStr != "" {
			logEntry["subject"] = subjStr
		}
	}

	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		// Fallback to plain text if JSON fails
		plog_direct_to_file(filename, message)
		return
	}

	// Use atomic write with no prefix for JSON logs
	writeFileAtomic(filename, jsonData, "")
}

// flock applies an exclusive file lock on Unix systems
func flock(file *os.File, block bool) error {
	if runtime.GOOS == "windows" {
		return nil // No locking on Windows
	}
	flag := syscall.LOCK_EX
	if !block {
		flag |= syscall.LOCK_NB
	}
	return syscall.Flock(int(file.Fd()), flag)
}

// funlock removes a file lock on Unix systems
func funlock(file *os.File) error {
	if runtime.GOOS == "windows" {
		return nil // No locking on Windows
	}
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

// writeFileAtomic performs atomic file writes with locking
func writeFileAtomic(filename string, data []byte, prefix string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Apply file lock on Unix systems (best effort)
	if runtime.GOOS != "windows" {
		if err := flock(f, false); err == nil {
			defer funlock(f)
		}
		// If locking fails, continue anyway (best effort)
	}

	logger := log.New(f, prefix, log.LstdFlags)
	logger.Print(string(data))
	return f.Sync()
}
