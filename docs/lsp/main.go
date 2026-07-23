// za-lsp/main.go - Practical LSP server using Za introspection

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"za/lexer"
)

// FunctionLibrary holds all stdlib function metadata
type FunctionLibrary struct {
	Categories  map[string][]string            // category -> []function names
	Descriptions map[string]string             // function -> description
	Inputs      map[string]string              // function -> input params
	Outputs     map[string]string              // function -> output type
	ByName      map[string]*FunctionInfo       // quick lookup
	mu          sync.RWMutex
}

type FunctionInfo struct {
	Name        string
	Category    string
	Description string
	Inputs      string
	Outputs     string
}

// LoadLibrary queries Za binary once for all stdlib metadata
func LoadLibrary(zaPath string) (*FunctionLibrary, error) {
	lib := &FunctionLibrary{
		Categories:   make(map[string][]string),
		Descriptions: make(map[string]string),
		Inputs:       make(map[string]string),
		Outputs:      make(map[string]string),
		ByName:       make(map[string]*FunctionInfo),
	}

	log.Println("[LSP] Loading Za library metadata...")
	start := time.Now()

	// Single Za invocation gets all metadata at once
	zaScript := `println json_encode(map(
		.categories func_categories(),
		.descriptions func_descriptions(),
		.inputs func_inputs(),
		.outputs func_outputs()
	))`

	output, err := exec.Command(zaPath, "-c", "-e", zaScript).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query Za: %w", err)
	}

	// Parse the single response
	var metadata struct {
		Categories   map[string][]string `json:"categories"`
		Descriptions map[string]string   `json:"descriptions"`
		Inputs       map[string]string   `json:"inputs"`
		Outputs      map[string]string   `json:"outputs"`
	}

	if err := json.Unmarshal(output, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	lib.Categories = metadata.Categories
	lib.Descriptions = metadata.Descriptions
	lib.Inputs = metadata.Inputs
	lib.Outputs = metadata.Outputs

	// Build ByName index for O(1) lookups
	for category, functions := range lib.Categories {
		for _, funcName := range functions {
			lib.ByName[funcName] = &FunctionInfo{
				Name:        funcName,
				Category:    category,
				Description: lib.Descriptions[funcName],
				Inputs:      lib.Inputs[funcName],
				Outputs:     lib.Outputs[funcName],
			}
		}
	}

	log.Printf("[LSP] Loaded %d functions in %v", len(lib.ByName), time.Since(start))
	return lib, nil
}

// GetFunction returns function info by name
func (lib *FunctionLibrary) GetFunction(name string) *FunctionInfo {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	return lib.ByName[name]
}

// AllFunctions returns all functions in a category, or all if category is ""
func (lib *FunctionLibrary) AllFunctions(category string) []*FunctionInfo {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	var result []*FunctionInfo
	if category == "" {
		for _, info := range lib.ByName {
			result = append(result, info)
		}
	} else {
		for _, name := range lib.Categories[category] {
			if info, ok := lib.ByName[name]; ok {
				result = append(result, info)
			}
		}
	}
	return result
}

// ---- LSP Protocol Handler ----

type LSPServer struct {
	lib                *FunctionLibrary
	documents          map[string]*Document
	zaPath             string
	pendingNotifs      []*JSONRPCMessage
	timers             map[string]*time.Timer
	fullTimers         map[string]*time.Timer
	lastFullDiags      map[string][]Diagnostic
	writer             *bufio.Writer
	writeMu            sync.Mutex
	mu                 sync.RWMutex
}

const diagDebounceMs = 500
const wordBoundaryDebounceMs = 150

type Document struct {
	URI     string
	Content string
	Symbols map[string]*Symbol // local symbols: functions, vars, structs
}

type Symbol struct {
	Name     string
	Kind     string // "function", "variable", "struct", "enum"
	Location SourceLocation
	Type     string
}

type SourceLocation struct {
	Line   int
	Column int
}

// ---- Lexer-based tokenizer using za/lexer ----

type TokenKind int

const (
	TokEOF TokenKind = iota
	TokEOL
	TokIdentifier
	TokNumber
	TokString
	TokComment
	TokKeyword
	TokOperator
	TokLParen
	TokRParen
	TokLBrace
	TokRBrace
	TokLBracket
	TokRBracket
	TokComma
	TokAssign
	TokColon
	TokUnknown
)

type Token struct {
	Kind  TokenKind
	Type  int64
	Text  string
	Start int
	End   int
}

// noopBinder is a no-op binding function for lexer use in the LSP
// (the LSP doesn't need identifier binding resolution).
func noopBinder(fs uint32, name string) uint64 { return 0 }

// computeLineStarts returns the byte offsets of each line start in the input.
func computeLineStarts(input string) []int {
	starts := []int{0}
	for i := 0; i < len(input); i++ {
		if input[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// mapLexerType maps a lexer token type constant to the LSP TokenKind.
func mapLexerType(tokType int64) TokenKind {
	switch tokType {
	case lexer.Identifier:
		return TokIdentifier
	case lexer.StringLiteral:
		return TokString
	case lexer.NumericLiteral:
		return TokNumber
	case lexer.SingleComment:
		return TokComment
	case lexer.Operator:
		return TokOperator
	case lexer.LParen:
		return TokLParen
	case lexer.RParen:
		return TokRParen
	case lexer.LeftSBrace:
		return TokLBracket
	case lexer.RightSBrace:
		return TokRBracket
	case lexer.LeftCBrace:
		return TokLBrace
	case lexer.RightCBrace:
		return TokRBrace
	case lexer.O_Comma:
		return TokComma
	case lexer.SYM_COLON:
		return TokColon
	case lexer.O_Assign:
		return TokAssign
	case lexer.EOL, lexer.EOF:
		return TokEOL
	case lexer.Error:
		return TokUnknown
	default:
		// Check if it's a statement keyword (C_Define through C_Endtry, etc.)
		if tokType >= lexer.START_STATEMENTS && tokType <= lexer.END_STATEMENTS {
			return TokKeyword
		}
		return TokUnknown
	}
}

// blockOpeners maps lexer keyword types to their block names.
var blockOpeners = map[int64]string{
	lexer.C_Define:  "def",
	lexer.C_If:      "if",
	lexer.C_For:     "for",
	lexer.C_Foreach: "foreach",
	lexer.C_While:   "while",
	lexer.C_Struct:  "struct",
	lexer.C_Try:     "try",
	lexer.C_Case:    "case",
}

// blockClosers maps lexer end-keyword types to their matching block names.
var blockClosers = map[int64]string{
	lexer.C_Enddef:    "def",
	lexer.C_Endif:     "if",
	lexer.C_Endfor:    "for",
	lexer.C_Endwhile:  "while",
	lexer.C_Endstruct: "struct",
	lexer.C_Endtry:    "try",
	lexer.C_Endcase:   "case",
}

// tokenizeDocument uses the real za lexer to tokenize the entire document.
// It returns a map from line number (0-based) to tokens on that line.
func tokenizeDocument(content string) map[int][]Token {
	result := make(map[int][]Token)
	lineStarts := computeLineStarts(content)

	var curLine int16 = 0
	pos := 0
	for {
		res, err := lexer.NextToken(content, 0, &curLine, pos, noopBinder)
		if err != nil {
			break
		}
		if res.Eof || res.Pos < 0 {
			break
		}

		startPos := pos
		endPos := res.Pos

		// Compute line and column
		line := 0
		for line < len(lineStarts)-1 && lineStarts[line+1] <= startPos {
			line++
		}
		col := startPos - lineStarts[line]

		tok := Token{
			Kind:  mapLexerType(res.Tok.TokType),
			Type:  res.Tok.TokType,
			Text:  res.Tok.TokText,
			Start: col,
			End:   col + (endPos - startPos),
		}
		result[line] = append(result[line], tok)

		pos = endPos
	}
	return result
}

// tokenizeLine provides a lightweight fallback for single-line contexts
// (e.g., cursor position checks) that don't need the full lexer.
func tokenizeLine(line string) []Token {
	var tokens []Token
	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '\r') {
			i++
		}
		if i >= len(line) {
			break
		}
		start := i
		ch := line[i]

		// Comment
		if ch == '#' {
			for i < len(line) && line[i] != '\n' {
				i++
			}
			tokens = append(tokens, Token{Kind: TokComment, Text: line[start:i], Start: start, End: i})
			continue
		}
		// String
		if ch == '"' || ch == '`' || ch == '\'' {
			quote := ch
			i++
			for i < len(line) {
				if line[i] == '\\' && i+1 < len(line) {
					i += 2
					continue
				}
				if line[i] == quote {
					i++
					break
				}
				i++
			}
			tokens = append(tokens, Token{Kind: TokString, Text: line[start:i], Start: start, End: i})
			continue
		}
		// Number
		if (ch >= '0' && ch <= '9') || (ch == '.' && i+1 < len(line) && line[i+1] >= '0' && line[i+1] <= '9') {
			i++
			for i < len(line) && ((line[i] >= '0' && line[i] <= '9') || line[i] == '.' || line[i] == 'e' || line[i] == 'E' || line[i] == '+' || line[i] == '-') {
				i++
			}
			tokens = append(tokens, Token{Kind: TokNumber, Text: line[start:i], Start: start, End: i})
			continue
		}
		// Single-char tokens
		switch ch {
		case '(':
			i++
			tokens = append(tokens, Token{Kind: TokLParen, Text: "(", Start: start, End: i})
			continue
		case ')':
			i++
			tokens = append(tokens, Token{Kind: TokRParen, Text: ")", Start: start, End: i})
			continue
		case '{':
			i++
			tokens = append(tokens, Token{Kind: TokLBrace, Text: "{", Start: start, End: i})
			continue
		case '}':
			i++
			tokens = append(tokens, Token{Kind: TokRBrace, Text: "}", Start: start, End: i})
			continue
		case '[':
			i++
			tokens = append(tokens, Token{Kind: TokLBracket, Text: "[", Start: start, End: i})
			continue
		case ']':
			i++
			tokens = append(tokens, Token{Kind: TokRBracket, Text: "]", Start: start, End: i})
			continue
		case ',':
			i++
			tokens = append(tokens, Token{Kind: TokComma, Text: ",", Start: start, End: i})
			continue
		case ':':
			i++
			tokens = append(tokens, Token{Kind: TokColon, Text: ":", Start: start, End: i})
			continue
		}
		// Multi-char operators
		if i+1 < len(line) {
			two := line[i : i+2]
			if two == "==" || two == "!=" || two == "<=" || two == ">=" || two == "&&" || two == "||" || two == "<<" || two == ">>" || two == "++" || two == "--" || two == "**" || two == ".." || two == "??" || two == ":=" || two == "+=" || two == "-=" || two == "*=" || two == "/=" || two == "%=" {
				i += 2
				tokens = append(tokens, Token{Kind: TokOperator, Text: two, Start: start, End: i})
				continue
			}
		}
		// Single-char operators
		if ch == '=' || ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '%' || ch == '^' || ch == '!' || ch == '<' || ch == '>' || ch == '~' || ch == '.' || ch == '?' || ch == ';' || ch == '&' || ch == '|' {
			i++
			tokens = append(tokens, Token{Kind: TokOperator, Text: string(ch), Start: start, End: i})
			continue
		}
		// Identifier
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '$' || ch == '@' {
			i++
			for i < len(line) && ((line[i] >= 'a' && line[i] <= 'z') || (line[i] >= 'A' && line[i] <= 'Z') || (line[i] >= '0' && line[i] <= '9') || line[i] == '_' || line[i] == '-') {
				i++
			}
			tokens = append(tokens, Token{Kind: TokIdentifier, Text: line[start:i], Start: start, End: i})
			continue
		}
		// Unknown
		i++
		tokens = append(tokens, Token{Kind: TokUnknown, Text: string(ch), Start: start, End: i})
	}
	return tokens
}

// getPrefixAtCursor extracts the partial word being typed before the cursor
func getPrefixAtCursor(line string, char int) string {
	if char > len(line) {
		char = len(line)
	}
	// Walk backwards to find word start
	start := char
	for start > 0 {
		c := line[start-1]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '$' || c == '@' {
			start--
		} else {
			break
		}
	}
	return line[start:char]
}

// isInsideStringOrComment returns true if cursor is inside a string literal or comment
func isInsideStringOrComment(line string, char int) bool {
	tokens := tokenizeLine(line)
	for _, tok := range tokens {
		if tok.Kind == TokString || tok.Kind == TokComment {
			if char > tok.Start && char <= tok.End {
				return true
			}
		}
	}
	return false
}

// ---- LSP JSON-RPC message types
// ID must accept both int and string per LSP spec.
type JSONRPCMessage struct {
	JsonRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result"`
	Error   interface{}     `json:"error,omitempty"`
}

// hasID returns true if the message is a request (has an ID).
func (msg *JSONRPCMessage) hasID() bool {
	return msg.ID != nil && msg.ID != ""
}

func NewLSPServer(lib *FunctionLibrary, zaPath string) *LSPServer {
	return &LSPServer{
		lib:           lib,
		documents:     make(map[string]*Document),
		zaPath:        zaPath,
		timers:        make(map[string]*time.Timer),
		fullTimers:    make(map[string]*time.Timer),
		lastFullDiags: make(map[string][]Diagnostic),
	}
}

// HandleMessage processes incoming LSP messages
func (s *LSPServer) HandleMessage(msg *JSONRPCMessage) *JSONRPCMessage {
	log.Printf("[LSP] Method: %s (ID=%v)", msg.Method, msg.ID)

	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "initialized":
		// Notification, no response needed
		return nil
	case "shutdown":
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	case "textDocument/didOpen":
		s.handleDidOpen(msg)
		return nil
	case "textDocument/didChange":
		s.handleDidChange(msg)
		return nil
	case "textDocument/didClose":
		s.handleDidClose(msg)
		return nil
	case "textDocument/completion":
		return s.handleCompletion(msg)
	case "textDocument/hover":
		return s.handleHover(msg)
	case "textDocument/definition":
		return s.handleDefinition(msg)
	case "textDocument/references":
		return s.handleReferences(msg)
	case "textDocument/documentSymbol":
		return s.handleDocumentSymbol(msg)
	case "textDocument/signatureHelp":
		return s.handleSignatureHelp(msg)
	case "textDocument/didSave":
		s.handleDidSave(msg)
		return nil
	case "$/cancelRequest":
		// Cancellation notification, no response needed
		return nil
	default:
		// Only return error for requests (those with an ID)
		if msg.hasID() {
			return &JSONRPCMessage{
				JsonRPC: "2.0",
				ID:      msg.ID,
				Error: map[string]interface{}{
					"code":    -32601,
					"message": "Method not found",
				},
			}
		}
		return nil
	}
}

func (s *LSPServer) handleInitialize(msg *JSONRPCMessage) *JSONRPCMessage {
	return &JSONRPCMessage{
		JsonRPC: "2.0",
		ID:      msg.ID,
		Result: map[string]interface{}{
			"capabilities": map[string]interface{}{
				"completionProvider": map[string]interface{}{
					"resolveProvider": false,
					"triggerCharacters": []string{".", "(", " ", "\"", "`"},
				},
				"signatureHelpProvider": map[string]interface{}{
					"triggerCharacters": []string{"(", ","},
				},
				"hoverProvider":          true,
				"definitionProvider":     true,
				"referencesProvider":     true,
				"documentSymbolProvider": true,
				"textDocumentSync":     1, // FULL
			},
		},
	}
}

type DidOpenParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}

func (s *LSPServer) handleDidOpen(msg *JSONRPCMessage) {
	var params DidOpenParams
	json.Unmarshal(msg.Params, &params)

	s.mu.Lock()
	s.documents[params.TextDocument.URI] = &Document{
		URI:     params.TextDocument.URI,
		Content: params.TextDocument.Text,
		Symbols: make(map[string]*Symbol),
	}

	s.extractSymbols(params.TextDocument.URI, params.TextDocument.Text)
	s.mu.Unlock()

	// Run full diagnostics on open
	go s.runFullDiagnostics(params.TextDocument.URI)
}

type DidChangeParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

func (s *LSPServer) runDiagnostics(uri string) {
	s.mu.RLock()
	doc := s.documents[uri]
	fullDiags := s.lastFullDiags[uri]
	s.mu.RUnlock()

	if doc == nil {
		return
	}

	if !isZaFile(uri, doc.Content) {
		s.publishDiagnostics(uri, []Diagnostic{})
		return
	}

	// Run structural diagnostics (in-process, instant)
	structuralDiags := s.getStructuralDiagnostics(uri, doc.Content)

	// Merge with last full diagnostics (if any)
	allDiags := append(structuralDiags, fullDiags...)

	s.publishDiagnostics(uri, allDiags)
}

func (s *LSPServer) runFullDiagnostics(uri string) {
	s.mu.RLock()
	doc := s.documents[uri]
	s.mu.RUnlock()

	if doc == nil {
		return
	}

	if !isZaFile(uri, doc.Content) {
		return
	}

	// Run full diagnostics via subprocess
	fullDiags := s.getFullDiagnostics(uri, doc.Content)

	s.mu.Lock()
	s.lastFullDiags[uri] = fullDiags
	s.mu.Unlock()

	// Re-run structural diagnostics to merge and publish
	s.runDiagnostics(uri)
}

func (s *LSPServer) getFullDiagnostics(uri string, content string) []Diagnostic {
	diagnostics := []Diagnostic{}

	// Create temp file in the source file's directory so relative module paths resolve correctly
	dir := ""
	if fileURI := strings.TrimPrefix(uri, "file://"); fileURI != uri {
		dir = filepath.Dir(fileURI)
	}
	tmpFile, err := os.CreateTemp(dir, "za-lsp-*.za")
	if err != nil {
		log.Printf("[LSP] Failed to create temp file: %v", err)
		return diagnostics
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		log.Printf("[LSP] Failed to write temp file: %v", err)
		tmpFile.Close()
		return diagnostics
	}
	tmpFile.Close()

	// Run za -S -zz (level 2 enables module warnings + dynamic path warnings)
	cmd := exec.Command(s.zaPath, "-S", "-zz", "-f", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		// za -z exits 1 on parse errors, but still outputs JSON
		if len(output) == 0 {
			log.Printf("[LSP] za -S -zz failed with no output: %v", err)
			return diagnostics
		}
	}

	// Parse JSON output
	var result struct {
		Files   []struct {
			Path    string `json:"path"`
			ParseMs int64  `json:"parse_ms"`
			Status  string `json:"status"`
			Error   string `json:"error,omitempty"`
		} `json:"files"`
		TotalMs int64  `json:"total_ms"`
		Success bool   `json:"success"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		log.Printf("[LSP] Failed to parse za -S -z output: %v", err)
		return diagnostics
	}

	// Convert errors to diagnostics (main file only, not module sub-imports)
	for _, file := range result.Files {
		if file.Status == "error" && file.Error != "" && file.Path == tmpFile.Name() {
			// Map the error to line 1, column 0 as a fallback
			// (za -z doesn't yet emit line-level diagnostics in the JSON)
			diagnostics = append(diagnostics, Diagnostic{
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 1},
				},
				Severity: 1, // Error
				Message:  file.Error,
				Source:   "za-parse",
			})
		}
	}

	return diagnostics
}

func isWordBoundaryChange(content string) bool {
	if len(content) == 0 {
		return true
	}
	last := content[len(content)-1]
	return last == ' ' || last == '\t' || last == '\n' || last == '\r' ||
		last == ',' || last == ';' || last == '(' || last == ')' ||
		last == '[' || last == ']' || last == '{' || last == '}' ||
		last == '"' || last == '\'' || last == '`' || last == '.'
}

func (s *LSPServer) handleDidChange(msg *JSONRPCMessage) {
	var params DidChangeParams
	json.Unmarshal(msg.Params, &params)

	s.mu.Lock()
	if doc, ok := s.documents[params.TextDocument.URI]; ok {
		if len(params.ContentChanges) > 0 {
			doc.Content = params.ContentChanges[0].Text
			s.extractSymbols(params.TextDocument.URI, doc.Content)
		}
	}

	// Cancel any existing timer for this URI
	if oldTimer, ok := s.timers[params.TextDocument.URI]; ok {
		oldTimer.Stop()
		delete(s.timers, params.TextDocument.URI)
	}
	if oldTimer, ok := s.fullTimers[params.TextDocument.URI]; ok {
		oldTimer.Stop()
		delete(s.fullTimers, params.TextDocument.URI)
	}

	uri := params.TextDocument.URI
	content := params.ContentChanges[0].Text

	// Start structural diagnostics timer (always)
	s.timers[uri] = time.AfterFunc(time.Duration(diagDebounceMs)*time.Millisecond, func() {
		s.runDiagnostics(uri)
	})

	// Start full diagnostics timer on word boundary
	if isWordBoundaryChange(content) {
		s.fullTimers[uri] = time.AfterFunc(time.Duration(wordBoundaryDebounceMs)*time.Millisecond, func() {
			s.runFullDiagnostics(uri)
		})
	}

	s.mu.Unlock()
}

type DidCloseParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

func (s *LSPServer) handleDidClose(msg *JSONRPCMessage) {
	var params DidCloseParams
	json.Unmarshal(msg.Params, &params)

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.documents, params.TextDocument.URI)
	delete(s.lastFullDiags, params.TextDocument.URI)
	if timer, ok := s.timers[params.TextDocument.URI]; ok {
		timer.Stop()
		delete(s.timers, params.TextDocument.URI)
	}
	if timer, ok := s.fullTimers[params.TextDocument.URI]; ok {
		timer.Stop()
		delete(s.fullTimers, params.TextDocument.URI)
	}
}

type DidSaveParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

func isZaFile(uri string, content string) bool {
	// Check .za extension
	if strings.HasSuffix(uri, ".za") {
		return true
	}
	// Check shebang line
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0])
		if strings.HasPrefix(first, "#!") && strings.Contains(first, "za") {
			return true
		}
	}
	return false
}

func (s *LSPServer) handleDidSave(msg *JSONRPCMessage) {
	var params DidSaveParams
	json.Unmarshal(msg.Params, &params)

	s.mu.RLock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if doc == nil {
		return
	}

	// Cancel any pending debounce timers
	s.mu.Lock()
	if oldTimer, ok := s.timers[params.TextDocument.URI]; ok {
		oldTimer.Stop()
		delete(s.timers, params.TextDocument.URI)
	}
	if oldTimer, ok := s.fullTimers[params.TextDocument.URI]; ok {
		oldTimer.Stop()
		delete(s.fullTimers, params.TextDocument.URI)
	}
	s.mu.Unlock()

	// Run full diagnostics immediately on save
	s.runFullDiagnostics(params.TextDocument.URI)
}

// ---- Diagnostics ----

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source"`
}

func (s *LSPServer) publishDiagnostics(uri string, diagnostics []Diagnostic) {
	msg := JSONRPCMessage{
		JsonRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: json.RawMessage(fmt.Sprintf(`{"uri":"%s","diagnostics":%s}`,
			uri,
			func() []byte {
				data, _ := json.Marshal(diagnostics)
				return data
			}())),
	}
	data, _ := json.Marshal(msg)

	s.writeMu.Lock()
	if s.writer != nil {
		if err := writeLSPMessage(s.writer, data); err != nil {
			log.Printf("Failed to write diagnostic: %v", err)
		}
	} else {
		// Queue if writer not yet set
		s.pendingNotifs = append(s.pendingNotifs, &msg)
	}
	s.writeMu.Unlock()
}

func (s *LSPServer) getStructuralDiagnostics(uri string, content string) []Diagnostic {
	diagnostics := []Diagnostic{}
	lines := strings.Split(content, "\n")

	// Tokenize the whole document using the real lexer.
	tokenMap := tokenizeDocument(content)

	// ---- Block nesting using real lexer tokens ----
	blockStack := []string{}
	blockLines := []int{}

	for i := 0; i < len(lines); i++ {
		toks, ok := tokenMap[i]
		if !ok {
			continue
		}
		for _, tok := range toks {
			if tok.Kind != TokKeyword {
				continue
			}
			switch tok.Type {
			case lexer.C_Define:
				// 'pane define' is a method call, not a block opener.
				isMethodCall := false
				for _, t := range toks {
					if t.Type == lexer.C_Define {
						break
					}
					if t.Kind == TokIdentifier || t.Kind == TokKeyword {
						isMethodCall = true
						break
					}
				}
				if !isMethodCall {
					blockStack = append(blockStack, "def")
					blockLines = append(blockLines, i)
				}
			case lexer.C_If:
				// Statement-modifier 'if' (e.g., 'continue if cond') is not a block opener.
				isStmtMod := false
				for _, t := range toks {
					if t.Type == lexer.C_If {
						break
					}
					if t.Kind == TokIdentifier || t.Kind == TokKeyword {
						isStmtMod = true
						break
					}
				}
				if !isStmtMod {
					blockStack = append(blockStack, "if")
					blockLines = append(blockLines, i)
				}
			case lexer.C_For:
				blockStack = append(blockStack, "for")
				blockLines = append(blockLines, i)
			case lexer.C_Foreach:
				blockStack = append(blockStack, "foreach")
				blockLines = append(blockLines, i)
			case lexer.C_While:
				blockStack = append(blockStack, "while")
				blockLines = append(blockLines, i)
			case lexer.C_Struct:
				blockStack = append(blockStack, "struct")
				blockLines = append(blockLines, i)
			case lexer.C_Try:
				blockStack = append(blockStack, "try")
				blockLines = append(blockLines, i)
			case lexer.C_Case:
				blockStack = append(blockStack, "case")
				blockLines = append(blockLines, i)
			case lexer.C_Test:
				blockStack = append(blockStack, "test")
				blockLines = append(blockLines, i)
			case lexer.C_Enddef:
				// Generic 'end'/'enddef' - pop any block
				if len(blockStack) > 0 {
					blockStack = blockStack[:len(blockStack)-1]
					blockLines = blockLines[:len(blockLines)-1]
				}
			case lexer.C_Endif:
				if len(blockStack) > 0 && blockStack[len(blockStack)-1] == "if" {
					blockStack = blockStack[:len(blockStack)-1]
					blockLines = blockLines[:len(blockLines)-1]
				}
			case lexer.C_Endfor:
				if len(blockStack) > 0 && (blockStack[len(blockStack)-1] == "for" || blockStack[len(blockStack)-1] == "foreach") {
					blockStack = blockStack[:len(blockStack)-1]
					blockLines = blockLines[:len(blockLines)-1]
				}
			case lexer.C_Endwhile:
				if len(blockStack) > 0 && blockStack[len(blockStack)-1] == "while" {
					blockStack = blockStack[:len(blockStack)-1]
					blockLines = blockLines[:len(blockLines)-1]
				}
			case lexer.C_Endstruct:
				if len(blockStack) > 0 && blockStack[len(blockStack)-1] == "struct" {
					blockStack = blockStack[:len(blockStack)-1]
					blockLines = blockLines[:len(blockLines)-1]
				}
			case lexer.C_Endtry:
				if len(blockStack) > 0 && blockStack[len(blockStack)-1] == "try" {
					blockStack = blockStack[:len(blockStack)-1]
					blockLines = blockLines[:len(blockLines)-1]
				}
			case lexer.C_Endcase:
				if len(blockStack) > 0 && blockStack[len(blockStack)-1] == "case" {
					blockStack = blockStack[:len(blockStack)-1]
					blockLines = blockLines[:len(blockLines)-1]
				}
			case lexer.C_Endtest:
				if len(blockStack) > 0 && blockStack[len(blockStack)-1] == "test" {
					blockStack = blockStack[:len(blockStack)-1]
					blockLines = blockLines[:len(blockLines)-1]
				}
			}
		}
	}

	// Report unclosed blocks
	for i, block := range blockStack {
		diagnostics = append(diagnostics, Diagnostic{
			Range: Range{
				Start: Position{Line: blockLines[i], Character: 0},
				End:   Position{Line: blockLines[i], Character: 80},
			},
			Severity: 1, // Error
			Message:  fmt.Sprintf("Unclosed block: %s", block),
			Source:   "za-lsp",
		})
	}

	// ---- Bracket/paren/quote tracking using real lexer tokens ----
	fileParenDepth := 0
	fileBracketDepth := 0
	fileBraceDepth := 0
	parenStartLine := -1
	parenStartChar := 0
	bracketStartLine := -1
	bracketStartChar := 0
	braceStartLine := -1
	braceStartChar := 0

	for i := 0; i < len(lines); i++ {
		toks, ok := tokenMap[i]
		if !ok {
			continue
		}
		for _, tok := range toks {
			// Skip string and comment tokens entirely
			if tok.Kind == TokString || tok.Kind == TokComment {
				continue
			}

			// Bracket/paren/brace tracking
			switch tok.Kind {
			case TokLParen:
				if fileParenDepth == 0 {
					parenStartLine = i
					parenStartChar = tok.Start
				}
				fileParenDepth++
			case TokRParen:
				fileParenDepth--
				if fileParenDepth < 0 {
					diagnostics = append(diagnostics, Diagnostic{
						Range:    Range{Start: Position{Line: i, Character: tok.Start}, End: Position{Line: i, Character: tok.End}},
						Severity: 1,
						Message:  "Extra closing parenthesis",
						Source:   "za-lsp",
					})
					fileParenDepth = 0
				}
			case TokLBracket:
				if fileBracketDepth == 0 {
					bracketStartLine = i
					bracketStartChar = tok.Start
				}
				fileBracketDepth++
			case TokRBracket:
				fileBracketDepth--
				if fileBracketDepth < 0 {
					diagnostics = append(diagnostics, Diagnostic{
						Range:    Range{Start: Position{Line: i, Character: tok.Start}, End: Position{Line: i, Character: tok.End}},
						Severity: 1,
						Message:  "Extra closing bracket",
						Source:   "za-lsp",
					})
					fileBracketDepth = 0
				}
			case TokLBrace:
				if fileBraceDepth == 0 {
					braceStartLine = i
					braceStartChar = tok.Start
				}
				fileBraceDepth++
			case TokRBrace:
				fileBraceDepth--
				if fileBraceDepth < 0 {
					diagnostics = append(diagnostics, Diagnostic{
						Range:    Range{Start: Position{Line: i, Character: tok.Start}, End: Position{Line: i, Character: tok.End}},
						Severity: 1,
						Message:  "Extra closing brace",
						Source:   "za-lsp",
					})
					fileBraceDepth = 0
				}
			}
		}
	}

	// After scanning all lines, report anything still open at EOF
	if fileParenDepth > 0 {
		diagnostics = append(diagnostics, Diagnostic{
			Range:    Range{Start: Position{Line: parenStartLine, Character: parenStartChar}, End: Position{Line: parenStartLine, Character: parenStartChar + 1}},
			Severity: 1,
			Message:  "Unclosed parenthesis",
			Source:   "za-lsp",
		})
	}
	if fileBracketDepth > 0 {
		diagnostics = append(diagnostics, Diagnostic{
			Range:    Range{Start: Position{Line: bracketStartLine, Character: bracketStartChar}, End: Position{Line: bracketStartLine, Character: bracketStartChar + 1}},
			Severity: 1,
			Message:  "Unclosed bracket",
			Source:   "za-lsp",
		})
	}
	if fileBraceDepth > 0 {
		diagnostics = append(diagnostics, Diagnostic{
			Range:    Range{Start: Position{Line: braceStartLine, Character: braceStartChar}, End: Position{Line: braceStartLine, Character: braceStartChar + 1}},
			Severity: 1,
			Message:  "Unclosed brace",
			Source:   "za-lsp",
		})
	}

	return diagnostics
}

func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if s[i] == 'm' || s[i] == 'K' || s[i] == 'H' || s[i] == 'J' {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

// extractSymbols parses Za file for local definitions using the real lexer.
func (s *LSPServer) extractSymbols(uri string, content string) {
	doc := s.documents[uri]
	doc.Symbols = make(map[string]*Symbol)

	tokenMap := tokenizeDocument(content)

	for line, toks := range tokenMap {
		if len(toks) == 0 {
			continue
		}

		for t := 0; t < len(toks); t++ {
			tok := toks[t]
			if tok.Kind != TokKeyword {
				continue
			}

			switch tok.Type {
			case lexer.C_Define:
				if t+1 < len(toks) && toks[t+1].Kind == TokIdentifier {
					name := toks[t+1].Text
					doc.Symbols[name] = &Symbol{
						Name:     name,
						Kind:     "function",
						Location: SourceLocation{Line: line + 1, Column: toks[t+1].Start},
						Type:     "function",
					}
				}
			case lexer.C_Struct:
				if t+1 < len(toks) && toks[t+1].Kind == TokIdentifier {
					name := toks[t+1].Text
					doc.Symbols[name] = &Symbol{
						Name:     name,
						Kind:     "struct",
						Location: SourceLocation{Line: line + 1, Column: toks[t+1].Start},
						Type:     "struct",
					}
				}
			case lexer.C_Enum:
				if t+1 < len(toks) && toks[t+1].Kind == TokIdentifier {
					name := toks[t+1].Text
					doc.Symbols[name] = &Symbol{
						Name:     name,
						Kind:     "enum",
						Location: SourceLocation{Line: line + 1, Column: toks[t+1].Start},
						Type:     "enum",
					}
				}
			case lexer.C_Foreach:
				if t+1 < len(toks) && toks[t+1].Kind == TokIdentifier {
					name := toks[t+1].Text
					if _, exists := doc.Symbols[name]; !exists {
						doc.Symbols[name] = &Symbol{
							Name:     name,
							Kind:     "variable",
							Location: SourceLocation{Line: line + 1, Column: toks[t+1].Start},
							Type:     "variable",
						}
					}
				}
			case lexer.C_Var, lexer.C_SetGlob, lexer.C_Init:
				if t+1 < len(toks) && toks[t+1].Kind == TokIdentifier {
					name := toks[t+1].Text
					doc.Symbols[name] = &Symbol{
						Name:     name,
						Kind:     "variable",
						Location: SourceLocation{Line: line + 1, Column: toks[t+1].Start},
						Type:     "variable",
					}
				}
			}
		}

		// Find variable assignments: x = ... (not preceded by control keyword)
		for t := 0; t < len(toks); t++ {
			if toks[t].Kind != TokIdentifier {
				continue
			}
			if t+1 >= len(toks) {
				continue
			}
			next := toks[t+1]
			if (next.Kind != TokOperator && next.Kind != TokAssign) || next.Text != "=" {
				continue
			}
			if t > 0 && toks[t-1].Kind == TokKeyword && isControlKeyword(toks[t-1].Text) {
				continue
			}
			name := toks[t].Text
			if _, exists := doc.Symbols[name]; !exists {
				doc.Symbols[name] = &Symbol{
					Name:     name,
					Kind:     "variable",
					Location: SourceLocation{Line: line + 1, Column: toks[t].Start},
					Type:     "variable",
				}
			}
		}
	}
}

func isControlKeyword(text string) bool {
	switch text {
	case "def", "struct", "enum", "foreach", "for", "if", "while", "case", "try", "catch", "then":
		return true
	}
	return false
}

type CompletionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
}

type CompletionItem struct {
	Label       string `json:"label"`
	Kind        int    `json:"kind"`
	Detail      string `json:"detail,omitempty"`
	Description string `json:"description,omitempty"`
}

func (s *LSPServer) handleCompletion(msg *JSONRPCMessage) *JSONRPCMessage {
	var params CompletionParams
	json.Unmarshal(msg.Params, &params)

	s.mu.RLock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if doc == nil {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: []interface{}{}}
	}

	// Extract prefix being typed
	lines := strings.Split(doc.Content, "\n")
	prefix := ""
	if params.Position.Line < len(lines) {
		line := lines[params.Position.Line]
		// Don't complete inside strings or comments
		if isInsideStringOrComment(line, params.Position.Character) {
			return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: []interface{}{}}
		}
		prefix = getPrefixAtCursor(line, params.Position.Character)
	}
	prefixLower := strings.ToLower(prefix)

	items := []CompletionItem{}

	// Add stdlib functions (filtered)
	for _, funcInfo := range s.lib.AllFunctions("") {
		if prefix == "" || strings.HasPrefix(strings.ToLower(funcInfo.Name), prefixLower) {
			items = append(items, CompletionItem{
				Label:       funcInfo.Name,
				Kind:        12, // Function
				Detail:      fmt.Sprintf("[%s] %s(%s)", funcInfo.Category, funcInfo.Name, funcInfo.Inputs),
				Description: funcInfo.Description,
			})
		}
	}

	// Add local symbols (filtered)
	for name, sym := range doc.Symbols {
		if prefix == "" || strings.HasPrefix(strings.ToLower(name), prefixLower) {
			kind := 6 // Variable
			if sym.Kind == "function" {
				kind = 12 // Function
			} else if sym.Kind == "struct" {
				kind = 5 // Class
			} else if sym.Kind == "enum" {
				kind = 10 // Enum
			}
			items = append(items, CompletionItem{
				Label: name,
				Kind:  kind,
			})
		}
	}

	// Add keywords (filtered)
	keywords := []string{"if", "else", "endif", "for", "foreach", "endfor", "while", "endwhile",
		"def", "end", "struct", "endstruct", "enum", "case", "is", "contains", "try", "catch", "endtry",
		"local", "global", "const", "return", "break", "continue", "then", "throws", "throw"}
	for _, kw := range keywords {
		if prefix == "" || strings.HasPrefix(kw, prefixLower) {
			items = append(items, CompletionItem{
				Label: kw,
				Kind:  14, // Keyword
			})
		}
	}

	// Add builtins (language constructs not in stdlib metadata)
	builtins := []string{"println", "print", "input", "prompt", "log", "logging", "cls",
		"at", "showdef", "version", "exit", "require", "debug", "hist", "nop", "help",
		"pause", "quiet", "loud", "unset"}
	for _, b := range builtins {
		if prefix == "" || strings.HasPrefix(b, prefixLower) {
			items = append(items, CompletionItem{
				Label: b,
				Kind:  14, // Keyword
			})
		}
	}

	return &JSONRPCMessage{
		JsonRPC: "2.0",
		ID:      msg.ID,
		Result:  items,
	}
}

type HoverParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
}

type Hover struct {
	Contents string `json:"contents"`
}

func (s *LSPServer) handleHover(msg *JSONRPCMessage) *JSONRPCMessage {
	var params HoverParams
	json.Unmarshal(msg.Params, &params)

	s.mu.RLock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if doc == nil {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	}

	// Extract word at cursor (simple implementation)
	lines := strings.Split(doc.Content, "\n")
	if params.Position.Line >= len(lines) {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	}

	line := lines[params.Position.Line]
	word := extractWordAtPosition(line, params.Position.Character)

	// Check if it's a stdlib function
	if funcInfo := s.lib.GetFunction(word); funcInfo != nil {
		content := fmt.Sprintf("**%s** (%s)\n\n%s\n\nInputs: `%s`\n\nOutputs: `%s`",
			funcInfo.Name,
			funcInfo.Category,
			funcInfo.Description,
			funcInfo.Inputs,
			funcInfo.Outputs)

		return &JSONRPCMessage{
			JsonRPC: "2.0",
			ID:      msg.ID,
			Result: &Hover{Contents: content},
		}
	}

	// Check if it's a local symbol
	if sym, ok := doc.Symbols[word]; ok {
		content := fmt.Sprintf("**%s** (%s)\n\nDefined at line %d", sym.Name, sym.Kind, sym.Location.Line)
		return &JSONRPCMessage{
			JsonRPC: "2.0",
			ID:      msg.ID,
			Result: &Hover{Contents: content},
		}
	}

	return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
}

// ---- Signature Help ----

type SignatureHelpParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
}

type SignatureInformation struct {
	Label      string          `json:"label"`
	Parameters []ParameterInfo `json:"parameters,omitempty"`
}

type ParameterInfo struct {
	Label string `json:"label"`
}

type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

func (s *LSPServer) handleSignatureHelp(msg *JSONRPCMessage) *JSONRPCMessage {
	var params SignatureHelpParams
	json.Unmarshal(msg.Params, &params)

	s.mu.RLock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if doc == nil {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	}

	lines := strings.Split(doc.Content, "\n")
	if params.Position.Line >= len(lines) {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	}

	line := lines[params.Position.Line]
	char := params.Position.Character
	if char > len(line) {
		char = len(line)
	}

	// Find function name before cursor by walking backwards
	funcName := ""
	parenDepth := 0
	commaCount := 0

	for i := char - 1; i >= 0; i-- {
		ch := line[i]
		if ch == ')' {
			parenDepth++
			continue
		}
		if ch == '(' {
			if parenDepth == 0 {
				// Found the opening paren, now get function name before it
				nameEnd := i
				for nameEnd > 0 && line[nameEnd-1] == ' ' {
					nameEnd--
				}
				nameStart := nameEnd - 1
				for nameStart >= 0 && isWordChar(rune(line[nameStart])) {
					nameStart--
				}
				funcName = line[nameStart+1 : nameEnd]
				break
			}
			parenDepth--
			continue
		}
		if parenDepth == 0 && ch == ',' {
			commaCount++
		}
	}

	if funcName == "" {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	}

	// Look up function
	var funcInfo *FunctionInfo
	if fi := s.lib.GetFunction(funcName); fi != nil {
		funcInfo = fi
	} else if sym, ok := doc.Symbols[funcName]; ok && sym.Kind == "function" {
		// Local function - we don't have signature info for local defs yet
		funcInfo = &FunctionInfo{Name: funcName, Inputs: "?", Outputs: "?"}
	}

	if funcInfo == nil {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	}

	// Parse inputs into parameter list
	paramsList := []ParameterInfo{}
	if funcInfo.Inputs != "" {
		for _, p := range strings.Split(funcInfo.Inputs, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				paramsList = append(paramsList, ParameterInfo{Label: p})
			}
		}
	}

	sig := SignatureInformation{
		Label:      fmt.Sprintf("%s(%s) -> %s", funcInfo.Name, funcInfo.Inputs, funcInfo.Outputs),
		Parameters: paramsList,
	}

	return &JSONRPCMessage{
		JsonRPC: "2.0",
		ID:      msg.ID,
		Result: SignatureHelp{
			Signatures:      []SignatureInformation{sig},
			ActiveSignature: 0,
			ActiveParameter: commaCount,
		},
	}
}

type DefinitionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

func (s *LSPServer) handleDefinition(msg *JSONRPCMessage) *JSONRPCMessage {
	var params DefinitionParams
	json.Unmarshal(msg.Params, &params)

	s.mu.RLock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if doc == nil {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	}

	lines := strings.Split(doc.Content, "\n")
	if params.Position.Line >= len(lines) {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	}

	line := lines[params.Position.Line]
	word := extractWordAtPosition(line, params.Position.Character)

	// Check local symbols
	if sym, ok := doc.Symbols[word]; ok {
		return &JSONRPCMessage{
			JsonRPC: "2.0",
			ID:      msg.ID,
			Result: &Location{
				URI: params.TextDocument.URI,
				Range: Range{
					Start: Position{Line: sym.Location.Line - 1, Character: 0},
					End:   Position{Line: sym.Location.Line - 1, Character: 80},
				},
			},
		}
	}

	return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
}

func (s *LSPServer) handleReferences(msg *JSONRPCMessage) *JSONRPCMessage {
	return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: []interface{}{}}
}

type DocumentSymbolParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

type DocumentSymbol struct {
	Name     string `json:"name"`
	Kind     int    `json:"kind"`
	Location Location `json:"location"`
}

func (s *LSPServer) handleDocumentSymbol(msg *JSONRPCMessage) *JSONRPCMessage {
	var params DocumentSymbolParams
	json.Unmarshal(msg.Params, &params)

	s.mu.RLock()
	doc := s.documents[params.TextDocument.URI]
	s.mu.RUnlock()

	if doc == nil {
		return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: nil}
	}

	symbols := []DocumentSymbol{}
	for name, sym := range doc.Symbols {
		kind := 12 // Function
		if sym.Kind == "struct" {
			kind = 5 // Class
		} else if sym.Kind == "enum" {
			kind = 10 // Enum
		}
		symbols = append(symbols, DocumentSymbol{
			Name: name,
			Kind: kind,
			Location: Location{
				URI: params.TextDocument.URI,
				Range: Range{
					Start: Position{Line: sym.Location.Line - 1, Character: 0},
					End:   Position{Line: sym.Location.Line - 1, Character: 80},
				},
			},
		})
	}

	return &JSONRPCMessage{JsonRPC: "2.0", ID: msg.ID, Result: symbols}
}

// Helper to extract word at cursor position
func extractWordAtPosition(line string, char int) string {
	if char > len(line) {
		char = len(line)
	}

	// Find word boundaries
	start := char
	for start > 0 && (isWordChar(rune(line[start-1]))) {
		start--
	}

	end := char
	for end < len(line) && isWordChar(rune(line[end])) {
		end++
	}

	return line[start:end]
}

func isWordChar(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// ---- LSP stdio protocol helpers ----

func readLSPMessage(reader *bufio.Reader) ([]byte, error) {
	var contentLength int

	// Read headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // end of headers
		}
		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header")
	}

	// Read exactly contentLength bytes
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func writeLSPMessage(writer *bufio.Writer, data []byte) error {
	fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(data))
	_, err := writer.Write(data)
	if err != nil {
		return err
	}
	return writer.Flush()
}

// ---- Main Loop ----

func main() {
	// Redirect stderr to a log file for debugging when launched by LSP clients
	if logFile := os.Getenv("ZA_LSP_LOG"); logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			log.SetOutput(f)
		}
	}

	// Find Za binary
	zaPath := "za"
	if len(os.Args) > 1 {
		zaPath = os.Args[1]
	}

	log.Printf("[LSP] Starting with za binary: %s", zaPath)

	// Load library metadata
	lib, err := LoadLibrary(zaPath)
	if err != nil {
		log.Fatalf("Failed to load Za library: %v", err)
	}

	server := NewLSPServer(lib, zaPath)

	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	server.writer = writer

	for {
		body, err := readLSPMessage(reader)
		if err != nil {
			if err == io.EOF {
				log.Printf("[LSP] EOF on stdin, exiting")
				break
			}
			log.Printf("Failed to read message: %v", err)
			continue
		}

		var msg JSONRPCMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		response := server.HandleMessage(&msg)

		if response != nil {
			data, _ := json.Marshal(response)
			log.Printf("[LSP] Sending response (len=%d)", len(data))
			if err := writeLSPMessage(writer, data); err != nil {
				log.Printf("Failed to write response: %v", err)
			}
		}
	}
}
