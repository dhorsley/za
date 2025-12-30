package main

import (
    "archive/tar"
    "bytes"
    "crypto/rand"
    "encoding/binary"
    "encoding/hex"
    "fmt"
    "io"
    "os"
    "os/exec"
    "os/signal"
    "path/filepath"
    "strings"
    "syscall"
    "time"
)

// generateUniqueBundleDir creates a unique temporary directory using random bytes
func generateUniqueBundleDir() (string, error) {
    // Generate 8 random bytes for unique identifier
    randomBytes := make([]byte, 8)
    if _, err := rand.Read(randomBytes); err != nil {
        return "", fmt.Errorf("failed to generate random bytes: %v", err)
    }

    // Convert to hex string for safe filename
    randomID := hex.EncodeToString(randomBytes)

    // Create unique directory name
    pattern := "za_bundle_" + randomID

    // Create temporary directory with custom pattern
    tempDir, err := os.MkdirTemp("", pattern)
    if err != nil {
        return "", fmt.Errorf("failed to create temp directory: %v", err)
    }

    return tempDir, nil
}

// ExtraFileInfo stores information about additional files to include in bundle
type ExtraFileInfo struct {
    SourcePath string // Original path relative to working directory
    BundlePath string // Path within bundle
    IsDir      bool   // Whether this is a directory
}

// Bundle metadata for internal use
type BundleMetadata struct {
    FormatVersion  string          `json:"format_version"`
    MainScript     string          `json:"main_script"`
    Modules        []string        `json:"modules"`
    ModuleCount    int             `json:"module_count"`
    ExtraFiles     []ExtraFileInfo `json:"extra_files"`
    ExtraFileCount int             `json:"extra_file_count"`
    ZaVersion      string          `json:"za_version"`
    Created        string          `json:"created"`
}

// Module information for discovery and rewriting
type ModuleInfo struct {
    OriginalPath  string // Original MODULE path in script
    AbsolutePath  string // Resolved absolute path
    RelativePath  string // Relative path from main script
    RewrittenPath string // Path within bundle
    FileContent   []byte // Content of the module file
}

// ModuleStatement represents a parsed MODULE statement with positioning information
type ModuleStatement struct {
    StartPos                int    // Byte position where MODULE keyword starts
    PathStart               int    // Byte position of opening quote for module path
    PathEnd                 int    // Byte position of closing quote for module path
    Path                    string // Current module path (content between quotes)
    OriginalTokenWithQuotes string // Original token text including quotes (for exact replacement)
    HasAlias                bool   // Whether AS clause exists
    AliasStart              int    // Byte position where alias starts (if HasAlias)
    AliasEnd                int    // Byte position where alias ends (if HasAlias)
    Alias                   string // Alias identifier (if HasAlias)
    LineNum                 int    // Line number for error reporting
}

// findModulesTokens scans content using Za's lexer to find MODULE statements with precise positioning
func findModulesTokens(content string) ([]ModuleStatement, error) {
    var statements []ModuleStatement

    lineNum := int16(1)
    pos := 0

    // Track MODULE tokens to find path and optional alias
    var currentModule *ModuleStatement
    expectingPath := false
    expectingAlias := false

    for pos < len(content) {
        // Use Za's lexer to get next token from current position
        tok := nextToken(content, 0, &lineNum, pos)
        if tok == nil {
            break
        }

        tokenType := tok.carton.tokType
        tokenText := tok.carton.tokText
        tokenPos := tok.tokPos

        if expectingPath && tokenType == StringLiteral {
            // Found string literal after MODULE - extract path info
            if currentModule != nil {
                // Remove quotes from tokenText to get path content
                pathContent := tokenText
                if len(pathContent) >= 2 && (pathContent[0] == '"' || pathContent[0] == '`' || pathContent[0] == '\'') {
                    pathContent = pathContent[1 : len(pathContent)-1]
                }
                currentModule.Path = pathContent
                currentModule.OriginalTokenWithQuotes = tokenText // Store original token with quotes for exact replacement

                // Calculate exact token boundaries - tokPos is start of next token
                tokenStartPos := tokenPos - len(tokenText)
                currentModule.PathStart = tokenStartPos + 1 // After opening quote
                currentModule.PathEnd = tokenPos - 1        // Before closing quote
            }
            expectingPath = false
        } else if expectingAlias && tokenType == Identifier {
            // Found alias identifier after AS
            if currentModule != nil {
                currentModule.Alias = tokenText
                currentModule.AliasStart = tokenPos
                currentModule.AliasEnd = tokenPos + len(tokenText)
                currentModule.HasAlias = true
            }
            expectingAlias = false
        } else if tokenType == C_Module {
            // Found MODULE keyword - start new statement
            currentModule = &ModuleStatement{
                StartPos: tokenPos,
                LineNum:  int(lineNum),
                HasAlias: false,
            }
            expectingPath = true
            expectingAlias = false
        } else if tokenType == C_As && currentModule != nil && currentModule.Path != "" {
            // Found AS keyword after MODULE path - expect alias
            expectingAlias = true
        } else if (tokenType == EOL || tokenType == EOF) && currentModule != nil {
            // End of line - finalize current module statement if we have at least a path
            if currentModule.Path != "" {
                statements = append(statements, *currentModule)
            }
            currentModule = nil
            expectingPath = false
            expectingAlias = false
        }

        // Check for EOF flag to stop properly BEFORE updating position
        if tok.eof {
            break
        }

        // Move to next token position - tok.tokPos should be start of next token
        if tok.tokPos != -1 {
            pos = tok.tokPos
        } else {
            break
        }
    }

    // Handle final statement if file doesn't end with newline
    if currentModule != nil && currentModule.Path != "" {
        statements = append(statements, *currentModule)
    }

    return statements, nil
}

// rewriteContentWithTokens performs surgical string replacements using token positions
func rewriteContentWithTokens(content string, statements []ModuleStatement, newPath string) string {
    if len(statements) == 0 {
        return content
    }

    // Process in reverse order to maintain correct positions
    contentBytes := []byte(content)

    // Work backwards to avoid position shifting
    for i := len(statements) - 1; i >= 0; i-- {
        stmt := statements[i]

        // Replace only the path part within the stored token
        // Original token includes quotes: "\"path\"", so we need to replace just "path" with "newpath"
        originalPathPart := `"` + stmt.Path + `"` // Just the quoted path part
        newPathPart := `"` + newPath + `"`        // Just the new quoted path

        contentStr := string(contentBytes)
        contentStr = strings.Replace(contentStr, originalPathPart, newPathPart, -1)
        contentBytes = []byte(contentStr)
    }

    return string(contentBytes)
}

// processIncludeFiles processes comma-separated include files relative to working directory
func processIncludeFiles(includeFiles []string) ([]ExtraFileInfo, error) {
    var extraFiles []ExtraFileInfo

    for _, filePattern := range includeFiles {
        filePattern = strings.TrimSpace(filePattern)
        if filePattern == "" {
            continue
        }

        // Get current working directory for path resolution
        cwd, err := os.Getwd()
        if err != nil {
            return nil, fmt.Errorf("failed to get working directory: %v", err)
        }

        // Check if the path is absolute or relative
        var fullPath string
        if filepath.IsAbs(filePattern) {
            fullPath = filePattern
        } else {
            fullPath = filepath.Join(cwd, filePattern)
        }

        // Check if path exists and what type it is
        info, err := os.Stat(fullPath)
        if err != nil {
            return nil, fmt.Errorf("include file not found: %s", filePattern)
        }

        if info.IsDir() {
            // Handle directory - walk through it recursively
            err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
                if err != nil {
                    return err
                }

                if info.IsDir() {
                    return nil // Skip directories, only add files
                }

                // Calculate relative path from working directory
                relPath, err := filepath.Rel(cwd, path)
                if err != nil {
                    return fmt.Errorf("cannot calculate relative path for %s: %v", path, err)
                }

                // Calculate bundle path (preserve directory structure)
                bundlePath, err := filepath.Rel(cwd, path)
                if err != nil {
                    return fmt.Errorf("cannot calculate bundle path for %s: %v", path, err)
                }

                extraFiles = append(extraFiles, ExtraFileInfo{
                    SourcePath: relPath,
                    BundlePath: bundlePath,
                    IsDir:      false,
                })
                return nil
            })

            if err != nil {
                return nil, fmt.Errorf("error walking directory %s: %v", filePattern, err)
            }

            // Add the directory itself as an entry
            relDirPath, err := filepath.Rel(cwd, fullPath)
            if err != nil {
                return nil, fmt.Errorf("cannot calculate relative path for directory %s: %v", fullPath, err)
            }

            extraFiles = append(extraFiles, ExtraFileInfo{
                SourcePath: relDirPath,
                BundlePath: relDirPath,
                IsDir:      true,
            })

        } else {
            // Handle single file
            relPath, err := filepath.Rel(cwd, fullPath)
            if err != nil {
                return nil, fmt.Errorf("cannot calculate relative path for %s: %v", fullPath, err)
            }

            extraFiles = append(extraFiles, ExtraFileInfo{
                SourcePath: relPath,
                BundlePath: relPath,
                IsDir:      false,
            })
        }
    }

    return extraFiles, nil
}

// CreateBundledExecutable creates a self-extracting bundle from a Za script
func CreateBundledExecutable(scriptPath, outputPath string, extraFiles []ExtraFileInfo) error {
    // 1. Get current Za binary
    zaBinaryPath, err := os.Executable()
    if err != nil {
        return fmt.Errorf("failed to get current Za binary: %v", err)
    }

    zaData, err := os.ReadFile(zaBinaryPath)
    if err != nil {
        return fmt.Errorf("failed to read Za binary: %v", err)
    }

    // 2. Discover all modules recursively
    visited := make(map[string]bool)
    modules, err := discoverModules(scriptPath, visited)
    if err != nil {
        return fmt.Errorf("module discovery failed: %v", err)
    }

    fmt.Printf("Discovered %d modules\n", len(modules))
    for _, module := range modules {
        fmt.Printf("  Module: %s -> %s\n", module.OriginalPath, module.RelativePath)
    }

    if len(extraFiles) > 0 {
        fmt.Printf("Including %d additional files\n", len(extraFiles))
        for _, extraFile := range extraFiles {
            if extraFile.IsDir {
                fmt.Printf("  Directory: %s -> %s\n", extraFile.SourcePath, extraFile.BundlePath)
            } else {
                fmt.Printf("  File: %s -> %s\n", extraFile.SourcePath, extraFile.BundlePath)
            }
        }
    }

    // 3. Read and rewrite main script
    mainContent, err := os.ReadFile(scriptPath)
    if err != nil {
        return fmt.Errorf("failed to read main script: %v", err)
    }

    rewrittenMain, err := rewriteScript(mainContent, modules)
    if err != nil {
        return fmt.Errorf("failed to rewrite main script: %v", err)
    }

    // 4. Create bundle tar
    bundleData, err := createBundleTar(scriptPath, rewrittenMain, modules, zaData, false, extraFiles)
    if err != nil {
        return fmt.Errorf("failed to create bundle: %v", err)
    }

    // 5. Create output file with executable permissions
    outputFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0750)
    if err != nil {
        return fmt.Errorf("failed to create output file: %v", err)
    }
    defer outputFile.Close()

    // Write za binary first (the executable wrapper)
    _, err = outputFile.Write(zaData)
    if err != nil {
        return fmt.Errorf("failed to write Za binary: %v", err)
    }

    // Write bundle data (tar)
    _, err = outputFile.Write(bundleData)
    if err != nil {
        return fmt.Errorf("failed to write bundle data: %v", err)
    }

    // Calculate tar metrics
    tarStart := uint64(len(zaData))
    tarLength := uint64(len(bundleData))

    // Debug output
    fmt.Printf("- zaData size = %d, bundleData size = %d\n", len(zaData), len(bundleData))
    fmt.Printf("- tarStart = %d, tarLength = %d\n", tarStart, tarLength)

    // Write footer with position and length
    err = binary.Write(outputFile, binary.LittleEndian, tarStart)
    if err != nil {
        return fmt.Errorf("failed to write tar start position: %v", err)
    }

    err = binary.Write(outputFile, binary.LittleEndian, tarLength)
    if err != nil {
        return fmt.Errorf("failed to write tar length: %v", err)
    }

    // Write magic string as bytes to avoid any encoding issues
    magicBytes := []byte{'Z', 'A', 'B', 'U', 'N', 'D', 'L', 'E'}
    fmt.Printf("writing magic bytes: %v\n", magicBytes)

    // Flush any buffered data before writing magic
    err = outputFile.Sync()
    if err != nil {
        return fmt.Errorf("failed to sync file: %v", err)
    }

    // Explicitly seek to end to ensure magic bytes are written at correct position
    currentPos, err := outputFile.Seek(0, io.SeekEnd)
    if err != nil {
        return fmt.Errorf("failed to seek to end: %v", err)
    }

    fmt.Printf("DEBUG: Writing magic bytes at position %d, bytes: %v\n", currentPos, magicBytes)

    _, err = outputFile.Write(magicBytes)
    if err != nil {
        return fmt.Errorf("failed to write footer magic: %v", err)
    }

    // Final sync
    err = outputFile.Sync()
    if err != nil {
        return fmt.Errorf("failed to final sync: %v", err)
    }

    fmt.Printf("Bundle created: %s (%d bytes)\n", outputPath, getFileSize(outputPath))
    return nil
}

// IsBundledExecutable checks if current executable is a Za bundle
func IsBundledExecutable() bool {
    exePath, err := os.Executable()
    if err != nil {
        // fmt.Printf("DEBUG: IsBundledExecutable - failed to get executable: %v\n", err)
        return false
    }

    file, err := os.Open(exePath)
    if err != nil {
        // fmt.Printf("DEBUG: IsBundledExecutable - failed to open exe: %v\n", err)
        return false
    }
    defer file.Close()

    // Seek to last 8 bytes only - memory efficient
    _, err = file.Seek(-8, io.SeekEnd)
    if err != nil {
        // fmt.Printf("DEBUG: IsBundledExecutable - failed to seek to end: %v\n", err)
        return false
    }

    magic := make([]byte, 8)
    _, err = file.Read(magic)
    if err != nil {
        // fmt.Printf("DEBUG: IsBundledExecutable - failed to read magic: %v\n", err)
        return false
    }

    magicStr := string(magic)
    // fmt.Printf("DEBUG: IsBundledExecutable - magic='%s', valid=%v\n", magicStr, magicStr == "ZABUNDLE")
    if magicStr != "ZABUNDLE" {
        return false
    }
    return true
}

// ExecuteFromBundle extracts and executes bundle from current executable
func ExecuteFromBundle(args []string) error {
    // fmt.Printf("ExecuteFromBundle called with args: %v (len: %d)\n", args, len(args))
    // fmt.Printf("os.Args from parent: %v (len: %d)\n", os.Args, len(os.Args))

    // 1. Extract bundle from self
    tempDir, _, bundleMeta, err := extractBundleFromSelf()
    if err != nil {
        return fmt.Errorf("bundle extraction failed: %v", err)
    }

    // fmt.Printf("Bundle extracted to tempDir: %s\n", tempDir)
    // fmt.Printf("Bundle metadata: MainScript=%s, ModuleCount=%d\n", bundleMeta.MainScript, len(bundleMeta.Modules))

    // 2. Setup cleanup handlers
    setupCleanupHandlers(tempDir)

    // 3. Execute bundled Za with proper argument handling
    zaBinary := filepath.Join(tempDir, "za")
    mainScript := filepath.Join(tempDir, bundleMeta.MainScript)

    // fmt.Printf("DEBUG: Execution setup - tempDir=%s, zaBinary=%s, mainScript=%s\n",
    //  tempDir, zaBinary, mainScript)

    // Verify paths exist
    if _, err := os.Stat(zaBinary); err != nil {
        return fmt.Errorf("za binary not found at %s: %v", zaBinary, err)
    }
    if _, err := os.Stat(mainScript); err != nil {
        return fmt.Errorf("main script not found at %s: %v", mainScript, err)
    }

    // fmt.Printf("DEBUG: Both za binary and main script verified to exist\n")

    // Execute za with proper argument order: za -f script.za [user args...]
    // Use proper exec.Command pattern with spread operator
    finalArgs := append([]string{"-f", mainScript}, args...)
    // fmt.Printf("DEBUG: Final execution args - zaBinary=%s, args=%v\n", zaBinary, finalArgs)
    cmd := exec.Command(zaBinary, finalArgs...)

    // Set execution environment - preserve original working directory
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    // Note: cmd.Dir is left unset to preserve original working directory

    // 5. Execute and propagate exit code
    // fmt.Printf("DEBUG: Starting command execution...\n")
    err = cmd.Run()

    if err != nil {
        // fmt.Printf("DEBUG: Command execution failed with err=%v\n", err)
        if exitErr, ok := err.(*exec.ExitError); ok {
            // fmt.Printf("DEBUG: Process exited with code: %d\n", exitErr.ExitCode())
            os.Exit(exitErr.ExitCode())
        }
        // fmt.Printf("DEBUG: Unknown error type, exiting with code 1\n")
        os.Exit(1)
    }

    // fmt.Printf("DEBUG: Command execution completed successfully\n")
    cleanupTempDir(tempDir)
    return nil
}

// discoverModules finds all MODULE dependencies recursively
func discoverModules(scriptPath string, visited map[string]bool) ([]ModuleInfo, error) {
    content, err := os.ReadFile(scriptPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read script: %v", err)
    }

    // Add newline at end if missing to prevent lexer out-of-bounds issues
    contentStr := string(content)
    if len(contentStr) > 0 && !strings.HasSuffix(contentStr, "\n") {
        contentStr += "\n"
    }

    // Parse MODULE statements using token-based approach for robust parsing
    statements, err := findModulesTokens(contentStr)
    if err != nil {
        return nil, fmt.Errorf("failed to parse MODULE statements: %v", err)
    }

    scriptDir := filepath.Dir(scriptPath)
    var modules []ModuleInfo

    for _, stmt := range statements {
        modulePath := stmt.Path

        // Resolve module path using Za's logic from actor.go:5247-5265
        absolutePath, err := resolveModulePath(modulePath, scriptDir)
        if err != nil {
            return nil, fmt.Errorf("module not found: %s (from %s)", modulePath, scriptPath)
        }

        // Calculate relative path from main script
        relPath, err := filepath.Rel(scriptDir, absolutePath)
        if err != nil {
            return nil, fmt.Errorf("cannot calculate relative path for %s: %v", modulePath, err)
        }

        moduleInfo := ModuleInfo{
            OriginalPath:  modulePath,
            AbsolutePath:  absolutePath,
            RelativePath:  relPath,
            RewrittenPath: relPath, // Use calculated relative path
        }

        // Skip if already processed (avoid cycles)
        if visited[absolutePath] {
            continue
        }
        visited[absolutePath] = true

        // Read module content
        moduleContent, err := os.ReadFile(absolutePath)
        if err != nil {
            return nil, fmt.Errorf("failed to read module %s: %v", absolutePath, err)
        }

        // First, discover sub-modules in the original content
        // Recursively discover modules in this module file
        subModules, err := discoverModules(absolutePath, visited)
        if err != nil {
            return nil, err
        }

        // Rewrite MODULE statements in the module content to use relative paths
        // This needs to be done after discovering sub-modules but before storing content
        fmt.Printf("Rewriting MODULE statements in module: %s\n", absolutePath)
        rewrittenContent, rewriteErr := rewriteModuleContent(moduleContent, scriptDir, absolutePath)
        if rewriteErr != nil {
            return nil, fmt.Errorf("failed to rewrite module content for %s: %v", absolutePath, rewriteErr)
        }
        moduleInfo.FileContent = rewrittenContent

        modules = append(modules, moduleInfo)
        modules = append(modules, subModules...)
    }

    return modules, nil
}

// resolveModulePath resolves module path using Za's logic from actor.go:5247-5265
func resolveModulePath(modulePath, scriptDir string) (string, error) {
    if strings.Contains(modulePath, "/") {
        if filepath.IsAbs(modulePath) {
            // Absolute path
            if _, err := os.Stat(modulePath); err != nil {
                return "", err
            }
            return modulePath, nil
        } else {
            // Relative path - combine with script directory
            fullPath := filepath.Join(scriptDir, modulePath)
            if _, err := os.Stat(fullPath); err != nil {
                return "", err
            }
            return fullPath, nil
        }
    } else {
        // Module name - look up in ZA_MODPATH or ~/.za/modules
        modhome := os.Getenv("ZA_MODPATH")
        if modhome == "" {
            home, _ := os.UserHomeDir()
            modhome = filepath.Join(home, ".za", "modules")
        }

        // Try without .fom extension first
        fullPath := filepath.Join(modhome, modulePath)
        if _, err := os.Stat(fullPath); err == nil {
            return fullPath, nil
        }

        // Try with .fom extension
        fullPath = filepath.Join(modhome, modulePath+".fom")
        if _, err := os.Stat(fullPath); err != nil {
            return "", fmt.Errorf("module not found: %s", modulePath)
        }
        return fullPath, nil
    }
}

// rewriteModuleContent rewrites MODULE statements in a module file to use relative paths
func rewriteModuleContent(content []byte, scriptDir, modulePath string) ([]byte, error) {
    script := string(content)
    fmt.Printf("RewriteModuleContent called for scriptDir=%s, modulePath=%s\n", scriptDir, modulePath)

    // Add newline at end if missing to prevent lexer out-of-bounds issues
    if len(script) > 0 && !strings.HasSuffix(script, "\n") {
        script += "\n"
    }

    // Parse MODULE statements using token-based approach
    statements, err := findModulesTokens(script)
    if err != nil {
        return nil, fmt.Errorf("failed to parse MODULE statements: %v", err)
    }

    fmt.Printf("- Found %d MODULE statements in module %s\n", len(statements), modulePath)

    for _, stmt := range statements {
        originalModulePath := stmt.Path
        fmt.Printf("- Processing MODULE statement: %s\n", originalModulePath)

        // Skip rewriting relative paths that already start with "./"
        if strings.HasPrefix(originalModulePath, "./") {
            fmt.Printf("- Skipping rewrite for relative path: %s\n", originalModulePath)
            continue
        }

        // Ensure target path starts with "./" to avoid Za treating it as bare module name
        targetPath := originalModulePath
        if !strings.HasPrefix(targetPath, "./") {
            targetPath = "./" + targetPath
        }

        // Use position-based replacement
        script = rewriteContentWithTokens(script, []ModuleStatement{stmt}, targetPath)
        fmt.Printf("- Rewrote MODULE \"%s\" -> \"%s\"\n", originalModulePath, targetPath)
    }

    return []byte(script), nil
}

// rewriteScript rewrites MODULE statements to use bundled paths
func rewriteScript(content []byte, modules []ModuleInfo) ([]byte, error) {
    script := string(content)
    fmt.Printf("RewriteScript called with %d modules\n", len(modules))

    // Add newline at end if missing to prevent lexer out-of-bounds issues
    if len(script) > 0 && !strings.HasSuffix(script, "\n") {
        script += "\n"
    }

    // Parse MODULE statements once to get all positions
    statements, err := findModulesTokens(script)
    if err != nil {
        return nil, fmt.Errorf("failed to parse MODULE statements: %v", err)
    }

    // Process modules in reverse order to maintain position accuracy
    for i := len(modules) - 1; i >= 0; i-- {
        module := modules[i]
        fmt.Printf("- Rewriting module: %s -> %s\n", module.OriginalPath, module.RewrittenPath)

        // Skip rewriting for relative paths that start with "./" - they're already correct
        if strings.HasPrefix(module.OriginalPath, "./") {
            fmt.Printf("- Skipping rewrite for relative path: %s\n", module.OriginalPath)
            continue
        }

        // Ensure rewritten path starts with "./" to avoid Za treating it as bare module name
        targetPath := module.RewrittenPath
        if !strings.HasPrefix(targetPath, "./") {
            targetPath = "./" + targetPath
        }

        // Find all statements matching this module
        var matchingStatements []ModuleStatement
        for _, stmt := range statements {
            if stmt.Path == module.OriginalPath {
                matchingStatements = append(matchingStatements, stmt)
            }
        }

        // Replace all matching statements at once to avoid position drift
        if len(matchingStatements) > 0 {
            script = rewriteContentWithTokens(script, matchingStatements, targetPath)
            fmt.Printf("- Replaced %d instances of MODULE \"%s\" with \"%s\"\n", len(matchingStatements), module.OriginalPath, targetPath)
        }
    }

    return []byte(script), nil
}

// createBundleTar creates tar archive with all bundle components
func createBundleTar(scriptPath string, rewrittenMain []byte, modules []ModuleInfo, zaData []byte, includeZaBinary bool, extraFiles []ExtraFileInfo) ([]byte, error) {
    var buf bytes.Buffer
    tarWriter := tar.NewWriter(&buf)

    // Add Za binary if requested
    if includeZaBinary {
        zaHeader := &tar.Header{
            Name:     "za",
            Mode:     0755,
            Size:     int64(len(zaData)),
            ModTime:  time.Now(),
            Typeflag: tar.TypeReg,
        }

        if err := tarWriter.WriteHeader(zaHeader); err != nil {
            return nil, fmt.Errorf("failed to write Za header: %v", err)
        }

        if _, err := tarWriter.Write(zaData); err != nil {
            return nil, fmt.Errorf("failed to write Za data: %v", err)
        }
    }

    // Add main script name file first
    scriptName := filepath.Base(scriptPath)
    scriptNameBytes := []byte(scriptName)
    nameHeader := &tar.Header{
        Name:     "main_script.txt",
        Mode:     0644,
        Size:     int64(len(scriptNameBytes)),
        ModTime:  time.Now(),
        Typeflag: tar.TypeReg,
    }

    if err := tarWriter.WriteHeader(nameHeader); err != nil {
        return nil, fmt.Errorf("failed to write main script name header: %v", err)
    }

    if _, err := tarWriter.Write(scriptNameBytes); err != nil {
        return nil, fmt.Errorf("failed to write main script name: %v", err)
    }

    // Add main script
    mainHeader := &tar.Header{
        Name:     scriptName,
        Mode:     0644,
        Size:     int64(len(rewrittenMain)),
        ModTime:  time.Now(),
        Typeflag: tar.TypeReg,
    }

    if err := tarWriter.WriteHeader(mainHeader); err != nil {
        return nil, fmt.Errorf("failed to write script header: %v", err)
    }

    if _, err := tarWriter.Write(rewrittenMain); err != nil {
        return nil, fmt.Errorf("failed to write script data: %v", err)
    }

    // Add all modules
    for _, module := range modules {
        // Use relative path to preserve directory structure
        moduleHeader := &tar.Header{
            Name:     module.RelativePath,
            Mode:     0644,
            Size:     int64(len(module.FileContent)),
            ModTime:  time.Now(),
            Typeflag: tar.TypeReg,
        }
        if err := tarWriter.WriteHeader(moduleHeader); err != nil {
            return nil, fmt.Errorf("failed to create module header: %v", err)
        }

        if _, err := tarWriter.Write(module.FileContent); err != nil {
            return nil, fmt.Errorf("failed to write module data: %v", err)
        }
    }

    // Add all extra files
    for _, extraFile := range extraFiles {
        if extraFile.IsDir {
            // Create directory entry
            dirHeader := &tar.Header{
                Name:     extraFile.BundlePath,
                Mode:     0755,
                Size:     0,
                ModTime:  time.Now(),
                Typeflag: tar.TypeDir,
            }
            if err := tarWriter.WriteHeader(dirHeader); err != nil {
                return nil, fmt.Errorf("failed to create directory header: %v", err)
            }
        } else {
            // Read file content
            cwd, err := os.Getwd()
            if err != nil {
                return nil, fmt.Errorf("failed to get working directory: %v", err)
            }

            fullPath := filepath.Join(cwd, extraFile.SourcePath)
            content, err := os.ReadFile(fullPath)
            if err != nil {
                return nil, fmt.Errorf("failed to read extra file %s: %v", extraFile.SourcePath, err)
            }

            // Get file info to preserve permissions
            info, err := os.Stat(fullPath)
            if err != nil {
                return nil, fmt.Errorf("failed to stat extra file %s: %v", extraFile.SourcePath, err)
            }

            // Create file entry with original permissions
            fileHeader := &tar.Header{
                Name:     extraFile.BundlePath,
                Mode:     int64(info.Mode()),
                Size:     int64(len(content)),
                ModTime:  info.ModTime(),
                Typeflag: tar.TypeReg,
            }
            if err := tarWriter.WriteHeader(fileHeader); err != nil {
                return nil, fmt.Errorf("failed to create extra file header: %v", err)
            }

            if _, err := tarWriter.Write(content); err != nil {
                return nil, fmt.Errorf("failed to write extra file data: %v", err)
            }
        }
    }

    if err := tarWriter.Close(); err != nil {
        return nil, fmt.Errorf("failed to close tar writer: %v", err)
    }

    return buf.Bytes(), nil
}

// extractBundleFromSelf extracts bundle data from current executable
func extractBundleFromSelf() (string, []byte, BundleMetadata, error) {
    exePath, err := os.Executable()
    if err != nil {
        return "", nil, BundleMetadata{}, fmt.Errorf("failed to get executable path: %v", err)
    }

    data, err := os.ReadFile(exePath)
    if err != nil {
        return "", nil, BundleMetadata{}, fmt.Errorf("failed to read executable: %v", err)
    }

    // Check for magic string at end
    if len(data) < 8 {
        return "", nil, BundleMetadata{}, fmt.Errorf("invalid bundle file: too small")
    }

    magic := string(data[len(data)-8:])
    if magic != "ZABUNDLE" {
        return "", nil, BundleMetadata{}, fmt.Errorf("not a valid Za bundle")
    }

    // Calculate footer positions (24-byte footer)
    magicStart := len(data) - 8
    tarLengthStart := len(data) - 16
    tarStartStart := len(data) - 24

    // fmt.Printf("DEBUG: Bundle footer positions - magicStart=%d, tarLengthStart=%d, tarStartStart=%d, totalSize=%d\n",
    //  magicStart, tarLengthStart, tarStartStart, len(data))

    // Extract footer information
    tarStart := binary.LittleEndian.Uint64(data[tarStartStart : tarStartStart+8])
    tarLength := binary.LittleEndian.Uint64(data[tarLengthStart:magicStart])

    // fmt.Printf("DEBUG: Extracted bundle boundaries - tarStart=%d, tarLength=%d\n", tarStart, tarLength)

    // Extract tar data using explicit boundaries
    bundleData := data[tarStart : tarStart+uint64(tarLength)]

    // fmt.Printf("DEBUG: Bundle data slice - startPos=%d, endPos=%d, bundleDataLen=%d\n",
    //  tarStart, tarStart+uint64(tarLength), len(bundleData))

    // Extract tar to temp directory with unique ID
    tempDir, err := generateUniqueBundleDir()
    if err != nil {
        return "", nil, BundleMetadata{}, fmt.Errorf("failed to create temp directory: %v", err)
    }

    // fmt.Printf("DEBUG: Created temp directory: %s\n", tempDir)

    // Copy za binary from bundle start to temp directory FIRST
    zaBinaryPath := filepath.Join(tempDir, "za")
    zaBinaryData := data[0:tarStart] // Data before tar is the za binary

    err = os.WriteFile(zaBinaryPath, zaBinaryData, 0755)
    if err != nil {
        return "", nil, BundleMetadata{}, fmt.Errorf("failed to write za binary: %v", err)
    }

    // Verify za binary was written correctly
    if _, err := os.Stat(zaBinaryPath); err != nil {
        return "", nil, BundleMetadata{}, fmt.Errorf("za binary not found after extraction: %v", err)
    }

    // fmt.Printf("DEBUG: Za binary extracted successfully to: %s\n", zaBinaryPath)

    // Extract tar contents
    buf := bytes.NewReader(bundleData)
    tarReader := tar.NewReader(buf)

    // fmt.Printf("DEBUG: Starting tar extraction from %d bytes of data\n", len(bundleData))

    metadata := BundleMetadata{
        FormatVersion: "1.0",
        ZaVersion:     "unknown",
        Created:       time.Now().Format(time.RFC3339),
    }

    var mainScriptName string

    for {
        header, err := tarReader.Next()
        if err != nil {
            // fmt.Printf("DEBUG: Tar extraction ended with err=%v\n", err)
            break
        }

        // fmt.Printf("DEBUG: Extracting tar entry: %s (size: %d, type: %d)\n",
        //      header.Name, header.Size, header.Typeflag)

        // Handle main script name file specially
        if header.Name == "main_script.txt" {
            nameBytes := make([]byte, header.Size)
            _, err = io.ReadFull(tarReader, nameBytes)
            if err != nil {
                return "", nil, BundleMetadata{}, fmt.Errorf("failed to read main script name: %v", err)
            }
            mainScriptName = string(nameBytes)
            // fmt.Printf("DEBUG: Found main script name: %s\n", mainScriptName)
            continue
        }

        // Write file to temp directory
        filePath := filepath.Join(tempDir, header.Name)

        if header.Typeflag == tar.TypeDir {
            err = os.MkdirAll(filePath, 0755)
        } else {
            // Create directory if needed
            if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
                return "", nil, BundleMetadata{}, fmt.Errorf("failed to create directory: %v", err)
            }

            // Use permissions from tar header to preserve executable bits
            perm := os.FileMode(header.Mode)
            file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
            if err != nil {
                return "", nil, BundleMetadata{}, fmt.Errorf("failed to create file: %v", err)
            }

            _, err = io.CopyN(file, tarReader, header.Size)
            file.Close()

            if err != nil {
                return "", nil, BundleMetadata{}, fmt.Errorf("failed to extract file: %v", err)
            }

            // Track modules (exclude za binary and main script)
            if header.Name != "za" && header.Name != mainScriptName {
                metadata.Modules = append(metadata.Modules, header.Name)
            }
        }
    }

    // Set main script name from stored data
    if mainScriptName == "" {
        return "", nil, BundleMetadata{}, fmt.Errorf("main script name not found in bundle")
    }
    metadata.MainScript = mainScriptName

    metadata.ModuleCount = len(metadata.Modules)

    // fmt.Printf("DEBUG: Final bundle metadata - MainScript=%s, ModuleCount=%d\n",
    //  metadata.MainScript, metadata.ModuleCount)

    // Validate that we found a main script
    if metadata.MainScript == "" {
        return "", nil, BundleMetadata{}, fmt.Errorf("no main script found in bundle")
    }

    // fmt.Printf("DEBUG: Extraction completed successfully, returning tempDir: %s\n", tempDir)
    return tempDir, bundleData, metadata, nil
}

// setupCleanupHandlers sets up signal handlers for temp directory cleanup
func setupCleanupHandlers(tempDir string) {
    // Setup signal handling for graceful cleanup
    c := make(chan os.Signal, 1)
    signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

    go func() {
        <-c
        cleanupTempDir(tempDir)
        os.Exit(1)
    }()
}

// cleanupTempDir removes temporary directory
func cleanupTempDir(tempDir string) {
    if tempDir != "" {
        os.RemoveAll(tempDir)
    }
}

// getFileSize returns file size for reporting
func getFileSize(path string) int64 {
    info, err := os.Stat(path)
    if err != nil {
        return 0
    }
    return info.Size()
}
