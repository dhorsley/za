// +build !noui
// +build windows linux freebsd

package main

import (
        "errors"
        "fmt"
        "github.com/faiface/pixel"
        "github.com/faiface/pixel/pixelgl"
        "github.com/faiface/pixel/text"
        "github.com/faiface/pixel/imdraw"
        "image/color"
        "golang.org/x/image/colornames"
        "math/rand"
        "image"
        "os"
        _ "image/png"
       )

type win_table_entry struct {
    winHandle       *pixelgl.Window
    batch           *pixel.Batch
}

var winHandles = make ( map[string]win_table_entry )

func loadPicture(path string) (pixel.Picture, error) {
    file, err := os.Open(path)
        if err != nil {
            return nil, err
        }
    defer file.Close()
        img, _, err := image.Decode(file)
        if err != nil {
            return nil, err
        }
    return pixel.PictureDataFromImage(img), nil
}


func winCreateHandle() string {
    var id string
        for ;; {
b := make([]byte, 16)
       rand.Read(b)
       id = sf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
       if _,exists:=winHandles[id]; !exists { break }
        }
    return id
}

func cons_text(w *pixelgl.Window,x float64, y float64, atlas *text.Atlas, c color.RGBA, t string) {
    txt_writer := text.New(pixel.V(x, y), atlas)
    txt_writer.Color=c
    fmt.Fprintf(txt_writer, t)
    pf("%v\n",t)
    return
}

func ui_text(w *pixelgl.Window,x float64, y float64, atlas *text.Atlas, c color.RGBA, t string) {
    txt_writer := text.New(pixel.V(x, y), atlas)
    txt_writer.Color=c
    txt_writer.WriteString(t)
    txt_writer.Draw(w, pixel.IM)
    return
}


func buildUILib() {

    features["ui"] = Feature{version: 1, category: "ui"}
    categories["ui"] = []string{"ui_init", "ui_close", "ui_clear","ui_closed",
        "ui_handle", "ui_title", "ui_update", "ui_text", "ui_centre",
        "ui_polygon","ui_circle", "ui_circle_arc", "ui_line", "ui_bounds","ui_colour",
        "ui_rectangle","ui_new_draw","ui_draw_reset",
        "ui_pp","ui_batch_draw","ui_batch","ui_batch_clear",
        "ui_set_smooth", "pic_load", "ui_new_sprite","pic_bounds","ui_sprite_draw",
        "ui_new_matrix","ui_mat_move","ui_mat_rotate","ui_mat_scale","ui_new_vector",
        "ui_get_code","ui_just_released","ui_pressed","ui_just_pressed",
        "ui_mouse_pos","ui_cursor_visible",
        "ui_primary_monitor","ui_set_windowed","ui_set_full_screen","ui_get_monitors",
        "ui_h","ui_w",
    }

	slhelp["ui_get_monitors"] = LibHelp{in: "", out: "[]monitor_info", action: "retrieve a list of available monitors"}
	stdlib["ui_get_monitors"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_get_monitors",args,0); !ok { return nil,err }
        return pixelgl.Monitors(),nil
    }

	slhelp["ui_primary_monitor"] = LibHelp{in: "", out: "monitor_info", action: "retrieve data about the primary monitor"}
	stdlib["ui_primary_monitor"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_primary_monitor",args,0); !ok { return nil,err }
        return pixelgl.PrimaryMonitor(),nil
    }

	slhelp["ui_h"] = LibHelp{in: "[window_handle]", out: "float", action: "retrieve max height of window or the current monitor in [#i1]window_handle[#i0] is not set."}
	stdlib["ui_h"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_h",args,2,
            "1","string",
            "0"); !ok { return nil,err }
        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if len(args)>0 {
                h:=args[0].(string)
                if _,there:=winHandles[h]; there {
                    w:=winHandles[h].winHandle
                    return w.Bounds().Max.Y,nil
                }
            }
            _,height:=pixelgl.PrimaryMonitor().Size()
            return height,nil
        }
        return nil,errors.New("window not available in ui_h()")
    }

	slhelp["ui_w"] = LibHelp{in: "[window_handle]", out: "float", action: "retrieve max width of window or the current monitor in [#i1]window_handle[#i0] is not set."}
	stdlib["ui_w"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_w",args,2,
            "1","string",
            "0"); !ok { return nil,err }
        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if len(args)>0 {
                h:=args[0].(string)
                if _,there:=winHandles[h]; there {
                    w:=winHandles[h].winHandle
                    return w.Bounds().Max.X,nil
                }
            }
            width,_:=pixelgl.PrimaryMonitor().Size()
            return width,nil
        }
        return nil,errors.New("window not available in ui_w()")
    }

	slhelp["ui_set_full_screen"] = LibHelp{in: "win_handle,monitor_id", out: "", action: "set a window to display on monitor [#i1]monitor_id[#i0] as full-screen."}
	stdlib["ui_set_full_screen"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_set_full_screen",args,1,"2","string","*pixelgl.Monitor"); !ok { return nil,err }

        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            h:=args[0].(string)
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                w.SetMonitor(args[1].(*pixelgl.Monitor))
            }
        }
        return nil,nil
    }

	slhelp["ui_set_windowed"] = LibHelp{in: "win_handle", out: "bool_success", action: "return a window from full-screen mode to normal windowed display."}
	stdlib["ui_set_windowed"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_set_windowed",args,1,"1","string"); !ok { return false,err }

        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            h:=args[0].(string)
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                w.SetMonitor(nil)
                return true,nil
            }
        }
        return false,nil
    }

	slhelp["ui_init"] = LibHelp{in: "[float_w,float_h]", out: "handle", action: "open a new window"}
	stdlib["ui_init"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_init",args,2,
            "2","float64","float64",
            "0"); !ok { return nil,err }

        width:=512.0; height:=384.0
        switch len(args) {
        case 2:
            width=args[0].(float64)
            height=args[1].(float64)
        }

        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {

            default_win_cfg := pixelgl.WindowConfig{
                Title       : "Za Output",
                Bounds      : pixel.R(0, 0, width, height),
                VSync       : false,
                Invisible   : true,
            }
            w, err := pixelgl.NewWindow(default_win_cfg)
            if err == nil {
                w.Show()
                h:=winCreateHandle()
                winHandles[h]=win_table_entry{winHandle:w,
                    batch:pixel.NewBatch(&pixel.TrianglesData{}, nil),
                }
                return h,nil
            }
        }
		return nil, errors.New("Windowing system not available")
	}


	slhelp["ui_close"] = LibHelp{in: "handle", out: "bool_success", action: "Closes a UI."}
	stdlib["ui_close"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_close",args,1,"1","string"); !ok { return false,err }
        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            h:=args[0].(string)
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                delete(winHandles,h)
                w.Destroy()
                return true,nil
            } else {
                return false,nil
            }
        }
		return false, errors.New("Windowing system not available")

	}

	slhelp["ui_closed"] = LibHelp{in: "handle", out: "bool", action: "Window close requested?"}
	stdlib["ui_closed"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_closed",args,1,"1","string"); !ok { return nil,err }
        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            h:=args[0].(string)
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                return w.Closed(),nil
            } else {
                return false,nil
            }
        }
		return false, errors.New("Windowing system not available")

	}


	slhelp["ui_get_code"] = LibHelp{in: "string", out: "button", action: "Convert Pixel key name to button code for use in events."}
	stdlib["ui_get_code"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_get_code",args,1,"1","string"); !ok { return nil,err }
        globlock.RLock()
        defer globlock.RUnlock()
        if b,there:=buttons[args[0].(string)]; there {
            return b,nil
        }
        return nil,errors.New("invalid button code in ui_get_code()")
    }

	slhelp["ui_cursor_visible"] = LibHelp{in: "id,bool", out: "bool_success", action: "Set cursor visibility in window."}
	stdlib["ui_cursor_visible"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_cursor_visible",args,1,"2","string","bool"); !ok { return false,err }
        globlock.Lock()
        defer globlock.Unlock()
        if w,there:=winHandles[args[0].(string)]; winAvailable && there {
            w.winHandle.SetCursorVisible(args[1].(bool))
            return true,nil
        } else {
            return false,nil
        }
    }

	slhelp["ui_pressed"] = LibHelp{in: "id,button", out: "bool", action: "Has keyboard or mouse key [#i1]button[#i0] been pressed?"}
	stdlib["ui_pressed"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_pressed",args,1,"2","string","pixelgl.Button"); !ok { return false,err }
        globlock.Lock()
        defer globlock.Unlock()
        if w,there:=winHandles[args[0].(string)]; winAvailable && there {
            return w.winHandle.Pressed(args[1].(pixelgl.Button)),nil
        } else {
            return false,nil
        }
    }

	slhelp["ui_just_pressed"] = LibHelp{in: "id,button", out: "bool", action: "Has keyboard or mouse key [#i1]button[#i0] been released?"}
	stdlib["ui_just_pressed"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_just_pressed",args,1,"2","string","pixelgl.Button"); !ok { return false,err }
        globlock.Lock()
        defer globlock.Unlock()
        if w,there:=winHandles[args[0].(string)]; winAvailable && there {
            return w.winHandle.JustPressed(args[1].(pixelgl.Button)),nil
        } else {
            return false,nil
        }
    }

	slhelp["ui_just_released"] = LibHelp{in: "id,button", out: "bool", action: "Has keyboard or mouse key [#i1]button[#i0] just been released?"}
	stdlib["ui_just_released"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_just_released",args,1,"2","string","pixelgl.Button"); !ok { return false,err }
        globlock.Lock()
        defer globlock.Unlock()
        if w,there:=winHandles[args[0].(string)]; winAvailable && there {
            return w.winHandle.JustReleased(args[1].(pixelgl.Button)),nil
        } else {
            return false,nil
        }
    }

	slhelp["ui_mouse_pos"] = LibHelp{in: "id", out: "bool", action: "Returns the mouse pointer position for window [#i1]id[#i0]. Returns a vector nil."}
	stdlib["ui_mouse_pos"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_mouse_pos",args,1,"1","string"); !ok { return false,err }
        globlock.Lock()
        defer globlock.Unlock()
        if w,there:=winHandles[args[0].(string)]; winAvailable && there {
            return w.winHandle.MousePosition(),nil
        } else {
            return nil,nil
        }
    }

	slhelp["ui_handle"] = LibHelp{in: "id", out: "handle", action: "Returns the underlying structure."}
	stdlib["ui_handle"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_handle",args,1,"1","string"); !ok { return nil,err }
        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            switch h:=args[0].(type) {
            case string:
                if w,there:=winHandles[h]; there {
                return w,nil
                } else {
                    return nil,nil
                }
            default:
                // pf("not a window in ui_handle() - %v\n",h)
                return nil,nil
            }
        }
		return nil, errors.New("Windowing system not available")
    }

	slhelp["ui_title"] = LibHelp{in: "id,string", out: "", action: "Sets the window title."}
	stdlib["ui_title"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_title",args,1,"2","string","string"); !ok { return nil,err }

        h:=args[0].(string)
        t:=args[1].(string)

        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                w.SetTitle(t)
                return nil,nil
            } else {
                return nil,nil
            }
        }
		return nil, errors.New("Windowing system not available")
    }

	slhelp["ui_text"] = LibHelp{in: "id,x,y,color_name,string", out: "", action: "Write text to window."}
	stdlib["ui_text"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_text",args,1,
            "5","string","float64","float64","string","string"); !ok { return nil,err }

        h:=args[0].(string)

        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                ui_text(w,args[1].(float64),args[2].(float64),default_atlas,colornames.Map[args[3].(string)],args[4].(string))
                return nil,nil
            } else {
                return nil,nil
            }
        }
		return nil, errors.New("Windowing system not available")
    }

	slhelp["ui_set_smooth"] = LibHelp{in: "id,bool", out: "", action: "Set window transform smoothing."}
	stdlib["ui_set_smooth"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_set_smooth",args,1,"2","string","bool"); !ok { return nil,err }

        h:=args[0].(string)
        b:=args[1].(bool)

        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                w.SetSmooth(b)
            }
            return nil,nil
        }
		return nil, errors.New("Windowing system not available")
    }
	slhelp["ui_colour"] = LibHelp{in: "name|r,g,b", out: "colour", action: "generate a RGB value."}
	stdlib["ui_colour"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_colour",args,2,
            "3","int","int","int",
            "1","string"); !ok { return nil,err }

        var ink color.RGBA

        switch len(args) {
        case 1:
            ink=colornames.Map[args[0].(string)]
        case 3:
            ink=color.RGBA{uint8(args[0].(int)),uint8(args[1].(int)),uint8(args[2].(int)),uint8(255)}
        }

        return ink,nil
    }


	slhelp["ui_bounds"] = LibHelp{in: "window_id", out: "rect", action: "get bounds of window."}
	stdlib["ui_bounds"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_bounds",args,1,"1","string"); !ok { return nil,err }
        h:=args[0].(string)
        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                return w.Bounds(),nil
            }
            return nil,nil
        }
		return nil, errors.New("Windowing system not available")
    }

	slhelp["pic_bounds"] = LibHelp{in: "picture", out: "rect", action: "get bounds of picture."}
	stdlib["pic_bounds"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("pic_bounds",args,1,"1","*pixel.PictureData"); !ok { return nil,err }
        return args[0].(*pixel.PictureData).Bounds(),nil
    }

	slhelp["pic_load"] = LibHelp{in: "filename", out: "picture", action: "Read picture from file."}
	stdlib["pic_load"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("pic_load",args,1,"1","string"); !ok { return nil,err }
        p:=args[0].(string)
	    pic, err := loadPicture(p)
	    if err != nil {
		    return nil,errors.New(sf("could not load picture %s",p))
	    }
        return pic,nil
    }

	slhelp["ui_new_sprite"] = LibHelp{in: "picture[,rect]", out: "sprite", action: "Create new sprite from picture."}
	stdlib["ui_new_sprite"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_new_sprite",args,2,
            "2","*pixel.PictureData","pixel.Rect",
            "1","*pixel.PictureData"); !ok { return nil,err }
        var b pixel.Rect
        switch len(args) {
        case 2:
            b=args[1].(pixel.Rect)
        default:
            b=args[0].(*pixel.PictureData).Bounds()
        }
        return pixel.NewSprite(args[0].(*pixel.PictureData),b),nil
    }

	slhelp["ui_clear"] = LibHelp{in: "id[,colour_string|r,g,b]", out: "", action: "Clear the window."}
	stdlib["ui_clear"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_clear",args,2,
            "2","string","string",
            "4","string","int","int","int"); !ok { return nil,err }

        h:=args[0].(string)

        var col color.RGBA
        if len(args)==2 {
            switch args[1].(type) {
            case string:
                col=colornames.Map[args[1].(string)]
            }
        }

        if len(args)==4 {
            switch args[1].(type) {
            case int:
                r:=uint8(args[1].(int))
                g:=uint8(args[2].(int))
                b:=uint8(args[3].(int))
                col=color.RGBA{r,g,b,0xff}
            default:
                return nil,errors.New("Unknown colour type in ui_clear()")
            }
        }

        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                w.Clear(col)
                return nil,nil
            } else {
                // pf("not a window in ui_clear() - %v\n",h)
                return nil,nil
            }
        }
		return nil, errors.New("Windowing system not available")
    }

	slhelp["ui_new_draw"] = LibHelp{in: "", out: "draw_object", action: "Construct a new immediate mode drawing object."}
	stdlib["ui_new_draw"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_new_draw",args,1,"0"); !ok { return nil,err }
        return imdraw.New(nil),nil
    }

	slhelp["ui_new_matrix"] = LibHelp{in: "", out: "matrix", action: "Construct a new matrix."}
	stdlib["ui_new_matrix"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_new_matrix",args,1,"0"); !ok { return nil,err }
		return pixel.IM,nil
    }

	slhelp["ui_centre"] = LibHelp{in: "rect", out: "vector", action: "Returns vector to centre of rect."}
	stdlib["ui_centre"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_centre",args,1,"1","pixel.Rect"); !ok { return nil,err }
        return args[0].(pixel.Rect).Center(),nil
    }

	slhelp["ui_new_vector"] = LibHelp{in: "[float_x_offset,float_y_offset]", out: "vector", action: "Returns a vector struct. No parameters is a 0,0 vector."}
	stdlib["ui_new_vector"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_new_vector",args,2,
            "2","float64","float64",
            "0"); !ok { return nil,err }
        switch len(args) {
        case 2:
            return pixel.V(args[0].(float64),args[1].(float64)),nil
        }
        return pixel.ZV,nil
    }

	slhelp["ui_mat_move"] = LibHelp{in: "matrix,vector", out: "matrix", action: "Returns new matrix moved by vector."}
	stdlib["ui_mat_move"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_mat_move",args,1,"2","pixel.Matrix","pixel.Vec"); !ok { return nil,err }
        mat:=args[0].(pixel.Matrix)
        vec:=args[1].(pixel.Vec)
        return mat.Moved(vec),nil
    }

	slhelp["ui_mat_scale"] = LibHelp{in: "matrix,around_vector,scale_vector", out: "matrix", action: "Returns new matrix scaled by [#i1]scale_vector[#i0]."}
	stdlib["ui_mat_scale"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_mat_scale",args,1,"3","pixel.Matrix","pixel.Vec","pixel.Vec"); !ok { return nil,err }
        mat:=args[0].(pixel.Matrix)
        vec:=args[1].(pixel.Vec)
        sca:=args[2].(pixel.Vec)
        return mat.ScaledXY(vec,sca),nil
    }

	slhelp["ui_mat_rotate"] = LibHelp{in: "matrix,vector,angle", out: "matrix", action: "Returns new matrix rotated by [#i1]angle[#i0]."}
	stdlib["ui_mat_rotate"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_mat_rotate",args,1,"3","pixel.Matrix","pixel.Vec","float64"); !ok { return nil,err }
        return args[0].(pixel.Matrix).Rotated(args[1].(pixel.Vec),args[2].(float64)),nil
    }

	slhelp["ui_pp"] = LibHelp{in: "draw_object,r,g,b,vx,vy", out: "bool_success", action: "Push a point into a draw object."}
	stdlib["ui_pp"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_pp",args,1,
            "6","string","float64","float64","float64","float64","float64"); !ok { return false,err }
                // @note: should have a (parlock) mutex around this:
        if vi,ok := VarLookup(evalfs,args[0].(string)); ok {
            // vlock.Lock()
	        (*ident)[vi].IValue.(*imdraw.IMDraw).Color = pixel.RGB(args[1].(float64),args[2].(float64),args[3].(float64))
	        (*ident)[vi].IValue.(*imdraw.IMDraw).Push(pixel.V(args[4].(float64),args[5].(float64)))
            // vlock.Unlock()
        } else {
            return false,errors.New("ui_pp: invalid object name")
        }
        return true,nil
    }

	slhelp["ui_draw_reset"] = LibHelp{in: "draw_object,thickness", out: "bool_success", action: "Reset a drawing object to it's initial state."}
	stdlib["ui_draw_reset"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_draw_reset",args,1,"1","*imdraw.IMDraw"); !ok { return false,err }
	    args[0].(*imdraw.IMDraw).Reset()
        return true,nil
    }

	slhelp["ui_line"] = LibHelp{in: "draw_object,thickness", out: "bool_success", action: "Set shape of a draw object to line."}
	stdlib["ui_line"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_line",args,1,"2","*imdraw.IMDraw","float64"); !ok { return false,err }
	    args[0].(*imdraw.IMDraw).Line(args[1].(float64))
        return true,nil
    }

	slhelp["ui_rectangle"] = LibHelp{in: "draw_object,thickness", out: "bool_success", action: "Set shape of a draw object to rectangle."}
	stdlib["ui_rectangle"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_rectangle",args,1,"2","*imdraw.IMDraw","float64"); !ok { return false,err }
	    args[0].(*imdraw.IMDraw).Rectangle(args[1].(float64))
        return true,nil
    }

	slhelp["ui_circle"] = LibHelp{in: "draw_object,radius,thickness", out: "bool_success", action: "Set shape of a draw object to circle."}
	stdlib["ui_circle"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_circle",args,1,"3","*imdraw.IMDraw","float64","float64"); !ok { return false,err }
	    args[0].(*imdraw.IMDraw).Circle(args[1].(float64),args[2].(float64))
        return true,nil
    }

	slhelp["ui_circle_arc"] = LibHelp{in: "draw_object,radius,low,high,thickness", out: "bool_success", action: "Set shape of a draw object to arc of circle."}
	stdlib["ui_circle_arc"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_circle_arc",args,1,"5",
            "*imdraw.IMDraw","float64","float64","float64","float64"); !ok { return false,err }
	    args[0].(*imdraw.IMDraw).CircleArc(args[1].(float64),args[2].(float64),args[3].(float64),args[4].(float64))
        return true,nil
    }

	slhelp["ui_polygon"] = LibHelp{in: "draw_object,thickness", out: "bool_success", action: "Set shape of a draw object to polygon."}
	stdlib["ui_polygon"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_polygon",args,1,"2","*imdraw.IMDraw","float64"); !ok { return false,err }
	    args[0].(*imdraw.IMDraw).Polygon(args[1].(float64))
        return true,nil
    }

	slhelp["ui_sprite_draw"] = LibHelp{in: "window_handle,draw_object_name,matrix", out: "bool_success", action: "Directly render shape."}
	stdlib["ui_sprite_draw"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_sprite_draw",args,2,
            "4","string","*pixel.Sprite","pixel.Matrix","color.Color",
            "3","string","*pixel.Sprite","pixel.Matrix"); !ok { return false,err }
        tgt:=args[0].(string)
        mat:=args[2].(pixel.Matrix)
        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if _,there:=winHandles[tgt]; there {
                w:=winHandles[tgt].winHandle
                    switch len(args) {
                    case 3:
                        args[1].(*pixel.Sprite).Draw(w,mat)
                    default:
                        args[1].(*pixel.Sprite).DrawColorMask(w,mat,args[3].(color.Color))
                    }
                return true,nil
            } else {
                return false,nil
            }
        }
		return false, errors.New("Windowing system not available")
    }

	slhelp["ui_batch_draw"] = LibHelp{in: "window", out: "bool_success", action: "Render the batch list to target window."}
	stdlib["ui_batch_draw"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_batch_draw",args,1,"1","string"); !ok { return false,err }
        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if _,there:=winHandles[args[0].(string)]; there {
                w:=winHandles[args[0].(string)].winHandle
                winHandles[args[0].(string)].batch.Draw(w)
                return true,nil
            } else {
                return false,nil
            }
        }
		return false, errors.New("Windowing system not available")
    }

	slhelp["ui_batch"] = LibHelp{in: "target_window,drawing_object", out: "bool_success", action: "Send shape to a target window's draw batch."}
	stdlib["ui_batch"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_batch",args,1,"2","string","*imdraw.IMDraw"); !ok { return false,err }

        if winAvailable {
            globlock.Lock()
            defer globlock.Unlock()
            if w,there:=winHandles[args[0].(string)]; there {
                args[1].(*imdraw.IMDraw).Draw(w.batch)
                return true,nil
            } else {
                return false,nil
            }
        }
		return false, errors.New("Windowing system not available")
    }

	slhelp["ui_batch_clear"] = LibHelp{in: "window_handle", out: "bool_success", action: "Clear a window's batch draw object."}
	stdlib["ui_batch_clear"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_batch_clear",args,1,"1","string"); !ok { return false,err }
        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if _,there:=winHandles[args[0].(string)]; there {
                winHandles[args[0].(string)].batch.Clear()
                return true,nil
            } else {
                // pf("not a window target in ui_batch_clear() - %v\n",args[0].(string))
                return false,nil
            }
        }
		return false, errors.New("Windowing system not available")
    }

	slhelp["ui_update"] = LibHelp{in: "id", out: "bool_success", action: "Invalidate the window."}
	stdlib["ui_update"] = func(evalfs uint32,ident *[]Variable,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("ui_update",args,1,"1","string"); !ok { return false,err }
        h:=args[0].(string)

        globlock.Lock()
        defer globlock.Unlock()
        if winAvailable {
            if _,there:=winHandles[h]; there {
                w:=winHandles[h].winHandle
                w.Update()
                return true,nil
            } else {
                // pf("not a window in ui_update() - %v\n",h)
                return false,nil
            }
        }
		return false, errors.New("Windowing system not available")
    }

}

