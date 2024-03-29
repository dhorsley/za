// +build !test

package main

import (
    "os"
    "errors"
    "github.com/ajstarks/svgo"
    "math/rand"
    "sync"
)


// This package includes functions for interfacing with SVG components.

// underlying package: https://github.com/ajstarks/svgo

type svg_table_entry struct {
    location    string
    file        *os.File
    svg         *svg.SVG
}

var svg_handles = make ( map[string]svg_table_entry )

var svglock = &sync.RWMutex{}

func svgClose(h string) {
    // pf("* Closing svg (%s).\n",h)
    svg_handles[h].file.Sync()
    svg_handles[h].file.Close()
    delete(svg_handles,h)
}

func svgCreateHandle() string {
    var svgid string
    for ;; {
        b := make([]byte, 16)
        rand.Read(b)
        svgid = sf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
        if _,exists:=svg_handles[svgid]; !exists { break }
    }
    return svgid
}

func svgCreate(filename string) string {

    svglock.Lock()

    f, err := os.Create(filename)
    if err!=nil {
        return ""
    }
    svgid:=svgCreateHandle()
    hnd:=svg.New(f)
    svg_handles[svgid]=svg_table_entry{location:filename,file:f,svg:hnd}

    svglock.Unlock()

    return svgid

}


func buildImageLib() {

    // conversion

    features["image"] = Feature{version: 1, category: "image"}
    categories["image"] = []string{ "svg_start","svg_end","svg_title","svg_desc",
                                    "svg_plot","svg_circle","svg_ellipse",
                                    "svg_rect","svg_roundrect","svg_square",
                                    "svg_line","svg_polyline","svg_polygon",
                                    "svg_text","svg_image","svg_grid",
                                    "svg_def","svg_def_end","svg_group","svg_group_end",
                                    "svg_link","svg_link_end",
    }

    slhelp["svg_circle"] = LibHelp{in: "handle,x,y,radius[,attributes]", out: "", action: "Draws a circle on the canvas [#i1]handle[#i0]."}
    stdlib["svg_circle"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_circle",args,2,
            "5","string","int","int","int","string",
            "4","string","int","int","int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 5:
            attributes=args[4].(string)
        }
        x:=args[1].(int) ; y:=args[2].(int) ; r:=args[3].(int)
        svg_handles[handle].svg.Circle(x,y,r,attributes)
        return nil,nil
    }

    // Square(x int, y int, s int, style ...string)
    slhelp["svg_square"] = LibHelp{in: "handle,x,y,size[,attributes]", out: "", action: "Draws a square on the canvas [#i1]handle[#i0]."}
    stdlib["svg_square"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_square",args,2,
            "5","string","int","int","int","string",
            "4","string","int","int","int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 5:
            attributes=args[4].(string)
        }
        x:=args[1].(int) ; y:=args[2].(int) ; sz:=args[3].(int)
        svg_handles[handle].svg.Square(x,y,sz,attributes)
        return nil,nil
    }

    // Grid(x int, y int, w int, h int, n int, s ...string)
    slhelp["svg_grid"] = LibHelp{in: "handle,x,y,w,h,n[,attributes]", out: "", action: "Draws a grid on the canvas [#i1]handle[#i0]."}
    stdlib["svg_grid"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_grid",args,2,
            "7","string","int","int","int","int","int","string",
            "6","string","int","int","int","int","int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 7:
            attributes=args[6].(string)
        }
        x:=args[1].(int) ; y:=args[2].(int)
        w:=args[3].(int) ; h:=args[4].(int) ; n:=args[5].(int)
        svg_handles[handle].svg.Grid(x,y,w,h,n,attributes)
        return nil,nil
    }

    // Ellipse(x int, y int, w int, h int, s ...string)
    slhelp["svg_ellipse"] = LibHelp{in: "handle,x,y,w,h[,attributes]", out: "", action: "Draws an ellipse on the canvas [#i1]handle[#i0]."}
    stdlib["svg_ellipse"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_ellipse",args,2,
            "6","string","int","int","int","int","string",
            "5","string","int","int","int","int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 6:
            attributes=args[5].(string)
        }
        x:=args[1].(int) ; y:=args[2].(int) ; w:=args[3].(int); h:=args[4].(int)
        svg_handles[handle].svg.Ellipse(x,y,w,h,attributes)
        return nil,nil
    }

    slhelp["svg_plot"] = LibHelp{in: "handle,x,y[,attributes]", out: "", action: "Plots a point on the canvas [#i1]handle[#i0]."}
    stdlib["svg_plot"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_plot",args,2,
            "4","string","int","int","string",
            "3","string","int","int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 4:
            attributes=args[3].(string)
        }
        x:=args[1].(int) ; y:=args[2].(int)
        svg_handles[handle].svg.Circle(x,y,1,attributes)
        return nil,nil
    }

    // Text(x int, y int, t string, s ...string)
    slhelp["svg_text"] = LibHelp{in: "handle,x,y,text[,attributes]", out: "", action: "Writes text to the SVG canvas [#i1]handle[#i0]."}
    stdlib["svg_text"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_text",args,2,
            "5","string","int","int","string","string",
            "4","string","int","int","string"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 5:
            attributes=args[4].(string)
        }
        x:=args[1].(int) ; y:=args[2].(int) ; t:=args[3].(string)
        svg_handles[handle].svg.Text(x,y,t,attributes)
        return nil,nil
    }


    // Image(x int, y int, w int, h int, link string, s ...string)
    slhelp["svg_image"] = LibHelp{in: "handle,x,y,w,h,image_link[,attributes]", out: "", action: "Links an image to the SVG canvas [#i1]handle[#i0]."}
    stdlib["svg_image"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_image",args,2,
            "7","string","int","int","int","int","string","string",
            "6","string","int","int","int","int","string"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 7:
            attributes=args[6].(string)
        }
        x:=args[1].(int) ; y:=args[2].(int)
        w:=args[3].(int) ; h:=args[4].(int)
        im:=args[5].(string)
        svg_handles[handle].svg.Image(x,y,w,h,im,attributes)
        return nil,nil
    }


    slhelp["svg_polygon"] = LibHelp{in: "handle,[]x,[]y[,attributes]", out: "", action: "Draws a polygon on the canvas [#i1]handle[#i0]."}
    stdlib["svg_polygon"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_polygon",args,2,
            "4","string","[]int","[]int","string",
            "3","string","[]int","[]int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 4:
            attributes=args[3].(string)
        }
        ax:=args[1].([]int) ; ay:=args[2].([]int)
        svg_handles[handle].svg.Polygon(ax,ay,attributes)
        return nil,nil
    }


    // Polyline(x []int, y []int, s ...string)
    slhelp["svg_polyline"] = LibHelp{in: "handle,[]x,[]y[,attributes]", out: "", action: "Draws a polyline on the canvas [#i1]handle[#i0]."}
    stdlib["svg_polyline"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_polyline",args,2,
            "4","string","[]int","[]int","string",
            "3","string","[]int","[]int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 4:
            attributes=args[3].(string)
        }
        ax:=args[1].([]int) ; ay:=args[2].([]int)
        svg_handles[handle].svg.Polyline(ax,ay,attributes)
        return nil,nil
    }


    // Rect(x int, y int, w int, h int, s ...string)
    slhelp["svg_rect"] = LibHelp{in: "handle,x,y,w,h[,attributes]", out: "", action: "Draws a rectangle on the canvas [#i1]handle[#i0]."}
    stdlib["svg_rect"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_rect",args,2,
            "6","string","int","int","int","int","string",
            "5","string","int","int","int","int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 6:
            attributes=args[5].(string)
        }
        x:=args[1].(int) ; y:=args[2].(int)
        w:=args[3].(int) ; h:=args[4].(int)
        svg_handles[handle].svg.Rect(x,y,w,h,attributes)
        return nil,nil
    }

    // Roundrect(x int, y int, w int, h int, rx int, ry int, s ...string)
    slhelp["svg_roundrect"] = LibHelp{in: "handle,x,y,w,h,rx,ry[,attributes]", out: "", action: "Draws a rounded rectangle on the canvas [#i1]handle[#i0]."}
    stdlib["svg_roundrect"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_roundrect",args,2,
            "8","string","int","int","int","int","int","int","string",
            "7","string","int","int","int","int","int","int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 8:
            attributes=args[7].(string)
        }
        x:=args[1].(int) ; y:=args[2].(int)
        w:=args[3].(int) ; h:=args[4].(int)
        rx:=args[5].(int) ; ry:=args[6].(int)

        svg_handles[handle].svg.Roundrect(x,y,w,h,rx,ry,attributes)
        return nil,nil
    }


    slhelp["svg_line"] = LibHelp{in: "handle,x,y,w,h[,attributes]", out: "", action: "Draws a line on the canvas [#i1]handle[#i0]."}
    stdlib["svg_line"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_line",args,2,
            "6","string","int","int","int","int","string",
            "5","string","int","int","int","int"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        switch len(args) {
        case 6:
            attributes=args[5].(string)
        }
        x1:=args[1].(int) ; y1:=args[2].(int)
        x2:=args[3].(int) ; y2:=args[4].(int)
        svg_handles[handle].svg.Line(x1,y1,x2,y2,attributes)
        return nil,nil
    }

    slhelp["svg_title"] = LibHelp{in: "handle,title", out: "", action: "Places a title on the canvas [#i1]handle[#i0]."}
    stdlib["svg_title"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_title",args,1,"2","string","string"); !ok { return nil,err }
        handle:=args[0].(string)
        title:=args[1].(string)
        svg_handles[handle].svg.Title(title)
        return nil,nil
    }

    slhelp["svg_desc"] = LibHelp{in: "handle,description", out: "", action: "Attach a description to the canvas [#i1]handle[#i0]."}
    stdlib["svg_desc"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_desc",args,1,"2","string","string"); !ok { return nil,err }
        handle:=args[0].(string)
        desc:=args[1].(string)
        svg_handles[handle].svg.Desc(desc)
        return nil,nil
    }


    slhelp["svg_def"] = LibHelp{in: "handle", out: "", action: "Attach a definition to the canvas [#i1]handle[#i0]."}
    stdlib["svg_def"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_def",args,1,"1","string"); !ok { return nil,err }
        handle:=args[0].(string)
        svg_handles[handle].svg.Def()
        return nil,nil
    }

    slhelp["svg_def_end"] = LibHelp{in: "handle", out: "", action: "Ends a definition in [#i1]handle[#i0]."}
    stdlib["svg_def_end"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_def_end",args,1,"1","string"); !ok { return nil,err }
        handle:=args[0].(string)
        svg_handles[handle].svg.DefEnd()
        return nil,nil
    }

    slhelp["svg_group"] = LibHelp{in: "handle[,attributes]", out: "", action: "Attach a group definition to the canvas [#i1]handle[#i0]."}
    stdlib["svg_group"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_group",args,2,
            "2","string","string",
            "1","string"); !ok { return nil,err }
        handle:=args[0].(string)
        attributes:=""
        if len(args)==2 { attributes=args[1].(string) }
        svg_handles[handle].svg.Group(attributes)
        return nil,nil
    }

    slhelp["svg_group_end"] = LibHelp{in: "handle", out: "", action: "Ends a group definition in [#i1]handle[#i0]."}
    stdlib["svg_group_end"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_group_end",args,1,"1","string"); !ok { return nil,err }
        handle:=args[0].(string)
        svg_handles[handle].svg.Gend()
        return nil,nil
    }

    // Link(href string, title string)
    slhelp["svg_link"] = LibHelp{in: "handle,href,title", out: "", action: "Places a link on the canvas [#i1]handle[#i0]."}
    stdlib["svg_link"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_link",args,1,"3","string","string","string"); !ok { return nil,err }
        handle:=args[0].(string)
        href:=args[1].(string)
        title:=args[2].(string)
        svg_handles[handle].svg.Link(href,title)
        return nil,nil
    }

    // LinkEnd()
    slhelp["svg_link_end"] = LibHelp{in: "handle", out: "", action: "Ends a link definition in [#i1]handle[#i0]."}
    stdlib["svg_link_end"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_link_end",args,1,"1","string"); !ok { return nil,err }
        handle:=args[0].(string)
        svg_handles[handle].svg.LinkEnd()
        return nil,nil
    }


    slhelp["svg_start"] = LibHelp{in: "filename,w,h[,attributes]", out: "svg_handle", action: "Returns a handle to a new SVG canvas. You must use this call to initiate the image creation process.\nReturns an empty string on failure or a handle name on success."}
    stdlib["svg_start"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_start",args,2,
            "4","string","int","int","string",
            "3","string","int","int"); !ok { return nil,err }

        attributes:=""
        if len(args)==4 { attributes=args[3].(string) }

        filename:=args[0].(string)
        hnd:=svgCreate(filename)
        if hnd=="" {
            return nil,errors.New("Could not create the SVG file.")
        }
        svg_handles[hnd].svg.Start(args[1].(int),args[2].(int),attributes)
        return hnd,nil
    }

    slhelp["svg_end"] = LibHelp{in: "handle", out: "", action: "Signals completion of the SVG canvas [#i1]handle[#i0]."}
    stdlib["svg_end"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("svg_end",args,1,"1","string"); !ok { return nil,err }
        hnd:=args[0].(string)
        svg_handles[hnd].svg.End()
        svgClose(hnd)
        return nil,nil
    }

}

