// +build !noui
// +build windows freebsd linux

package main

import (
	"github.com/faiface/pixel/pixelgl"
    "github.com/go-gl/glfw/v3.3/glfw"
)


var buttons = make(map[string]pixelgl.Button)

const (
	MouseButton1      = pixelgl.Button(glfw.MouseButton1)
	MouseButton2      = pixelgl.Button(glfw.MouseButton2)
	MouseButton3      = pixelgl.Button(glfw.MouseButton3)
	MouseButton4      = pixelgl.Button(glfw.MouseButton4)
	MouseButton5      = pixelgl.Button(glfw.MouseButton5)
	MouseButton6      = pixelgl.Button(glfw.MouseButton6)
	MouseButton7      = pixelgl.Button(glfw.MouseButton7)
	MouseButton8      = pixelgl.Button(glfw.MouseButton8)
	MouseButtonLast   = pixelgl.Button(glfw.MouseButtonLast)
	MouseButtonLeft   = pixelgl.Button(glfw.MouseButtonLeft)
	MouseButtonRight  = pixelgl.Button(glfw.MouseButtonRight)
	MouseButtonMiddle = pixelgl.Button(glfw.MouseButtonMiddle)
)

const (
	KeyUnknown      = pixelgl.Button(glfw.KeyUnknown)
	KeySpace        = pixelgl.Button(glfw.KeySpace)
	KeyApostrophe   = pixelgl.Button(glfw.KeyApostrophe)
	KeyComma        = pixelgl.Button(glfw.KeyComma)
	KeyMinus        = pixelgl.Button(glfw.KeyMinus)
	KeyPeriod       = pixelgl.Button(glfw.KeyPeriod)
	KeySlash        = pixelgl.Button(glfw.KeySlash)
	Key0            = pixelgl.Button(glfw.Key0)
	Key1            = pixelgl.Button(glfw.Key1)
	Key2            = pixelgl.Button(glfw.Key2)
	Key3            = pixelgl.Button(glfw.Key3)
	Key4            = pixelgl.Button(glfw.Key4)
	Key5            = pixelgl.Button(glfw.Key5)
	Key6            = pixelgl.Button(glfw.Key6)
	Key7            = pixelgl.Button(glfw.Key7)
	Key8            = pixelgl.Button(glfw.Key8)
	Key9            = pixelgl.Button(glfw.Key9)
	KeySemicolon    = pixelgl.Button(glfw.KeySemicolon)
	KeyEqual        = pixelgl.Button(glfw.KeyEqual)
	KeyA            = pixelgl.Button(glfw.KeyA)
	KeyB            = pixelgl.Button(glfw.KeyB)
	KeyC            = pixelgl.Button(glfw.KeyC)
	KeyD            = pixelgl.Button(glfw.KeyD)
	KeyE            = pixelgl.Button(glfw.KeyE)
	KeyF            = pixelgl.Button(glfw.KeyF)
	KeyG            = pixelgl.Button(glfw.KeyG)
	KeyH            = pixelgl.Button(glfw.KeyH)
	KeyI            = pixelgl.Button(glfw.KeyI)
	KeyJ            = pixelgl.Button(glfw.KeyJ)
	KeyK            = pixelgl.Button(glfw.KeyK)
	KeyL            = pixelgl.Button(glfw.KeyL)
	KeyM            = pixelgl.Button(glfw.KeyM)
	KeyN            = pixelgl.Button(glfw.KeyN)
	KeyO            = pixelgl.Button(glfw.KeyO)
	KeyP            = pixelgl.Button(glfw.KeyP)
	KeyQ            = pixelgl.Button(glfw.KeyQ)
	KeyR            = pixelgl.Button(glfw.KeyR)
	KeyS            = pixelgl.Button(glfw.KeyS)
	KeyT            = pixelgl.Button(glfw.KeyT)
	KeyU            = pixelgl.Button(glfw.KeyU)
	KeyV            = pixelgl.Button(glfw.KeyV)
	KeyW            = pixelgl.Button(glfw.KeyW)
	KeyX            = pixelgl.Button(glfw.KeyX)
	KeyY            = pixelgl.Button(glfw.KeyY)
	KeyZ            = pixelgl.Button(glfw.KeyZ)
	KeyLeftBracket  = pixelgl.Button(glfw.KeyLeftBracket)
	KeyBackslash    = pixelgl.Button(glfw.KeyBackslash)
	KeyRightBracket = pixelgl.Button(glfw.KeyRightBracket)
	KeyGraveAccent  = pixelgl.Button(glfw.KeyGraveAccent)
	KeyWorld1       = pixelgl.Button(glfw.KeyWorld1)
	KeyWorld2       = pixelgl.Button(glfw.KeyWorld2)
	KeyEscape       = pixelgl.Button(glfw.KeyEscape)
	KeyEnter        = pixelgl.Button(glfw.KeyEnter)
	KeyTab          = pixelgl.Button(glfw.KeyTab)
	KeyBackspace    = pixelgl.Button(glfw.KeyBackspace)
	KeyInsert       = pixelgl.Button(glfw.KeyInsert)
	KeyDelete       = pixelgl.Button(glfw.KeyDelete)
	KeyRight        = pixelgl.Button(glfw.KeyRight)
	KeyLeft         = pixelgl.Button(glfw.KeyLeft)
	KeyDown         = pixelgl.Button(glfw.KeyDown)
	KeyUp           = pixelgl.Button(glfw.KeyUp)
	KeyPageUp       = pixelgl.Button(glfw.KeyPageUp)
	KeyPageDown     = pixelgl.Button(glfw.KeyPageDown)
	KeyHome         = pixelgl.Button(glfw.KeyHome)
	KeyEnd          = pixelgl.Button(glfw.KeyEnd)
	KeyCapsLock     = pixelgl.Button(glfw.KeyCapsLock)
	KeyScrollLock   = pixelgl.Button(glfw.KeyScrollLock)
	KeyNumLock      = pixelgl.Button(glfw.KeyNumLock)
	KeyPrintScreen  = pixelgl.Button(glfw.KeyPrintScreen)
	KeyPause        = pixelgl.Button(glfw.KeyPause)
	KeyF1           = pixelgl.Button(glfw.KeyF1)
	KeyF2           = pixelgl.Button(glfw.KeyF2)
	KeyF3           = pixelgl.Button(glfw.KeyF3)
	KeyF4           = pixelgl.Button(glfw.KeyF4)
	KeyF5           = pixelgl.Button(glfw.KeyF5)
	KeyF6           = pixelgl.Button(glfw.KeyF6)
	KeyF7           = pixelgl.Button(glfw.KeyF7)
	KeyF8           = pixelgl.Button(glfw.KeyF8)
	KeyF9           = pixelgl.Button(glfw.KeyF9)
	KeyF10          = pixelgl.Button(glfw.KeyF10)
	KeyF11          = pixelgl.Button(glfw.KeyF11)
	KeyF12          = pixelgl.Button(glfw.KeyF12)
	KeyF13          = pixelgl.Button(glfw.KeyF13)
	KeyF14          = pixelgl.Button(glfw.KeyF14)
	KeyF15          = pixelgl.Button(glfw.KeyF15)
	KeyF16          = pixelgl.Button(glfw.KeyF16)
	KeyF17          = pixelgl.Button(glfw.KeyF17)
	KeyF18          = pixelgl.Button(glfw.KeyF18)
	KeyF19          = pixelgl.Button(glfw.KeyF19)
	KeyF20          = pixelgl.Button(glfw.KeyF20)
	KeyF21          = pixelgl.Button(glfw.KeyF21)
	KeyF22          = pixelgl.Button(glfw.KeyF22)
	KeyF23          = pixelgl.Button(glfw.KeyF23)
	KeyF24          = pixelgl.Button(glfw.KeyF24)
	KeyF25          = pixelgl.Button(glfw.KeyF25)
	KeyKP0          = pixelgl.Button(glfw.KeyKP0)
	KeyKP1          = pixelgl.Button(glfw.KeyKP1)
	KeyKP2          = pixelgl.Button(glfw.KeyKP2)
	KeyKP3          = pixelgl.Button(glfw.KeyKP3)
	KeyKP4          = pixelgl.Button(glfw.KeyKP4)
	KeyKP5          = pixelgl.Button(glfw.KeyKP5)
	KeyKP6          = pixelgl.Button(glfw.KeyKP6)
	KeyKP7          = pixelgl.Button(glfw.KeyKP7)
	KeyKP8          = pixelgl.Button(glfw.KeyKP8)
	KeyKP9          = pixelgl.Button(glfw.KeyKP9)
	KeyKPDecimal    = pixelgl.Button(glfw.KeyKPDecimal)
	KeyKPDivide     = pixelgl.Button(glfw.KeyKPDivide)
	KeyKPMultiply   = pixelgl.Button(glfw.KeyKPMultiply)
	KeyKPSubtract   = pixelgl.Button(glfw.KeyKPSubtract)
	KeyKPAdd        = pixelgl.Button(glfw.KeyKPAdd)
	KeyKPEnter      = pixelgl.Button(glfw.KeyKPEnter)
	KeyKPEqual      = pixelgl.Button(glfw.KeyKPEqual)
	KeyLeftShift    = pixelgl.Button(glfw.KeyLeftShift)
	KeyLeftControl  = pixelgl.Button(glfw.KeyLeftControl)
	KeyLeftAlt      = pixelgl.Button(glfw.KeyLeftAlt)
	KeyLeftSuper    = pixelgl.Button(glfw.KeyLeftSuper)
	KeyRightShift   = pixelgl.Button(glfw.KeyRightShift)
	KeyRightControl = pixelgl.Button(glfw.KeyRightControl)
	KeyRightAlt     = pixelgl.Button(glfw.KeyRightAlt)
	KeyRightSuper   = pixelgl.Button(glfw.KeyRightSuper)
	KeyMenu         = pixelgl.Button(glfw.KeyMenu)
	KeyLast         = pixelgl.Button(glfw.KeyLast)
)

var buttonNames = map[pixelgl.Button]string{
	MouseButton4:      "MouseButton4",
	MouseButton5:      "MouseButton5",
	MouseButton6:      "MouseButton6",
	MouseButton7:      "MouseButton7",
	MouseButton8:      "MouseButton8",
	MouseButtonLeft:   "MouseButtonLeft",
	MouseButtonRight:  "MouseButtonRight",
	MouseButtonMiddle: "MouseButtonMiddle",
	KeyUnknown:        "Unknown",
	KeySpace:          "Space",
	KeyApostrophe:     "Apostrophe",
	KeyComma:          "Comma",
	KeyMinus:          "Minus",
	KeyPeriod:         "Period",
	KeySlash:          "Slash",
	Key0:              "0",
	Key1:              "1",
	Key2:              "2",
	Key3:              "3",
	Key4:              "4",
	Key5:              "5",
	Key6:              "6",
	Key7:              "7",
	Key8:              "8",
	Key9:              "9",
	KeySemicolon:      "Semicolon",
	KeyEqual:          "Equal",
	KeyA:              "A",
	KeyB:              "B",
	KeyC:              "C",
	KeyD:              "D",
	KeyE:              "E",
	KeyF:              "F",
	KeyG:              "G",
	KeyH:              "H",
	KeyI:              "I",
	KeyJ:              "J",
	KeyK:              "K",
	KeyL:              "L",
	KeyM:              "M",
	KeyN:              "N",
	KeyO:              "O",
	KeyP:              "P",
	KeyQ:              "Q",
	KeyR:              "R",
	KeyS:              "S",
	KeyT:              "T",
	KeyU:              "U",
	KeyV:              "V",
	KeyW:              "W",
	KeyX:              "X",
	KeyY:              "Y",
	KeyZ:              "Z",
	KeyLeftBracket:    "LeftBracket",
	KeyBackslash:      "Backslash",
	KeyRightBracket:   "RightBracket",
	KeyGraveAccent:    "GraveAccent",
	KeyWorld1:         "World1",
	KeyWorld2:         "World2",
	KeyEscape:         "Escape",
	KeyEnter:          "Enter",
	KeyTab:            "Tab",
	KeyBackspace:      "Backspace",
	KeyInsert:         "Insert",
	KeyDelete:         "Delete",
	KeyRight:          "Right",
	KeyLeft:           "Left",
	KeyDown:           "Down",
	KeyUp:             "Up",
	KeyPageUp:         "PageUp",
	KeyPageDown:       "PageDown",
	KeyHome:           "Home",
	KeyEnd:            "End",
	KeyCapsLock:       "CapsLock",
	KeyScrollLock:     "ScrollLock",
	KeyNumLock:        "NumLock",
	KeyPrintScreen:    "PrintScreen",
	KeyPause:          "Pause",
	KeyF1:             "F1",
	KeyF2:             "F2",
	KeyF3:             "F3",
	KeyF4:             "F4",
	KeyF5:             "F5",
	KeyF6:             "F6",
	KeyF7:             "F7",
	KeyF8:             "F8",
	KeyF9:             "F9",
	KeyF10:            "F10",
	KeyF11:            "F11",
	KeyF12:            "F12",
	KeyF13:            "F13",
	KeyF14:            "F14",
	KeyF15:            "F15",
	KeyF16:            "F16",
	KeyF17:            "F17",
	KeyF18:            "F18",
	KeyF19:            "F19",
	KeyF20:            "F20",
	KeyF21:            "F21",
	KeyF22:            "F22",
	KeyF23:            "F23",
	KeyF24:            "F24",
	KeyF25:            "F25",
	KeyKP0:            "KP0",
	KeyKP1:            "KP1",
	KeyKP2:            "KP2",
	KeyKP3:            "KP3",
	KeyKP4:            "KP4",
	KeyKP5:            "KP5",
	KeyKP6:            "KP6",
	KeyKP7:            "KP7",
	KeyKP8:            "KP8",
	KeyKP9:            "KP9",
	KeyKPDecimal:      "KPDecimal",
	KeyKPDivide:       "KPDivide",
	KeyKPMultiply:     "KPMultiply",
	KeyKPSubtract:     "KPSubtract",
	KeyKPAdd:          "KPAdd",
	KeyKPEnter:        "KPEnter",
	KeyKPEqual:        "KPEqual",
	KeyLeftShift:      "LeftShift",
	KeyLeftControl:    "LeftControl",
	KeyLeftAlt:        "LeftAlt",
	KeyLeftSuper:      "LeftSuper",
	KeyRightShift:     "RightShift",
	KeyRightControl:   "RightControl",
	KeyRightAlt:       "RightAlt",
	KeyRightSuper:     "RightSuper",
	KeyMenu:           "Menu",
}

