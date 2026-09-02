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
		C_With:    "with",
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
		C_Endwith:   "with",
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
			// Inline closers: the runtime discards everything after a
			// def header (and after try) on the same line, so an inline
			// end/endtry on the opener's own line closes the block.
			if (tokType == C_Define || tokType == C_Try) && len(phrase.Tokens) > 1 {
				closer := C_Enddef
				if tokType == C_Try {
					closer = C_Endtry
				}
				closed := false
				for _, t := range phrase.Tokens[1:] {
					if t.tokType == closer {
						closed = true
						break
					}
				}
				if closed {
					continue
				}
			}
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

// checkExprTokens applies expression-completeness checks (trailing operator,
// missing RHS, bare '::', chained range) to a token slice.
func checkExprTokens(tks []Token) string {
	if len(tks) == 0 {
		return "empty expression"
	}
	last := tks[len(tks)-1].tokType
	if assignmentOps[last] {
		return "missing expression after assignment"
	}
	if trailingBinaryOps[last] {
		return fmt.Sprintf("incomplete expression: trailing %s", tks[len(tks)-1].tokText)
	}
	for i, t := range tks {
		if t.tokType == SYM_DoubleColon {
			if i == 0 || i == len(tks)-1 || tks[i-1].tokType != Identifier {
				return "'::' must be preceded by a name"
			}
		}
	}
	rangeCount := 0
	for _, t := range tks {
		if t.tokType == SYM_RANGE {
			rangeCount++
		}
	}
	if rangeCount > 1 && tks[len(tks)-1].tokType != RightSBrace {
		return "multiple range operators"
	}
	return ""
}

// statement-ending operators that can never legitimately close a statement
var trailingBinaryOps = map[int64]bool{
	O_Plus: true, O_Minus: true, O_Multiply: true, O_Divide: true, O_Percent: true,
	SYM_POW: true, SYM_LAND: true, SYM_LOR: true, SYM_BAND: true, SYM_BOR: true,
	SYM_EQ: true, SYM_NE: true, SYM_LT: true, SYM_LE: true, SYM_GT: true, SYM_GE: true,
	SYM_LSHIFT: true, SYM_RSHIFT: true, SYM_RANGE: true, O_Query: true,
}

// assignment operators that require an expression on their right-hand side
var assignmentOps = map[int64]bool{
	O_Assign: true, O_AssCommand: true, O_AssOutCommand: true,
	SYM_PLE: true, SYM_MIE: true, SYM_MUE: true, SYM_DIE: true, SYM_MOE: true,
}

var varTypeKeywords = map[int64]bool{
	T_Number: true, T_Nil: true, T_Bool: true, T_Int: true, T_Uint: true,
	T_Float: true, T_Bigi: true, T_Bigf: true, T_String: true, T_Map: true,
	T_Array: true, T_Any: true, T_Pointer: true,
}

// clause keywords that legitimately end an assignment LHS run (a fresh
// assignment-target list may begin after one of them).
func lhsBoundaryKeyword(tokType int64) bool {
	switch tokType {
	case C_On, C_If, C_While, C_For, C_Foreach, C_Return, C_Break, C_Continue,
		C_Do, C_Then, C_To, C_In, C_Global, C_Var, C_With, C_Async:
		return true
	}
	return false
}

// assignmentLhsError returns "" when every depth-0 token of an assignment LHS
// run is a legal assignment-target part (a variable Identifier, member '.',
// brackets/parens, target-list comma, or the global-write prefix '@' which
// lexes as C_SetGlob). Any other token -- a reserved keyword, literal or bare
// operator -- is a keyword-as-identifier / invalid-name misuse. Inside brackets
// and parens (depth >= 1) any expression is legal and is not checked.
func assignmentLhsError(lhs []Token, line int) (int, string) {
	depth := 0
	for idx, t := range lhs {
		if depth == 0 {
			switch t.tokType {
			case Identifier, O_Comma, LParen, RParen, LeftSBrace, RightSBrace,
				SYM_DOT, C_SetGlob:
				// legal depth-0 LHS token
			default:
				return idx, fmt.Sprintf("expected an identifier on the LHS of an assignment (got '%s') at line %d", t.tokText, line)
			}
		}
		switch t.tokType {
		case LParen, LeftSBrace:
			depth++
		case RParen, RightSBrace:
			if depth > 0 {
				depth--
			}
		}
	}
	return -1, ""
}

// declNameSlotError mirrors the runtime var/global name-list parse: the leading
// run of tokens alternates Identifier/comma, and everything after the first
// non-name token is the size/type/initializer. A reserved keyword or literal
// in a name slot is a misuse.
func declNameSlotError(decl []Token, kind string, line int) (int, string) {
	expectName := true
	for i, t := range decl {
		if expectName {
			if t.tokType != Identifier {
				return i, fmt.Sprintf("expected an identifier for '%s' in %s declaration at line %d", t.tokText, kind, line)
			}
			expectName = false
		} else if t.tokType != O_Comma {
			break
		} else {
			expectName = true
		}
	}
	return -1, ""
}

// validateStatementShapes checks per-statement structural validity that the
// phraser tolerates: missing assignment RHS, incomplete expressions, bare '::',
// chained ranges, malformed var declarations and unterminated parens/brackets.
func validateStatementShapes(phrases []Phrase) []string {
	var errs []string

	for _, phrase := range phrases {
		tks := phrase.Tokens
		if len(tks) == 0 {
			continue
		}
		line := int(phrase.SourceLine) + 1
		first := tks[0].tokType
		last := tks[len(tks)-1].tokType

		// Statement keywords (e.g. 'use -', 'lib', 'module') are not
		// expressions, so shape checks on trailing operators do not apply.
		isStmt := first >= START_STATEMENTS && first < END_STATEMENTS

		// missing expression after an assignment operator
		if !isStmt && assignmentOps[last] {
			errs = append(errs, fmt.Sprintf("missing expression after assignment at line %d", line))
			continue
		}

		// statement ends in an operator that needs a right-hand operand
		if !isStmt && trailingBinaryOps[last] {
			errs = append(errs, fmt.Sprintf("incomplete expression at line %d: trailing %s", line, tks[len(tks)-1].tokText))
			continue
		}

		// '::' must be preceded by a name and not end the statement
		for i, t := range tks {
			if t.tokType != SYM_DoubleColon {
				continue
			}
			if i == 0 || i == len(tks)-1 || tks[i-1].tokType != Identifier {
				errs = append(errs, fmt.Sprintf("'::' must be preceded by a name at line %d", line))
				break
			}
		}

		// chained ranges (1..2..3) are only valid as multi-dimensional slices
		rangeCount := 0
		for _, t := range tks {
			if t.tokType == SYM_RANGE {
				rangeCount++
			}
		}
		if rangeCount > 1 && last != RightSBrace {
			errs = append(errs, fmt.Sprintf("multiple range operators at line %d", line))
			continue
		}

		// var/global declarations: name slots must be real identifiers; a
		// comma must be followed by another name, not a type.
		if (first == C_Var || first == C_Global) && len(tks) > 1 {
			decl := tks[1:]
			if _, msg := declNameSlotError(decl, tks[0].tokText, line); msg != "" {
				errs = append(errs, msg)
				continue
			}
			for i, t := range decl {
				if t.tokType != O_Comma {
					continue
				}
				if i+1 >= len(decl) || varTypeKeywords[decl[i+1].tokType] {
					errs = append(errs, fmt.Sprintf("expected variable name after ',' in var declaration at line %d", line))
					break
				}
			}
			lastDecl := decl[len(decl)-1].tokType
			if assignmentOps[lastDecl] {
				errs = append(errs, fmt.Sprintf("missing expression after '=' in var declaration at line %d", line))
				continue
			}
		}

		// Keyword-specific arity and expression checks
		switch first {
		case C_If:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing condition in if at line %d", line))
				continue
			}
			if msg := checkExprTokens(tks[1:]); msg != "" {
				errs = append(errs, fmt.Sprintf("%s in if condition at line %d", msg, line))
				continue
			}
		case C_While:
			if len(tks) > 1 {
				if msg := checkExprTokens(tks[1:]); msg != "" {
					errs = append(errs, fmt.Sprintf("%s in while condition at line %d", msg, line))
					continue
				}
			}
		case C_For:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing loop index in for at line %d", line))
				continue
			}
			if tks[1].tokType != Identifier {
				errs = append(errs, fmt.Sprintf("expected an identifier for the for-iterator (got '%s') at line %d", tks[1].tokText, line))
				continue
			}
			hasComma := false
			hasTo := false
			for _, t := range tks {
				if t.tokType == O_Comma {
					hasComma = true
				}
				if t.tokType == C_To {
					hasTo = true
				}
			}
			if !hasComma && !hasTo {
				errs = append(errs, fmt.Sprintf("missing 'to' or commas in for at line %d", line))
				continue
			}
		case C_Foreach:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing loop variable in foreach at line %d", line))
				continue
			}
			if tks[1].tokType != Identifier {
				errs = append(errs, fmt.Sprintf("expected an identifier for the foreach iterator (got '%s') at line %d", tks[1].tokText, line))
				continue
			}
			hasIn := false
			inIdx := -1
			for i, t := range tks {
				if t.tokType == C_In {
					hasIn = true
					inIdx = i
					break
				}
			}
			if !hasIn {
				errs = append(errs, fmt.Sprintf("missing 'in' in foreach at line %d", line))
				continue
			}
			if inIdx+1 >= len(tks) {
				errs = append(errs, fmt.Sprintf("missing iterable in foreach at line %d", line))
				continue
			}
			if msg := checkExprTokens(tks[inIdx+1:]); msg != "" {
				errs = append(errs, fmt.Sprintf("%s in foreach iterable at line %d", msg, line))
				continue
			}
		case C_Break, C_Continue:
			hasIf := false
			ifIdx := -1
			for i, t := range tks {
				if t.tokType == C_If {
					hasIf = true
					ifIdx = i
					break
				}
			}
			if hasIf {
				if ifIdx+1 >= len(tks) {
					errs = append(errs, fmt.Sprintf("missing condition after 'if' in %s at line %d", tks[0].tokText, line))
					continue
				}
				if msg := checkExprTokens(tks[ifIdx+1:]); msg != "" {
					errs = append(errs, fmt.Sprintf("%s in %s condition at line %d", msg, tks[0].tokText, line))
					continue
				}
			}
		case C_Return:
			hasIf := false
			ifIdx := -1
			for i, t := range tks {
				if t.tokType == C_If {
					hasIf = true
					ifIdx = i
					break
				}
			}
			if hasIf && ifIdx+1 < len(tks) {
				if msg := checkExprTokens(tks[ifIdx+1:]); msg != "" {
					errs = append(errs, fmt.Sprintf("%s in return condition at line %d", msg, line))
					continue
				}
			}
		case C_On:
			doIdx := -1
			for i, t := range tks {
				if t.tokType == C_Do {
					doIdx = i
					break
				}
			}
			if doIdx == -1 {
				errs = append(errs, fmt.Sprintf("missing 'do' in on at line %d", line))
				continue
			}
			if doIdx <= 1 {
				errs = append(errs, fmt.Sprintf("missing condition in on at line %d", line))
				continue
			}
		case C_With:
			if len(tks) < 3 {
				errs = append(errs, fmt.Sprintf("invalid with statement at line %d", line))
				continue
			}
			if len(tks) == 3 && tks[1].tokType != C_Enum && tks[1].tokType != C_Struct {
				errs = append(errs, fmt.Sprintf("unknown with type at line %d", line))
				continue
			}
		case C_Async:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing async target at line %d", line))
				continue
			}
			if tks[1].tokType != Identifier {
				errs = append(errs, fmt.Sprintf("async target must be an identifier at line %d", line))
				continue
			}
			hasParen := false
			for i := 2; i < len(tks); i++ {
				if tks[i].tokType == LParen {
					hasParen = true
					break
				}
			}
			if !hasParen {
				errs = append(errs, fmt.Sprintf("async target must be a function call at line %d", line))
				continue
			}
		case C_Throw:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing throw expression at line %d", line))
				continue
			}
			if msg := checkExprTokens(tks[1:]); msg != "" {
				errs = append(errs, fmt.Sprintf("%s in throw expression at line %d", msg, line))
				continue
			}
		case C_Struct:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing struct name at line %d", line))
				continue
			}
		case C_Require:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing require target at line %d", line))
				continue
			}
		case C_Module:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing module path at line %d", line))
				continue
			}
		case C_Namespace:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing namespace name at line %d", line))
				continue
			}
		case C_Assert:
			if len(tks) < 2 {
				errs = append(errs, fmt.Sprintf("missing assert expression at line %d", line))
				continue
			}
			commaIdx := -1
			for i, t := range tks {
				if t.tokType == O_Comma {
					commaIdx = i
					break
				}
			}
			exprEnd := len(tks)
			if commaIdx != -1 {
				exprEnd = commaIdx
			}
			if exprEnd > 1 {
				if msg := checkExprTokens(tks[1:exprEnd]); msg != "" {
					errs = append(errs, fmt.Sprintf("%s in assert expression at line %d", msg, line))
					continue
				}
			}
		case C_Exit, C_Pause:
			if len(tks) > 1 {
				if msg := checkExprTokens(tks[1:]); msg != "" {
					errs = append(errs, fmt.Sprintf("%s in %s expression at line %d", msg, tks[0].tokText, line))
					continue
				}
			}
		}

		// unterminated parentheses or brackets
		parenDepth := 0
		sbraceDepth := 0
		for _, t := range tks {
			switch t.tokType {
			case LParen:
				parenDepth++
			case RParen:
				parenDepth--
			case LeftSBrace:
				sbraceDepth++
			case RightSBrace:
				sbraceDepth--
			}
		}
		if parenDepth > 0 {
			errs = append(errs, fmt.Sprintf("unclosed parenthesis at line %d", line))
			continue
		}
		if sbraceDepth > 0 {
			errs = append(errs, fmt.Sprintf("unclosed bracket at line %d", line))
			continue
		}
		if parenDepth < 0 {
			errs = append(errs, fmt.Sprintf("unexpected ')' at line %d", line))
			continue
		}
		if sbraceDepth < 0 {
			errs = append(errs, fmt.Sprintf("unexpected ']' at line %d", line))
			continue
		}

		// Reserved keywords / non-identifier tokens on the LHS of assignments.
		// For each top-level assignment operator, bound the LHS run by the
		// nearest preceding clause keyword and require every depth-0 token to
		// be a legal assignment-target part. var/global declarations are
		// excluded: their name slots are validated separately and their
		// declared type (e.g. `var x int = 5`) legitimately sits before '='.
		depth := 0
		for i := 0; i < len(tks); i++ {
			if first == C_Var || first == C_Global {
				break
			}
			switch tks[i].tokType {
			case LParen, LeftSBrace:
				depth++
			case RParen, RightSBrace:
				if depth > 0 {
					depth--
				}
			}
			if depth != 0 || !assignmentOps[tks[i].tokType] {
				continue
			}
			start := 0
			for k := i - 1; k >= 0; k-- {
				if lhsBoundaryKeyword(tks[k].tokType) {
					start = k + 1
					break
				}
			}
			if _, msg := assignmentLhsError(tks[start:i], line); msg != "" {
				errs = append(errs, msg)
				break
			}
		}
	}

	return errs
}

func runParseTiming(entryPath string, level int, clean bool) bool {
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
		lexSoftErrors = true
		parseStart := time.Now()
		badword, _ := phraseParse(context.Background(), item.name, string(content), 0, 0)
		parseElapsed := time.Since(parseStart)
		lexSoftErrors = false
		restore()

		fileResult.ParseMs = parseElapsed.Milliseconds()

		if badword {
			fileResult.Status = "error"
			if lastSoftErrorMsg != "" {
				fileResult.Error = lastSoftErrorMsg
				lastSoftErrorMsg = ""
			} else {
				fileResult.Error = "parse error"
			}
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
			if shapeErrs := validateStatementShapes(phrases); len(shapeErrs) > 0 {
				fileResult.Status = "error"
				fileResult.Error = shapeErrs[0]
				if len(shapeErrs) > 1 {
					for _, e := range shapeErrs[1:] {
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
	if clean {
		root := filepath.Dir(entryPath)
		filtered := result.Files[:0]
		for _, file := range result.Files {
			path, err := filepath.Abs(file.Path)
			if err != nil {
				continue
			}
			if path != entryPath && filepath.Ext(path) != ".za" {
				continue
			}
			rel, err := filepath.Rel(root, path)
			if err != nil || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
				continue
			}
			file.Path = path
			filtered = append(filtered, file)
		}
		result.Files = filtered
		result.Success = true
		for _, file := range result.Files {
			if file.Status != "ok" {
				result.Success = false
				break
			}
		}
	}

	// Output JSON
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal JSON: %v\n", err)
		return false
	}
	fmt.Println(string(jsonBytes))

	return result.Success
}
