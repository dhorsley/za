//+build !test

package main

import (
    "os"
    "errors"
    "github.com/ajstarks/svgo"
    "math/rand"
    "sync"
)


// This package includes functions for interfacing with SVG components.

// https://github.com/ajstarks/svgo

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

/*
type graph_entry struct {
    svg         string      // * handle of parent SVG
    style       string      // passed through to the title rect from graph
    builder     string      // current SVG of the graph. will be shoved into svg at end as a group
    minx        int         // * lowest point on x scale
    miny        int         // * lowest point on y scale
    maxx        int         // * etc, etc..
    maxy        int         // *
    insetx      int         // 1.
    insety      int         // 2. how far from the bottom-left do ox,oy begin
    ox          int         // * 1.
    oy          int         // * 2. 
    pw          int         // * 3.
    ph          int         // * 4. these 4 fields locate the bounding area within the svg that will host the graph.
    logx        bool        // is x-scale logarithmic?
    logy        bool        // is y-scale logarithmic?
    title       bool
    divmarks    bool        // disable/enable division markers
    // not all of these will be needed. we just want state that may be read
    // by the complementary functions.
}

var graph_handles = make ( map[string]graph_entry )
var graphlock = &sync.RWMutex{}

func graphCreateHandle() string {
    var gid string
    for ;; {
        b := make([]byte, 16)
        rand.Read(b)
        gid = sf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
        if _,exists:=graph_handles[gid]; !exists { break }
    }
    return gid
}
*/

//
// N.B.:
//
// svg not thread-safe yet.
//

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
                                   // "graph","graph_output"


    // @todo: "svg_linear_gradient","svg_radial_gradient",
    //      : requires an offcol container for last param.

/*
    // set background colour, borders, etc with 'style'
    slhelp["graph"] = LibHelp{in: "handle,ox,oy,w,h[,attr]", out: "graph_handle", action: "Sets up a new graph."}
    stdlib["graph"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return nil,nil
        if len(args)!=6 {
            return nil, errors.New("Bad arguments supplied to graph()")
        }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
            sf("%T",args[3])!="int" || sf("%T",args[4])!="int" || sf("%T",args[5])!="string" {
            return nil, errors.New("Bad arguments supplied to graph()")
        }

        svgid:=args[0].(string); ox:=args[1].(int); oy:=args[2].(int)
        w:=args[3].(int); h:=args[4].(int); attr:=args[5].(string)

        graphlock.Lock()
        gid:=graphCreateHandle()
        builder:=sf("<g>\n")
        builder+=sf(` <rect x="%v" y="%v" width="%v" height="%v" style="%v" />\n`,ox,oy,w,h,attr)
        graph_handles[gid]=graph_entry{svg:svgid,insetx:8,insety:8,ox:ox,oy:oy,pw:w,ph:h,style:attr,builder:builder}
        graphlock.Unlock()
        return gid,nil
    }

    slhelp["graph_output"] = LibHelp{in: "handle", out: "", action: "Sends the graph to the canvas."}
    stdlib["graph_output"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        return nil,nil
        if len(args)!=1 {
            return nil, errors.New("Bad arguments supplied to graph_output()")
        }
        if sf("%T",args[0])!="string" {
            return nil, errors.New("Bad arguments supplied to graph_output()")
        }

        gid:=args[0].(string)

        // build+end_group
        graphlock.Lock()
        graph:=graph_handles[gid]
        graph.builder+="</g>\n"
        // graph.svg.writer.printf(builder)
        svglock.Lock()
        fmt.Fprintf(svg_handles[graph.svg].svg.Writer,graph.builder)
        svglock.Unlock()
        // dispose of graph handle
        delete(graph_handles,gid)
        graphlock.Unlock()
        return nil,nil
}
*/

    slhelp["svg_circle"] = LibHelp{in: "handle,x,y,radius[,attributes]", out: "", action: "Draws a circle on the canvas [#i1]handle[#i0]."}
    stdlib["svg_circle"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments supplied to svg_circle()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_circle()")
        }
        attributes:=""
        switch len(args) {
        case 4:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" || sf("%T",args[3])!="int" {
                return nil, errors.New("Bad arguments supplied to svg_circle()")
            }
        case 5:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" || sf("%T",args[3])!="int" || sf("%T",args[4])!="string" {
                return nil, errors.New("Bad arguments supplied to svg_circle()")
            }
            attributes=args[4].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_circle()")
        }
        x:=args[1].(int) ; y:=args[2].(int) ; r:=args[3].(int)
        svg_handles[handle].svg.Circle(x,y,r,attributes)
        return nil,nil
    }

    // Square(x int, y int, s int, style ...string)
    slhelp["svg_square"] = LibHelp{in: "handle,x,y,size[,attributes]", out: "", action: "Draws a square on the canvas [#i1]handle[#i0]."}
    stdlib["svg_square"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments (count) supplied to svg_square()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments (handle) supplied to svg_square()")
        }
        attributes:=""
        switch len(args) {
        case 4:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" || sf("%T",args[3])!="int" {
                return nil, errors.New("Bad arguments (types) supplied to svg_square()")
            }
        case 5:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" || sf("%T",args[3])!="int" || sf("%T",args[4])!="string" {
                return nil, errors.New("Bad arguments (types) supplied to svg_square()")
            }
            attributes=args[4].(string)
        default:
            return nil, errors.New("Bad arguments (count) supplied to svg_square()")
        }
        x:=args[1].(int) ; y:=args[2].(int) ; sz:=args[3].(int)
        svg_handles[handle].svg.Square(x,y,sz,attributes)
        return nil,nil
    }

    // Grid(x int, y int, w int, h int, n int, s ...string)
    slhelp["svg_grid"] = LibHelp{in: "handle,x,y,w,h,n[,attributes]", out: "", action: "Draws a grid on the canvas [#i1]handle[#i0]."}
    stdlib["svg_grid"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments (count) supplied to svg_grid()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments (handle) supplied to svg_grid()")
        }
        attributes:=""
        switch len(args) {
        case 6:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" || sf("%T",args[3])!="int" ||
                sf("%T",args[4])!="int" || sf("%T",args[5])!="int" {
                return nil, errors.New("Bad arguments (types) supplied to svg_grid()")
            }
        case 7:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" || sf("%T",args[3])!="int" ||
                sf("%T",args[4])!="int" || sf("%T",args[5])!="int" || sf("%T",args[6])!="string" {
                return nil, errors.New("Bad arguments (types) supplied to svg_grid()")
            }
            attributes=args[6].(string)
        default:
            return nil, errors.New("Bad arguments (count) supplied to svg_grid()")
        }
        x:=args[1].(int) ; y:=args[2].(int)
        w:=args[3].(int) ; h:=args[4].(int) ; n:=args[5].(int)
        svg_handles[handle].svg.Grid(x,y,w,h,n,attributes)
        return nil,nil
    }

    slhelp["svg_ellipse"] = LibHelp{in: "handle,x,y,w,h[,attributes]", out: "", action: "Draws an ellipse on the canvas [#i1]handle[#i0]."}
    stdlib["svg_ellipse"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        // Ellipse(x int, y int, w int, h int, s ...string)
        if len(args)==0 { return nil, errors.New("Bad arguments supplied to svg_ellipse()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_ellipse()")
        }
        attributes:=""
        switch len(args) {
        case 5:
            if  sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" {
                return nil, errors.New("Bad arguments supplied to svg_ellipse()")
            }
        case 6:
            if  sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" ||
                sf("%T",args[5])!="string" {
                return nil, errors.New("Bad arguments supplied to svg_ellipse()")
            }
            attributes=args[5].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_ellipse()")
        }
        x:=args[1].(int) ; y:=args[2].(int) ; w:=args[3].(int); h:=args[4].(int)
        svg_handles[handle].svg.Ellipse(x,y,w,h,attributes)
        return nil,nil
    }

    slhelp["svg_plot"] = LibHelp{in: "handle,x,y[,attributes]", out: "", action: "Plots a point on the canvas [#i1]handle[#i0]."}
    stdlib["svg_plot"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments (count) supplied to svg_plot()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments (handle) supplied to svg_plot()")
        }
        attributes:=""
        switch len(args) {
        case 3:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" {
                return nil, errors.New("Bad arguments (types) supplied to svg_plot()")
            }
        case 4:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" || sf("%T",args[3])!="string" {
                return nil, errors.New("Bad arguments (types) supplied to svg_plot()")
            }
            attributes=args[3].(string)
        default:
            return nil, errors.New("Bad arguments (count) supplied to svg_plot()")
        }
        x:=args[1].(int) ; y:=args[2].(int)
        svg_handles[handle].svg.Circle(x,y,1,attributes)
        return nil,nil
    }

    // Text(x int, y int, t string, s ...string)
    slhelp["svg_text"] = LibHelp{in: "handle,x,y,text[,attributes]", out: "", action: "Writes text to the SVG canvas [#i1]handle[#i0]."}
    stdlib["svg_text"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments (count) supplied to svg_text()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments (handle) supplied to svg_text()")
        }
        attributes:=""
        switch len(args) {
        case 4:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" || sf("%T",args[3])!="string" {
                return nil, errors.New("Bad arguments (types) supplied to svg_text()")
            }
        case 5:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="string" || sf("%T",args[4])!="string" {
                return nil, errors.New("Bad arguments (types) supplied to svg_text()")
            }
            attributes=args[4].(string)
        default:
            return nil, errors.New("Bad arguments (count) supplied to svg_text()")
        }
        x:=args[1].(int) ; y:=args[2].(int) ; t:=args[3].(string)
        svg_handles[handle].svg.Text(x,y,t,attributes)
        return nil,nil
    }


    // Image(x int, y int, w int, h int, link string, s ...string)
    slhelp["svg_image"] = LibHelp{in: "handle,x,y,w,h,image_link[,attributes]", out: "", action: "Links an image to the SVG canvas [#i1]handle[#i0]."}
    stdlib["svg_image"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments (count) supplied to svg_image()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments (handle) supplied to svg_image()")
        }
        attributes:=""
        switch len(args) {
        case 6:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" ||
                sf("%T",args[5])!="string" {
                return nil, errors.New("Bad arguments (types) supplied to svg_image()")
            }
        case 7:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" ||
                sf("%T",args[5])!="string" || sf("%T",args[6])!="string" {
                return nil, errors.New("Bad arguments (types) supplied to svg_image()")
            }
            attributes=args[6].(string)
        default:
            return nil, errors.New("Bad arguments (count) supplied to svg_image()")
        }
        x:=args[1].(int) ; y:=args[2].(int)
        w:=args[3].(int) ; h:=args[4].(int)
        im:=args[5].(string)
        svg_handles[handle].svg.Image(x,y,w,h,im,attributes)
        return nil,nil
    }


    slhelp["svg_polygon"] = LibHelp{in: "handle,[]x,[]y[,attributes]", out: "", action: "Draws a polygon on the canvas [#i1]handle[#i0]."}
    stdlib["svg_polygon"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments supplied to svg_polygon()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_polygon()")
        }
        attributes:=""
        switch len(args) {
        case 3:
            if sf("%T",args[1])!="[]int" || sf("%T",args[2])!="[]int" {
                return nil, errors.New("Bad arguments supplied to svg_polygon()")
            }
        case 4:
            if sf("%T",args[1])!="[]int" || sf("%T",args[2])!="[]int" || sf("%T",args[3])!="string" {
                return nil, errors.New("Bad arguments supplied to svg_polygon()")
            }
            attributes=args[3].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_polygon()")
        }
        ax:=args[1].([]int) ; ay:=args[2].([]int)
        svg_handles[handle].svg.Polygon(ax,ay,attributes)
        return nil,nil
    }


    // Polyline(x []int, y []int, s ...string)
    slhelp["svg_polyline"] = LibHelp{in: "handle,[]x,[]y[,attributes]", out: "", action: "Draws a polyline on the canvas [#i1]handle[#i0]."}
    stdlib["svg_polyline"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments supplied to svg_polyline()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_polyline()")
        }
        attributes:=""
        switch len(args) {
        case 3:
            if sf("%T",args[1])!="[]int" || sf("%T",args[2])!="[]int" {
                return nil, errors.New("Bad arguments supplied to svg_polyline()")
            }
        case 4:
            if sf("%T",args[1])!="[]int" || sf("%T",args[2])!="[]int" || sf("%T",args[3])!="string" {
                return nil, errors.New("Bad arguments supplied to svg_polyline()")
            }
            attributes=args[3].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_polyline()")
        }
        ax:=args[1].([]int) ; ay:=args[2].([]int)
        svg_handles[handle].svg.Polyline(ax,ay,attributes)
        return nil,nil
    }


    // Rect(x int, y int, w int, h int, s ...string)
    slhelp["svg_rect"] = LibHelp{in: "handle,x,y,w,h[,attributes]", out: "", action: "Draws a rectangle on the canvas [#i1]handle[#i0]."}
    stdlib["svg_rect"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments supplied to svg_rect()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_rect()")
        }
        attributes:=""
        switch len(args) {
        case 5:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" {
                return nil, errors.New("Bad arguments supplied to svg_rect()")
            }
        case 6:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" ||
                sf("%T",args[5])!="string" {
                return nil, errors.New("Bad arguments supplied to svg_rect()")
            }
            attributes=args[5].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_rect()")
        }
        x:=args[1].(int) ; y:=args[2].(int)
        w:=args[3].(int) ; h:=args[4].(int)
        svg_handles[handle].svg.Rect(x,y,w,h,attributes)
        return nil,nil
    }

    // Roundrect(x int, y int, w int, h int, rx int, ry int, s ...string)
    slhelp["svg_roundrect"] = LibHelp{in: "handle,x,y,w,h,rx,ry[,attributes]", out: "", action: "Draws a rounded rectangle on the canvas [#i1]handle[#i0]."}
    stdlib["svg_roundrect"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments (count) supplied to svg_roundrect()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments (handle) supplied to svg_roundrect()")
        }
        attributes:=""
        switch len(args) {
        case 7:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" ||
                sf("%T",args[5])!="int" || sf("%T",args[6])!="int" {
                return nil, errors.New("Bad arguments (types) supplied to svg_roundrect()")
            }
        case 8:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" ||
                sf("%T",args[5])!="int" || sf("%T",args[6])!="int" ||
                sf("%T",args[7])!="string" {
                return nil, errors.New("Bad arguments (types) supplied to svg_roundrect()")
            }
            attributes=args[7].(string)
        default:
            return nil, errors.New("Bad arguments (count) supplied to svg_roundrect()")
        }
        x:=args[1].(int) ; y:=args[2].(int)
        w:=args[3].(int) ; h:=args[4].(int)
        rx:=args[5].(int) ; ry:=args[6].(int)

        svg_handles[handle].svg.Roundrect(x,y,w,h,rx,ry,attributes)
        return nil,nil
    }


    slhelp["svg_line"] = LibHelp{in: "handle,x,y,w,h[,attributes]", out: "", action: "Draws a line on the canvas [#i1]handle[#i0]."}
    stdlib["svg_line"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments supplied to svg_line()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_line()")
        }
        attributes:=""
        switch len(args) {
        case 5:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" {
                return nil, errors.New("Bad arguments supplied to svg_line()")
            }
        case 6:
            if sf("%T",args[1])!="int" || sf("%T",args[2])!="int" ||
                sf("%T",args[3])!="int" || sf("%T",args[4])!="int" ||
                sf("%T",args[5])!="string" {
                return nil, errors.New("Bad arguments supplied to svg_line()")
            }
            attributes=args[5].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_line()")
        }
        x1:=args[1].(int) ; y1:=args[2].(int)
        x2:=args[3].(int) ; y2:=args[4].(int)
        svg_handles[handle].svg.Line(x1,y1,x2,y2,attributes)
        return nil,nil
    }

    slhelp["svg_title"] = LibHelp{in: "handle,title", out: "", action: "Places a title on the canvas [#i1]handle[#i0]."}
    stdlib["svg_title"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments supplied to svg_title()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_title()")
        }
        title:=""
        switch len(args) {
        case 2:
            if sf("%T",args[1])!="string" {
                return nil, errors.New("Bad arguments supplied to svg_title()")
            }
            title=args[1].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_title()")
        }
        svg_handles[handle].svg.Title(title)
        return nil,nil
    }

    slhelp["svg_desc"] = LibHelp{in: "handle,description", out: "", action: "Attach a description to the canvas [#i1]handle[#i0]."}
    stdlib["svg_desc"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments supplied to svg_desc()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_desc()")
        }
        desc:=""
        switch len(args) {
        case 2:
            if sf("%T",args[1])!="string" {
                return nil, errors.New("Bad arguments supplied to svg_desc()")
            }
            desc=args[1].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_desc()")
        }
        svg_handles[handle].svg.Desc(desc)
        return nil,nil
    }


    slhelp["svg_def"] = LibHelp{in: "handle", out: "", action: "Attach a definition to the canvas [#i1]handle[#i0]."}
    stdlib["svg_def"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return nil, errors.New("Bad arguments supplied to svg_def()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_def()")
        }
        svg_handles[handle].svg.Def()
        return nil,nil
    }

    slhelp["svg_def_end"] = LibHelp{in: "handle", out: "", action: "Ends a definition in [#i1]handle[#i0]."}
    stdlib["svg_def_end"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return nil, errors.New("Bad arguments supplied to svg_def_end()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_def_end()")
        }
        svg_handles[handle].svg.DefEnd()
        return nil,nil
    }

    slhelp["svg_group"] = LibHelp{in: "handle[,attributes]", out: "", action: "Attach a group definition to the canvas [#i1]handle[#i0]."}
    stdlib["svg_group"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 || len(args)>2 { return nil, errors.New("Bad arguments supplied to svg_group()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_group()")
        }
        attributes:=""
        if len(args)==2 && sf("%T",args[1])=="string" {
            attributes=args[1].(string)
        }
        svg_handles[handle].svg.Group(attributes)
        return nil,nil
    }

    slhelp["svg_group_end"] = LibHelp{in: "handle", out: "", action: "Ends a group definition in [#i1]handle[#i0]."}
    stdlib["svg_group_end"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return nil, errors.New("Bad arguments supplied to svg_group_end()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments supplied to svg_group_end()")
        }
        svg_handles[handle].svg.Gend()
        return nil,nil
    }

    // Link(href string, title string)
    slhelp["svg_link"] = LibHelp{in: "handle,href,title", out: "", action: "Places a link on the canvas [#i1]handle[#i0]."}
    stdlib["svg_link"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)==0 { return nil, errors.New("Bad arguments (count) supplied to svg_link()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments (handle) supplied to svg_link()")
        }
        href:=""
        title:=""
        switch len(args) {
        case 3:
            if sf("%T",args[1])!="string" || sf("%T",args[2])!="string" {
                return nil, errors.New("Bad arguments (types) supplied to svg_link()")
            }
            href=args[1].(string)
            title=args[2].(string)
        default:
            return nil, errors.New("Bad arguments (count) supplied to svg_link()")
        }
        svg_handles[handle].svg.Link(href,title)
        return nil,nil
    }

    // LinkEnd()
    slhelp["svg_link_end"] = LibHelp{in: "handle", out: "", action: "Ends a link definition in [#i1]handle[#i0]."}
    stdlib["svg_link_end"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 { return nil, errors.New("Bad arguments (count) supplied to svg_link_end()") }
        var handle string
        switch args[0].(type) {
        case string:
            handle=args[0].(string)
        default:
            return nil, errors.New("Bad arguments (handle) supplied to svg_link_end()")
        }
        svg_handles[handle].svg.LinkEnd()
        return nil,nil
    }

// @todo: finish these, and others...
// LinearGradient(id string, x1, y1, x2, y2 uint8, sc []Offcolor)
// RadialGradient(id string, cx, cy, r, fx, fy uint8, sc []Offcolor)


    slhelp["svg_start"] = LibHelp{in: "filename,w,h[,attributes]", out: "svg_handle", action: "Returns a handle to a new SVG canvas."}
    stdlib["svg_start"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {

        if len(args)<3 || len(args)>4 {
            return nil, errors.New("Bad arguments supplied to svg_start()")
        }
        if sf("%T",args[0])!="string" || sf("%T",args[1])!="int" || sf("%T",args[2])!="int" {
            return nil, errors.New("Bad arguments supplied to svg_start()")
        }
        attributes:=""
        if len(args)==4 && sf("%T",args[3])=="string" {
            attributes=args[3].(string)
        }

        filename:=args[0].(string)
        hnd:=svgCreate(filename)
        if hnd=="" {
            return nil,errors.New("Could not create the SVG file.")
        }
        svg_handles[hnd].svg.Start(args[1].(int),args[2].(int),attributes)
        return hnd,nil
    }

    slhelp["svg_end"] = LibHelp{in: "handle", out: "", action: "Signals completion of the SVG canvas [#i1]handle[#i0]."}
    stdlib["svg_end"] = func(evalfs uint64,args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 {
            return nil, errors.New("Bad arguments supplied to svg_end()")
        }
        switch args[0].(type) {
        case string:
            hnd:=args[0].(string)
            svg_handles[hnd].svg.End()
            svgClose(hnd)
        default:
            // pf("Closing : %T/%v\n",args[0],args[0])
        }
        return nil,nil
    }

}
