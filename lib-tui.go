//go:build !test

package main

import (
	"fmt"
	"reflect"
	"regexp"
	str "strings"
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
	Multi     bool
	Reset     bool
	Headers   bool
	TableSend string
	Index     int // menu cursor index
}

type tui_style struct {
	bg     string
	fg     string
	border map[string]string
	fill   bool
	wrap   bool
	hi_bg  string
	hi_fg  string
	list   []string
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

func tui_table(t tui, s tui_style) (os string, err error) {

	// Draw border if requested
	if t.Border {
		tui_box(tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2}, s)
	}

	// Options []string     // Field Headers
	// Sizes   []int        // Field Display Widths
	// Display []int        // Toggle display for columns
	// Content string       // final output
	// Data    any          // input data (either array of struct or a string
	// Format  string       // to specify input data type
	// Sep     string       // to specify the regex to use as a field separator
	/* style fields:
	   border["iv"] // inner-vertical
	   border["ih"] // inner-horizontal
	   hi_bg   string
	   hi_fg   string
	   list    []string
	*/

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
		return "", fmt.Errorf("Unknown separator type in tui_table() [%s]", t.Format)
	}

	var hasHeader bool
	var aaos [][]string
	var aos []string
	var maxSize int
	var colMax int
	var fieldNames []string

	if t.Headers {
		hasHeader = true
	}

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
		// convert to array of strings, newline separated
		var first bool = true
		for i, v := range aos {
			cols := delimitedSplit(v, sep)
			if first {
				colMax = len(cols)
				aaos[0] = make([]string, colMax)
			}
			aaos[i+1] = make([]string, colMax)
			if len(cols) != colMax {
				return "", fmt.Errorf("Column count mismatch (%d,%d) in tui_table() at .Data line %d", len(cols), colMax, i)
			}
			for j, c := range cols {
				l := len(c)
				c = stripDoubleQuotes(c)
				if l != len(c) {
					c = stripSingleQuotes(c)
				}
				if first && hasHeader {
					aaos[0][j] = c
					// append(aaos[0],c)
				}
				cols[j] = c
			}
			if !(first && hasHeader) {
				aaos[i] = cols
			}
			first = false
		}

	case "struct":
		// convert to array of strings, from array of struct (reflect on each field)
		isArray := (reflect.TypeOf(t.Data).Kind() == reflect.Array || reflect.TypeOf(t.Data).Kind() == reflect.Slice)
		if !isArray {
			return "", fmt.Errorf(".Data not an array (%#v)", reflect.TypeOf(t.Data).Kind().String())
		}

		var first bool = true
		var refstruct reflect.Value
		// pf("report: t.data.len : %d\n",len(t.Data.([]any)))
		for i := 0; i < len(t.Data.([]any)); i += 1 {
			// pf("report: i # %d : v %#v\n",i,t.Data.([]any)[i])
			switch refstruct = reflect.ValueOf(t.Data.([]any)[i]); refstruct.Kind() {
			case reflect.Struct:
			default:
				return "", fmt.Errorf(".Data element %d not a struct", i)
			}
			// get each field value, append to aaos
			rvalue := reflect.ValueOf(t.Data.([]any)[i])
			if first { // set header field names also
				colMax = rvalue.NumField()
				fieldNames = make([]string, colMax)
				aaos[0] = make([]string, colMax)
				for j := 0; j < colMax; j += 1 {
					rname := rvalue.Type().Field(j).Name
					fieldNames[j] = rname
					aaos[0][j] = rname
				}
				first = false
			}
			if rvalue.NumField() != colMax {
				return "", fmt.Errorf("Column count mismatch in tui_table() at .Data line %d", i)
			}
			aaos[i+1] = make([]string, colMax)
			for j := 0; j < colMax; j += 1 {
				field_value := refstruct.FieldByName(aaos[0][j])
				aaos[i+1][j] = sf("%v", field_value) // .String()
			}
		}
		hasHeader = true
	}

	// get field headers from either options array (manually provided), or from field names
	// if reading data from an array of structs

	if lineMethod != "struct" && hasHeader { // user instructed to take header names from data line 0
		fieldNames = make([]string, colMax)
		fieldNames = aaos[0]
	}

	if len(t.Options) > 0 {
		if lineMethod != "struct" {
			if len(t.Options) != colMax {
				return "", fmt.Errorf("Column count does not match provided header name count in tui_table() .Options field")
			}
			fieldNames = make([]string, len(t.Options))
			if len(t.Options) != 0 { // user provided list of header names from tui struct
				copy(fieldNames, t.Options)
				hasHeader = true
			}
		}
	}

	/*
	   pf("Row Max    %d\n",len(aaos))
	   pf("Column Max %d\n",colMax)
	   pf("Field Names : %+v\n",fieldNames)
	   pf("Has Header  : %+v\n",hasHeader)
	*/

	// set which columns will be displayed, and set user width preferences
	var selected []bool
	selected = make([]bool, colMax)

	if len(t.Display) > 0 {
		for _, v := range t.Display {
			selected[v] = true
		}
	} else { // display all
		for j := 0; j < colMax; j += 1 {
			selected[j] = true
		}
	}

	// do some thing to calculate max column width for each column, to use in the formatter afterwards
	cw := make([]int, colMax)

	if len(t.Sizes) == colMax {
		cw = t.Sizes
	} else {
		for _, l := range aaos {
			for j, v := range l {
				if len(v) > cw[j] {
					cw[j] = len(v)
				}
			}
		}
	}

	// formatter

	iv := s.border["iv"]
	ih := s.border["ih"]
	hb := s.hi_bg
	hf := s.hi_fg

	table_width := 5
	dispColCount := 0
	for j := range cw {
		if selected[j] {
			table_width += 2 + cw[j]
			dispColCount += 1
		}
	}
	if iv == "" {
		table_width -= dispColCount
	}

	cllen := len(s.list)

	if cllen > 0 && cllen != colMax {
		return "", fmt.Errorf("Column count does not match provided colour list length in tui_table() style .list field")
	}

	// header display
	if hasHeader {
		if ih != "" {
			os += rep(ih, table_width) + "\n"
		}
		os += iv
		ansiLine := ""
		for e := 0; e < len(fieldNames); e += 1 {
			if selected[e] {
				section := sf("%s %-*s [##][#-]%s", hb+hf, cw[e], fieldNames[e], iv)
				ansiLine += section
				os += section
			}
		}

		if ih != "" {
			os += "\n" + rep(ih, table_width) + "\n"
		} else {
			os += "\n"
		}
	}

	// data display
	for trow := 1; trow < len(aaos); trow += 1 {
		line := aaos[trow]

		if s.bg != "" {
			os += "[#b" + s.bg + "]"
		}
		if s.fg != "" {
			os += "[#" + s.fg + "]"
		}

		os += iv
		ansiLine := ""
		for j, v := range line {
			field_colour := ""
			if cllen > 0 {
				field_colour = s.list[j]
			}

			if selected[j] {
				ansiLine = sf("%s %-*s [##][#-]%s", field_colour, cw[j], v, iv)
				os += ansiLine
			}
		}

		if ih != "" {
			os += "\n" + rep(ih, table_width) + "\n"
		} else {
			os += "\n"
		}
	}

	// pass through to pager/other
	// Row     int          // Passed through to pager
	// Col     int          // Passed through to pager
	// Height  int          // Passed through to pager
	// Width   int          // Passed through to pager
	// Border  bool         // Passed through to pager
	// Title   string       // Passed through to pager

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
		case 13, 'q', 'Q', 27: // enter, q or escape
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
			scroll_pos := int(float64(cpos) * float64(t.Height-1) / float64(len(ra)))
			absat(t.Row+1+scroll_pos, t.Col+t.Width-2)
			pf("[#invert]*[##][#-]")
			// process keypresses
			k := wrappedGetCh(0, false)
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
			case 'b', 15:
				cpos -= t.Height - 1
				if cpos < 0 {
					cpos = 0
				}
			case ' ', 14:
				if cpos+max < len(ra)-1 {
					cpos += t.Height - 1
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
	// pf("\n\n\nentered tui_menu func with:\nt : %+v\ns : %+v\n\n\n",t,s)
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

	// Draw border if requested - border should be drawn around the content area
	if t.Border {
		tui_box(tui{Title: t.Title, Row: t.Row - 1, Width: t.Width + 2, Col: t.Col - 1, Height: t.Height + 2}, s)
	}

	pf(addbg)
	pf(addfg)

	absat(row+2, col+2)
	pf(prompt)

	sel := t.Index

	/*
	   ol:=len(t.Options)
	   key_range = -1
	   // determine shortcut keys per option
	   if t.shortcut {
	       if ol<30 {
	           key_range = ( "1".asc .. "9".asc ) +  ( "a".asc .. "a".asc+ol-10 )
	       } else {
	           if ol<10 {
	               key_range = "1".asc .. "1".asc+ol-1
	           }
	       }
	   }
	*/

	// display menu
	for k, p := range t.Options {
		absat(row+4+k, col+6)
		// short_code=" "
		// if key_range!=nil { short_code=key_range[key_p].char }
		pf(p)
		// "[{=short_code}]{p}"
	}

	// maxchoice=49+len(t.Options)

	// input loop
	finished := false
	t.Cancel = false

	for !finished {

		absat(row+4+sel, col+4)
		pf(cursor)
		absat(row+4+sel, col+6)
		pf(addhibg + addhifg + t.Options[sel] + "[##][#-]")
		k := wrappedGetCh(0, false)
		absat(row+4+sel, col+4)
		pf(addbg)
		pf(addfg)
		pf(" ")
		absat(row+4+sel, col+6)
		pf(t.Options[sel])

		//if k>=49 && k<maxchoice {
		//    result=k-48
		//}

		if k == 'q' || k == 'Q' {
			t.Cancel = true
			break
		}

		switch k {
		case 11:
			if sel > 0 {
				sel--
			}
		case 10:
			if sel < len(t.Options)-1 {
				sel++
			}
		case 13:
			t.Result = sel + 1
			finished = true
		}

	}
	t.Index = sel
	return t
}

/////////////////////////////////////////////////////////////////

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
		"tui_progress", "tui_progress_reset", "tui_input", "tui_clear", "tui_template", "tui_table", "editor",
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

}

// Helper functions to convert map literals to structs
func mapToTui(m map[string]any) tui {
	t := tui{}
	if v, ok := m["row"]; ok {
		if i, ok := v.(int); ok {
			t.Row = i
		}
	}
	if v, ok := m["col"]; ok {
		if i, ok := v.(int); ok {
			t.Col = i
		}
	}
	if v, ok := m["height"]; ok {
		if i, ok := v.(int); ok {
			t.Height = i
		}
	}
	if v, ok := m["width"]; ok {
		if i, ok := v.(int); ok {
			t.Width = i
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
			// Use default colors if style colors are empty
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
		// Use default colors if style colors are empty
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
	return s
}
