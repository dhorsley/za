package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type parseTimingFile struct {
	Path         string   `json:"path"`
	ParseMs      int64    `json:"parse_ms"`
	Status       string   `json:"status"`
	Error        string   `json:"error,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	DynamicPaths []string `json:"dynamic_paths,omitempty"`
}

type parseTimingResult struct {
	Files   []parseTimingFile `json:"files"`
	TotalMs int64             `json:"total_ms"`
	Success bool              `json:"success"`
}

func suppressOutput() func() {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	done := make(chan struct{})
	go func() {
		io.Copy(io.Discard, r)
		close(done)
	}()
	return func() {
		w.Close()
		<-done
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}
}

func validateBlockNesting(phrases []Phrase) []string {
	var errs []string

	// Map of block openers to their names
	openers := map[int64]string{
		C_Define:  "def",
		C_If:      "if",
		C_For:     "for",
		C_Foreach: "foreach",
		C_While:   "while",
		C_Struct:  "struct",
		C_Try:     "try",
		C_Case:    "case",
		C_Test:    "test",
	}
	// Map of block closers to their expected opener names
	closers := map[int64]string{
		C_Enddef:    "def",
		C_Endif:     "if",
		C_Endfor:    "for",
		C_Endwhile:  "while",
		C_Endstruct: "struct",
		C_Endtry:    "try",
		C_Endcase:   "case",
		C_Endtest:   "test",
	}

	var stack []string
	var stackLines []int

	for _, phrase := range phrases {
		if len(phrase.Tokens) == 0 {
			continue
		}
		tokType := phrase.Tokens[0].tokType
		line := int(phrase.SourceLine) + 1

		if name, ok := openers[tokType]; ok {
			stack = append(stack, name)
			stackLines = append(stackLines, line)
		} else if expected, ok := closers[tokType]; ok {
			if len(stack) == 0 {
				errs = append(errs, fmt.Sprintf("stray %s at line %d (no matching block opener)", expected, line))
				continue
			}
			top := stack[len(stack)-1]
			// endfor closes both 'for' and 'foreach' blocks
			if tokType == C_Endfor && (top == "for" || top == "foreach") {
				stack = stack[:len(stack)-1]
				stackLines = stackLines[:len(stackLines)-1]
			} else if top != expected {
				errs = append(errs, fmt.Sprintf("mismatched block at line %d: found %s but expected %s (opened at line %d)", line, expected, top, stackLines[len(stackLines)-1]))
				continue
			} else {
				stack = stack[:len(stack)-1]
				stackLines = stackLines[:len(stackLines)-1]
			}
		} else if tokType == C_Enddef {
			// Generic 'end'/'enddef' - pop any block (runtime allows end to close any block)
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
				stackLines = stackLines[:len(stackLines)-1]
			} else {
				errs = append(errs, fmt.Sprintf("stray end/enddef at line %d (no matching block opener)", line))
			}
		}
	}

	if len(stack) > 0 {
		errs = append(errs, fmt.Sprintf("unclosed block(s): %s at line %d", stack[len(stack)-1], stackLines[len(stackLines)-1]))
	}
	return errs
}

func runParseTiming(entryPath string, level int) bool {
	totalStart := time.Now()
	result := parseTimingResult{
		Files:   []parseTimingFile{},
		Success: true,
	}

	// Ensure absolute path
	if !filepath.IsAbs(entryPath) {
		cwd, err := os.Getwd()
		if err == nil {
			entryPath = filepath.Join(cwd, entryPath)
		}
	}

	type queueItem struct {
		path string
		name string
	}

	queue := []queueItem{{path: entryPath, name: "main"}}
	seen := make(map[string]bool)

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if seen[item.path] {
			continue
		}
		seen[item.path] = true

		// Read file
		content, err := os.ReadFile(item.path)
		if err != nil {
			result.Files = append(result.Files, parseTimingFile{
				Path:    item.path,
				ParseMs: 0,
				Status:  "error",
				Error:   err.Error(),
			})
			result.Success = false
			continue
		}

		fileResult := parseTimingFile{
			Path:         item.path,
			Warnings:     []string{},
			DynamicPaths: []string{},
		}

		// Allocate function space
		_, _ = GetNextFnSpace(true, item.name, call_s{prepared: false})

		// Parse with suppressed output
		restore := suppressOutput()
		parseStart := time.Now()
		badword, _ := phraseParse(context.Background(), item.name, string(content), 0, 0)
		parseElapsed := time.Since(parseStart)
		restore()

		fileResult.ParseMs = parseElapsed.Milliseconds()

		if badword {
			fileResult.Status = "error"
			fileResult.Error = "parse error"
			result.Success = false
			result.Files = append(result.Files, fileResult)
			continue
		}

		// Validate block nesting
		fsID, _ := fnlookup.lmget(item.name)
		if fsID != 0 {
			fspacelock.RLock()
			phrases := functionspaces[fsID]
			fspacelock.RUnlock()
			if nestingErrs := validateBlockNesting(phrases); len(nestingErrs) > 0 {
				fileResult.Status = "error"
				fileResult.Error = nestingErrs[0]
				if len(nestingErrs) > 1 {
					for _, e := range nestingErrs[1:] {
						fileResult.Warnings = append(fileResult.Warnings, e)
					}
				}
				result.Success = false
				result.Files = append(result.Files, fileResult)
				continue
			}
		}

		fileResult.Status = "ok"

		// Find module imports
		fsID, _ = fnlookup.lmget(item.name)
		if fsID != 0 {
			fspacelock.RLock()
			phrases := functionspaces[fsID]
			fspacelock.RUnlock()

			for _, phrase := range phrases {
				if len(phrase.Tokens) == 0 {
					continue
				}
				if phrase.Tokens[0].tokType != C_Module {
					continue
				}
				if len(phrase.Tokens) < 2 {
					continue
				}

				pathToken := phrase.Tokens[1]
				if pathToken.tokType != StringLiteral {
					// Dynamic path
					if level >= 2 {
						dp := pathToken.tokText
						if dp == "" {
							dp = "<expression>"
						}
						fileResult.DynamicPaths = append(fileResult.DynamicPaths, dp)
						fileResult.Warnings = append(fileResult.Warnings, fmt.Sprintf("dynamic module path at line %d: %s", phrase.SourceLine+1, dp))
					}
					continue
				}

				modPath := pathToken.tokText
				// Strip quotes
				if len(modPath) >= 2 {
					first := modPath[0]
					last := modPath[len(modPath)-1]
					if (first == '"' && last == '"') ||
						(first == '`' && last == '`') ||
						(first == '\'' && last == '\'') {
						modPath = modPath[1 : len(modPath)-1]
					}
				}

				// Resolve path relative to the current file's directory
				resolved, err := resolveModulePath(modPath, filepath.Dir(item.path))
				if err != nil {
					if level >= 2 {
						fileResult.Warnings = append(fileResult.Warnings, fmt.Sprintf("missing module at line %d: %s (%v)", phrase.SourceLine+1, modPath, err))
					}
					result.Files = append(result.Files, parseTimingFile{
						Path:    modPath,
						ParseMs: 0,
						Status:  "error",
						Error:   err.Error(),
					})
					result.Success = false
					continue
				}

				queue = append(queue, queueItem{path: resolved, name: resolved})
			}
		}

		result.Files = append(result.Files, fileResult)
	}

	result.TotalMs = time.Since(totalStart).Milliseconds()

	// Output JSON
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal JSON: %v\n", err)
		return false
	}
	fmt.Println(string(jsonBytes))

	return result.Success
}
