package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

func buildINILib() {
	features["ini"] = Feature{version: 1, category: "ini"}
	categories["ini"] = []string{
		"ini_read",
		"ini_write",
		"ini_meta_update",
		"ini_get_global",
		"ini_set_global",
		"ini_new_section",
		"ini_insert_section",
		"ini_delete_section",
	}

	slhelp["ini_read"] = LibHelp{
		in:     "filepath",
		out:    "map",
		action: "Read an INI file and return a map of sections. Empty string key is global section.",
	}

	slhelp["ini_write"] = LibHelp{
		in:     "ini_map,filepath",
		out:    "nil",
		action: "Write an INI map to file. Preserves comments and formatting.",
	}

	slhelp["ini_meta_update"] = LibHelp{
		in:     "ini_map",
		out:    "map",
		action: "Renumber section orders in map to ensure sequential consistency.",
	}

	slhelp["ini_get_global"] = LibHelp{
		in:     "ini_map",
		out:    "array",
		action: "Get global section entries (before any [section] header).",
	}

	slhelp["ini_set_global"] = LibHelp{
		in:     "ini_map,entries",
		out:    "map",
		action: "Set global section entries.",
	}

	slhelp["ini_new_section"] = LibHelp{
		in:     "ini_map,section_name",
		out:    "map",
		action: "Append new section at end of order and renumber.",
	}

	slhelp["ini_insert_section"] = LibHelp{
		in:     "ini_map,section_name,position",
		out:    "map",
		action: "Insert section at position (1-indexed, 0=prepend) and renumber.",
	}

	slhelp["ini_delete_section"] = LibHelp{
		in:     "ini_map,section_name",
		out:    "map",
		action: "Delete section by name and renumber remaining sections.",
	}

	stdlib["ini_read"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ini_read", args, 1, "1", "string"); !ok {
			return nil, err
		}

		filepath := args[0].(string)
		tokens, originalContent, err := lexINIFile(filepath)
		if err != nil {
			return nil, err
		}

		iniMap, err := parseINITokens(tokens, originalContent)
		if err != nil {
			return nil, err
		}

		return iniMap, nil
	}

	stdlib["ini_write"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ini_write", args, 1, "2", "map", "string"); !ok {
			return nil, err
		}

		iniMap := args[0].(map[string][]any)
		filepath := args[1].(string)
		err = iniWrite(iniMap, filepath)
		if err != nil {
			return nil, err
		}

		return nil, nil
	}

	stdlib["ini_meta_update"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ini_meta_update", args, 1, "1", "map"); !ok {
			return nil, err
		}

		iniMap := args[0].(map[string][]any)
		result, err := iniMetaUpdate(iniMap)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	stdlib["ini_get_global"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ini_get_global", args, 1, "1", "map"); !ok {
			return nil, err
		}

		iniMap := args[0].(map[string][]any)
		result, err := iniGetGlobal(iniMap)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	stdlib["ini_set_global"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ini_set_global", args, 1, "2", "map", "array"); !ok {
			return nil, err
		}

		iniMap := args[0].(map[string][]any)
		entries := args[1].([]any)
		err = iniSetGlobal(iniMap, entries)
		if err != nil {
			return nil, err
		}

		return iniMap, nil
	}

	stdlib["ini_new_section"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ini_new_section", args, 1, "2", "map", "string"); !ok {
			return nil, err
		}

		iniMap := args[0].(map[string][]any)
		sectionName := args[1].(string)
		result, err := iniNewSection(iniMap, sectionName)
        pf("[ins] result -> [%T] %+v\n",result,result)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	stdlib["ini_insert_section"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ini_insert_section", args, 1, "3", "map", "string", "int"); !ok {
			return nil, err
		}

		iniMap := args[0].(map[string][]any)
		sectionName := args[1].(string)
		position := args[2].(int)
		result, err := iniInsertSection(iniMap, sectionName, position)
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	stdlib["ini_delete_section"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ini_delete_section", args, 1, "2", "map", "string"); !ok {
			return nil, err
		}

		iniMap := args[0].(map[string][]any)
		sectionName := args[1].(string)
		result, err := iniDeleteSection(iniMap, sectionName)
		if err != nil {
			return nil, err
		}

		return result, nil
	}
}

func lexINIFile(filepath string) ([]*lcstruct, string, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, "", err
	}

	if len(content) == 0 {
		return nil, "", errors.New("ini_read: file is empty")
	}

	tempFS, _ := GetNextFnSpace(true, "ini_lex_temp@", call_s{
		prepared:   true,
		base:       0,
		caller:     0,
		gc:         true,
		disposable: true,
	})

	tokens := lexINIString(string(content), tempFS)
	return tokens, string(content), nil
}

func lexINIString(content string, fs uint32) []*lcstruct {
	var tokens []*lcstruct
	pos := 0
	curLine := int16(0)

	for {
		lc := nextToken(content, fs, &curLine, pos)
		tokens = append(tokens, lc)

		if lc.eof {
			break
		}
		if lc.tokPos == -1 {
			return nil
		}
		pos = lc.tokPos
	}

	return tokens
}

func reconstructSectionName(tokens []Token) string {
	var name strings.Builder
	for _, tok := range tokens {
		name.WriteString(tok.tokText)
	}
	return name.String()
}

func isBlankLine(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			return false
		}
	}
	return true
}

func isZaArrayLiteral(tokens []Token) bool {
	return len(tokens) >= 2 &&
		tokens[0].tokType == LeftSBrace &&
		tokens[len(tokens)-1].tokType == RightSBrace
}

func parseZaArrayLiteral(tokens []Token) ([]any, error) {
	var values []any
	for i := 1; i < len(tokens)-1; i++ {
		tok := tokens[i]
		if tok.tokType == O_Comma {
			continue
		}
		if tok.tokType == NumericLiteral || tok.tokType == StringLiteral ||
			tok.subtype == subtypeConst {
			values = append(values, tok.tokVal)
		} else {
			return nil, fmt.Errorf("unsupported type in array literal")
		}
	}
	return values, nil
}

func isCSVArray(tokens []Token) bool {
	if len(tokens) < 3 {
		return false
	}
	for i, tok := range tokens {
		if i%2 == 0 {
			if tok.tokType != NumericLiteral && tok.tokType != StringLiteral &&
				tok.subtype != subtypeConst {
				return false
			}
		} else {
			if tok.tokType != O_Comma {
				return false
			}
		}
	}
	return true
}

func parseCSVArray(tokens []Token) ([]any, error) {
	var values []any
	for _, tok := range tokens {
		if tok.tokType == O_Comma {
			continue
		}
		if tok.tokType == NumericLiteral || tok.tokType == StringLiteral ||
			tok.subtype == subtypeConst {
			values = append(values, tok.tokVal)
		} else {
			return nil, fmt.Errorf("unsupported type in CSV array")
		}
	}
	return values, nil
}

func parseValueAndComment(tokens []*lcstruct, startIdx int) (any, string, string, int, error) {
	valueTokens := []Token{}
	comment := ""
	idx := startIdx

	for idx < len(tokens) {
		tok := tokens[idx].carton

		if tok.tokType == EOL || tokens[idx].eol {
			break
		}

		if tok.tokType == SYM_Semicolon {
			for ci := idx + 1; ci < len(tokens) && tokens[ci].carton.tokType != EOL && !tokens[ci].eol; ci++ {
				comment += tokens[ci].carton.tokText
			}
			break
		}

		valueTokens = append(valueTokens, tok)
		idx++
	}

	if len(valueTokens) == 0 {
		return nil, "", comment, idx, nil
	}

	if isZaArrayLiteral(valueTokens) {
		values, err := parseZaArrayLiteral(valueTokens)
		if err != nil {
			return nil, "", comment, idx, err
		}
		return values, "za", comment, idx, nil
	}

	if isCSVArray(valueTokens) {
		values, err := parseCSVArray(valueTokens)
		if err != nil {
			return nil, "", comment, idx, err
		}
		return values, "csv", comment, idx, nil
	}

	if len(valueTokens) == 1 {
		tok := valueTokens[0]
		if tok.tokVal != nil {
			return tok.tokVal, "", comment, idx, nil
		}
		if tok.tokType == StringLiteral {
			return tok.tokText, "", comment, idx, nil
		}
		return tok.tokText, "", comment, idx, nil
	}

	result := ""
	for _, vt := range valueTokens {
		if vt.tokType == StringLiteral {
			result += vt.tokText
		}
	}

	if result == "" {
		return nil, "", comment, idx, fmt.Errorf("complex expressions not supported in INI values")
	}

	return result, "", comment, idx, nil
}

func parseINITokens(tokens []*lcstruct, originalContent string) (map[string][]any, error) {
	result := make(map[string][]any)
	currentSection := ""
	sectionOrder := 0
	i := 0
	iterations := 0
	const maxIterations = 100000
	lastContentPos := 0

	for i < len(tokens) {
		iterations++
		if iterations > maxIterations {
			return nil, fmt.Errorf("parseINITokens: infinite loop detected at position %d", i)
		}

		if i >= len(tokens) {
			break
		}

		tok := tokens[i].carton
		currentPos := tokens[i].tokPos

		if tok.tokType != EOL && lastContentPos < currentPos && lastContentPos < len(originalContent) && currentPos <= len(originalContent) {
			whitespace := originalContent[lastContentPos:currentPos]
			for j := 0; j < len(whitespace)-1; j++ {
				if whitespace[j] == '\n' && whitespace[j+1] == '\n' {
					if _, exists := result[currentSection]; exists {
						result[currentSection] = append(result[currentSection], map[string]any{
							"type": "space",
						})
					}
				}
			}
		}

		if tok.tokType == LeftSBrace {
			sectionTokens := []Token{}
			braceIdx := i + 1

			for braceIdx < len(tokens) {
				if tokens[braceIdx].carton.tokType == RightSBrace {
					break
				}
				sectionTokens = append(sectionTokens, tokens[braceIdx].carton)
				braceIdx++
			}

			if braceIdx >= len(tokens) {
				return nil, fmt.Errorf("unclosed section header")
			}

			currentSection = reconstructSectionName(sectionTokens)
			sectionOrder++

			if len(result[currentSection]) == 0 {
				result[currentSection] = []any{
					map[string]any{
						"type":  "metadata",
						"value": map[string]any{"section_order": sectionOrder},
					},
				}
			}

			lastContentPos = currentPos
			i = braceIdx + 1
			continue
		}

		if currentSection == "" {
			if _, exists := result[currentSection]; !exists {
				result[currentSection] = []any{
					map[string]any{
						"type":  "metadata",
						"value": map[string]any{"section_order": 0},
					},
				}
			}
		}

		if tok.tokType == Identifier && i+1 < len(tokens) && tokens[i+1].carton.tokType == O_Assign {
			key := tok.tokText
			value, format, comment, endIdx, err := parseValueAndComment(tokens, i+2)
			if err != nil {
				return nil, err
			}

			entry := map[string]any{
				"type":    "data",
				"key":     key,
				"value":   value,
				"comment": comment,
			}

			if format != "" {
				entry["format"] = format
			}

			result[currentSection] = append(result[currentSection], entry)
			lastContentPos = currentPos
			i = endIdx + 1
			continue
		}

		if tok.tokType == SingleComment || tok.tokType == SYM_Semicolon {
			commentText := tok.tokText
			if tok.tokType == SYM_Semicolon {
				for ci := i + 1; ci < len(tokens) && tokens[ci].carton.tokType != EOL && !tokens[ci].eol; ci++ {
					commentText += tokens[ci].carton.tokText
				}
			}

			result[currentSection] = append(result[currentSection], map[string]any{
				"type":    "comment",
				"comment": commentText,
			})
			lastContentPos = currentPos
			i++
			continue
		}

		if tok.tokType == EOL {
			i++
			continue
		}

		i++
	}

	return result, nil
}

func getOrderedSections(iniMap map[string][]any) []string {
	var sections []string
	type sectionOrder struct {
		name  string
		order int
	}

	var orders []sectionOrder
	for section, entries := range iniMap {
		if len(entries) > 0 {
			if entry, ok := entries[0].(map[string]any); ok {
				if entry["type"] == "metadata" {
					if value, ok := entry["value"].(map[string]any); ok {
						if order, ok := value["section_order"].(int); ok {
							orders = append(orders, sectionOrder{section, order})
							continue
						}
					}
				}
			}
		}
		orders = append(orders, sectionOrder{section, 999})
	}

	for i := 0; i < len(orders); i++ {
		for j := i + 1; j < len(orders); j++ {
			if orders[i].order > orders[j].order {
				orders[i], orders[j] = orders[j], orders[i]
			}
		}
	}

	for _, o := range orders {
		sections = append(sections, o.name)
	}

	return sections
}

func formatINIValue(value any, format string) string {
	switch v := value.(type) {
	case []any:
		if format == "csv" {
			var parts []string
			for _, item := range v {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
			return strings.Join(parts, ",")
		}
		var parts []string
		for _, item := range v {
			switch item.(type) {
			case string:
				parts = append(parts, fmt.Sprintf("\"%v\"", item))
			default:
				parts = append(parts, fmt.Sprintf("%v", item))
			}
		}
		return "[" + strings.Join(parts, ",") + "]"
	case string:
		return fmt.Sprintf("\"%s\"", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func validateSectionMetadata(sectionName string, entries []any) error {
	if len(entries) == 0 {
		return nil
	}

	if entry, ok := entries[0].(map[string]any); ok {
		if entry["type"] == "metadata" {
			return nil
		}
	}

	return fef("section '%s' is missing metadata", sectionName)
}

func iniMetaUpdate(iniMap map[string][]any) (map[string][]any, error) {
	var sections []struct {
		name    string
		entries []any
	}

	for sectionName, entries := range iniMap {
		sections = append(sections, struct {
			name    string
			entries []any
		}{sectionName, entries})
	}

	sort.Slice(sections, func(i, j int) bool {
		orderI := getSectionOrder(sections[i].entries)
		orderJ := getSectionOrder(sections[j].entries)
		return orderI < orderJ
	})

	for i, section := range sections {
		newOrder := i

		for _, entry := range section.entries {
			if entryMap, ok := entry.(map[string]any); ok {
				if entryMap["type"].(string) == "metadata" {
					if value, ok := entryMap["value"].(map[string]any); ok {
						value["section_order"] = newOrder
					}
					break
				}
			}
		}
	}

	return iniMap, nil
}

func getSectionOrder(entries []any) int {
	for _, entry := range entries {
		if entryMap, ok := entry.(map[string]any); ok {
			if entryMap["type"].(string) == "metadata" {
				if value, ok := entryMap["value"].(map[string]any); ok {
					if order, ok := value["section_order"].(int); ok {
						return order
					}
				}
			}
		}
	}
	return 0
}

func iniGetGlobal(iniMap map[string][]any) ([]any, error) {
	entries, exists := iniMap[""]
	if !exists {
		return nil, fef("no global section found in INI file")
	}

	if len(entries) == 0 {
		iniMap[""] = []any{
			map[string]any{
				"type":  "metadata",
				"value": map[string]any{"section_order": 0},
			},
		}
		return iniMap[""], nil
	}

	return entries, nil
}

func iniSetGlobal(iniMap map[string][]any, entries []any) error {
	_, exists := iniMap[""]
	if !exists {
		return fef("no global section found in INI file")
	}

	err := validateSectionMetadata("", entries)
	if err != nil {
		return err
	}

	iniMap[""] = entries
	return nil
}

func iniWrite(iniMap map[string][]any, filepath string) error {
	var builder strings.Builder

	sections := getOrderedSections(iniMap)

	for _, section := range sections {
		entries := iniMap[section]

		if section != "" {
            if builder.Len()>1 {
                lastChar1:=builder.String()[builder.Len()-2]
                lastChar2:=builder.String()[builder.Len()-1]
                if !(lastChar1=='\n' && lastChar2=='\n') {
                    builder.WriteString("\n")
                }
            }
			builder.WriteString("[")
			builder.WriteString(section)
			builder.WriteString("]\n")
		}

		for _, entry := range entries {
			e, ok := entry.(map[string]any)
			if !ok {
				continue
			}

			if e["type"] == "comment" {
				comment, _ := e["comment"].(string)
				builder.WriteString(comment)
				builder.WriteString("\n")
			} else if e["type"] == "space" {
				builder.WriteString("\n")
			} else if e["type"] == "data" {
				key, _ := e["key"].(string)
				value := e["value"]
				format := ""
				if f, ok := e["format"].(string); ok {
					format = f
				}
				comment := ""
				if c, ok := e["comment"].(string); ok {
					comment = c
				}

				builder.WriteString(key)
				builder.WriteString("=")
				builder.WriteString(formatINIValue(value, format))

				if comment != "" {
					builder.WriteString(" ")
					builder.WriteString(comment)
				}

				builder.WriteString("\n")
			}
		}
	}

	return os.WriteFile(filepath, []byte(builder.String()), 0644)
}

func findMaxSectionOrder(iniMap map[string][]any) int {
	max := 0
	for _, entries := range iniMap {
		for _, entry := range entries {
			if entryMap, ok := entry.(map[string]any); ok {
				if entryMap["type"].(string) == "metadata" {
					if value, ok := entryMap["value"].(map[string]any); ok {
						if order, ok := value["section_order"].(int); ok {
							if order > max {
								max = order
							}
						}
					}
					break
				}
			}
		}
	}
	return max
}

func iniNewSection(iniMap map[string][]any, sectionName string) (map[string][]any, error) {
	maxOrder := findMaxSectionOrder(iniMap)

	iniMap[sectionName] = []any{
		map[string]any{
			"type":  "metadata",
			"value": map[string]any{"section_order": maxOrder + 1},
		},
	}

	_, err := iniMetaUpdate(iniMap)
	if err != nil {
		return nil, err
	}

	return iniMap, nil
}

func iniInsertSection(iniMap map[string][]any, sectionName string, position int) (map[string][]any, error) {
	if position < 0 {
		return nil, fef("ini_insert_section: position must be >= 0")
	}

	iniMap[sectionName] = []any{
		map[string]any{
			"type":  "metadata",
			"value": map[string]any{"section_order": position},
		},
	}

	for _, entries := range iniMap {
		for _, entry := range entries {
			if entryMap, ok := entry.(map[string]any); ok {
				if entryMap["type"].(string) == "metadata" {
					if value, ok := entryMap["value"].(map[string]any); ok {
						if existingOrder, ok := value["section_order"].(int); ok && existingOrder >= position {
							value["section_order"] = existingOrder + 1
						}
					}
					break
				}
			}
		}
	}

	_, err := iniMetaUpdate(iniMap)
	if err != nil {
		return nil, err
	}

	return iniMap, nil
}

func iniDeleteSection(iniMap map[string][]any, sectionName string) (map[string][]any, error) {
	deletedOrder := -1
	if entries, exists := iniMap[sectionName]; exists {
		for _, entry := range entries {
			if entryMap, ok := entry.(map[string]any); ok {
				if entryMap["type"].(string) == "metadata" {
					if value, ok := entryMap["value"].(map[string]any); ok {
						deletedOrder = value["section_order"].(int)
					}
					break
				}
			}
		}
	}

	delete(iniMap, sectionName)

	if deletedOrder > 0 {
		for _, entries := range iniMap {
			for _, entry := range entries {
				if entryMap, ok := entry.(map[string]any); ok {
					if entryMap["type"].(string) == "metadata" {
						if value, ok := entryMap["value"].(map[string]any); ok {
							if existingOrder, ok := value["section_order"].(int); ok && existingOrder > deletedOrder {
								value["section_order"] = existingOrder - 1
							}
						}
					}
					break
				}
			}
		}
	}

	_, err := iniMetaUpdate(iniMap)
	if err != nil {
		return nil, err
	}

	return iniMap, nil
}
