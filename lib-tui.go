//+build !test

package main

import (
    "fmt"
    "bytes"
    "unicode"
    str "strings"
)

type tui struct {
    Row     int
    Col     int
    Height  int
    Width   int
    Action  string
    Options []any
    Selected []bool
    Vertical bool
    Cursor  string
    Title   string
    Prompt  string
    Content string
    Value   float64
    Border  bool
    Bdrawn  bool
    data    any
    format  string
    Sep     string
    Result  any
    Cancel  bool
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

   table data output formatting (and pass through to pager)
    - possible input formats, specified in tui.format/tui.Sep, with data in tui.data:
      - csv, tsv, space or pipe delimited (or some other custom separator)
      - array of struct
      - newline separated (i.e. consume fixed number of lines)?
      - newline separated with record separator (consume variable lines as columns)?
      - json?
      - yaml/toml/etc?
      - option to bypass pager and go straight to stdout or file
    - this would require some further style choices
      - e.g. fixed bg/fg for table/columns.
      - per column and row options for colour
      - modulus-based bg for rows (i.e. fixed/odd/even/every X bg shading)
      - inner border styles as well as outer
    - column headings should be optional
    - table title should be taken from tui.Title if present.
    - tui.Height would be ignored
    - tui.Width could be ignored, flag dependent.
      - i.e. permit dynamic growth of width to accommodate columns.

   structured templates:
    - i.e. pass in a template and a struct,
    - var replacement in template using struct fields
    - then output parsed template using tui_text. (and style)

   mouse support?
   call-back support and async actions? timers?
*/


// switch to secondary buffer
func secScreen() {
    pf("\033[?1049h\033[H")
}

// switch to primary buffer
func priScreen() {
    pf("\033[?1049l")
}

func absClearChars(row int,col int,l int) {
    if l<1 { return }
    absat(row,col)
    fmt.Print(str.Repeat(" ",l))
}


/* mitchell hashimoto word wrap code below
    from: https://github.com/mitchellh/go-wordwrap/blob/master/wordwrap.go
   mit licensed.
   not likely to need an update, so just taking the func.
   may add a left/right/full justify option to it later.
*/
const nbsp = 0xA0
func wrapString(s string, lim uint) string {
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


func str_inset(n int) (string) {
    return sf("\033[%dG",n)
}

//   horizontal / vertical radio button (single/multi-selector)
func tui_radio(t tui, s tui_style) {

    options:=[]string{}
    for _,v:=range t.Options {
        o:=GetAsString(v)
        options=append(options,o)
    }

    cursor:="x"
    if t.Cursor!="" {
        cursor=t.Cursor
    }

    // build output string
    op:=t.Prompt
    sep:=" "
    if t.Sep != "" { sep=t.Sep }

    for k:=0; k<len(options); k+=1 {
        op+="["
        if t.Selected[k] {
            op+=cursor
        } else {
            op+=" "
        }
        op+="] "+options[k]
        if t.Vertical {
            op+="\n"+str_inset(t.Col+len(t.Prompt))
        } else {
            op+=sep
        }
    }

    // display
    absat(t.Row,t.Col)
    pf(op)

    // key loop
    // key:=wrappedGetCh(0,false)
    wrappedGetCh(0,false)

}



func tui_progress(t tui,s tui_style) {
    hsize:=t.Width
    row:=t.Row
    col:=t.Col
    pc:=t.Value
    c:="█"
    if t.Cursor != "" { c=t.Cursor }

    hideCursor()
    bgcolour:="[#b"+s.bg+"]"
    fgcolour:="[#"+s.fg+"]"

    absat(row,col)
    if pc==0 && t.Border {
        fmt.Print(rep(" ",hsize))
        border:=empty_border_map
        tui_box(
            tui{ Title:t.Title,Row:t.Row-1,Width:t.Width+2,Col:t.Col-1,Height:t.Height+2 },
            tui_style{ border:border },
        )
        return
    } else {
        if !t.Bdrawn && t.Border {
            tui_box(tui{ Title:t.Title,Row:t.Row-1,Width:t.Width+2,Col:t.Col-1,Height:t.Height+2 }, s)
            t.Bdrawn=true
        }
    }

    d  := pc*float64(hsize)        // width of input percent

    absat(row,col)
    pf(bgcolour+fgcolour)
    for e:=0;e<hsize;e+=1 {
        if e>int(d) { break }
        fmt.Print(c)
    }
    pf("[#-]")
    fmt.Print(rep(" ",hsize-int(d)-1))
}


func tui_text_modal(t tui,s tui_style) {
    addbg:=""; addfg:=""
    if s.bg!="" { addbg="[#b"+s.bg+"]" }
    if s.fg!="" { addfg="[#"+s.fg+"]" }
    pf(addbg); pf(addfg)

    cpos:=0
    
    var w uint
    var rs string
    w=uint(t.Width-3)
    if s.wrap {
        rs=wrapString(t.Content,w)
    } else {
        rs=t.Content
    }
    ra:=str.Split(rs,"\n")
    if !s.wrap {
        // do something to clip long lines here
        //  if we add horizontal scroll bars later, this will
        //  need to change to a bounded clip on display instead.
        for k,v:=range ra {
            if len(v) > t.Width-2 {
                ra[k]=ra[k][:t.Width-2]
            }
        }
    }

    t.Cancel=false
    max:=t.Height-1
    for ;!t.Cancel; {
        if cpos+t.Height-1>len(ra) { max=len(ra)-cpos }
        for k,v:=range ra[cpos:cpos+max] {
            absClearChars(t.Row+k+1,t.Col+1,t.Width-2)
            absat(t.Row+k+1,t.Col+1)
            pf(addbg+addfg+v)
            absClearChars(t.Row+k+1,t.Col+1+len(v),t.Width-2-len(v))
        }
        pf("[##][#-]")
        // scroll position
        scroll_pos:=int(float64(cpos)*float64(t.Height-1)/float64(len(ra)))
        absat(t.Row+1+scroll_pos,t.Col+t.Width-2)
        pf("[#invert]*[#-]")
        // process keypresses
        k:=wrappedGetCh(0,false)
        switch k {
        case 10: //down
            if cpos<len(ra)-t.Height { cpos++ }
        case 11: //up
            if cpos>0 { cpos-- }
        case 'q','Q',27:
            t.Cancel=true
        case 'b',15:
            cpos-=t.Height-1
            if cpos<0 { cpos=0 }
        case ' ',14:
            cpos+=t.Height-1
            if cpos>len(ra)-t.Height { cpos=len(ra)-t.Height }
        }
    }
}

// at row,col, width of t.Width-2, print wordWrap'd t.Content
func tui_text(t tui,s tui_style) {
    addbg:=""; addfg:=""
    if s.bg!="" { addbg="[#b"+s.bg+"]" }
    if s.fg!="" { addfg="[#"+s.fg+"]" }
    pf(addbg); pf(addfg)

    var w uint
    w=uint(t.Width-2)
    rs:=t.Content
    if s.wrap { rs=wrapString(rs,w) }
    ra:=str.Split(rs,"\n")
    if len(ra)>t.Height-2 {
        ra=ra[len(ra)-t.Height:]
    }
    for k,v:=range ra {
        absat(t.Row+k+1,t.Col+1)
        pf(addbg+addfg+v)
    }
    pf("[##][#-]")
}

func tui_clear(t tui, s tui_style) {
    pf("[##][#-]") 
    for e:=0;e<t.Height+1;e+=1 {
        absat(t.Row+e,t.Col)
        fmt.Print(rep(" ",t.Width))
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

    addbg:=""; addfg:=""
    if s.bg!="" { addbg="[#b"+s.bg+"]" }
    if s.fg!="" { addfg="[#"+s.fg+"]" }

    // draw border box
    if t.Border {
        tui_box(tui{ Title:t.Title,Row:t.Row-1,Width:t.Width+2,Col:t.Col-1,Height:t.Height+1 }, s)
    }

    // get input
    mask:="*"
    oldmask:=""
    if t.Cursor!="" {
        emask,_:=gvget("@echomask")
        oldmask=emask.(string)
        gvset("@echomask",t.Cursor)
        mask=t.Cursor
    }
    promptColour:=addbg+addfg
    input, _, _ := getInput(t.Prompt, t.Content, "global", t.Row, t.Col, t.Width, promptColour, false, false, mask)
    input=sanitise(input)

    // remove border box
    if t.Border {
        border:=empty_border_map
        tui_box(
            tui{ Title:t.Title,Row:t.Row-1,Width:t.Width+2,Col:t.Col-1,Height:t.Height+2 },
            tui_style{ border:border },
        )
        tui_clear(t,s)
    }

    if t.Cursor!="" {
        gvset("@echomask",oldmask)
    }

    t.Result=input
    return t
}


func tui_box(t tui,s tui_style) {

    row:=t.Row; col:=t.Col
    height:=t.Height; width:=t.Width
    title:=t.Title

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

    if tl==" " && tr==" " && bl==" " && br==" " { // should probably deep compare to empty_border_map instead
        title=""
    }
    
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
    for r:=row+1; r<row+height; r+=1 {
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
        pf(" "+title+" ")
    }

    if bg!="" { pf("[##]") }
    if fg!="" { pf("[#-]") }

}


/////////////////////////////////////////////////////////////////

func tui_menu(t tui,s tui_style) tui {
    row:=t.Row
    col:=t.Col
    cursor:=t.Cursor
    prompt:=t.Prompt
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
    for k,p := range t.Options {
        absat(row+4+k,col+6)
        // short_code=" "
        // on key_range!=nil do short_code=key_range[key_p].char
        pf(p.(string))
        // "[{=short_code}]{p}"
    }

    // maxchoice=49+len(t.Options)

    // input loop
    finished:=false
    t.Cancel=false

    for ;!finished; {

        absat(row+4+sel,col+4); pf(cursor)
        absat(row+4+sel,col+6); pf(addhibg+addhifg+t.Options[sel].(string)+"[##][#-]")
        k:=wrappedGetCh(0,false)
        absat(row+4+sel,col+4)
        pf(addbg); pf(addfg)
        pf(" ")
        absat(row+4+sel,col+6); pf(t.Options[sel].(string))

        //if k>=49 && k<maxchoice {
        //    result=k-48
        //}

        if k=='q' || k=='Q' {
            t.Cancel=true
            break
        }

        switch k {
        case 11:
            if sel>0 { sel-- }
        case 10:
            if sel<len(t.Options)-1 { sel++ }
        case 13:
            t.Result=sel+1
            finished=true
        }

    }
    return t
}

/////////////////////////////////////////////////////////////////

var default_tui_style tui_style
var default_border_map map[string]string
var empty_border_map map[string]string

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

    empty_border_map = make(map[string]string,10)
    empty_border_map["tl"]=" "
    empty_border_map["tr"]=" "
    empty_border_map["bl"]=" "
    empty_border_map["br"]=" "
    empty_border_map["tm"]=" "
    empty_border_map["bm"]=" "
    empty_border_map["lm"]=" "
    empty_border_map["rm"]=" "
    empty_border_map["bg"]="default"
    empty_border_map["fg"]="default"

    default_tui_style = tui_style{
        bg: "0", 
        fg: "7",
        border: default_border_map, 
        wrap: false,
    }


    features["tui"] = Feature{version: 1, category: "io"}
    categories["tui"] = []string{
        "tui_new","tui_new_style","tui","tui_box","tui_screen","tui_text","tui_text_modal","tui_menu",
        "tui_progress","tui_input","tui_clear",
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
        if ok,err:=expect_args("tui",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        switch str.ToLower(t.Action) {
        case "box"      : stdlib["tui_box"](ns,evalfs,ident,t,s)
        case "menu"     : stdlib["tui_menu"](ns,evalfs,ident,t,s)
        case "text"     : stdlib["tui_text"](ns,evalfs,ident,t,s)
        case "modal"    : stdlib["tui_text_modal"](ns,evalfs,ident,t,s)
        case "input"    : stdlib["tui_input"](ns,evalfs,ident,t,s)
        case "radio"    : stdlib["tui_radio"](ns,evalfs,ident,t,s)
        case "progress" : stdlib["tui_text_progress"](ns,evalfs,ident,t,s)
        }
        return "",err
    }

    slhelp["tui_progress"] = LibHelp{in: "tui_struct[,tui_style]", out: "", action: "update a progress bar"}
    stdlib["tui_progress"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_progress",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        tui_progress(t,s) 
        return nil,err
    }

    slhelp["tui_radio"] = LibHelp{in: "tui_struct[,tui_style]", out: "", action: "checkbox selector"}
    stdlib["tui_radio"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_radio",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        tui_radio(t,s) 
        return nil,err
    }

    slhelp["tui_clear"] = LibHelp{in: "tui_struct[,tui_style]", out: "", action: "clear a tui element's area"}
    stdlib["tui_clear"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_clear",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        tui_clear(t,s) 
        return nil,err
    }

    slhelp["tui_box"] = LibHelp{in: "tui_struct[,tui_style]", out: "", action: "draw box"}
    stdlib["tui_box"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_box",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        tui_box(t,s) 
        return nil,err
    }

    slhelp["tui_input"] = LibHelp{in: "tui_struct[,tui_style]", out: "string", action: "input text. relevant tui struct fields: .Border, .Content, .Prompt, .Cursor, .Title, .Width, .Height, .Row, .Col"}
    stdlib["tui_input"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_input",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        return tui_input(t,s),err
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

    slhelp["tui_text_modal"] = LibHelp{in: "tui_struct[,tui_style]", out: "", action: "pager for text"}
    stdlib["tui_text_modal"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_text_modal",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        tui_text_modal(t,s)
        return nil,err
    }


    slhelp["tui_menu"] = LibHelp{in: "tui_struct[,tui_style]", out: "int_selection_position", action: "present menu"}
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

