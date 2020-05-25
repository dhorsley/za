// +build !windows
//+build !test

package main

import (
	"errors"
	"os"
    "syscall"
    "regexp"
    "runtime"
)

func buildOsLib() {

	// os level

	features["os"] = Feature{version: 1, category: "os"}
	categories["os"] = []string{"env", "get_env", "set_env", "cwd", "cd", "dir", "umask", "chroot", }

    slhelp["dir"] = LibHelp{in: "[filepath[,filter]]", out: "array", action: "Returns an array containing file information on path [#i1]filepath[#i0]. [#i1]filter[#i0] can be specified, as a regex, to narrow results. Each array element contains name,mode,size,mtime and isdir key-value pairs. These specify filename, file mode, file size, modification time and directory status respectively."}
    stdlib["dir"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)>2 { return nil,errors.New("Bad arguments (count) in dir()") }
        dir:="."; filter:="^.*$"
        if len(args)>0 {
            if sf("%T",args[0])!="string" { return nil,errors.New("Bad arguments (type) in dir()") }
            dir=args[0].(string)
        }
        if len(args)>1 {
            if sf("%T",args[1])!="string" { return nil,errors.New("Bad arguments (type) in dir()") }
            filter=args[1].(string)
        }
        // get file list
        f, err := os.Open(dir)
        if err != nil { return nil,errors.New("Path not found in dir()") }
        files, err := f.Readdir(-1)
        f.Close()
        if err != nil { return nil,errors.New("Could not complete directory listing in dir()") }

        var dl []map[string]interface{}
        for _, file := range files {
            fs:=make(map[string]interface{})
            if match, _ := regexp.MatchString(filter, file.Name()); !match { continue }
            fs["name"]=file.Name()
            fs["size"]=file.Size()
            fs["mode"]=file.Mode()
            fs["mtime"]=file.ModTime()
            fs["isdir"]=file.IsDir()
            dl=append(dl,fs)
        }

        return dl,nil
    }


	slhelp["cwd"] = LibHelp{in: "", out: "string", action: "Returns the current working directory."}
	stdlib["cwd"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=0               { return -1,errors.New("Bad argument count in cwd()")  }
		return syscall.Getwd()
	}

    slhelp["umask"] = LibHelp{in: "int", out: "int", action: "Sets the umask value. Returns the previous value."}
    stdlib["umask"] = func(args ...interface{}) (ret interface{}, err error) {
        if runtime.GOOS=="windows" { return -1,errors.New("umask not supported on this OS") }
        if len(args)!=1            { return -1,errors.New("Bad argument count in umask()")  }
        if sf("%T",args[0])!="int" { return -1,errors.New("Bad argument type in umask()")   }
        return syscall.Umask(args[0].(int)), nil
    }

    slhelp["chroot"] = LibHelp{in: "string", out: "", action: "Performs a chroot to a given path."}
    stdlib["chroot"] = func(args ...interface{}) (ret interface{}, err error) {
        if runtime.GOOS=="windows"    { return nil,errors.New("chroot not supported on this OS") }
        if interactive                { return nil,errors.New("chroot not permitted in interactive mode.") }
        if len(args)!=1               { return nil,errors.New("Bad argument count in chroot()")  }
        if sf("%T",args[0])!="string" { return nil,errors.New("Bad argument type in chroot()")   }
        err=syscall.Chroot(args[0].(string))
        return nil, err
    }

	slhelp["cd"] = LibHelp{in: "string", out: "", action: "Changes directory to a given path."}
	stdlib["cd"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1               { return nil,errors.New("Bad argument count in cd()")  }
        if sf("%T",args[0])!="string" { return nil,errors.New("Bad argument type in cd()")   }
        err=syscall.Chdir(args[0].(string))
		return nil, err
	}

	slhelp["env"] = LibHelp{in: "", out: "string", action: "Return all available environmental variables."}
	stdlib["env"] = func(args ...interface{}) (ret interface{}, err error) {
		return os.Environ(), err
	}

	// get environmental variable. arg should *usually* be in upper-case.
	slhelp["get_env"] = LibHelp{in: "key_name", out: "string", action: "Return the value of the environmental variable [#i1]key_name[#i0]."}
	stdlib["get_env"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args)!=1 { return "",errors.New("Bad args (count) in get_env()") }
        return os.Getenv(args[0].(string)), err
	}

	// set environmental variable.
	slhelp["set_env"] = LibHelp{in: "key_name,value_string", out: "", action: "Set the value of the environmental variable [#i1]key_name[#i0]."}
	stdlib["set_env"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 2 {
			return nil, errors.New("Error: bad arguments to set_env()")
		}
		key := args[0].(string)
		val := args[1].(string)
		return os.Setenv(key, val), err
	}

}
