//+build !test

package main

import (
    "fmt"
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
}

type tui_style struct {
    bg      string
    fg      string
    border  map[string]string
    fill    bool
    wrap    bool 
}


/*
   actions to add:
   vertical menu
   horizontal menu
   display pane (with word splitting)
   progress bar
   input box
   cascading input selector
   mouse support
   movable panes

*/

// switch to secondary buffer
func secScreen() {
    pf("\033[?1049h\033[H")
}

// switch to primary buffer
func priScreen() {
    pf("\033[?1049l")
}


func tui_box(row,col,height,width int,title string,s tui_style) {

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

    if bg!="" { pf("[#b"+bg+"]") }
    if fg!="" { pf("[#"+fg+"]") }

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
/*
func menu(t tui,s tui_style) {
    row:=t.row
    col:=t.col
    cursor:=t.cursor
    title:=t.title
    prompt:=t.prompt
    bg:=s.bg
    fg:=s.fg
    border:=s.border
    wrap:=s.wrap
    cursoroff()

    at 2,10
    print "[#b1]{p}[##]"

    sel=0
    ol=opts.len

    # determine shortcut keys per option
    case
    has ol<10
        key_range = "1".asc .. "1".asc+ol-1
        break
    has ol<30
        key_range = ( "1".asc .. "9".asc ) +  ( "a".asc .. "a".asc+ol-10 )
        break
    has ol>=30
        key_range = nil
    ec

    # display menu
    foreach p in opts
        at 4+key_p,12
        short_code=" "
        on key_range!=nil do short_code=key_range[key_p].char
        print "[#b1][{=short_code}][##] {p}"
    endfor

    maxchoice=49+len(opts)

    at 5+opts.len,10
    print "[#b2][q][##] Quit menu"

    # input loop
    finished=false
    n=-1
    while n==-1 and not finished

        at 4+sel,10,"[#invert]-[#-]"
        at 4+sel,16,"[#3]",opts[sel],"[#-]"
        k=keypress()
        at 4+sel,10," "
        at 4+sel,16,opts[sel]

        if k>=49 && k<maxchoice
            n=k-48
        endif

        on char(k)=="q" do break

        case k
        is 11
            sel=sel-btoi(sel>0)
        is 10
            sel=sel+btoi(sel<opts.len-1)
        is 13
            n=sel+1
            finished=true
        ec

    endwhile

    cursoron()
    return n

end

*/

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
        case "box": stdlib["tui_box"](ns,evalfs,ident,t,s)
        }
        return "",err
    }

    slhelp["tui_box"] = LibHelp{in: "tui_struct", out: "", action: "draw box"}
    stdlib["tui_box"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("tui_box",args,2,
            "1","main.tui",
            "2","main.tui","main.tui_style"); !ok { return nil,err }
        t:=args[0].(tui)
        s:=default_tui_style
        if len(args)==2 { s=args[1].(tui_style) }
        tui_box(t.row,t.col,t.height,t.width,t.title,s) 
        return nil,err
    }

}


