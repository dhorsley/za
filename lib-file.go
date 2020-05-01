//+build !test

package main

import (
	"errors"
	"io/ioutil"
	"os"
	sc "strconv"
)

func buildFileLib() {

	// file handling

	features["file"] = Feature{version: 1, category: "os"}
	categories["file"] = []string{
                        "file_mode", "file_size", "read_file", "write_file",
                        "is_file", "is_dir", "is_soft", "is_pipe", "perms",
                        "file_create", "file_close",
    }

    slhelp["file_create"] = LibHelp{in: "filename", out: "filehandle", action: "Returns a file handle for a new file."}
    stdlib["file_create"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 || sf("%T",args[0])!="string" {
            return nil,errors.New("Bad arguments to file_create()")
        }
        f, err := os.Create(args[0].(string))
        return f,nil
    }

    slhelp["file_close"] = LibHelp{in: "filehandle", out: "", action: "Closes an open file handle."}
    stdlib["file_close"] = func(args ...interface{}) (ret interface{}, err error) {
        if len(args)!=1 || sf("%T",args[0])!="*os.File" {
            return nil,errors.New("Bad arguments to file_create()")
        }
        args[0].(*os.File).Sync()
        args[0].(*os.File).Close()
        return nil,nil
    }

	slhelp["file_mode"] = LibHelp{in: "file_name", out: "file_mode", action: "Returns the file mode attributes of a given file."}
	stdlib["file_mode"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return 0, errors.New("invalid arguments provided to file_mode()")
		}
		switch args[0].(type) {
		case string:
			f, err := os.Stat(args[0].(string))
			if err == nil {
				return f.Mode(), err
			}
		}
		return -1, err
	}

	slhelp["file_size"] = LibHelp{in: "file_name", out: "integer", action: "Returns the file size, in bytes, of a given file."}
	stdlib["file_size"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return 0, errors.New("invalid arguments provided to file_size()")
		}
		switch args[0].(type) {
		case string:
			f, err := os.Stat(args[0].(string))
			if err == nil {
				return f.Size(), err
			}
		}
		return -1, err
	}

	slhelp["read_file"] = LibHelp{in: "file_name", out: "string", action: "Returns the contents of the named file."}
	stdlib["read_file"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return "", errors.New("invalid arguments provided to read_file()")
		}
		switch args[0].(type) {
		case string:
			f := args[0].(string)
			s, err := ioutil.ReadFile(f)
			return string(s), err
		}
		return "", errors.New("Filename in read_file() must be a string.")
	}

	slhelp["write_file"] = LibHelp{in: "filename,variable,mode_string", out: "bool", action: "Writes the contents of the string [#i1]variable[#i0] to file [#i1]filename[#i0]. Optionally sets the umasked file mode on new files."}
	stdlib["write_file"] = func(args ...interface{}) (ret interface{}, err error) {
		var outVar string
		var filename string
		var outMode os.FileMode = 0600
		var omconv uint64
		var convErr error
		switch len(args) {
		case 2:
			filename = args[0].(string)
			outVar = args[1].(string)
		case 3:
			filename = args[0].(string)
			outVar = args[1].(string)
			omconv, convErr = sc.ParseUint(args[2].(string), 8, 32)
			if convErr != nil {
				return false, errors.New("could not make an octal mode from the provided string.")
			}
			outMode = os.FileMode(omconv)
		default:
			return false, errors.New("Error: bad arguments supplied to write_file()")
		}
		err = ioutil.WriteFile(filename, []byte(outVar), outMode)
		if err != nil {
			return false, err
		}
		return true, err
	}

	slhelp["is_file"] = LibHelp{in: "file_name", out: "bool", action: "is file_name a regular file?"}
	stdlib["is_file"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return false, errors.New("invalid arguments provided to is_file()")
		}
		switch args[0].(type) {
		case string:
			f, err := os.Stat(args[0].(string))
			if err == nil {
				return f.Mode().IsRegular(), err
			} else {
				return false, nil
			}
		}
		return false, errors.New("argument to is_file() not a string.")
	}

	slhelp["is_dir"] = LibHelp{in: "file_name", out: "bool", action: "is file_name a directory?"}
	stdlib["is_dir"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return false, errors.New("invalid arguments provided to is_dir()")
		}
		switch args[0].(type) {
		case string:
			f, err := os.Stat(args[0].(string))
			if err == nil {
				return f.Mode().IsDir(), nil
			} else {
				return false, nil
			}
		}
		return false, errors.New("argument to is_dir() not a string.")
	}

	slhelp["is_soft"] = LibHelp{in: "file_name", out: "bool", action: "is file_name a symbolic link?"}
	stdlib["is_soft"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return false, errors.New("invalid arguments provided to is_soft()")
		}
		switch args[0].(type) {
		case string:
			f, err := os.Stat(args[0].(string))
			if err == nil {
				return f.Mode()&os.ModeSymlink != 0, err
			} else {
				return false, err
			}
		}
		return false, errors.New("argument to is_soft() not a string.")
	}

	slhelp["is_pipe"] = LibHelp{in: "file_name", out: "bool", action: "is file_name a named pipe?"}
	stdlib["is_pipe"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return false, errors.New("invalid arguments provided to is_pipe()")
		}
		switch args[0].(type) {
		case string:
			f, err := os.Stat(args[0].(string))
			if err == nil {
				return f.Mode()&os.ModeNamedPipe != 0, err
			} else {
				return false, err
			}
		}
		return false, errors.New("argument to is_pipe() not a string.")
	}

	slhelp["perms"] = LibHelp{in: "file_name", out: "int", action: "Returns the file access permissions as an integer."}
	stdlib["perms"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 1 {
			return false, errors.New("invalid arguments provided to perms()")
		}
		switch args[0].(type) {
		case string:
			f, err := os.Stat(args[0].(string))
			if err == nil {
				return f.Mode().Perm(), err
			} else {
				return 0, err
			}
		}
		return 0, errors.New("argument to perms() not a string.")
	}

}
