// +build windows

package main

import (
    "errors"
    "os"
    "path/filepath"
    "fmt"
    "io"
    "syscall"
    "regexp"
)


func fileStatSys(fp string) (*syscall.Win32FileAttributeData) {
    var data syscall.Win32FileAttributeData
    err := syscall.GetFileAttributesEx(syscall.StringToUTF16Ptr(fp), syscall.GetFileExInfoStandard, (*byte)(unsafe.Pointer(&data)))
    if err != nil {
        return nil
    }
    return &data



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

    /* linux has these extras:

    categories["os"] = []string{
        "can_read", "can_write", "umask", "chroot",
        "is_symlink", "is_device", "is_pipe", "is_socket", "is_sticky", "is_setuid", "is_setgid", }

        most of those don't really have an equivalent in the base FS type in windows

    */

    features["os"] = Feature{version: 1, category: "os"}
    categories["os"] = []string{"env", "get_env", "set_env",
        "cwd", "cd", "dir",
        "parent", 
        "delete", "rename", "copy",
    }
    // replaced by operators: "filebase", "fileabs",

    slhelp["dir"] = LibHelp{in: "[filepath[,filter]]", out: "[]structs", action: "Returns an array containing file information on path [#i1]filepath[#i0]. [#i1]filter[#i0] can be specified, as a regex, to narrow results. Each array element contains name,mode,size,mtime and is_dir fields. These specify filename, file mode, file size, modification time and directory status respectively."}
    stdlib["dir"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("dir",args,3,
            "2","string","string",
            "1","string",
            "0"); !ok { return nil,err }

        dir:="."; filter:="^.*$"
        if len(args)>0 {
            dir=args[0].(string)
        }
        if len(args)>1 {
            filter=args[1].(string)
        }

        if !regexWillCompile(filter) {
            return nil,fmt.Errorf("invalid regex in dir() : %s",filter)
        }

        // get file list
        f, err := os.Open(dir)
        if err != nil { return nil,errors.New("Path not found in dir()") }
        files, err := f.Readdir(-1)
        f.Close()
        if err != nil { return nil,errors.New("Could not complete directory listing in dir()") }

        var dl []dirent
        for _, file := range files {
            if match, _ := regexp.MatchString(filter, file.Name()); !match { continue }
            var fs dirent
            fs.name=file.Name()
            fs.size=file.Size()
            fs.mode=int(file.Mode())
            fs.mtime=file.ModTime().Unix()
            fs.is_dir=file.IsDir()
            dl=append(dl,fs)
        }

        return dl,nil
    }

    slhelp["cwd"] = LibHelp{in: "", out: "string", action: "Returns the current working directory."}
    stdlib["cwd"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("cwd",args,0); !ok { return nil,err }
        return syscall.Getwd()
    }

    slhelp["cd"] = LibHelp{in: "string", out: "", action: "Changes directory to a given path."}
    stdlib["cd"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("cd",args,1,"1","string"); !ok { return nil,err }
        err=syscall.Chdir(args[0].(string))
        return nil, err
    }

    slhelp["delete"] = LibHelp{in: "string", out: "", action: "Delete a file."}
    stdlib["delete"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("delete",args,1,"2","string","string"); !ok { return nil,err }
        err=os.Remove(args[0].(string))
        suc:=true
        if err!=nil { suc=false }
        return suc, err
    }

    slhelp["rename"] = LibHelp{in: "src_string,dest_string", out: "bool", action: "Rename a file."}
    stdlib["rename"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("rename",args,1,"2","string","string"); !ok { return nil,err }
        err=os.Rename(args[0].(string),args[1].(string))
        suc:=true
        if err!=nil { suc=false }
        return suc, err
    }

    slhelp["copy"] = LibHelp{in: "src_string,dest_string", out: "bool", action: "Copy a single file."}
    stdlib["copy"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("copy",args,1,"2","string","string"); !ok { return nil,err }
        _,err=fcopy(args[0].(string),args[1].(string))
        suc:=true
        if err!=nil { suc=false }
        return suc, err
    }

    slhelp["parent"] = LibHelp{in: "string", out: "string", action: "Returns the parent directory."}
    stdlib["parent"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("parent",args,1,"1","string"); !ok { return nil,err }
        return filepath.Dir(args[0].(string)),nil
    }

    /*
    slhelp["filebase"] = LibHelp{in: "string", out: "string", action: "Returns the base name of filename string."}
    stdlib["filebase"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("filebase",args,1,"1","string"); !ok { return nil,err }
        fp:=filepath.Base(args[0].(string))
        return fp,nil
    }

    slhelp["fileabs"] = LibHelp{in: "string", out: "string", action: "Returns the absolute pathname of input string."}
    stdlib["fileabs"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("fileabs",args,1,"1","string"); !ok { return nil,err }
        fp,err:=filepath.Abs(args[0].(string))
        return fp,nil
    }
    */

    slhelp["env"] = LibHelp{in: "", out: "string", action: "Return all available environmental variables."}
    stdlib["env"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("env",args,0); !ok { return nil,err }
        return os.Environ(), err
    }

    // get environmental variable. arg should *usually* be in upper-case.
    slhelp["get_env"] = LibHelp{in: "key_name", out: "string", action: "Return the value of the environmental variable [#i1]key_name[#i0]."}
    stdlib["get_env"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("get_env",args,1,"1","string"); !ok { return nil,err }
        return os.Getenv(args[0].(string)), err
    }

    // set environmental variable.
    slhelp["set_env"] = LibHelp{in: "key_name,value_string", out: "", action: "Set the value of the environmental variable [#i1]key_name[#i0]."}
    stdlib["set_env"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
        if ok,err:=expect_args("set_env",args,1,"2","string","string"); !ok { return nil,err }
        key := args[0].(string)
        val := args[1].(string)
        return os.Setenv(key, val), err
    }

}
