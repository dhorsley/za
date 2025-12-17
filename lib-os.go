//go:build !test

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"syscall"
)

func fcopy(s, d string) (int64, error) {

	sfs, err := os.Stat(s)
	if err != nil {
		return 0, err
	}
	if !sfs.Mode().IsRegular() {
		return 0, fef("%s is not a regular file", s)
	}

	src, err := os.Open(s)
	if err != nil {
		return 0, err
	}
	defer src.Close()

	dst, err := os.Create(d)
	if err != nil {
		return 0, err
	}
	defer dst.Close()

	n, err := io.Copy(dst, src)

	return n, err
}

func buildOsLib() {

	// os level

	features["os"] = Feature{version: 1, category: "os"}
	categories["os"] = []string{"env", "get_env", "set_env", "cwd", "can_read",
		"can_write", "cd", "dir", "glob", "umask", "chroot", "delete", "rename", "copy",
		"parent", "is_symlink", "is_device", "is_pipe", "is_socket", "is_sticky",
		"is_setuid", "is_setgid", "username", "groupname", "user_list", "group_list",
		"user_add", "user_del", "group_add", "group_del", "group_membership",
		"user_info", "group_info"}
	// "fileabs", "filebase" - replaced by operator

	slhelp["dir"] = LibHelp{in: "[filepath[,filter]]",
		out: "[]structs",
		action: "Returns an array containing file information on path [#i1]filepath[#i0].\n[#SOL]" +
			"[#i1]filter[#i0] can be specified, as a regex, to narrow results.\n[#SOL]" +
			"Each array element contains name,mode,size,mtime and is_dir fields.\n[#SOL]" +
			"These specify filename, file mode, file size, modification time and directory status respectively."}
	stdlib["dir"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("dir", args, 3,
			"2", "string", "string",
			"1", "string",
			"0"); !ok {
			return nil, err
		}

		dir := "."
		filter := "^.*$"
		if len(args) > 0 {
			dir = args[0].(string)
		}
		if len(args) > 1 {
			filter = args[1].(string)
		}

		if !regexWillCompile(filter) {
			return nil, fmt.Errorf("invalid regex in dir() : %s", filter)
		}

		// get file list
		f, err := os.Open(dir)
		if err != nil {
			return []dirent{}, nil
		} // errors.New("Path not found in dir()") }

		files, err := f.Readdir(-1)
		f.Close()
		if err != nil {
			return []dirent{}, nil
		} // errors.New("Could not complete directory listing in dir()") }

		var dl []dirent
		for _, file := range files {
			if match, _ := regexp.MatchString(filter, file.Name()); !match {
				continue
			}
			var fs dirent
			fs.Name = file.Name()
			fs.Size = file.Size()
			fs.Mode = int(file.Mode())
			fs.Mtime = file.ModTime().Unix()
			fs.Is_dir = file.IsDir()
			dl = append(dl, fs)
		}

		return dl, nil
	}

	slhelp["is_symlink"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a symbolic link."}
	stdlib["is_symlink"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("is_symlink", args, 2,
			"1", "int",
			"1", "fs.FileMode"); !ok {
			return nil, err
		}
		switch args[0].(type) {
		case fs.FileMode:
			return uint32(os.ModeSymlink)&uint32(args[0].(fs.FileMode)) != 0, nil
		case int:
			return uint32(os.ModeSymlink)&uint32(args[0].(int)) != 0, nil
		}
		return false, nil
	}

	slhelp["is_device"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a device."}
	stdlib["is_device"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("is_device", args, 2,
			"1", "int",
			"1", "fs.FileMode"); !ok {
			return nil, err
		}
		switch args[0].(type) {
		case fs.FileMode:
			return uint32(os.ModeDevice)&uint32(args[0].(fs.FileMode)) != 0, nil
		case int:
			return uint32(os.ModeDevice)&uint32(args[0].(int)) != 0, nil
		}
		return false, nil
	}

	slhelp["is_pipe"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a named pipe."}
	stdlib["is_pipe"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("is_pipe", args, 2,
			"1", "int",
			"1", "fs.FileMode"); !ok {
			return nil, err
		}
		switch args[0].(type) {
		case fs.FileMode:
			return uint32(os.ModeNamedPipe)&uint32(args[0].(fs.FileMode)) != 0, nil
		case int:
			return uint32(os.ModeNamedPipe)&uint32(args[0].(int)) != 0, nil
		}
		return false, nil
	}

	slhelp["is_socket"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a socket."}
	stdlib["is_socket"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("is_socket", args, 2,
			"1", "int",
			"1", "fs.FileMode"); !ok {
			return nil, err
		}
		switch args[0].(type) {
		case fs.FileMode:
			return uint32(os.ModeSocket)&uint32(args[0].(fs.FileMode)) != 0, nil
		case int:
			return uint32(os.ModeSocket)&uint32(args[0].(int)) != 0, nil
		}
		return false, nil
	}

	slhelp["is_sticky"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a sticky file."}
	stdlib["is_sticky"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("is_sticky", args, 2,
			"1", "int",
			"1", "fs.FileMode"); !ok {
			return nil, err
		}
		switch args[0].(type) {
		case fs.FileMode:
			return uint32(os.ModeSticky)&uint32(args[0].(fs.FileMode)) != 0, nil
		case int:
			return uint32(os.ModeSticky)&uint32(args[0].(int)) != 0, nil
		}
		return false, nil
	}

	slhelp["is_setuid"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a setuid file."}
	stdlib["is_setuid"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("is_setuid", args, 2,
			"1", "int",
			"1", "fs.FileMode"); !ok {
			return nil, err
		}
		switch args[0].(type) {
		case fs.FileMode:
			return uint32(os.ModeSetuid)&uint32(args[0].(fs.FileMode)) != 0, nil
		case int:
			return uint32(os.ModeSetuid)&uint32(args[0].(int)) != 0, nil
		}
		return false, nil
	}

	slhelp["is_setgid"] = LibHelp{in: "mode_number", out: "bool", action: "Checks if a file mode indicates a setgid file."}
	stdlib["is_setgid"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("is_setgid", args, 2,
			"1", "int",
			"1", "fs.FileMode"); !ok {
			return nil, err
		}
		switch args[0].(type) {
		case fs.FileMode:
			return uint32(os.ModeSetgid)&uint32(args[0].(fs.FileMode)) != 0, nil
		case int:
			return uint32(os.ModeSetgid)&uint32(args[0].(int)) != 0, nil
		}
		return false, nil
	}

	slhelp["username"] = LibHelp{in: "int", out: "string", action: "Lookup a username by user id."}
	stdlib["username"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("username", args, 1, "1", "number"); !ok {
			return nil, err
		}
		uid, _ := GetAsInt(args[0])
		str_uid := sf("%d", uid)
		if s_user, err := user.LookupId(str_uid); err == nil {
			return (*s_user).Username, nil
		}
		return "", nil
	}

	slhelp["groupname"] = LibHelp{in: "int", out: "string", action: "Lookup a group name by group id."}
	stdlib["groupname"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("groupname", args, 1, "1", "number"); !ok {
			return nil, err
		}
		gid, _ := GetAsInt(args[0])
		str_gid := sf("%d", gid)
		if s_group, err := user.LookupGroupId(str_gid); err == nil && s_group != nil {
			return (*s_group).Name, nil
		}
		return "", nil
	}

	slhelp["glob"] = LibHelp{in: "pattern[,base_dir]", out: "[]string", action: "Returns an array of file paths matching a glob pattern.\n[#SOL]" +
		"[#i1]pattern[#i0] specifies the glob pattern (supports *, ?, []).\n[#SOL]" +
		"[#i1]base_dir[#i0] optionally specifies the base directory to search in (defaults to current directory).\n[#SOL]" +
		"Returns full paths to all matching files and directories."}
	stdlib["glob"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("glob", args, 2,
			"2", "string","string",
			"1", "string"); !ok {
			return nil, err
		}

		pattern := args[0].(string)
		baseDir := "."
		if len(args) > 1 {
			baseDir = args[1].(string)
		}

		// Validate pattern
		if pattern == "" {
			return []string{}, nil
		}

		// Use filepath.Glob for cross-platform glob matching
		matches, err := filepath.Glob(filepath.Join(baseDir, pattern))
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern '%s': %v", pattern, err)
		}

		return matches, nil
	}

	slhelp["parent"] = LibHelp{in: "string", out: "string", action: "Returns the parent directory."}
	stdlib["parent"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("parent", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return filepath.Dir(args[0].(string)), nil
	}

	/* replaced by operators:
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

	slhelp["cwd"] = LibHelp{in: "", out: "string", action: "Returns the current working directory."}
	stdlib["cwd"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("cwd", args, 0); !ok {
			return nil, err
		}
		return syscall.Getwd()
	}

	slhelp["umask"] = LibHelp{in: "int", out: "int", action: "Sets the umask value. Returns the previous value. umask() without args just returns the current value."}
	stdlib["umask"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("umask", args, 2,
			"1", "int",
			"0"); !ok {
			return nil, err
		}
		if len(args) == 0 {
			return umask(0), nil
		}
		return umask(args[0].(int)), nil
	}

	slhelp["chroot"] = LibHelp{in: "string", out: "", action: "Performs a chroot to a given path."}
	stdlib["chroot"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("chroot", args, 1, "1", "string"); !ok {
			return nil, err
		}

		if interactive {
			return nil, errors.New("chroot not permitted in interactive mode.")
		}

		return nil, chroot(args[0].(string))
	}

	slhelp["cd"] = LibHelp{in: "string", out: "bool", action: "Changes directory to a given path."}
	stdlib["cd"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("cd", args, 1, "1", "string"); !ok {
			return nil, err
		}
		cwd := args[0].(string)
		err = syscall.Chdir(cwd)
		if err == nil {
			system(sf("cd %s", cwd), false)
			gvset("@cwd", cwd)
			return true, nil
		}
		return false, nil
	}

	slhelp["can_read"] = LibHelp{in: "string", out: "bool", action: "Check if path is readable."}
	stdlib["can_read"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("can_read", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return canRead(args[0].(string)), nil
	}

	slhelp["can_write"] = LibHelp{in: "string", out: "bool", action: "Check if path is writeable."}
	stdlib["can_write"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("can_write", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return canWrite(args[0].(string)), nil
	}

	slhelp["delete"] = LibHelp{in: "string", out: "bool", action: "Delete a file."}
	stdlib["delete"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("delete", args, 1, "1", "string"); !ok {
			return nil, err
		}
		err = os.Remove(args[0].(string))
		return err == nil, err
	}

	slhelp["rename"] = LibHelp{in: "src_string,dest_string", out: "bool", action: "Rename a file."}
	stdlib["rename"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("rename", args, 1, "2", "string", "string"); !ok {
			return nil, err
		}
		err = os.Rename(args[0].(string), args[1].(string))
		return err == nil, err
	}

	slhelp["copy"] = LibHelp{in: "src_string,dest_string", out: "bool", action: "Copy a single file."}
	stdlib["copy"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("copy", args, 1, "2", "string", "string"); !ok {
			return nil, err
		}
		_, err = fcopy(args[0].(string), args[1].(string))
		return err == nil, err
	}

	slhelp["env"] = LibHelp{in: "", out: "string", action: "Return all available environmental variables."}
	stdlib["env"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("env", args, 0); !ok {
			return nil, err
		}
		return os.Environ(), err
	}

	// get environmental variable.
	slhelp["get_env"] = LibHelp{in: "key_name", out: "string", action: "Return the value of the environmental variable [#i1]key_name[#i0]."}
	stdlib["get_env"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("get_env", args, 1, "1", "string"); !ok {
			return nil, err
		}
		return os.Getenv(args[0].(string)), err
	}

	// set environmental variable.
	slhelp["set_env"] = LibHelp{in: "key_name,value_string", out: "", action: "Set the value of the environmental variable [#i1]key_name[#i0]."}
	stdlib["set_env"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("set_env", args, 1, "2", "string", "string"); !ok {
			return nil, err
		}
		key := args[0].(string)
		val := args[1].(string)
		return os.Setenv(key, val), err
	}

	// User and Group Management Functions
	slhelp["user_list"] = LibHelp{in: "", out: "[]struct", action: "List all system users with details (uid, gid, home, shell)."}
	stdlib["user_list"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("user_list", args, 0); !ok {
			return nil, err
		}
		return getUserList()
	}

	slhelp["group_list"] = LibHelp{in: "", out: "[]struct", action: "List all system groups with details (gid, members)."}
	stdlib["group_list"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("group_list", args, 0); !ok {
			return nil, err
		}
		return getGroupList()
	}

	slhelp["user_add"] = LibHelp{in: "username[,options]", out: "bool", action: "Add a system user. Options: uid, gid, home, shell, groups, create_home."}
	stdlib["user_add"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("user_add", args, 3,
			"2", "string", "map",
			"1", "string"); !ok {
			return nil, err
		}

		username := args[0].(string)

		// Set up defaults
		defaults := map[string]interface{}{
			"uid":         -1, // Auto-allocate
			"gid":         -1, // Auto-allocate
			"home":        "",
			"shell":       "",
			"groups":      "",
			"create_home": false,
		}

		// Merge with provided options
		var optionsMap map[string]interface{}
		if len(args) > 1 {
			// Handle map argument
			if providedMap, ok := args[1].(map[string]interface{}); ok {
				// Merge provided options with defaults
				optionsMap = make(map[string]interface{})
				for k, v := range defaults {
					optionsMap[k] = v
				}
				for k, v := range providedMap {
					optionsMap[k] = v
				}
			} else {
				return false, fmt.Errorf("user_add: second argument must be a map, got %T", args[1])
			}
		} else {
			optionsMap = defaults
		}

		err = addUser(username, optionsMap)
		return err == nil, err
	}

	slhelp["user_del"] = LibHelp{in: "username[,options]", out: "bool", action: "Remove a system user. Options: remove_home."}
	stdlib["user_del"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("user_del", args, 3,
			"2", "string", "map",
			"1", "string"); !ok {
			return nil, err
		}

		username := args[0].(string)

		// Set up defaults
		defaults := map[string]interface{}{
			"remove_home": false,
		}

		// Merge with provided options
		var optionsMap map[string]interface{}
		if len(args) > 1 {
			// Handle map argument
			if providedMap, ok := args[1].(map[string]interface{}); ok {
				// Merge provided options with defaults
				optionsMap = make(map[string]interface{})
				for k, v := range defaults {
					optionsMap[k] = v
				}
				for k, v := range providedMap {
					optionsMap[k] = v
				}
			} else {
				return false, fmt.Errorf("user_del: second argument must be a map, got %T", args[1])
			}
		} else {
			optionsMap = defaults
		}

		err = removeUser(username, optionsMap)
		return err == nil, err
	}

	slhelp["group_add"] = LibHelp{in: "groupname[,options]", out: "bool", action: "Add a system group. Options: gid (auto-allocated if not provided)."}
	stdlib["group_add"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("group_add", args, 3,
			"2", "string", "map",
			"1", "string"); !ok {
			return nil, err
		}

		groupname := args[0].(string)

		// Set up defaults
		defaults := map[string]interface{}{
			"gid": -1, // Auto-allocate
		}

		// Merge with provided options
		var optionsMap map[string]interface{}
		if len(args) > 1 {
			// Handle map argument
			if providedMap, ok := args[1].(map[string]interface{}); ok {
				// Merge provided options with defaults
				optionsMap = make(map[string]interface{})
				for k, v := range defaults {
					optionsMap[k] = v
				}
				for k, v := range providedMap {
					optionsMap[k] = v
				}
			} else {
				return false, fmt.Errorf("group_add: second argument must be a map, got %T", args[1])
			}
		} else {
			optionsMap = defaults
		}

		err = addGroup(groupname, optionsMap)
		return err == nil, err
	}

	slhelp["group_del"] = LibHelp{in: "groupname", out: "bool", action: "Remove a system group."}
	stdlib["group_del"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("group_del", args, 1, "1", "string"); !ok {
			return nil, err
		}

		groupname := args[0].(string)
		err = removeGroup(groupname)
		return err == nil, err
	}

	slhelp["group_membership"] = LibHelp{in: "username,groupname,action", out: "bool", action: "Manage group membership. Action: add, remove."}
	stdlib["group_membership"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("group_membership", args, 1, "3", "string", "string", "string"); !ok {
			return nil, err
		}

		username := args[0].(string)
		groupname := args[1].(string)
		action := args[2].(string)

		err = manageGroupMembership(username, groupname, action)
		return err == nil, err
	}

	slhelp["user_info"] = LibHelp{in: "username", out: "struct", action: "Get detailed information about a user."}
	stdlib["user_info"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("user_info", args, 1, "1", "string"); !ok {
			return nil, err
		}

		username := args[0].(string)
		return getUserInfo(username)
	}

	slhelp["group_info"] = LibHelp{in: "groupname", out: "struct", action: "Get detailed information about a group."}
	stdlib["group_info"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("group_info", args, 1, "1", "string"); !ok {
			return nil, err
		}

		groupname := args[0].(string)
		return getGroupInfo(groupname)
	}

	slhelp["user_mod"] = LibHelp{in: "username,options", out: "bool", action: "Modify an existing user. Options: uid, gid, home, shell, groups, create_home."}
	stdlib["user_mod"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("user_mod", args, 1, "2", "string", "map"); !ok {
			return nil, err
		}

		username := args[0].(string)
		options := args[1].(map[string]interface{})

		err = modifyUser(username, options)
		return err == nil, err
	}

	slhelp["group_mod"] = LibHelp{in: "groupname,options", out: "bool", action: "Modify an existing group. Options: gid."}
	stdlib["group_mod"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("group_mod", args, 1, "2", "string", "map"); !ok {
			return nil, err
		}

		groupname := args[0].(string)
		options := args[1].(map[string]interface{})

		err = modifyGroup(groupname, options)
		return err == nil, err
	}
}

// User and Group Management Implementation

type UserInfo struct {
	Username string   `json:"username"`
	UID      int      `json:"uid"`
	GID      int      `json:"gid"`
	Home     string   `json:"home"`
	Shell    string   `json:"shell"`
	Groups   []string `json:"groups"`
}

type GroupInfo struct {
	Name    string   `json:"name"`
	GID     int      `json:"gid"`
	Members []string `json:"members"`
}
