// +build !windows linux freebsd
//+build !test

package main

import (
    "errors"
    "os"
    "path/filepath"
    "golang.org/x/sys/unix"
    "io"
    "io/fs"
    "syscall"
    "regexp"
    "runtime"
)

func fcopy(s, d string) (int64, error) {

        sfs, err := os.Stat(s)
        if err != nil { return 0, err }
        if !sfs.Mode().IsRegular() { return 0, fef("%s is not a regular file", s) }

        src, err := os.Open(s)
        if err != nil { return 0, err }
        defer src.Close()

        dst, err := os.Create(d)
        if err != nil { return 0, err }
        defer dst.Close()

        n, err := io.Copy(dst, src)

        return n, err
}


func buildOsLib() {

    // os level

    features["os"] = Feature{version: 1, category: "os"}
    categories["os"] = []string{"env", "get_env", "set_env", "cwd", "can_read", "can_write", "cd", "dir", "umask", "chroot", "delete", "rename", "copy", "parent", "filebase", "fileabs", "is_symlink", "is_device", "is_pipe", "is_socket", "is_sticky", "is_setuid", "is_setgid", }

    slhelp["dir"] = LibHelp{in: "[filepath[,filter]]", out: "[]structs", action: "Returns an array containing file information on path [#i1]filepath[#i0]. [#i1]filter[#i0] can be specified, as a regex, to narrow results. Each array element contains name,mode,size,mtime and isdir fields. These specify filename, file mode, file size, modification time and directory status respectively."}
    stdlib["dir"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("dir",args,3,
            "2","string","string",
            "1","string",
            "0"); !ok { return nil,err }

        dir:="."; filter:="^.*$"
        if len(args)>0 { dir=args[0].(string) }
        if len(args)>1 { filter=args[1].(string) }

        // get file list
        f, err := os.Open(dir)
        if err != nil { return []dirent{},nil } // errors.New("Path not found in dir()") }

        files, err := f.Readdir(-1)
        f.Close()
        if err != nil { return []dirent{},nil } // errors.New("Could not complete directory listing in dir()") }

        var dl []dirent
        for _, file := range files {
            if match, _ := regexp.MatchString(filter, file.Name()); !match { continue }
            var fs dirent
            fs.name=file.Name()
            fs.size=file.Size()
            fs.mode=uint32(file.Mode())
            fs.mtime=file.ModTime().Unix()
            fs.isdir=file.IsDir()
            dl=append(dl,fs)
        }

        return dl,nil
    }

    slhelp["is_symlink"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a symbolic link."}
    stdlib["is_symlink"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("is_symlink",args,3,
        "1","int",
        "1","uint32",
        "1","fs.FileMode"); !ok { return nil,err }
        switch args[0].(type) {
        case fs.FileMode:
            return uint32(os.ModeSymlink) & uint32(args[0].(fs.FileMode)) != 0, nil
        case int:
            return uint32(os.ModeSymlink) & uint32(args[0].(int)) != 0, nil
        case uint32:
            return uint32(os.ModeSymlink) & args[0].(uint32) != 0, nil
        }
        return false,nil
    }

    slhelp["is_device"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a device."}
    stdlib["is_device"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("is_device",args,3,
        "1","int",
        "1","uint32",
        "1","fs.FileMode"); !ok { return nil,err }
        switch args[0].(type) {
        case fs.FileMode:
            return uint32(os.ModeDevice) & uint32(args[0].(fs.FileMode)) != 0, nil
        case int:
            return uint32(os.ModeDevice) & uint32(args[0].(int)) != 0, nil
        case uint32:
            return uint32(os.ModeDevice) & args[0].(uint32) != 0, nil
        }
        return false,nil
    }

    slhelp["is_pipe"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a named pipe."}
    stdlib["is_pipe"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("is_pipe",args,3,
        "1","int",
        "1","uint32",
        "1","fs.FileMode"); !ok { return nil,err }
        switch args[0].(type) {
        case fs.FileMode:
            return uint32(os.ModeNamedPipe) & uint32(args[0].(fs.FileMode)) != 0, nil
        case int:
            return uint32(os.ModeNamedPipe) & uint32(args[0].(int)) != 0, nil
        case uint32:
            return uint32(os.ModeNamedPipe) & args[0].(uint32) != 0, nil
        }
        return false,nil
    }

    slhelp["is_socket"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a socket."}
    stdlib["is_socket"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("is_socket",args,3,
        "1","int",
        "1","uint32",
        "1","fs.FileMode"); !ok { return nil,err }
        switch args[0].(type) {
        case fs.FileMode:
            return uint32(os.ModeSocket) & uint32(args[0].(fs.FileMode)) != 0, nil
        case int:
            return uint32(os.ModeSocket) & uint32(args[0].(int)) != 0, nil
        case uint32:
            return uint32(os.ModeSocket) & args[0].(uint32) != 0, nil
        }
        return false,nil
    }

    slhelp["is_sticky"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a sticky file."}
    stdlib["is_sticky"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("is_sticky",args,3,
        "1","int",
        "1","uint32",
        "1","fs.FileMode"); !ok { return nil,err }
        switch args[0].(type) {
        case fs.FileMode:
            return uint32(os.ModeSticky) & uint32(args[0].(fs.FileMode)) != 0, nil
        case int:
            return uint32(os.ModeSticky) & uint32(args[0].(int)) != 0, nil
        case uint32:
            return uint32(os.ModeSticky) & args[0].(uint32) != 0, nil
        }
        return false,nil
    }

    slhelp["is_setuid"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a setuid file."}
    stdlib["is_setuid"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("is_setuid",args,3,
        "1","int",
        "1","uint32",
        "1","fs.FileMode"); !ok { return nil,err }
        switch args[0].(type) {
        case fs.FileMode:
            return uint32(os.ModeSetuid) & uint32(args[0].(fs.FileMode)) != 0, nil
        case int:
            return uint32(os.ModeSetuid) & uint32(args[0].(int)) != 0, nil
        case uint32:
            return uint32(os.ModeSetuid) & args[0].(uint32) != 0, nil
        }
        return false,nil
    }

    slhelp["is_setgid"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a setgid file."}
    stdlib["is_setgid"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("is_setgid",args,3,
        "1","int",
        "1","uint32",
        "1","fs.FileMode"); !ok { return nil,err }
        switch args[0].(type) {
        case fs.FileMode:
            return uint32(os.ModeSetgid) & uint32(args[0].(fs.FileMode)) != 0, nil
        case int:
            return uint32(os.ModeSetgid) & uint32(args[0].(int)) != 0, nil
        case uint32:
            return uint32(os.ModeSetgid) & args[0].(uint32) != 0, nil
        }
        return false,nil
    }

    slhelp["parent"] = LibHelp{in: "string", out: "string", action: "Returns the parent directory."}
    stdlib["parent"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("parent",args,1,"1","string"); !ok { return nil,err }
        return filepath.Dir(args[0].(string)),nil
    }

    slhelp["filebase"] = LibHelp{in: "string", out: "string", action: "Returns the base name of filename string."}
    stdlib["filebase"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("filebase",args,1,"1","string"); !ok { return nil,err }
        fp:=filepath.Base(args[0].(string))
        return fp,nil
    }

    slhelp["fileabs"] = LibHelp{in: "string", out: "string", action: "Returns the absolute pathname of input string."}
    stdlib["fileabs"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("fileabs",args,1,"1","string"); !ok { return nil,err }
        fp,err:=filepath.Abs(args[0].(string))
        return fp,nil
    }

    slhelp["cwd"] = LibHelp{in: "", out: "string", action: "Returns the current working directory."}
    stdlib["cwd"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("cwd",args,0); !ok { return nil,err }
        return syscall.Getwd()
    }

    slhelp["umask"] = LibHelp{in: "int", out: "int", action: "Sets the umask value. Returns the previous value."}
    stdlib["umask"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("umask",args,1,"1","int"); !ok { return nil,err }
        if runtime.GOOS=="windows" { return -1,errors.New("umask not supported on this OS") }
        return syscall.Umask(args[0].(int)), nil
    }

    slhelp["chroot"] = LibHelp{in: "string", out: "", action: "Performs a chroot to a given path."}
    stdlib["chroot"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("chroot",args,1,"1","string"); !ok { return nil,err }
        if runtime.GOOS=="windows"    { return nil,errors.New("chroot not supported on this OS") }
        if interactive                { return nil,errors.New("chroot not permitted in interactive mode.") }
        err=syscall.Chroot(args[0].(string))
        return nil, err
    }

    slhelp["cd"] = LibHelp{in: "string", out: "bool", action: "Changes directory to a given path."}
    stdlib["cd"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("cd",args,1,"1","string"); !ok { return nil,err }
        err=syscall.Chdir(args[0].(string))
        if err==nil { return true,nil }
        return false, nil
    }

    slhelp["can_read"] = LibHelp{in: "string", out: "bool", action: "Check if path is readable."}
    stdlib["can_read"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("can_read",args,1,"1","string"); !ok { return nil,err }
        return unix.Access(args[0].(string),unix.R_OK) == nil, nil
    }

    slhelp["can_write"] = LibHelp{in: "string", out: "bool", action: "Check if path is writeable."}
    stdlib["can_write"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("can_write",args,1,"1","string"); !ok { return nil,err }
        return unix.Access(args[0].(string),unix.W_OK) == nil, nil
    }

    slhelp["delete"] = LibHelp{in: "string", out: "bool", action: "Delete a file."}
    stdlib["delete"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("delete",args,1,"1","string"); !ok { return nil,err }
        err=os.Remove(args[0].(string))
        suc:=true
        if err!=nil { suc=false }
        return suc, err
    }

    slhelp["rename"] = LibHelp{in: "src_string,dest_string", out: "bool", action: "Rename a file."}
    stdlib["rename"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("rename",args,1,"2","string","string"); !ok { return nil,err }
        err=os.Rename(args[0].(string),args[1].(string))
        suc:=true
        if err!=nil { suc=false }
        return suc, err
    }

    slhelp["copy"] = LibHelp{in: "src_string,dest_string", out: "bool", action: "Copy a single file."}
    stdlib["copy"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("copy",args,1,"2","string","string"); !ok { return nil,err }
        _,err=fcopy(args[0].(string),args[1].(string))
        suc:=true
        if err!=nil { suc=false }
        return suc, err
    }

    slhelp["env"] = LibHelp{in: "", out: "string", action: "Return all available environmental variables."}
    stdlib["env"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("env",args,0); !ok { return nil,err }
        return os.Environ(), err
    }

    // get environmental variable.
    slhelp["get_env"] = LibHelp{in: "key_name", out: "string", action: "Return the value of the environmental variable [#i1]key_name[#i0]."}
    stdlib["get_env"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("get_env",args,1,"1","string"); !ok { return nil,err }
        return os.Getenv(args[0].(string)), err
    }

    // set environmental variable.
    slhelp["set_env"] = LibHelp{in: "key_name,value_string", out: "", action: "Set the value of the environmental variable [#i1]key_name[#i0]."}
    stdlib["set_env"] = func(evalfs uint32,ident *[szIdent]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("set_env",args,1,"2","string","string"); !ok { return nil,err }
        key := args[0].(string)
        val := args[1].(string)
        return os.Setenv(key, val), err
    }

}

