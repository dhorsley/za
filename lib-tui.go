//+build !test

package main

import (
    "fmt"
	"bytes"
	"unicode"
    str "strings"
)

type tui struct {
    row     int
    col     int
    height  int
    width   int
    action  string
    options []any
    cursor  string
    title   string
    prompt  string
    content string
}

type tui_style struct {
    bg      string
    fg      string
    border  map[string]string
    fill    bool
    wrap    bool 
    hi_bg   string
    hi_fg   string
}


/*
   actions to add:
   horizontal menu
   progress bar
   input box
   cascading input selector
   mouse support?
   movable panes? 

*/

// switch to secondary buffer
func secScreen() {
    pf("\033[?1049h\033[H")
}

// switch to primary buffer
func priScreen() {
    pf("\033[?1049l")
}


/* mitchell hashimoto word wrap code below
    from: https://github.com/mitchellh/go-wordwrap/blob/master/wordwrap.go
   mit licensed.
   not likely to need an update, so just taking the func.
   may add a left/right/full justify option to it later.
*/
const nbsp = 0xA0
func WrapString(s string, lim uint) string {
	// Initialize a buffer with a slightly larger size to account for breaks
	init := make([]byte, 0, len(s))
	buf := bytes.NewBuffer(init)

	var current uint
	var wordBuf, spaceBuf bytes.Buffer
	var wordBufLen, spaceBufLen uint

	for _, char := range s {
		if char == '\n' {
			if wordBuf.Len() == 0 {
				if current+spaceBufLen > lim {
					current = 0
				} else {
					current += spaceBufLen
					spaceBuf.WriteTo(buf)
				}
				spaceBuf.Reset()
				spaceBufLen = 0
			} else {
				current += spaceBufLen + wordBufLen
				spaceBuf.WriteTo(buf)
				spaceBuf.Reset()
				spaceBufLen = 0
				wordBuf.WriteTo(buf)
				wordBuf.Reset()
				wordBufLen = 0
			}
			buf.WriteRune(char)
			current = 0
		} else if unicode.IsSpace(char) && char != nbsp {
			if spaceBuf.Len() == 0 || wordBuf.Len() > 0 {
				current += spaceBufLen + wordBufLen
				spaceBuf.WriteTo(buf)
				spaceBuf.Reset()
				spaceBufLen = 0
				wordBuf.WriteTo(buf)
				wordBuf.Reset()
				wordBufLen = 0
			}

			spaceBuf.WriteRune(char)
			spaceBufLen++
		} else {
			wordBuf.WriteRune(char)
			wordBufLen++

			if current+wordBufLen+spaceBufLen > lim && wordBufLen < lim {
				buf.WriteRune('\n')
				current = 0
				spaceBuf.Reset()
				spaceBufLen = 0
			}
		}
	}

	if wordBuf.Len() == 0 {
		if current+spaceBufLen <= lim {
			spaceBuf.WriteTo(buf)
		}
	} else {
		spaceBuf.WriteTo(buf)
		wordBuf.WriteTo(buf)
	}

	return buf.String()
}
/* end of theft */

func tui_text(t tui,s tui_style) {
    // at row,col, width of t.width-2, print wordWrap'd t.content using inset() to move line starts to col+2
}


// func tui_box(row,col,height,width int,title string,s tui_style) {
func tui_box(t tui,s tui_style) {

    row:=t.row; col:=t.col
    height:=t.height; width:=t.width
    title:=t.title

    // pf("\n%d,%d,%d,%d,%s,%#v\n",row,col,height,width,title,s)
    tl:=s.border["tl"]
    tr:=s.border["tr"]
    bl:=s.border["bl"]
    br:=s.border["br"]
    tm:=s.border["tm"]
    bm:=s.border["bm"]
    lm:=s.border["lm"]
    rm:=s.border["rm"]
    bg:=s.border["bg"]
    fg:=s.border["fg"]

    addbg:=""; addfg:=""
    if bg!="" { addbg="[#b"+bg+"]" }
    if fg!="" { addfg="[#"+fg+"]" }
    pf(addbg); pf(addfg)

    // top
    absat(row,col)
    fmt.Print(tl)
    fmt.Print(rep(tm,width-2))
    fmt.Print(tr)

    // sides
    for r:=row+1; r<row+height; r++ {
        absat(r,col)
        fmt.Print(lm)
        if s.fill {
            fmt.Print(rep(" ",width-2))
        } else {
            absat(r,col+width-1)
        }
        fmt.Print(rm)
    }

    // bottom
    absat(row+height,col)
    fmt.Print(bl)
    fmt.Print(rep(bm,width-2))
    fmt.Print(br)

    // title
    if title != "" {
        absat(row,col+4)
        pf(title)
    }

    if bg!="" { pf("[##]") }
    if fg!="" { pf("[#-]") }

}


/////////////////////////////////////////////////////////////////

func tui_menu(t tui,s tui_style) (result int) {
    row:=t.row
    col:=t.col
    cursor:=t.cursor
    prompt:=t.prompt
    bg:=s.bg
    fg:=s.fg
    hi_bg:=s.hi_bg
    hi_fg:=s.hi_fg

    addbg:=""; addfg:=""
    addhibg:=""; addhifg:=""
    if bg!="" { addbg="[#b"+bg+"]" }
    if fg!="" { addfg="[#"+fg+"]" }
    if hi_bg!="" { addhibg="[#b"+hi_bg+"]" }
    if hi_fg!="" { addhifg="[#"+hi_fg+"]" }
    pf(addbg); pf(addfg)

    absat(row+2,col+2)
    pf(prompt)

    sel:=0

    /*
    ol:=len(t.options)
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
    for k,p := range t.options {
        absat(row+4+k,col+6)
        // short_code=" "
        // on key_range!=nil do short_code=key_range[key_p].char
        pf(p.(string))
        // "[{=short_code}]{p}"
    }

    // maxchoice=49+len(t.options)

    // input loop
    finished:=false

    for ;!finished; {

        absat(row+4+sel,col+4); pf(cursor)
        absat(row+4+sel,col+6); pf(addhibg+addhifg+t.options[sel].(string)+"[##][#-]")
        k:=wrappedGetCh(0,false)
        absat(row+4+sel,col+4)
        pf(addbg); pf(addfg)
        pf(" ")
        absat(row+4+sel,col+6); pf(t.options[sel].(string))

        //if k>=49 && k<maxchoice {
        //    result=k-48
        //}

        if k=='q' || k=='Q' {
            break
        }

        switch k {
        case 11:
            if sel>0 { sel-- }
        case 10:
            if sel<len(t.options)-1 { sel++ }
        case 13:
            result=sel+1
            finished=true
        }

    }
    return result
}

/////////////////////////////////////////////////////////////////

var default_tui_style tui_style
var default_border_map map[string]string

func buildTuiLib() {

    default_border_map = make(map[string]string,10)
    default_border_map["tl"]="╒"
    default_border_map["tr"]="╕"
    default_border_map["bl"]="╘"
    default_border_map["br"]="╛"
    default_border_map["tm"]="═"
    default_border_map["bm"]="═"
    default_border_map["lm"]="│"
    default_border_map["rm"]="│"
    default_border_map["bg"]="0"
    default_border_map["fg"]="7"

    default_tui_style = tui_style{
        bg: "0", 
        fg: "7",
        border: default_border_map, 
        wrap: false,
    }


    features["tui"] = Feature{version: 1, category: "io"}
    categories["tui"] = []string{
        "tui_new","tui_new_style","tui","tui_box","tui_screen",
    }

    slhelp["tui_new"] = LibHelp{in: "", out: "tui_struct", action: "create a tui options struct"}
    stdlib["tui_new"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_new",args,0); !ok { return nil,err }
        return tui{},nil
    }

    slhelp["tui_screen"] = LibHelp{in: "int", out: "", action: "switch to primary (0) or secondary (1) screen buffer"}
    stdlib["tui_screen"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_screen",args,1,"1","int"); !ok { return nil,err }
            switch args[0].(int) {
            case 0:
                priScreen()
            case 1:
                secScreen()
            default:
                return nil,fmt.Errorf("invalid buffer specified in tui_screen() : %d",args[0].(int))
            }
        return nil,nil
    }

    slhelp["tui_new_style"] = LibHelp{in: "", out: "tui_style_struct", action: "create a tui style struct"}
    stdlib["tui_new_style"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_new_style",args,0); !ok { return nil,err }
        return default_tui_style,nil
    }

    slhelp["tui"] = LibHelp{in: "tui_struct", out: "result", action: "perform tui action"}
    stdlib["tui"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        // pf("tui in  %#v\n",args[0])
        if ok,err:=expect_args("tui",args,1,"1","main.tui"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        switch str.ToLower(t.action) {
        case "box"  : stdlib["tui_box"](ns,evalfs,ident,t,s)
        case "menu" : stdlib["tui_menu"](ns,evalfs,ident,t,s)
        case "text" : stdlib["tui_text"](ns,evalfs,ident,t,s)
        }
        return "",err
    }

    slhelp["tui_box"] = LibHelp{in: "tui_struct[,tui_style]", out: "", action: "draw box"}
    stdlib["tui_box"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_box",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        // tui_box(t.row,t.col,t.height,t.width,t.title,s) 
        tui_box(t,s) 
        return nil,err
    }

    slhelp["tui_text"] = LibHelp{in: "tui_struct[,tui_style]", out: "", action: "output text"}
    stdlib["tui_text"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_text",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        tui_text(t,s)
        return nil,err
    }


    slhelp["tui_menu"] = LibHelp{in: "tui_struct[,tui_style]", out: "", action: "present menu"}
    stdlib["tui_menu"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_menu",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        return tui_menu(t,s),err
    }

}


