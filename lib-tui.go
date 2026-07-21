//go:build !test

package main

import (
    "fmt"
    "reflect"
    "regexp"
    str "strings"
    "unicode/utf8"
)

type tui struct {
    Row       int
    Col       int
    Height    int
    Width     int
    Action    string
    Options   []string
    Selected  []bool
    Sizes     []int
    Display   []int
    Started   bool
    Vertical  bool
    Cursor    string
    Title     string
    Prompt    string
    Content   string
    Value     float64
    Border    bool
    Data      any
    Format    string
    Sep       string
    Result    any
    Cancel    bool
    Back      bool
    Multi     bool
    Reset     bool
    Headers   bool
    TableSend string
    Index     int // menu cursor index
}

type tui_style struct {
	bg        string
	fg        string
	border    map[string]string
	fill      bool
	wrap      bool
	hi_bg     string
	hi_fg     string
	list      []string
	select_bg string
	select_fg string
}

type tableData struct {
	aaos       [][]string
	fieldNames []string
	cw         []int
	selected   []bool
	colMax     int
	hasHeader  bool
}

func delimitedSplit(s string, sep string) (os []string) {
    inQuote := false
    wordStart := 0
    for e := 0; e < len(s)-1; e += 1 {
        if !inQuote && s[e] == ' ' {
            continue
        }
        if inQuote && s[e] == '\\' {
            e++
            continue
        }
        if s[e] == '"' {
            inQuote = !inQuote
        }
        if s[e] == sep[0] {
            if !inQuote {
                os = append(os, str.TrimSpace(s[wordStart:e]))
                wordStart = e + 1
            }
        }
    }
    return os
}

func buildTableData(t tui) (td tableData, err error) {
	// read data
	sep := ","
	lineMethod := ""
	switch str.ToLower(t.Format) {
	case "csv":
		sep = ","
		lineMethod = "regex"
	case "tsv":
		sep = "\t"
		lineMethod = "regex"
	case "ssv":
		sep = " "
		lineMethod = "regex"
	case "psv":
		sep = "|"
		lineMethod = "regex"
	case "custom":
		sep = t.Sep
		lineMethod = "regex"
	case "aos":
		lineMethod = "struct"
	default:
		return td, fmt.Errorf("Unknown separator type in tui_table() [%s]", t.Format)
	}

	td.hasHeader = t.Headers
	var aaos [][]string
	var aos []string
	var maxSize int
	var colMax int
	var fieldNames []string

	switch lineMethod {
	case "regex":
		aos = str.Split(t.Data.(string), "\n")
		maxSize = len(aos)
	case "struct":
		maxSize = len(t.Data.([]any)) + 1 // 0 element is headers
	}
	aaos = make([][]string, maxSize)

	switch lineMethod {
	case "regex":
		var first bool = true
		for i, v := range aos {
			cols := delimitedSplit(v, sep)
			if first {
				colMax = len(cols)
				aaos[0] = make([]string, colMax)
			}
			aaos[i+1] = make([]string, colMax)
			if len(cols) != colMax {
				return td, fmt.Errorf("Column count mismatch (%d,%d) in tui_table() at .Data line %d", len(cols), colMax, i)
			}
			for j, c := range cols {
				l := len(c)
				c = stripDoubleQuotes(c)
				if l != len(c) {
					c = stripSingleQuotes(c)
				}
				if first && td.hasHeader {
					aaos[0][j] = c
				}
				cols[j] = c
			}
			if !(first && td.hasHeader) {
				aaos[i] = cols
			}
			first = false
		}

	case "struct":
		isArray := (reflect.TypeOf(t.Data).Kind() == reflect.Array || reflect.TypeOf(t.Data).Kind() == reflect.Slice)
		if !isArray {
			return td, fmt.Errorf(".Data not an array (%#v)", reflect.TypeOf(t.Data).Kind().String())
		}

		var first bool = true
		var fieldOrder []string
		for i := 0; i < len(t.Data.([]any)); i += 1 {
			item := t.Data.([]any)[i]
			rv := reflect.ValueOf(item)
			switch rv.Kind() {
			case reflect.Struct, reflect.Map:
			default:
				return td, fmt.Errorf(".Data element %d not a struct or map", i)
			}

			if first {
				switch rv.Kind() {
				case reflect.Struct:
					colMax = rv.NumField()
					fieldOrder = make([]string, colMax)
					fieldNames = make([]string, colMax)
					aaos[0] = make([]string, colMax)
					for j := 0; j < colMax; j += 1 {
						rname := rv.Type().Field(j).Name
						fieldOrder[j] = rname
						fieldNames[j] = rname
						aaos[0][j] = rname
					}
				case reflect.Map:
					keys := rv.MapKeys()
					colMax = len(keys)
					fieldOrder = make([]string, colMax)
					fieldNames = make([]string, colMax)
					aaos[0] = make([]string, colMax)
					for j, k := range keys {
						sk := sf("%v", k.Interface())
						fieldOrder[j] = sk
						fieldNames[j] = sk
						aaos[0][j] = sk
					}
				}
				first = false
			}
			aaos[i+1] = make([]string, colMax)
			switch rv.Kind() {
			case reflect.Struct:
				if rv.NumField() != colMax {
					return td, fmt.Errorf("Column count mismatch in tui_table() at .Data line %d", i)
				}
				for j := 0; j < colMax; j += 1 {
					field_value := rv.FieldByName(fieldOrder[j])
					aaos[i+1][j] = sf("%v", field_value)
				}
			case reflect.Map:
				if rv.Len() != colMax {
					return td, fmt.Errorf("Column count mismatch in tui_table() at .Data line %d", i)
				}
				for j := 0; j < colMax; j += 1 {
					fv := rv.MapIndex(reflect.ValueOf(fieldOrder[j]))
					if fv.IsValid() {
						aaos[i+1][j] = sf("%v", fv.Interface())
					} else {
						aaos[i+1][j] = ""
					}
				}
			}
		}
		td.hasHeader = true
	}

	if lineMethod != "struct" && td.hasHeader {
		fieldNames = make([]string, colMax)
		fieldNames = aaos[0]
	}

	if len(t.Options) > 0 {
		if lineMethod == "struct" && len(t.Display) > 0 {
			if len(t.Options) != len(t.Display) {
				return td, fmt.Errorf("Options count (%d) does not match Display count (%d) in tui_table()", len(t.Options), len(t.Display))
			}
			for i, idx := range t.Display {
				if idx < colMax {
					fieldNames[idx] = t.Options[i]
				}
			}
			td.hasHeader = true
		} else {
			if len(t.Options) != colMax {
				return td, fmt.Errorf("Column count does not match provided header name count in tui_table() .Options field")
			}
			fieldNames = make([]string, len(t.Options))
			if len(t.Options) != 0 {
				copy(fieldNames, t.Options)
				td.hasHeader = true
			}
		}
	}

	var selected []bool
	selected = make([]bool, colMax)

	if len(t.Display) > 0 {
		for _, v := range t.Display {
			selected[v] = true
		}
	} else {
		for j := 0; j < colMax; j += 1 {
			selected[j] = true
		}
	}

	cw := make([]int, colMax)

	if len(t.Sizes) == colMax {
		cw = t.Sizes
	} else if lineMethod == "struct" && len(t.Display) > 0 && len(t.Sizes) == len(t.Display) {
		for i, idx := range t.Display {
			if idx < colMax {
				cw[idx] = t.Sizes[i]
			}
		}
	} else {
		for _, l := range aaos {
			for j, v := range l {
				if len(v) > cw[j] {
					cw[j] = len(v)
				}
			}
		}
	}

	td.aaos = aaos
	td.fieldNames = fieldNames
	td.cw = cw
	td.selected = selected
	td.colMax = colMax
	return td, nil
}

func tui_table(t tui, s tui_style) (os string, err error) {
	// Draw border if requested
	if t.Border {
		tui_box(tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2}, s)
	}

	td, err := buildTableData(t)
	if err != nil {
		return "", err
	}

	iv := ""
	ih := ""
	if s.border != nil {
		iv = s.border["iv"]
		ih = s.border["ih"]
	}
	hb := s.hi_bg
	hf := s.hi_fg

	table_width := 5
	dispColCount := 0
	for j := range td.cw {
		if td.selected[j] {
			table_width += 2 + td.cw[j]
			dispColCount += 1
		}
	}
	if iv == "" {
		table_width -= dispColCount
	}

	cllen := len(s.list)
	if cllen > 0 && cllen != td.colMax {
		return "", fmt.Errorf("Column count does not match provided colour list length in tui_table() style .list field")
	}

	// header display
	if td.hasHeader {
		if ih != "" {
			os += rep(ih, table_width) + "\n"
		}
		os += iv
		for e := 0; e < len(td.fieldNames); e += 1 {
			if td.selected[e] {
				os += sf("%s %-*s [##][#-]%s", hb+hf, td.cw[e], td.fieldNames[e], iv)
			}
		}
		if ih != "" {
			os += "\n" + rep(ih, table_width) + "\n"
		} else {
			os += "\n"
		}
	}

	// data display
	for trow := 1; trow < len(td.aaos); trow += 1 {
		line := td.aaos[trow]
		if s.bg != "" {
			os += "[#b" + s.bg + "]"
		}
		if s.fg != "" {
			os += "[#" + s.fg + "]"
		}
		os += iv
		for j, v := range line {
			field_colour := ""
			if cllen > 0 {
				field_colour = s.list[j]
			}
			if td.selected[j] {
				os += sf("%s %-*s [##][#-]%s", field_colour, td.cw[j], v, iv)
			}
		}
		if ih != "" {
			os += "\n" + rep(ih, table_width) + "\n"
		} else {
			os += "\n"
		}
	}

	if t.TableSend != "" {
		t.Content = os
		switch str.ToLower(t.TableSend) {
		case "pager":
			t.Width = table_width + 2
			tui_pager(t, s)
		}
	}

	return os, nil
}

func tui_table_select(t tui, s tui_style) tui {
	td, err := buildTableData(t)
	if err != nil {
		t.Cancel = true
		return t
	}

	iv := ""
	ih := ""
	if s.border != nil {
		iv = s.border["iv"]
		ih = s.border["ih"]
	}
	hb := s.hi_bg
	hf := s.hi_fg

	table_width := 5
	dispColCount := 0
	for j := range td.cw {
		if td.selected[j] {
			table_width += 2 + td.cw[j]
			dispColCount += 1
		}
	}
	if iv == "" {
		table_width -= dispColCount
	}

	innerWidth := t.Width - 2
	if innerWidth < 1 {
		innerWidth = 1
	}

	ivWidth := 0
	if iv != "" {
		ivWidth = utf8.RuneCountInString(iv)
	}

	cllen := len(s.list)
	if cllen > 0 && cllen != td.colMax {
		t.Cancel = true
		return t
	}

	selBg := s.select_bg
	selFg := s.select_fg
	if selBg == "" && selFg == "" {
		selBg = "[#invert]"
		selFg = ""
	} else {
		if selBg != "" {
			selBg = "[#b" + selBg + "]"
		}
		if selFg != "" {
			selFg = "[#" + selFg + "]"
		}
	}

	addbg := ""
	addfg := ""
	if s.bg != "" {
		addbg = "[#b" + s.bg + "]"
	}
	if s.fg != "" {
		addfg = "[#" + s.fg + "]"
	}

	// Draw border if requested
	if t.Border {
		tui_box(tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2}, s)
	}

	hideCursor()

	dataRows := len(td.aaos) - 1
	if dataRows < 0 {
		dataRows = 0
	}

	dataStart := 1
	if td.hasHeader {
		dataStart = 2
		if ih != "" {
			dataStart = 3
		}
	}

	scrollOffset := 0
	selectedRow := 0
	visibleHeight := t.Height - dataStart
	if visibleHeight < 1 {
		visibleHeight = dataRows
	}
	if visibleHeight > dataRows {
		visibleHeight = dataRows
	}

	render := func() {
		for k := 0; k < visibleHeight; k++ {
			absat(t.Row+dataStart+k, t.Col+1)
			pf("[#-]")
			absClearChars(t.Row+dataStart+k, t.Col+1, innerWidth)
			absat(t.Row+dataStart+k, t.Col+1)
			dataIdx := scrollOffset + k
			if dataIdx >= dataRows {
				continue
			}
			line := td.aaos[dataIdx+1]
			isSelected := dataIdx == selectedRow
			rowPrefix := ""
			rowSuffix := "[##][#-]"
			if isSelected {
				rowPrefix = selBg + selFg
			} else {
				if addbg != "" || addfg != "" {
					rowPrefix = addbg + addfg
				}
			}
			pf(rowPrefix)
			pf(iv)
			rowContentWidth := ivWidth
			for j, v := range line {
				field_colour := ""
				if cllen > 0 {
					field_colour = s.list[j]
				}
				if td.selected[j] {
					cellWidth := 2 + td.cw[j] + ivWidth
					if iv == "" {
						cellWidth = 2 + td.cw[j]
					}
					rowContentWidth += cellWidth
					if isSelected {
						pf(sf("%s %-*s %s", field_colour, td.cw[j], v, iv))
					} else {
						pf(sf("%s %-*s [##][#-]%s", field_colour, td.cw[j], v, iv))
					}
				}
			}
			// Reset colors and pad to end of row to prevent highlight bleed
			pf(rowSuffix)
			if rowContentWidth < innerWidth {
				pf(str.Repeat(" ", innerWidth-rowContentWidth))
			}
		}
		for k := visibleHeight; k < t.Height-dataStart; k++ {
			absat(t.Row+dataStart+k, t.Col+1)
			pf("[#-]")
			absClearChars(t.Row+dataStart+k, t.Col+1, innerWidth)
		}
	}

	// render header
	if td.hasHeader {
		if ih != "" {
			absat(t.Row, t.Col+1)
			pf(rep(ih, innerWidth))
		}
		absat(t.Row+1, t.Col+1)
		pf("[#-]")
		absClearChars(t.Row+1, t.Col+1, innerWidth)
		absat(t.Row+1, t.Col+1)
		pf(hb + hf)
		pf(iv)
		headerWidth := ivWidth
		for e := 0; e < len(td.fieldNames); e += 1 {
			if td.selected[e] {
				cellWidth := 2 + td.cw[e] + ivWidth
				if iv == "" {
					cellWidth = 2 + td.cw[e]
				}
				headerWidth += cellWidth
				pf(sf(" %-*s [##][#-]%s", td.cw[e], td.fieldNames[e], iv))
			}
		}
		pf("[##][#-]")
		if headerWidth < innerWidth {
			pf(str.Repeat(" ", innerWidth-headerWidth))
		}
		if ih != "" {
			absat(t.Row+2, t.Col+1)
			pf(rep(ih, innerWidth))
		}
	}

	render()

	t.Cancel = false
	finished := false
	prevMW, prevMH := MW, MH
	for !finished {
		k := wrappedGetCh(100, false)
		if k == 0 {
			if MW != prevMW || MH != prevMH {
				prevMW, prevMH = MW, MH
				visibleHeight = t.Height - dataStart
				if visibleHeight < 1 {
					visibleHeight = dataRows
				}
				if visibleHeight > dataRows {
					visibleHeight = dataRows
				}
				if selectedRow >= dataRows {
					selectedRow = dataRows - 1
				}
				if scrollOffset+visibleHeight > dataRows {
					scrollOffset = dataRows - visibleHeight
					if scrollOffset < 0 {
						scrollOffset = 0
					}
				}
				render()
			}
			continue
		}
		switch k {
		case 10: // down
			if selectedRow < dataRows-1 {
				selectedRow++
				if selectedRow >= scrollOffset+visibleHeight {
					scrollOffset++
				}
			}
		case 11: // up
			if selectedRow > 0 {
				selectedRow--
				if selectedRow < scrollOffset {
					scrollOffset--
				}
			}
		case 14, ' ': // pgdown
			selectedRow += visibleHeight
			if selectedRow >= dataRows {
				selectedRow = dataRows - 1
			}
			scrollOffset = selectedRow - visibleHeight + 1
			if scrollOffset < 0 {
				scrollOffset = 0
			}
		case 15: // pgup
			selectedRow -= visibleHeight
			if selectedRow < 0 {
				selectedRow = 0
			}
			scrollOffset = selectedRow
		case 16, 'g': // home
			selectedRow = 0
			scrollOffset = 0
		case 17, 'G': // end
			selectedRow = dataRows - 1
			scrollOffset = dataRows - visibleHeight
			if scrollOffset < 0 {
				scrollOffset = 0
			}
		case 13: // enter
			t.Result = selectedRow
			finished = true
		case 'q', 'Q', 27: // q, escape
			t.Cancel = true
			finished = true
		case 'b', 'B': // back
			t.Back = true
			t.Cancel = true
			finished = true
		case 3: // ctrl-c
			t.Cancel = true
			finished = true
			lastlock.Lock()
			sig_int = true
			lastlock.Unlock()
		default:
			if (k >= 'a' && k <= 'z') || (k >= 'A' && k <= 'Z') || (k >= '0' && k <= '9') {
				t.Action = string(rune(k))
				finished = true
			}
		}
		render()
	}

	// remove border box
	if t.Border {
		border := empty_border_map
		tui_box(
			tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2},
			tui_style{border: border},
		)
		tui_clear(t, s)
	}

	t.Index = selectedRow
	return t
}

// switch to secondary buffer
func secScreen() {
    pf("\033[?1049h\033[H")
    altScreen = true
}

// switch to primary buffer
func priScreen() {
    pf("\033[?1049l")
    altScreen = false
}

func absClearChars(row int, col int, l int) {
    if l < 1 {
        return
    }
    absat(row, col)
    fmt.Print(str.Repeat(" ", l))
}

const nbsp = 26 // ascii substitute char

func wrapString(s string, lim uint) string {
    if len(s) == 0 || lim == 0 {
        return s
    }

    lines := str.Split(s, "\n")
    var wrappedLines []string

    for _, line := range lines {
        wrappedLines = append(wrappedLines, wrapLine(line, lim))
    }

    return str.Join(wrappedLines, "\n")
}

func wrapLine(line string, lim uint) string {
    var result str.Builder
    words := str.FieldsFunc(line, func(r rune) bool {
        return r == ' ' // || r == '\t'
    })

    currentLine := ""
    currentVisibleLen := 0

    for _, word := range words {
        wordVisibleLen := displayedLen(word)

        // Check if adding word (with space) exceeds limit
        spaceNeeded := 0
        if len(currentLine) > 0 {
            spaceNeeded = 1 // Visible length of space
        }

        if currentVisibleLen+spaceNeeded+wordVisibleLen > int(lim) {
            // Wrap: output current line
            result.WriteString(currentLine)
            result.WriteRune('\n')
            currentLine = word
            currentVisibleLen = wordVisibleLen
        } else {
            // Add to current line
            if len(currentLine) > 0 {
                currentLine += " "
                currentVisibleLen += 1
            }
            currentLine += word
            currentVisibleLen += wordVisibleLen
        }
    }

    // Append remaining line
    if len(currentLine) > 0 {
        result.WriteString(currentLine)
    }

    return result.String()
}

func str_inset(n int) string {
    return sf("\033[%dG", n)
}

func tui_template(t tui, s tui_style) (string, error) {

    // t.Content : inbound template
    // t.Data    : inbound struct

    var refstruct reflect.Value
    switch refstruct = reflect.ValueOf(t.Data); refstruct.Kind() {
    case reflect.Struct:
    default:
        return "", fmt.Errorf(".Data not a struct")
    }

    // find all {.field} (non-greedy shortest matches)
    r := regexp.MustCompile(`{\.([^{}]*)}`)

    // loop through each match
    matches := r.FindAllStringSubmatch(t.Content, -1)
    for _, v := range matches {
        // get name from match
        field_name := v[1]
        // get t.Data.<field> with reflection
        field_value := refstruct.FieldByName(renameSF(field_name))
        // search/replace all {.<field>} with value from above
        t.Content = str.Replace(t.Content, "{."+field_name+"}", sf("%v", field_value), -1)
    }

    // pass result through to tui_text
    tui_text(t, s)

    return t.Content, nil
}

// horizontal / vertical radio button (single/multi-selector)
func tui_radio(t tui, s tui_style) any {

    hi_bg := s.hi_bg
    hi_fg := s.hi_fg
    fg := s.fg
    bg := s.bg
    addbg := ""
    addfg := ""
    addhibg := ""
    addhifg := ""
    selectedText := "✓"

    if bg != "" {
        addbg = "[#b" + bg + "]"
    }
    if fg != "" {
        addfg = "[#" + fg + "]"
    }
    if hi_bg != "" {
        addhibg = "[#b" + hi_bg + "]"
    }
    if hi_fg != "" {
        addhifg = "[#" + hi_fg + "]"
    }

    // Draw border if requested - border should be drawn around the content area
    if t.Border {
        tui_box(tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2}, s)
    }

    // Safety check: ensure Options and Selected are initialized
    if t.Options == nil {
        t.Options = []string{}
    }
    if t.Selected == nil {
        t.Selected = make([]bool, len(t.Options))
    }

    options := []string{}
    for _, v := range t.Options {
        o := GetAsString(v)
        options = append(options, o)
    }

    cursor := "x"
    if t.Cursor != "" {
        cursor = t.Cursor
    }

    orig := make([]bool, len(t.Selected))
    copy(orig, t.Selected)

    // key loop
    var key int
    cpos := 0
    t.Cancel = false

    selCount := 0
    for k := range t.Selected {
        if t.Selected[k] {
            selCount++
        }
    }

    for !t.Cancel {

        // build output string
        op := "[##][#-]" + t.Prompt
        sep := " "
        if t.Sep != "" {
            sep = t.Sep
        }

        for k := 0; k < len(options); k += 1 {
            op += "["
            if cpos == k {
                op += addhibg + addhifg
            }

            if t.Selected[k] {
                op += selectedText
            } else {
                if cpos == k {
                    op += cursor
                } else {
                    op += " "
                }
            }

            if cpos == k {
                op += "[##][#-]"
            }

            op += "] " + addbg + addfg + options[k] + "[##][#-]"
            if t.Vertical {
                op += "\n" + str_inset(t.Col+len(t.Prompt))
            } else {
                if k != len(options)-1 {
                    op += sep
                }
            }
        }

        // display
        absat(t.Row, t.Col)
        pf(op)

        key = wrappedGetCh(0, false)
        switch key {
        case 9, 10: // right or down
            if cpos < len(options)-1 {
                cpos += 1
            }
        case 8, 11: //left or up
            if cpos > 0 {
                cpos -= 1
            }
        case 13, 'q', 'Q', 'b', 'B', 27: // enter, q, b or escape
            if key != 13 {
                // discard changes
                copy(t.Selected, orig)
            }
            t.Cancel = true
        case ' ': // toggle
            if cpos < len(t.Options) {
                t.Selected[cpos] = !t.Selected[cpos]
                if t.Selected[cpos] {
                    selCount++
                } else {
                    selCount--
                }
                if selCount > 1 && !t.Multi {
                    // exclude all the others
                    for k := range t.Selected {
                        t.Selected[k] = false
                    }
                    t.Selected[cpos] = true
                    selCount = 1
                }
            }
        case 3: // ctrl-c
            copy(t.Selected, orig)
            t.Cancel = true
            lastlock.Lock()
            sig_int = true
            lastlock.Unlock()
        }
    }

    // Clear the border when exiting
    if t.Border {
        border := empty_border_map
        tui_box(
            tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2},
            tui_style{border: border},
        )
        tui_clear(t, s)
    }

    if t.Multi {
        return t.Selected
    }

    for i := 0; i < len(t.Selected); i++ {
        if t.Selected[i] == true {
            return i
        }
    }
    return -1

}

func tui_progress_reset(t tui) tui {
    t.Reset = true
    return tui_progress(t, default_tui_style)
}

func tui_progress(t tui, s tui_style) tui {
    hsize := t.Width
    row := t.Row
    col := t.Col
    pc := t.Value
    c := "█"
    d := pc * float64(hsize) // width of input percent

    if t.Cursor != "" {
        c = t.Cursor
    }

    hideCursor()
    bgcolour := "[#b" + s.bg + "]"
    fgcolour := "[#" + s.fg + "]"

    absat(row, col)
    if t.Reset && t.Border {
        // reset
        fmt.Print(rep(" ", hsize))
        border := empty_border_map
        tui_box(
            tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2},
            tui_style{border: border},
        )
        t.Value = 0
        t.Started = false
        t.Reset = false
        return t
    }

    if !t.Started && t.Border {
        // initial box
        tui_box(tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2}, s)
    }

    if !t.Started {
        t.Started = true
    }

    absat(row, col)
    pf(bgcolour + fgcolour)
    for e := 0; e < hsize; e += 1 {
        if e > int(d) {
            break
        }
        fmt.Print(c)
    }
    pf("[#-]")
    fmt.Print(rep(" ", hsize-int(d)-1))
    return t
}

func tui_pager(t tui, s tui_style) {
    addbg := ""
    addfg := ""
    if s.bg != "" {
        addbg = "[#b" + s.bg + "]"
    }
    if s.fg != "" {
        addfg = "[#" + s.fg + "]"
    }

    // Draw border if requested
    if t.Border {
        tui_box(tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2}, s)
    }

    pf(addbg)
    pf(addfg)

    cpos := 0
    hideCursor()

    var w uint
    var rs string
    var ra []string
    if len(t.Content) > 0 {
        w = uint(t.Width - 3)
        if s.wrap {
            rs = wrapString(t.Content, w)
        } else {
            rs = t.Content
        }
        if len(rs) > 0 {
            rs = str.Replace(rs, "%", "%%", -1)
            ra = str.Split(rs, "\n")
        }
    }
    if !s.wrap {
        // do something much cleverer than this to clip long lines here:
        // (it doesn't check for ansi code breakage.)
        /*
           for k,v:=range ra {
               if displayedLen(v) > t.Width-2 {
                   ra[k]=ra[k][:t.Width-2]+"[##][#-]"
               }
           }
        */
    }
    if len(ra) > 0 {
        t.Cancel = false
        omax := t.Height - 1
        searchQuery := ""
        searchMode := false
        lastMatch := -1
        prevMW, prevMH := MW, MH
        for !t.Cancel {
            max := omax
            if cpos+t.Height-1 > len(ra) {
                max = len(ra) - cpos
            }
            for k, v := range ra[cpos : cpos+max] {
                absClearChars(t.Row+k+1, t.Col+1, t.Width-2)
                absat(t.Row+k+1, t.Col+1)
                pf(addbg + addfg + v)
                absClearChars(t.Row+k+1, t.Col+1+len(v), t.Width-2-len(v))
            }
            for k := max - 1; k < omax; k++ {
                absClearChars(t.Row+k+1, t.Col+1, t.Width-1)
            }
            pf("[##][#-]")
            // scroll position
            if len(ra) > 0 {
                scroll_pos := int(float64(cpos) * float64(t.Height-1) / float64(len(ra)))
                absat(t.Row+1+scroll_pos, t.Col+t.Width-2)
                pf("[#invert]*[##][#-]")
            }
            // search status line
            if searchMode {
                absat(t.Row+t.Height, t.Col+1)
                pf("[#b1][#6]/" + searchQuery + str.Repeat(" ", t.Width-3-len(searchQuery)-1) + "[##][#-]")
            }
            // process keypresses
            k := wrappedGetCh(100, false)
            if k == 0 {
                if MW != prevMW || MH != prevMH {
                    prevMW, prevMH = MW, MH
                    omax = t.Height - 1
                    if cpos+omax > len(ra) {
                        cpos = len(ra) - omax
                        if cpos < 0 {
                            cpos = 0
                        }
                    }
                    // redraw
                    for k := max - 1; k < omax; k++ {
                        absClearChars(t.Row+k+1, t.Col+1, t.Width-1)
                    }
                    pf("[##][#-]")
                    if len(ra) > 0 {
                        scroll_pos := int(float64(cpos) * float64(t.Height-1) / float64(len(ra)))
                        absat(t.Row+1+scroll_pos, t.Col+t.Width-2)
                        pf("[#invert]*[##][#-]")
                    }
                    if searchMode {
                        absat(t.Row+t.Height, t.Col+1)
                        pf("[#b1][#6]/" + searchQuery + str.Repeat(" ", t.Width-3-len(searchQuery)-1) + "[##][#-]")
                    }
                }
                continue
            }
            if searchMode {
                switch k {
                case 13: // enter - execute search
                    searchMode = false
                    lastMatch = -1
                    for i := cpos; i < len(ra); i++ {
                        if str.Contains(str.ToLower(ra[i]), str.ToLower(searchQuery)) {
                            cpos = i
                            lastMatch = i
                            break
                        }
                    }
                    absat(t.Row+t.Height, t.Col+1)
                    absClearChars(t.Row+t.Height, t.Col+1, t.Width-2)
                case 127: // backspace
                    if len(searchQuery) > 0 {
                        searchQuery = searchQuery[:len(searchQuery)-1]
                    }
                case 27: // escape
                    searchMode = false
                    absat(t.Row+t.Height, t.Col+1)
                    absClearChars(t.Row+t.Height, t.Col+1, t.Width-2)
                default:
                    if k > 31 && k < 127 {
                        searchQuery += string(rune(k))
                    }
                }
                continue
            }
            switch k {
            case 10: //down
                if cpos < len(ra)-t.Height {
                    cpos++
                }
            case 11: //up
                if cpos > 0 {
                    cpos--
                }
            case 'q', 'Q', 27:
                t.Cancel = true
            case 'b', 'B':
                t.Back = true
                t.Cancel = true
            case 3: // ctrl-c
                t.Cancel = true
                lastlock.Lock()
                sig_int = true
                lastlock.Unlock()
            case 15:
                cpos -= t.Height - 1
                if cpos < 0 {
                    cpos = 0
                }
            case ' ', 14:
                if cpos+max < len(ra)-1 {
                    cpos += t.Height - 1
                }
            case 16, 'g': // home
                cpos = 0
            case 17, 'G': // end
                cpos = len(ra) - t.Height
                if cpos < 0 {
                    cpos = 0
                }
            case '/':
                searchMode = true
                searchQuery = ""
                lastMatch = -1
            case 'n':
                if lastMatch >= 0 && searchQuery != "" {
                    for i := lastMatch + 1; i < len(ra); i++ {
                        if str.Contains(str.ToLower(ra[i]), str.ToLower(searchQuery)) {
                            cpos = i
                            lastMatch = i
                            break
                        }
                    }
                }
            case 'N':
                if lastMatch >= 0 && searchQuery != "" {
                    for i := lastMatch - 1; i >= 0; i-- {
                        if str.Contains(str.ToLower(ra[i]), str.ToLower(searchQuery)) {
                            cpos = i
                            lastMatch = i
                            break
                        }
                    }
                }
            }
        }
    }
    // remove border box
    if t.Border {
        border := empty_border_map
        tui_box(
            tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2},
            tui_style{border: border},
        )
        tui_clear(t, s)
    }

}

// at row,col, width of t.Width-2, print wordWrap'd t.Content
func tui_text(t tui, s tui_style) {
    addbg := ""
    addfg := ""
    if s.bg != "" {
        addbg = "[#b" + s.bg + "]"
    }
    if s.fg != "" {
        addfg = "[#" + s.fg + "]"
    }

    // Draw border if requested
    if t.Border {
        tui_box(tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2}, s)
    }

    pf(addbg)
    pf(addfg)
    if s.fill {
        tui_inner_fill(t, s)
    }

    var w uint
    w = uint(t.Width - 2)
    rs := t.Content
    if s.wrap {
        rs = wrapString(rs, w)
    }
    rs = str.Replace(rs, "%", "%%", -1)
    if rs[len(rs)-1] == '\n' {
        rs = rs[:len(rs)-1]
    }
    ra := str.Split(rs, "\n")

    // if len(ra)>t.Height-2 {
    if len(ra) > t.Height-1 {
        ra = ra[len(ra)-t.Height:]
    }
    for k, v := range ra {
        absat(t.Row+k+1, t.Col+1)
        pf(addbg + addfg + v)
    }
    pf("[##][#-]")
}

func tui_clear(t tui, s tui_style) {
    pf("[##][#-]")
    borderedCount := 0
    if t.Border {
        borderedCount = 1
    }
    for e := -borderedCount; e <= t.Height+borderedCount; e += 1 {
        absat(t.Row+e, t.Col-1)
        fmt.Print(rep(" ", t.Width+2))
    }
}

func tui_inner_fill(t tui, s tui_style) {
    for e := 1; e < t.Height; e += 1 {
        absat(t.Row+e, t.Col+1)
        fmt.Print(rep(" ", t.Width-2))
    }
}

// problems:  getInput uses either clearToEOL / clearToEOP
//              this is breaking through the right border
//              and smearing the colours to EOL also.

func tui_input(t tui, s tui_style) tui {

    // t.Result  : final result in here
    // t.Content : default value
    // t.Prompt  : prompt string
    // t.Cursor  : echo mask
    // s.bg,s.fg : input colours
    // t.Row,t.Col: position
    // t.Title   : border title
    // t.Border  : border toggle
    // t.Height,t.Width : border size
    // t.Options : drop down options, if present

    addbg := ""
    addfg := ""
    if s.bg != "" {
        addbg = "[#b" + s.bg + "]"
    }
    if s.fg != "" {
        addfg = "[#" + s.fg + "]"
    }

    // draw border box
    if t.Border {
        tui_box(tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 1}, s)
    }

    // get input
    mask := "*"
    oldmask := ""
    if t.Cursor != "" {
        emask, _ := gvget("@echomask")
        oldmask = emask.(string)
        gvset("@echomask", t.Cursor)
        mask = t.Cursor
    }
    promptColour := addbg + addfg
    input, _, _ := getInput(t.Prompt, t.Content, "global", t.Row, t.Col, t.Width, t.Options, promptColour, false, false, mask)
    input = sanitise(input)

    // remove border box
    if t.Border {
        border := empty_border_map
        tui_box(
            tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2},
            tui_style{border: border},
        )
        tui_clear(t, s)
    }

    if t.Cursor != "" {
        gvset("@echomask", oldmask)
    }

    t.Result = input
    return t
}

func tui_box(t tui, s tui_style) {

    row := t.Row
    col := t.Col
    height := t.Height
    width := t.Width
    title := t.Title

    tl := s.border["tl"]
    tr := s.border["tr"]
    bl := s.border["bl"]
    br := s.border["br"]
    tm := s.border["tm"]
    bm := s.border["bm"]
    lm := s.border["lm"]
    rm := s.border["rm"]
    bg := s.border["bg"]
    fg := s.border["fg"]

    if tl == " " && tr == " " && bl == " " && br == " " { // should probably deep compare to empty_border_map instead
        title = ""
    }

    addbg := ""
    addfg := ""
    if bg != "" {
        addbg = "[#b" + bg + "]"
    }
    if fg != "" {
        addfg = "[#" + fg + "]"
    }
    pf(addbg)
    pf(addfg)

    // top
    absat(row, col)
    fmt.Print(tl)
    fmt.Print(rep(tm, width-2))
    fmt.Print(tr)

    // sides
    for r := row + 1; r < row+height; r += 1 {
        absat(r, col)
        fmt.Print(lm)
        if s.fill {
            fmt.Print(rep(" ", width-2))
        } else {
            absat(r, col+width-1)
        }
        fmt.Print(rm)
    }

    // bottom
    absat(row+height, col)
    fmt.Print(bl)
    fmt.Print(rep(bm, width-2))
    fmt.Print(br)

    // title
    if title != "" {
        absat(row, col+4)
        pf(" " + title + " ")
    }

    if bg != "" {
        pf("[##]")
    }
    if fg != "" {
        pf("[#-]")
    }

}

/////////////////////////////////////////////////////////////////

func tui_menu(t tui, s tui_style) tui {
    row := t.Row
    col := t.Col
    cursor := t.Cursor
    prompt := t.Prompt
    bg := s.bg
    fg := s.fg
    hi_bg := s.hi_bg
    hi_fg := s.hi_fg

    addbg := ""
    addfg := ""
    addhibg := ""
    addhifg := ""

    if bg != "" {
        addbg = "[#b" + bg + "]"
    }
    if fg != "" {
        addfg = "[#" + fg + "]"
    }
    if hi_bg != "" {
        addhibg = "[#b" + hi_bg + "]"
    }
    if hi_fg != "" {
        addhifg = "[#" + hi_fg + "]"
    }

    // Calculate prompt offset early for auto-height calculation
    offset := 0
    if prompt != "" {
        offset = 3
    }

    // Auto-calculate height if not specified or invalid
    if t.Height <= 0 {
        autoHeight := len(t.Options) + offset + 1
        if autoHeight > MH - 6 {
            autoHeight = MH - 6
        }
        if autoHeight < 3 {
            autoHeight = 3
        }
        t.Height = autoHeight
    }

    if t.Height < 3 {
        t.Height = 3
    }

    // scroll if necessary — inner scroll handles overflow when useScroll is true
    if !(t.Height > 0 && len(t.Options) > t.Height) && row+len(t.Options)+6>MH {
        for ssize:=row+len(t.Options)+6; ssize>MH; ssize-=1 {
            fmt.Println()
        }
        row-=len(t.Options)+6
    }

    // Draw border if requested - border should be drawn around the content area
    if t.Border {
        tui_box(tui{Title: t.Title, Row: row-1, Width: t.Width + 2, Col: col - 1, Height: t.Height + 2}, s)
    }

    pf(addbg)
    pf(addfg)

    // Draw prompt
    if prompt != "" {
        absat(row+1, col+2)
        pf(prompt)
    }

    // determine if scrolling is needed (must be after offset is known)
    visibleCount := len(t.Options)
    scrollOffset := 0
    useScroll := false
    usable := t.Height - offset
    if usable < 1 { usable = 1 }
    if t.Height > 0 && len(t.Options) > usable {
        visibleCount = usable
        useScroll = true
    }

    sel := t.Index

    // display visible menu items
    redraw := func() {
        if useScroll {
            // clear previous visible area
            for k := 0; k < visibleCount; k++ {
                absClearChars(row+offset+k, col+4, t.Width-5)
            }
            for k := 0; k < visibleCount; k++ {
                idx := scrollOffset + k
                if idx >= len(t.Options) {
                    break
                }
                absat(row+offset+k, col+6)
                pf(t.Options[idx])
            }
        }
    }

    // display menu
    for k, p := range t.Options {
        if !useScroll || k < visibleCount {
            absat(row+offset+k, col+6)
            pf(p)
        }
    }

    // input loop
    finished := false
    t.Cancel = false
    prevMW, prevMH := MW, MH

    for !finished {

        absat(row+offset+sel-scrollOffset, col+4)
        pf(cursor)
        absat(row+offset+sel-scrollOffset, col+6)
        pf(addhibg + addhifg + t.Options[sel] + "[##][#-]")
        k := wrappedGetCh(100, false)
        absat(row+offset+sel-scrollOffset, col+4)
        pf(addbg)
        pf(addfg)
        pf(" ")
        absat(row+offset+sel-scrollOffset, col+6)
        pf(t.Options[sel])

        if k == 0 {
            if MW != prevMW || MH != prevMH {
                prevMW, prevMH = MW, MH
                if useScroll {
                    visibleCount = usable
                    if visibleCount > len(t.Options) {
                        visibleCount = len(t.Options)
                    }
                    if sel >= scrollOffset+visibleCount {
                        scrollOffset = sel - visibleCount + 1
                    }
                    if scrollOffset < 0 {
                        scrollOffset = 0
                    }
                    redraw()
                }
            }
            continue
        }

        if k == 'q' || k == 'Q' {
            t.Cancel = true
            break
        }
        if k == 'b' || k == 'B' {
            t.Back = true
            t.Cancel = true
            break
        }

        switch k {
        case 11:
            if sel > 0 {
                sel--
                if useScroll && sel < scrollOffset {
                    scrollOffset = sel
                    redraw()
                }
            }
        case 10:
            if sel < len(t.Options)-1 {
                sel++
                if useScroll && sel >= scrollOffset+visibleCount {
                    scrollOffset = sel - visibleCount + 1
                    redraw()
                }
            }
        case 14, ' ':
            if useScroll {
                sel += visibleCount
                if sel >= len(t.Options) {
                    sel = len(t.Options) - 1
                }
                scrollOffset = sel - visibleCount + 1
                if scrollOffset < 0 {
                    scrollOffset = 0
                }
                redraw()
            }
        case 15:
            if useScroll {
                sel -= visibleCount
                if sel < 0 {
                    sel = 0
                }
                scrollOffset = sel
                redraw()
            }
        case 13:
            t.Result = sel + 1
            finished = true
        }

    }
    t.Index = sel
    return t
}

///////////////////////////////////////////////////////////////

func selector(options []string, style *tui_style) tui {
    // Read global cursor position directly
    currentRow := row+1 // Access global from main.go
    currentCol := col // Access global from main.go

    // Calculate optimal dimensions
    maxWidth := 14 // default = default prompt length
    for _, option := range options {
        if len(option) > maxWidth {
            maxWidth = len(option)
        }
    }
    menuWidth := maxWidth + 10      // padding for cursor and border
    menuHeight := len(options)-1

    // Build TUI config
    t := tui{
        Row:     currentRow+1,
        Col:     currentCol+1,
        Title:   "",
        Height:  menuHeight,
        Width:   menuWidth,
        Options: options,
        Prompt:  "",
        Cursor:  ">",
        Index:   0,
        Border:  true,
    }

    s := default_tui_style
    if style != nil {
        s = *style
    }

    res:=tui_menu(t,s)

    row+=menuHeight+4
    col=1

    return res
}

///////////////////////////////////////////////////////////////

var default_tui_style tui_style
var default_border_map map[string]string
var empty_border_map map[string]string

func buildTuiLib() {

    default_border_map = make(map[string]string, 10)
    default_border_map["tl"] = "╒"
    default_border_map["tr"] = "╕"
    default_border_map["bl"] = "╘"
    default_border_map["br"] = "╛"
    default_border_map["tm"] = "═"
    default_border_map["bm"] = "═"
    default_border_map["lm"] = "│"
    default_border_map["rm"] = "│"
    default_border_map["iv"] = "│"
    default_border_map["ih"] = "─"
    default_border_map["bg"] = "0"
    default_border_map["fg"] = "7"

    empty_border_map = make(map[string]string, 10)
    empty_border_map["tl"] = " "
    empty_border_map["tr"] = " "
    empty_border_map["bl"] = " "
    empty_border_map["br"] = " "
    empty_border_map["tm"] = " "
    empty_border_map["bm"] = " "
    empty_border_map["lm"] = " "
    empty_border_map["rm"] = " "
    empty_border_map["iv"] = " "
    empty_border_map["ih"] = " "
    empty_border_map["bg"] = "default"
    empty_border_map["fg"] = "default"

    default_tui_style = tui_style{
        bg:     "0",
        fg:     "7",
        border: default_border_map,
        wrap:   false,
    }

    features["tui"] = Feature{version: 1, category: "io"}
    categories["tui"] = []string{
        "tui_new", "tui_new_style", "tui", "tui_box", "tui_screen", "tui_text", "tui_pager", "tui_menu",
        "tui_progress", "tui_progress_reset", "tui_input", "tui_clear", "tui_template", "tui_table",
        "tui_table_select", "editor", "selector", "tui_radio",
    }

    slhelp["editor"] = LibHelp{
        in:     "default_content_string,width,height,title_string",
        out:    "string",
        action: "Launches a multiline text editor and returns the edited string",
    }

    stdlib["editor"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        ok, err := expect_args("editor", args, 1, "4", "string", "number", "number", "string")
        if !ok {
            return nil, err
        }

        // Direct extraction of arguments after type checking
        content := args[0].(string)
        width, _ := GetAsInt(args[1])
        height, _ := GetAsInt(args[2])
        title := args[3].(string)

        // Call the editor
        result, eof, broken := multilineEditor(content, width, height, "", "", title)

        // If cancelled (ESC) or EOF (Ctrl-D), return empty string
        if broken || eof {
            return "", nil
        }

        // Otherwise, return the edited text
        return result, nil
    }

    slhelp["tui_new"] = LibHelp{in: "", out: "map", action: "create a tui options map"}
    stdlib["tui_new"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_new", args, 0); !ok {
            return nil, err
        }
        return map[string]any{}, nil
    }

    slhelp["tui_screen"] = LibHelp{in: "int", out: "", action: "switch to primary (0) or secondary (1) screen buffer"}
    stdlib["tui_screen"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_screen", args, 1, "1", "int"); !ok {
            return nil, err
        }
        switch args[0].(int) {
        case 0:
            priScreen()
        case 1:
            secScreen()
        default:
            return nil, fmt.Errorf("invalid buffer specified in tui_screen() : %d", args[0].(int))
        }
        return nil, nil
    }

    slhelp["tui_new_style"] = LibHelp{in: "", out: "map", action: "create a tui style map"}
    stdlib["tui_new_style"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_new_style", args, 0); !ok {
            return nil, err
        }
        return map[string]any{
            "bg": "0",
            "fg": "7",
            "border": map[string]any{
                "tl": "╒", "tr": "╕", "bl": "╘", "br": "╛",
                "tm": "═", "bm": "═", "lm": "│", "rm": "│",
                "iv": "│", "ih": "─", "bg": "0", "fg": "7",
            },
            "wrap": false,
        }, nil
    }

    slhelp["tui"] = LibHelp{in: "map[,map]", out: "result", action: "perform tui action"}
    stdlib["tui"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        switch str.ToLower(t.Action) {
        case "box":
            stdlib["tui_box"](ns, evalfs, ident, args[0], s)
        case "menu":
            stdlib["tui_menu"](ns, evalfs, ident, args[0], s)
        case "text":
            stdlib["tui_text"](ns, evalfs, ident, args[0], s)
        case "pager":
            stdlib["tui_pager"](ns, evalfs, ident, args[0], s)
        case "input":
            stdlib["tui_input"](ns, evalfs, ident, args[0], s)
        case "radio":
            stdlib["tui_radio"](ns, evalfs, ident, args[0], s)
        case "progress":
            stdlib["tui_progress"](ns, evalfs, ident, args[0], s)
        case "table_select":
            stdlib["tui_table_select"](ns, evalfs, ident, args[0], s)
        }
        return "", err
    }

    slhelp["tui_template"] = LibHelp{in: "map[,map]", out: "", action: "replace {.field} matches in template string, with struct field values"}
    stdlib["tui_template"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_template", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        return tui_template(t, s)
    }

    slhelp["tui_progress"] = LibHelp{in: "map[,map]", out: "", action: "update a progress bar"}
    stdlib["tui_progress"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_progress", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        return tui_progress(t, s), err
    }

    slhelp["tui_progress_reset"] = LibHelp{in: "map", out: "", action: "reset a progress bar"}
    stdlib["tui_progress_reset"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_progress_reset", args, 1, "1", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        return tui_progress_reset(t), err
    }

    slhelp["tui_table"] = LibHelp{in: "map[,map]", out: "", action: "table formatter (.Format=\"csv|tsv|psv|ssv|custom|aos\" .Data=input string"}
    stdlib["tui_table"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_table", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        return tui_table(t, s)
    }

    slhelp["tui_table_select"] = LibHelp{in: "map[,map]", out: "tui_result", action: "interactive table row selector. Returns .Result=selected row index, .Cancel=true if aborted. Supports select_bg/select_fg in style."}
    stdlib["tui_table_select"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_table_select", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        return tui_table_select(t, s), nil
    }

    slhelp["tui_radio"] = LibHelp{in: "map[,map]", out: "", action: "checkbox selector"}
    stdlib["tui_radio"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_radio", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        return tui_radio(t, s), nil
    }

    slhelp["tui_clear"] = LibHelp{in: "map[,map]", out: "", action: "clear a tui element's area"}
    stdlib["tui_clear"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_clear", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        tui_clear(t, s)
        return nil, err
    }

    slhelp["tui_box"] = LibHelp{in: "map[,map]", out: "", action: "draw box"}
    stdlib["tui_box"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_box", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        tui_box(t, s)
        return nil, err
    }

    slhelp["tui_input"] = LibHelp{in: "map[,map]", out: "string", action: "input text. relevant tui struct fields: .Border, .Content, .Prompt, .Cursor, .Title, .Width, .Height, .Row, .Col"}
    stdlib["tui_input"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_input", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        return tui_input(t, s), err
    }

    slhelp["tui_text"] = LibHelp{in: "map[,map]", out: "", action: "output text"}
    stdlib["tui_text"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_text", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        tui_text(t, s)
        return nil, err
    }

    slhelp["tui_pager"] = LibHelp{in: "map[,map]", out: "", action: "pager for text"}
    stdlib["tui_pager"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_pager", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        tui_pager(t, s)
        return nil, err
    }

    slhelp["tui_menu"] = LibHelp{in: "map[,map]", out: "int_selection_position", action: "present menu"}
    stdlib["tui_menu"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("tui_menu", args, 2, "1", "map", "2", "map", "map"); !ok {
            return nil, err
        }
        t := mapToTui(args[0].(map[string]any))
        s := default_tui_style
        if len(args) == 2 {
            s = mapToTuiStyle(args[1].(map[string]any))
        }
        return tui_menu(t, s), err
    }

    slhelp["selector"] = LibHelp{in: "array[,map]", out: "tui_struct", action: "present selection menu at current cursor position"}
    stdlib["selector"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
        if ok, err := expect_args("selector", args, 4,
            "1", "[]any",
            "1", "[]string",
            "2", "[]any", "map",
            "2", "[]string","map"); !ok {
            return nil, err
        }

        // Convert options array to string slice
        var options []string
        switch args[0].(type) {
        case []any:
            optionsArray := args[0].([]any)
            options = make([]string, len(optionsArray))
            for i, opt := range optionsArray {
                options[i] = fmt.Sprintf("%v", opt)
            }
        default:
            options = args[0].([]string)
        }

        // Handle optional style parameter
        var s *tui_style
        if len(args) == 2 {
            styleMap := args[1].(map[string]any)
            style := mapToTuiStyle(styleMap)
            s = &style
        }

        return selector(options, s), err
    }

}

// Helper functions to convert map literals to structs
func mapToTui(m map[string]any) tui {
    t := tui{}
    if v, ok := m["row"]; ok {
        if i, ok := v.(int); ok {
            t.Row = i
        } else if f, ok := v.(float64); ok {
            t.Row = int(f)
        }
    }
    if v, ok := m["col"]; ok {
        if i, ok := v.(int); ok {
            t.Col = i
        } else if f, ok := v.(float64); ok {
            t.Col = int(f)
        }
    }
    if v, ok := m["height"]; ok {
        if i, ok := v.(int); ok {
            t.Height = i
        } else if f, ok := v.(float64); ok {
            t.Height = int(f)
        }
    }
    if v, ok := m["width"]; ok {
        if i, ok := v.(int); ok {
            t.Width = i
        } else if f, ok := v.(float64); ok {
            t.Width = int(f)
        }
    }
    if v, ok := m["action"]; ok {
        if s, ok := v.(string); ok {
            t.Action = s
        }
    }
    if v, ok := m["options"]; ok {
        if opts, ok := v.([]string); ok {
            t.Options = opts
        } else if optsAny, ok := v.([]any); ok {
            // Convert []any to []string
            opts := make([]string, len(optsAny))
            for i, opt := range optsAny {
                if str, ok := opt.(string); ok {
                    opts[i] = str
                }
            }
            t.Options = opts
        }
        // Initialize Selected field if not provided
        if t.Selected == nil && len(t.Options) > 0 {
            t.Selected = make([]bool, len(t.Options))
        }
    }
    if v, ok := m["selected"]; ok {
        if sel, ok := v.([]bool); ok {
            t.Selected = sel
        } else if selAny, ok := v.([]any); ok {
            // Convert []any to []bool
            sel := make([]bool, len(selAny))
            for i, s := range selAny {
                if b, ok := s.(bool); ok {
                    sel[i] = b
                }
            }
            t.Selected = sel
        }
    }
    if v, ok := m["sizes"]; ok {
        if sizes, ok := v.([]int); ok {
            t.Sizes = sizes
        } else if sizesAny, ok := v.([]any); ok {
            // Convert []any to []int
            sizes := make([]int, len(sizesAny))
            for i, s := range sizesAny {
                if n, ok := s.(int); ok {
                    sizes[i] = n
                }
            }
            t.Sizes = sizes
        }
    }
    if v, ok := m["display"]; ok {
        if disp, ok := v.([]int); ok {
            t.Display = disp
        } else if dispAny, ok := v.([]any); ok {
            // Convert []any to []int
            disp := make([]int, len(dispAny))
            for i, d := range dispAny {
                if n, ok := d.(int); ok {
                    disp[i] = n
                }
            }
            t.Display = disp
        }
    }
    if v, ok := m["started"]; ok {
        if b, ok := v.(bool); ok {
            t.Started = b
        }
    }
    if v, ok := m["vertical"]; ok {
        if b, ok := v.(bool); ok {
            t.Vertical = b
        }
    }
    if v, ok := m["cursor"]; ok {
        if s, ok := v.(string); ok {
            t.Cursor = s
        }
    }
    if v, ok := m["title"]; ok {
        if s, ok := v.(string); ok {
            t.Title = s
        }
    }
    if v, ok := m["prompt"]; ok {
        if s, ok := v.(string); ok {
            t.Prompt = s
        }
    }
    if v, ok := m["content"]; ok {
        if s, ok := v.(string); ok {
            t.Content = s
        }
    }
    if v, ok := m["value"]; ok {
        if f, ok := v.(float64); ok {
            t.Value = f
        }
    }
    if v, ok := m["border"]; ok {
        if b, ok := v.(bool); ok {
            t.Border = b
        }
    }
    if v, ok := m["data"]; ok {
        t.Data = v
    }
    if v, ok := m["format"]; ok {
        if s, ok := v.(string); ok {
            t.Format = s
        }
    }
    if v, ok := m["sep"]; ok {
        if s, ok := v.(string); ok {
            t.Sep = s
        }
    }
    if v, ok := m["result"]; ok {
        t.Result = v
    }
    if v, ok := m["cancel"]; ok {
        if b, ok := v.(bool); ok {
            t.Cancel = b
        }
    }
    if v, ok := m["multi"]; ok {
        if b, ok := v.(bool); ok {
            t.Multi = b
        }
    }
    if v, ok := m["reset"]; ok {
        if b, ok := v.(bool); ok {
            t.Reset = b
        }
    }
    if v, ok := m["headers"]; ok {
        if b, ok := v.(bool); ok {
            t.Headers = b
        }
    }
    if v, ok := m["tablesend"]; ok {
        if s, ok := v.(string); ok {
            t.TableSend = s
        }
    }
    if v, ok := m["index"]; ok {
        if i, ok := v.(int); ok {
            t.Index = i
        }
    }
    return t
}

func mapToTuiStyle(m map[string]any) tui_style {
    s := tui_style{}
    if v, ok := m["bg"]; ok {
        if str, ok := v.(string); ok {
            s.bg = str
        }
    }
    if v, ok := m["fg"]; ok {
        if str, ok := v.(string); ok {
            s.fg = str
        }
    }
    if v, ok := m["border"]; ok {
        if border, ok := v.(map[string]any); ok {
            // Initialize border map with defaults first
            s.border = make(map[string]string)
            s.border["tl"] = "╒"
            s.border["tr"] = "╕"
            s.border["bl"] = "╘"
            s.border["br"] = "╛"
            s.border["tm"] = "═"
            s.border["bm"] = "═"
            s.border["lm"] = "│"
            s.border["rm"] = "│"
            s.border["iv"] = "│"
            s.border["ih"] = "─"
            // Use default colours if style colours are empty
            if s.bg == "" {
                s.border["bg"] = "0"
            } else {
                s.border["bg"] = s.bg
            }
            if s.fg == "" {
                s.border["fg"] = "7"
            } else {
                s.border["fg"] = s.fg
            }
            // Now override with provided border values
            for k, v := range border {
                if str, ok := v.(string); ok {
                    s.border[k] = str
                }
            }
        }
    } else {
        // Initialize default border set if not provided
        s.border = make(map[string]string)
        s.border["tl"] = "╒"
        s.border["tr"] = "╕"
        s.border["bl"] = "╘"
        s.border["br"] = "╛"
        s.border["tm"] = "═"
        s.border["bm"] = "═"
        s.border["lm"] = "│"
        s.border["rm"] = "│"
        s.border["iv"] = "│"
        s.border["ih"] = "─"
        // Use default colours if style colours are empty
        if s.bg == "" {
            s.border["bg"] = "0"
        } else {
            s.border["bg"] = s.bg
        }
        if s.fg == "" {
            s.border["fg"] = "7"
        } else {
            s.border["fg"] = s.fg
        }
    }
    if v, ok := m["fill"]; ok {
        if b, ok := v.(bool); ok {
            s.fill = b
        }
    }
    if v, ok := m["wrap"]; ok {
        if b, ok := v.(bool); ok {
            s.wrap = b
        }
    }
    if v, ok := m["hi_bg"]; ok {
        if str, ok := v.(string); ok {
            s.hi_bg = str
        }
    }
    if v, ok := m["hi_fg"]; ok {
        if str, ok := v.(string); ok {
            s.hi_fg = str
        }
    }
    if v, ok := m["list"]; ok {
        if list, ok := v.([]string); ok {
            s.list = list
        } else if listAny, ok := v.([]any); ok {
            // Convert []any to []string
            list := make([]string, len(listAny))
            for i, item := range listAny {
                if str, ok := item.(string); ok {
                    list[i] = str
                }
            }
            s.list = list
        }
    }
    if v, ok := m["select_bg"]; ok {
        if str, ok := v.(string); ok {
            s.select_bg = str
        }
    }
    if v, ok := m["select_fg"]; ok {
        if str, ok := v.(string); ok {
            s.select_fg = str
        }
    }
    return s
}
