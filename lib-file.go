//+build !test

package main

import (
    "errors"
    "io/ioutil"
    "io"
    "os"
    sc "strconv"
    str "strings"
)


type pfile struct {
    hnd     *os.File
    name    string
}

func buildFileLib() {

    // file handling

    features["file"] = Feature{version: 1, category: "os"}
    categories["file"] = []string{
                        "file_mode", "file_size", "read_file", "write_file",
                        "is_file", "is_dir", "is_soft", "is_pipe", "perms",
                        "fopen", "fclose","fseek","fread","fwrite","feof",
    }

    slhelp["fopen"] = LibHelp{in: "filename,mode", out: "filehandle", action: "Opens a file and returns a file handle. [#i1]mode[#i0] can be either w (write), wa (write-append) or r (read)."}
    stdlib["fopen"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("fopen",args,1,"2","string","string"); !ok { return nil,err }

        fn:=args[0].(string)
        mode:=str.ToLower(args[1].(string))

        var f *os.File
        switch mode {
        case "w":
            f, err = os.Create(fn)
        case "wa":
            f, err = os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
        case "r":
            f, err = os.Open(fn)
        default:
            return nil,errors.New("Unknown mode specified in fopen()")
        }
        var fw pfile
        fw.name=fn
        fw.hnd=f
        if f!=nil {
            return fw,nil
        }
        return nil,nil
    }

    slhelp["fseek"] = LibHelp{in: "filehandle,offset,relativity", out: "position", action: "Move the current position of reads or writes to an open file. relativity indicates where the offset is relative to. (0:start of file,1:current position, 2:end of file) The newly sought position is returned."}
    stdlib["fseek"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("fseek",args,1,"3","main.pfile","int","int"); !ok { return nil,err }
        fw :=args[0].(pfile)
        off:=int64(args[1].(int))
        rel:=args[2].(int)
        return fw.hnd.Seek(off,rel)
    }

    slhelp["fread"] = LibHelp{in: "filehandle,delim", out: "string", action: "Reads a string from an open file until [#i1]delim[#i0] is encountered (or end-of-file)."}
    stdlib["fread"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("fread",args,1,"2","main.pfile","string"); !ok { return nil,err }

        fw:=args[0].(pfile)
        de:=(args[1].(string))

        if len(de)==0 {
            return nil,errors.New("Empty delimiter in fread()")
        }

        deby:=byte(de[0])
        var s str.Builder
        b:=make([]byte,64)
        add:=make([]byte,64)
        var n int
        done:=false
        for ;; {
            n,err=fw.hnd.Read(b)
            // search returned buffer for delimiter
            add=b[:n]
            for p:=0; p<n; p++ {
                if b[p]==deby {
                    // seek to deby+1
                    fw.hnd.Seek(int64(1-(n-p)),1)
                    add=b[:p]
                    done=true
                    break
                }
            }
            s.Write(add)
            if done { break }
            if err==io.EOF { break }
        }
        // @note: does not currently handle a multi-char delim,
        //  which means windows EOL files aren't exactly compatible without fudging.
        return s.String(),nil
    }

    // issues with race cond when file open in write-append mode?
    slhelp["feof"] = LibHelp{in: "filehandle", out: "bool", action: "Check if open file cursor is at end-of-file"}
    stdlib["feof"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("feof",args,1,"1","main.pfile"); !ok { return nil,err }
        fw:=args[0].(pfile)
        cp,_:=fw.hnd.Seek(0,io.SeekCurrent)
        eps,_:=fw.hnd.Stat()
        ep:=eps.Size()
        return cp==ep,nil
    }

    slhelp["fwrite"] = LibHelp{in: "filehandle,string", out: "", action: "Writes a string to an open file."}
    stdlib["fwrite"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("fwrite",args,1,"2","main.pfile","string"); !ok { return nil,err }
        fw:=args[0].(pfile)
        fw.hnd.WriteString(args[1].(string))
        return nil,nil
    }

    slhelp["fclose"] = LibHelp{in: "filehandle", out: "", action: "Closes an open file."}
    stdlib["fclose"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("fclose",args,1,"1","main.pfile"); !ok { return nil,err }
        fw:=args[0].(pfile)
        fw.hnd.Sync()
        fw.hnd.Close()
        fw.hnd=nil
        return nil,nil
    }

    slhelp["file_mode"] = LibHelp{in: "file_name", out: "file_mode", action: "Returns the file mode attributes of a given file, or -1 on error."}
    stdlib["file_mode"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("file_mode",args,1,"1","string"); !ok { return nil,err }
        f, err := os.Stat(args[0].(string))
        if err == nil { return f.Mode(), err }
        return -1, err
    }

    slhelp["file_size"] = LibHelp{in: "string", out: "integer", action: "Returns the file size, in bytes, of a given file [#i1]string[#i0], or -1 if the file cannot be checked."}
    stdlib["file_size"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("file_size",args,1,"1","string"); !ok { return nil,err }
        f, err := os.Stat(args[0].(string))
        if err == nil { return f.Size(), err }
        return -1, err
    }

    slhelp["read_file"] = LibHelp{in: "string", out: "string", action: "Returns the contents of the named file [#i1]string[#i0], or errors."}
    stdlib["read_file"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("read_file",args,1,"1","string"); !ok { return nil,err }
        s, err := ioutil.ReadFile(args[0].(string))
        return string(s), err
    }

    slhelp["write_file"] = LibHelp{in: "filename,wstring[,mode_number_or_string]", out: "bool", action: "Writes the contents of [#i1]wstring[#i0] to file [#i1]filename[#i0]. Optionally sets the umasked file mode on new files. Returns true on success."}
    stdlib["write_file"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("write_file",args,2,
            "3","string","string","string",
            "2","string","string"); !ok { return nil,err }

        var outMode os.FileMode = 0600
        var omconv uint64
        var convErr error

        filename := args[0].(string)
        outVar   := args[1].(string)

        switch len(args) {
        case 3:
            switch args[2].(type) {
            case string:
                omconv, convErr = sc.ParseUint(args[2].(string), 8, 32)
            case int:
                omconv = uint64(args[2].(int))
            case uint:
                omconv = uint64(args[2].(uint))
            case int64:
                omconv = uint64(args[2].(int64))
            }
            if convErr != nil {
                return false, errors.New("could not make an octal mode from the provided string.")
            }
            outMode = os.FileMode(omconv)
        }

        err = ioutil.WriteFile(filename, []byte(outVar), outMode)
        if err != nil {
            return false, err
        }
        return true, err
    }

    slhelp["is_file"] = LibHelp{in: "file_name", out: "bool", action: "Returns true if [#i1]file_name[#i0] is a regular file."}
    stdlib["is_file"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("is_file",args,1,"1","string"); !ok { return nil,err }
        f, err := os.Stat(args[0].(string))
        if err == nil {
            return f.Mode().IsRegular(), nil
        } else {
            return false, nil
        }
    }

    slhelp["is_dir"] = LibHelp{in: "file_name", out: "bool", action: "Returns true if [#i1]file_name[#i0] is a directory."}
    stdlib["is_dir"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("is_dir",args,1,"1","string"); !ok { return nil,err }
        f, err := os.Stat(args[0].(string))
        if err == nil {
            return f.Mode().IsDir(), nil
        } else {
            return false, nil
        }
    }

    slhelp["is_soft"] = LibHelp{in: "file_name", out: "bool", action: "Returns true if [#i1]file_name[#i0] is a symbolic link."}
    stdlib["is_soft"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("is_soft",args,1,"1","string"); !ok { return nil,err }
        f, err := os.Stat(args[0].(string))
        if err == nil {
            return f.Mode()&os.ModeSymlink != 0, err
        } else {
            return false, err
        }
    }

    slhelp["is_pipe"] = LibHelp{in: "file_name", out: "bool", action: "Returns true if [#i1]file_name[#i0] is a named pipe."}
    stdlib["is_pipe"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("is_pipe",args,1,"1","string"); !ok { return nil,err }
        f, err := os.Stat(args[0].(string))
        if err == nil {
            return f.Mode()&os.ModeNamedPipe != 0, err
        } else {
            return false, err
        }
    }

    slhelp["perms"] = LibHelp{in: "file_name", out: "int", action: "Returns the file access permissions as an integer."}
    stdlib["perms"] = func(evalfs uint32,args ...interface{}) (ret interface{}, err error) {
        if ok,err:=expect_args("perms",args,1,"1","string"); !ok { return nil,err }
        f, err := os.Stat(args[0].(string))
        if err == nil {
            return f.Mode().Perm(), err
        } else {
            return 0, err
        }
    }

}



