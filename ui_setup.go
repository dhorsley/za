// +build !noui
// +build windows linux freebsd

package main

import (
	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"github.com/faiface/pixel/text"
    "golang.org/x/image/font/basicfont"
    "os"
)

import (
    "unicode"
)

var win *pixelgl.Window
var default_win_cfg pixelgl.WindowConfig
var default_atlas *text.Atlas


func init_ui_features() {

    if hasUI {
        var err error

        config_check_win := pixelgl.WindowConfig{
            Title       : "Za Config Check",
            Bounds      : pixel.R(0, 0, 1, 1),
            VSync       : false,
            Invisible   : true,
        }

        win, err = pixelgl.NewWindow(config_check_win)
        if err == nil {
            win.Destroy()
            winAvailable=true
        } else {
            pf("Could not initialise windowing features.\n")
            os.Exit(121)
        }

	    default_atlas = text.NewAtlas(basicfont.Face7x13, text.ASCII, text.RangeTable(unicode.Latin))

        for key_button,value_string:=range buttonNames {
            buttons[value_string]=key_button
        }

    }

}

func main() {
    pixelgl.Run(run)
}


