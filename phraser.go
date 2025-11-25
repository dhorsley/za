package main

import (
	"context"
	"fmt"
	str "strings"
	"sync"
	"time"
)

// func space to source file name mappings
var fileMap sync.Map

// global binding list - populated during phrasing
var bindings = make([]map[string]uint64, SPACE_CAP)
var bindlock = &sync.RWMutex{}

// try block metadata storage - maps parent function space to try block info
var tryBlocks = make(map[uint32][]tryBlockInfo)
var tryBlockLock = &sync.RWMutex{}

// global try block registry for enhanced nested context tracking
var tryBlockRegistry = make(map[int]*tryBlockInfo)
var tryBlockCounter int = 0
var tryBlockRegistryLock = &sync.RWMutex{}

// DOC statement metadata storage
type docInfo struct {
	content string
	varName string
	delim   string
	hasGen  bool
	line    int16
}

var docRegistry []docInfo
var docRegistryLock = &sync.RWMutex{}

func bindResize() {
	newar := make([]map[string]uint64, cap(bindings)*2)
	copy(newar, bindings)
	bindings = newar
}

func bind_int(fs uint32, name string) (i uint64) {

	// fmt.Printf("Bind request for %s (fs:%d)\n",name,fs)

	bindlock.Lock()

	if bindings[fs] == nil {
		bindings[fs] = make(map[string]uint64)
		// fmt.Printf("** CLEANED BINDINGS FOR FS %d\n",fs)
	}

	var present bool
	i, present = bindings[fs][name]
	if present {
		// fmt.Printf("present @ %d\n",i)
		bindlock.Unlock()
		return
	}

	// assign if unused:
	loop := true
	i = uint64(len(bindings[fs]))
	for loop {
		loop = false
		for _, vp := range bindings[fs] {
			if vp == i {
				i += 1
				loop = true
				break
			}
		}
		if !loop {
			break
		}
	}

	bindings[fs][name] = i
	// fmt.Printf("new binding @ %d\n",i)
	bindlock.Unlock()
	return
}

func getFileFromIFS(ifs uint32) string {
	v, ok := fileMap.Load(ifs)
	if !ok {
		panic(fmt.Sprintf("getFileFromIFS: IFS %d not found in fileMap", ifs))
	}
	return v.(string)
}

func getIFSFromFile(f string) uint32 {
	var found uint32 = 0
	fileMap.Range(func(k, v any) bool {
		if v.(string) == f {
			found = k.(uint32)
			return false // stop iteration
		}
		return true
	})
	if found == 0 {
		panic(fmt.Sprintf("getIFSFromFile: file %q not found in fileMap", f))
	}
	return found
}

// Try block processing is now handled completely inline in the main parsing loop

// phraseParse():
//
//   process an input string into separate lines of commands (Phrases). Each phrase is
//   built from successive calls to nextToken(). Input ends at end-of-string or earlier
//   if an invalid token is found.
//
//   Each phrase is appended to the 'function space' (i.e. function body) of the function
//   referenced by fs. A phrase is a list of processed tokens.
//
//   functionspaces[] is a global.
//

func phraseParse(ctx context.Context, fs string, input string, start int, lineOffset int) (badword bool, eof bool) {

	startTime := time.Now()

	input += "\n"

	input = macroExpand(input)
	pos := start
	lstart := start

	var tempToken *lcstruct
	phrase := Phrase{
		Tokens: make([]Token, 0, 8),
	}
	var base = BaseCode{}

	tokenType := Error
	curLine := int16(lineOffset)

	// simple handler for parens nesting
	var braceNestLevel int       // round braces
	var sbraceNestLevel int      // square braces
	var defNest int              // C_Define nesting
	var tryNest int              // C_Try nesting
	var tryStartOffset int       // character offset in input string where try block started
	var tryContentStart int = -1 // character offset where try block content starts (after first EOL)
	var tryStartLine int16       // line number where try block started
	var tryEndLine int16         // line number where try block ended
	var tryBlockCounter int = 0  // counter for unique try block naming

	// DOC parsing variables
	var docMode bool
	var docDelim string
	var docHasGen bool
	var docVarName string
	var docLine int16

	lmv, _ := fnlookup.lmget(fs)
	isSource[lmv] = true

	fspacelock.Lock()
	functionspaces[lmv] = make([]Phrase, 0, 8)
	basecode[lmv] = make([]BaseCode, 0, 8)
	fspacelock.Unlock()

	addToPhrase := false
	vref_found := false

	assert_found := false
	on_found := false
	do_found := false
	borpos := -1
	discard_phrase := false

	lastTokenType := Error

	for pos < len(input) {

		if tempToken != nil {
			lastTokenType = tempToken.carton.tokType
		}

		tempToken = nextToken(input, lmv, &curLine, pos)
		eof = tempToken.eof

		if on_found && do_found || !(on_found || do_found) {
			if tempToken.borpos > borpos && borpos == -1 {
				borpos = tempToken.borpos
			}
		}

		// If we found something then move the cursor along to next word
		if tempToken.tokPos != -1 {
			pos = tempToken.tokPos
		}

		tokenType = tempToken.carton.tokType

		// var_refs display
		if var_refs && tokenType == Identifier {
			if tempToken.carton.tokText == var_refs_name {
				vref_found = true
			}
		}

		// remove asserts?
		if !assert_found && tokenType == C_Assert && !enableAsserts {
			discard_phrase = true
			assert_found = true
		}

		// ON present?
		if !on_found && tokenType == C_On {
			on_found = true
		}

		// DO present?
		if !do_found && tokenType == C_Do {
			do_found = true
		}

		// function name token mangling:
		if phrase.TokenCount > 0 {
			if tokenType == LParen {
				prevText := phrase.Tokens[phrase.TokenCount-1].tokText
				if _, isFunc := stdlib[prevText]; !isFunc {
					if fnlookup.lmexists(prevText) {
						phrase.Tokens[phrase.TokenCount-1].subtype = subtypeUser
					}
				} else {
					phrase.Tokens[phrase.TokenCount-1].subtype = subtypeStandard
				}
			}
		}

		// @note: this may trip up if these tokens are beyond
		// position 0, but not had any issues yet:
		switch tokenType {
		case C_Define:
			defNest += 1
		case C_Enddef:
			defNest -= 1
		case C_Try:
			tryNest += 1
			if tryNest == 1 {
				tryStartOffset = lstart
				tryStartLine = curLine + 1
				tryContentStart = -1 // Will be set when we encounter first content

			}
		case C_Doc:
			docMode = true
			docLine = curLine
		case C_Endtry:
			// Process try block when exiting outermost try
			if tryNest == 1 {
				tryEndLine = curLine + 1

				// Extract and process try block content
				if tryStartOffset > 0 && tryContentStart >= 0 {
					if tryContentStart < pos {
						tryBlockContent := input[tryContentStart:pos]
						// pf("DEBUG: phraseParse try block - content length=%d\n", len(tryBlockContent))
						// pf("DEBUG: try block content:\n'%s'\n", tryBlockContent)

						// Create new function space for try block
						tryBlockCounter++

						// Temporarily ensure globseq is at least 4 to avoid interfering with main execution IDs
						originalGlobseq := globseq
						if globseq < 4 {
							globseq = 4
						}

						tryFS, tryFSName := GetNextFnSpace(true, sf("try_block_%d_%d@", lmv, tryBlockCounter), call_s{
							prepared:   true,
							base:       lmv, // Temporarily set to main, will be updated after parsing
							caller:     lmv,
							gc:         false,
							gcShyness:  100,
							isTryBlock: true, // Mark this function space as a try block
						})

						// Restore original globseq if it was modified
						if originalGlobseq < 4 {
							globseq = originalGlobseq
						}

						// Set base to tryFS so try block executes its own code
						calllock.Lock()
						calltable[tryFS].base = tryFS
						// fmt.Printf("[DEBUG] Set calltable[%d].base = %d (so it executes its own code)\n", tryFS, tryFS)
						calllock.Unlock()

						// Set up fileMap entry for try block function space
						if parentFileMap, exists := fileMap.Load(lmv); exists {
							fileMap.Store(tryFS, parentFileMap)
						}

						// Recursively parse try block content
						ctx := context.Background()
						badword_try, _ := phraseParse(ctx, tryFSName, tryBlockContent, 0, int(tryStartLine))
						if badword_try {
							fmt.Printf("Error parsing try block content\n")
							badword = true
						} else {
							// Determine where to store try block metadata
							// Store in immediate parent function space, but not in try block function spaces
							storageFS := lmv
							currentFSName, _ := numlookup.lmget(lmv)
							if str.Contains(currentFSName, "try_block_") {
								calllock.RLock()
								storageFS = calltable[lmv].caller
								calllock.RUnlock()
							}

							// For user-defined functions, we need to store the try block in the function space
							// where the function is defined, not where it's called from
							if defNest > 0 {
								storageFS = lmv
							}

							// Create execution path for context tracking
							executionPath := make([]uint32, 0)
							executionPath = append(executionPath, lmv)

							// Determine parent try block ID (for nested try blocks)
							parentTryBlockID := -1

							// Line numbers are already correct from lineOffset parameter
							relativePC := tryStartLine
							adjustedStartLine := tryStartLine
							adjustedEndLine := tryEndLine

							registerTryBlock(tryFS, adjustedStartLine, adjustedEndLine, storageFS, tryNest, parentTryBlockID, executionPath, relativePC)
						}
					}
				}

				// Reset try block tracking
				tryStartOffset = 0
				tryContentStart = -1
				tryStartLine = 0
				tryEndLine = 0
			}
			tryNest -= 1

		case LParen:
			braceNestLevel += 1
		case RParen:
			braceNestLevel -= 1
		case LeftSBrace:
			sbraceNestLevel += 1
		case RightSBrace:
			sbraceNestLevel -= 1
		}

		if sbraceNestLevel > 0 || braceNestLevel > 0 {
			if tempToken.eol || tokenType == EOL {
				curLine += 1
				continue
			}
		}

		// handle end-of-line dot character continuation.
		// we check borpos to ensure we are not inside a | statement also.
		// this is just meant to catch using . operator in Za multi-line expressions:
		if borpos == -1 && !permit_cmd_fallback && tempToken.eol && lastTokenType == SYM_DOT {
			curLine += 1
			continue
		}

		if tokenType == Error {
			fmt.Printf("Error found on line %d in %s\n", curLine+1, tempToken.carton.tokText)
			break
		}

		addToPhrase = true

		if tokenType == SingleComment {
			// at this point we have returned the full comment so throw it away!
			// fmt.Printf("[parse] Discarding comment : '%+v'\n",tempToken.carton.tokText)
			addToPhrase = false
		}

		if tokenType == SYM_Semicolon || tokenType == EOL { // ditto
			addToPhrase = false
		}

		if addToPhrase {
			phrase.Tokens = append(phrase.Tokens, tempToken.carton)
			phrase.TokenCount += 1
		}

		if tokenType == EOL || tokenType == SYM_Semicolon {

			// -- add original version
			if pos > 0 {
				if phrase.TokenCount > 0 {
					base.Original = input[lstart:pos]
					if borpos >= 0 {
						base.borcmd = input[borpos:pos]
					}
					if tempToken.carton.tokType == EOL {
						base.Original = base.Original[:pos-lstart-1]
					}

				} else {
					base.Original = ""
				}
				// pf(".Original -> ·%s·\n",base.Original)
			}

			if vref_found {
				pf("[#3]%s[#-] | Line [#6]%4d[#-] : %s\n", getFileFromIFS(lmv), curLine+1, str.TrimLeft(base.Original, " \t"))
				vref_found = false
			}

			phrase.SourceLine = curLine
			lstart = pos

			if tokenType == EOL {
				curLine += 1
			}

			// fmt.Printf("\nCurrent phrase: %+v\n",phrase)

			// -- discard empty lines, add phrase to func store
			if phrase.TokenCount != 0 {
				if !discard_phrase {
					if docMode {
						// parse clauses
						var content string
						var hasDelim bool
						for i, tok := range phrase.Tokens[1:] {
							lowerText := str.ToLower(tok.tokText)
							if lowerText == "gen" || lowerText == "delim" || lowerText == "var" {
								switch lowerText {
								case "gen":
									docHasGen = true
								case "delim":
									hasDelim = true
									if i+1 < len(phrase.Tokens[1:]) {
										docDelim = phrase.Tokens[1+i+1].tokText
									}
								case "var":
									if i+1 < len(phrase.Tokens[1:]) {
										docVarName = phrase.Tokens[1+i+1].tokText
									}
								}
							} else if tok.tokType == StringLiteral {
								content = tok.tokText
							}
						}
						if !hasDelim {
							docDelim = "\n\n"
							hasDelim = true
						}
						// read raw content if needed
						if hasDelim && content == "" {
							endPos := str.Index(input[pos:], docDelim)
							if endPos != -1 {
								content = input[pos : pos+endPos]
								content = str.TrimSuffix(content, "\n")
								// update curLine for skipped lines
								skipped := input[pos : pos+endPos+len(docDelim)]
								numLines := str.Count(skipped, "\n")
								if len(skipped) > 0 && skipped[len(skipped)-1] != '\n' {
									numLines++
								}
								curLine += int16(numLines)
								pos += endPos + len(docDelim)
							} else {
								badword = true
							}
						}
						// store in registry
						docRegistryLock.Lock()
						docRegistry = append(docRegistry, docInfo{
							content: content,
							varName: docVarName,
							delim:   docDelim,
							hasGen:  docHasGen,
							line:    docLine,
						})
						docRegistryLock.Unlock()
						// reset
						docMode = false
						docDelim = ""
						docHasGen = false
						docVarName = ""
					}

					// Record content start position if we're inside a try block
					if tryNest > 0 && tryContentStart == -1 {
						tryContentStart = lstart // Start of first phrase inside try block

					}

					// Only add phrases to function space if not inside try block
					// but DO include try/endtry statements themselves for execution
					// Include endtry in both parent and try block function spaces
					if tryNest == 0 || phrase.Tokens[0].tokType == C_Try || phrase.Tokens[0].tokType == C_Endtry || phrase.Tokens[0].tokType == C_Doc {
						fspacelock.Lock()
						functionspaces[lmv] = append(functionspaces[lmv], phrase)
						basecode[lmv] = append(basecode[lmv], base)
						fspacelock.Unlock()
					}

				}
			}

			// reset phrase
			phrase = Phrase{}
			base = BaseCode{}
			borpos = -1
			do_found = false
			on_found = false
			assert_found = false
			discard_phrase = false

		}

		if eof {
			break
		}

	}

	// Try block extraction happens during phrasing, not after

	recordPhase(ctx, "parse", time.Since(startTime))

	return badword, eof

}

// Note: Try blocks are now handled directly during phraseParse(), not as a separate post-processing step

// Helper function to register a try block with enhanced metadata
func registerTryBlock(functionSpace uint32, startLine int16, endLine int16, parentFS uint32, nestLevel int, parentTryBlockID int, executionPath []uint32, relativePC int16) *tryBlockInfo {
	// Generate unique try block ID
	tryBlockRegistryLock.Lock()
	tryBlockCounter++
	tryBlockID := tryBlockCounter
	tryBlockRegistryLock.Unlock()

	// Create try block info with enhanced metadata
	tryInfo := &tryBlockInfo{
		functionSpace: functionSpace,
		startLine:     startLine,
		endLine:       endLine,
		category:      "", // Will be extracted from try statement during execution
		parentFS:      parentFS,
		nestLevel:     nestLevel,
		catchBlocks:   nil, // Parsed during execution
		finallyBlock:  nil, // Parsed during execution

		// Enhanced nested context fields
		parentTryBlockID: parentTryBlockID,
		tryBlockID:       tryBlockID,
		executionPath:    executionPath,
		relativePC:       relativePC,
		childTryBlocks:   make([]int, 0),
	}

	// Register in global registry
	tryBlockRegistryLock.Lock()
	tryBlockRegistry[tryBlockID] = tryInfo
	tryBlockRegistryLock.Unlock()

	// Add to legacy storage for backward compatibility
	tryBlockLock.Lock()
	if tryBlocks[parentFS] == nil {
		tryBlocks[parentFS] = make([]tryBlockInfo, 0)
	}
	tryBlocks[parentFS] = append(tryBlocks[parentFS], *tryInfo)
	tryBlockLock.Unlock()

	// Update parent try block's child list if there is a parent
	if parentTryBlockID != -1 {
		tryBlockRegistryLock.Lock()
		if parentTryInfo, exists := tryBlockRegistry[parentTryBlockID]; exists {
			parentTryInfo.childTryBlocks = append(parentTryInfo.childTryBlocks, tryBlockID)
		}
		tryBlockRegistryLock.Unlock()
	}

	return tryInfo
}

// Helper function to check if a character is alphanumeric
func isAlphaNumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}
