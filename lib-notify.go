//go:build !test

package main

import (
	"errors"
	"github.com/fsnotify/fsnotify"
	// "reflect"
	"regexp"
	"strconv"
	str "strings"
	// "unsafe"
)

/*
func GetWatcherField(obj any,fieldname string) any {
    field:=reflect.ValueOf(obj).Elem().FieldByName(fieldname)
    return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface()
}
*/

var notify_handler_list = make(map[*fsnotify.Watcher]bool) // map of each instantiated watcher

func buildNotifyLib() {

	features["notify"] = Feature{version: 1, category: "file"}
	categories["notify"] = []string{
		"ev_watch", "ev_watch_close", "ev_watch_add", "ev_watch_remove",
		"ev_exists", "ev_event", "ev_mask",
	}

	slhelp["ev_exists"] = LibHelp{in: "watcher", out: "bool", action: "True if [#i1]watcher[#i0] should still be available for use."}
	stdlib["ev_exists"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ev_exists", args, 1, "1", "*fsnotify.Watcher"); !ok {
			return nil, err
		}
		id := args[0].(*fsnotify.Watcher)
		if _, there := notify_handler_list[id]; !there {
			return false, nil
		}
		return true, nil // if the watcher port/file descriptor should still be open
	}

	slhelp["ev_watch_close"] = LibHelp{in: "watcher", out: "bool", action: "Dispose of a watcher object."}
	stdlib["ev_watch_close"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {

		if ok, err := expect_args("ev_watch_close", args, 1, "1", "*fsnotify.Watcher"); !ok {
			return nil, err
		}
		id := args[0].(*fsnotify.Watcher)
		if _, there := notify_handler_list[id]; !there {
			return nil, errors.New("Unknown watcher specified in ev_watch_close")
		} else {
			err = id.Close()
			if err != nil {
				return nil, err
			}
			delete(notify_handler_list, id)
		}
		return nil, nil
	}

	slhelp["ev_watch"] = LibHelp{in: "filepath_string",
		out: "watcher,int_error_code",
		action: 
            "Initialise a file system watch object. Returns the new watcher and 0 error code on success,\n" +
			"[#SOL]otherwise nil and >0 code. 1->create_watcher_failed, 2->file_path_failure",
    }
	stdlib["ev_watch"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {

		if ok, err := expect_args("ev_watch", args, 1, "1", "string"); !ok {
			return nil, err
		}
		fspath := args[0].(string)

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return []any{nil, 1}, nil
		}

		err = watcher.Add(fspath)
		if err != nil {
			return []any{nil, 2}, nil
		}

		notify_handler_list[watcher] = true
		return []any{watcher, 0}, nil
	}

	slhelp["ev_watch_add"] = LibHelp{in: "watcher,filepath_string", out: "int_error_code", action: "Add another file path to [#i1]watcher[#i0]. Returns 0 on success, otherwise >0 code. e.g. 13->file permissions."}
	stdlib["ev_watch_add"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ev_watch_add", args, 1, "2", "*fsnotify.Watcher", "string"); !ok {
			return nil, err
		}
		id := args[0].(*fsnotify.Watcher)
		fspath := args[1].(string)
		err = id.Add(fspath)
		erv, _ := strconv.ParseInt(sf("%#v", err), 0, 64)
		return erv, nil
	}

	slhelp["ev_watch_remove"] = LibHelp{in: "watcher,filepath_string", out: "int_error_code", action: "Remove an existing file path in [#i1]watcher[#i0]. Returns 0 on success, otherwise >0 code."}
	stdlib["ev_watch_remove"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ev_watch_remove", args, 1, "2", "*fsnotify.Watcher", "string"); !ok {
			return nil, err
		}
		id := args[0].(*fsnotify.Watcher)
		fspath := args[1].(string)
		err = id.Remove(fspath)
		erv, _ := strconv.ParseInt(sf("%#v", err), 0, 64)
		return erv, nil
	}

	slhelp["ev_event"] = LibHelp{in: "watcher", out: "notify_event", action: "Sample events in [#i1]watcher[#i0]. Returns an event or nil."}
	stdlib["ev_event"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ev_event", args, 1, "1", "*fsnotify.Watcher"); !ok {
			return nil, err
		}
		id := args[0].(*fsnotify.Watcher)
		var event fsnotify.Event
		var ok bool
		select {
		case event, ok = <-id.Events:
			if !ok {
				return 0, errors.New("Could not read events in ev_event")
			}
		case err, ok := <-id.Errors:
			if !ok {
				return 0, err
			}
		}
		// ignore our own temp files:
		if match, _ := regexp.MatchString("/tmp/copper.*.err$", event.Name); match {
			return nil, nil
		}
		return event, nil
	}

	slhelp["ev_mask"] = LibHelp{in: "notify_event,str_event_type", out: "filename_or_nil", action: "Is [#i1]notify_event[#i0] of type [#i1]str_event_type[#i0]? str_event_type can be one of create, write, remove, rename or chmod."}
	stdlib["ev_mask"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ev_mask", args, 1, "2", "fsnotify.Event", "string"); !ok {
			return nil, err
		}
		ev := args[0].(fsnotify.Event)
		mask_op := args[1].(string)
		nm := ev.Name

		if str.ToLower(mask_op) == "create" {
			if ev.Op&fsnotify.Create == fsnotify.Create {
				return nm, nil
			}
		}
		if str.ToLower(mask_op) == "write" {
			if ev.Op&fsnotify.Write == fsnotify.Write {
				return nm, nil
			}
		}
		if str.ToLower(mask_op) == "remove" {
			if ev.Op&fsnotify.Remove == fsnotify.Remove {
				return nm, nil
			}
		}
		if str.ToLower(mask_op) == "rename" {
			if ev.Op&fsnotify.Rename == fsnotify.Rename {
				return nm, nil
			}
		}
		if str.ToLower(mask_op) == "chmod" {
			if ev.Op&fsnotify.Chmod == fsnotify.Chmod {
				return nm, nil
			}
		}
		return nil, nil
	}

}
