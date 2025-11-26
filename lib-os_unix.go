//go:build linux && !freebsd && !openbsd && !netbsd && !dragonfly && !windows && !test

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func addUser(username string, options map[string]interface{}) error {
	if hasUserManagementCapability() {
		return cliAddUser(username, options)
	}
	return fmt.Errorf("user management requires CLI tools (useradd) and root privileges")
}

func removeUser(username string, options map[string]interface{}) error {
	if hasUserManagementCapability() {
		return cliRemoveUser(username, options)
	}
	return fmt.Errorf("user management requires CLI tools (userdel) and root privileges")
}

func addGroup(groupname string, options map[string]interface{}) error {
	if hasUserManagementCapability() {
		return cliAddGroup(groupname, options)
	}
	return fmt.Errorf("group management requires CLI tools (groupadd) and root privileges")
}

func removeGroup(groupname string) error {
	if hasUserManagementCapability() {
		return cliRemoveGroup(groupname)
	}
	return fmt.Errorf("group management requires CLI tools (groupdel) and root privileges")
}

func manageGroupMembership(username, groupname, action string) error {
	if hasUserManagementCapability() {
		return cliManageGroupMembership(username, groupname, action)
	}
	return fmt.Errorf("group membership management requires CLI tools (usermod) and root privileges")
}

func modifyUser(username string, options map[string]interface{}) error {
	if hasUserManagementCapability() {
		return cliModifyUser(username, options)
	}
	return fmt.Errorf("user modification requires CLI tools (usermod) and root privileges")
}

func modifyGroup(groupname string, options map[string]interface{}) error {
	if hasUserManagementCapability() {
		return cliModifyGroup(groupname, options)
	}
	return fmt.Errorf("group modification requires CLI tools (groupmod) and root privileges")
}

func getUserList() ([]UserInfo, error) {
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var users []UserInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 7 {
			uid, _ := strconv.Atoi(parts[2])
			gid, _ := strconv.Atoi(parts[3])
			user := UserInfo{
				Username: parts[0],
				UID:      uid,
				GID:      gid,
				Home:     parts[5],
				Shell:    parts[6],
				Groups:   []string{},
			}
			groups, _ := getUserGroups(user.Username)
			user.Groups = groups
			users = append(users, user)
		}
	}
	return users, scanner.Err()
}

func getGroupList() ([]GroupInfo, error) {
	file, err := os.Open("/etc/group")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var groups []GroupInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 4 {
			gid, _ := strconv.Atoi(parts[2])
			members := []string{}
			if parts[3] != "" {
				members = strings.Split(parts[3], ",")
			}
			group := GroupInfo{
				Name:    parts[0],
				GID:     gid,
				Members: members,
			}
			groups = append(groups, group)
		}
	}
	return groups, scanner.Err()
}

func getUserInfo(username string) (UserInfo, error) {
	users, err := getUserList()
	if err != nil {
		return UserInfo{}, err
	}
	for _, u := range users {
		if u.Username == username {
			return u, nil
		}
	}
	return UserInfo{}, fmt.Errorf("user not found: %s", username)
}

func getGroupInfo(groupname string) (GroupInfo, error) {
	groups, err := getGroupList()
	if err != nil {
		return GroupInfo{}, err
	}
	for _, g := range groups {
		if g.Name == groupname {
			return g, nil
		}
	}
	return GroupInfo{}, fmt.Errorf("group not found: %s", groupname)
}

func hasUserManagementCapability() bool {
	tools := []string{"useradd", "userdel", "groupadd", "groupdel"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return false
		}
	}
	if runtime.GOOS != "windows" {
		if !canWrite("/etc/passwd") {
			return false
		}
	}
	return true
}

// --- CLI helpers (not exported) ---

func cliAddUser(username string, options map[string]interface{}) error {
	cmd := exec.Command("useradd")

	// Only pass UID if explicitly set (not -1)
	if uid, ok := options["uid"].(int); ok && uid != -1 {
		cmd.Args = append(cmd.Args, "-u", strconv.Itoa(uid))
	}

	// Only pass GID if explicitly set (not -1)
	if gid, ok := options["gid"].(int); ok && gid != -1 {
		cmd.Args = append(cmd.Args, "-g", strconv.Itoa(gid))
	}

	// Only pass home directory if explicitly set and not empty
	if home, ok := options["home"].(string); ok && home != "" {
		cmd.Args = append(cmd.Args, "-d", home)
	}

	if shell, ok := options["shell"].(string); ok && shell != "" {
		cmd.Args = append(cmd.Args, "-s", shell)
	}
	if groups, ok := options["groups"].(string); ok && groups != "" {
		cmd.Args = append(cmd.Args, "-G", groups)
	}
	if createHome, ok := options["create_home"].(bool); ok && createHome {
		cmd.Args = append(cmd.Args, "-m")
	}
	cmd.Args = append(cmd.Args, username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("useradd failed: %v, output: %s", err, string(output))
	}
	return nil
}

func cliRemoveUser(username string, options map[string]interface{}) error {
	cmd := exec.Command("userdel")
	if removeHome, ok := options["remove_home"].(bool); ok && removeHome {
		cmd.Args = append(cmd.Args, "-r")
	}
	if removeFiles, ok := options["remove_files"].(bool); ok && removeFiles {
		cmd.Args = append(cmd.Args, "-f")
	}
	cmd.Args = append(cmd.Args, username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("userdel failed: %v, output: %s", err, string(output))
	}
	return nil
}

func cliAddGroup(groupname string, options map[string]interface{}) error {
	cmd := exec.Command("groupadd")

	// Only pass GID if explicitly set (not -1)
	if gid, ok := options["gid"].(int); ok && gid != -1 {
		cmd.Args = append(cmd.Args, "-g", strconv.Itoa(gid))
	}

	cmd.Args = append(cmd.Args, groupname)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("groupadd failed: %v, output: %s", err, string(output))
	}
	return nil
}

func cliRemoveGroup(groupname string) error {
	cmd := exec.Command("groupdel", groupname)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("groupdel failed: %v, output: %s", err, string(output))
	}
	return nil
}

func cliManageGroupMembership(username, groupname, action string) error {
	var cmd *exec.Cmd
	switch action {
	case "add":
		cmd = exec.Command("usermod", "-a", "-G", groupname, username)
	case "remove":
		currentGroups, err := getUserGroups(username)
		if err != nil {
			return err
		}
		var newGroups []string
		for _, group := range currentGroups {
			if group != groupname {
				newGroups = append(newGroups, group)
			}
		}
		if len(newGroups) > 0 {
			cmd = exec.Command("usermod", "-G", strings.Join(newGroups, ","), username)
		} else {
			cmd = exec.Command("usermod", "-G", "", username)
		}
	default:
		return fmt.Errorf("invalid action: %s (use 'add' or 'remove')", action)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("usermod failed: %v, output: %s", err, string(output))
	}
	return nil
}

func cliModifyUser(username string, options map[string]interface{}) error {
	cmd := exec.Command("usermod")

	// Only pass UID if explicitly set (not -1)
	if uid, ok := options["uid"].(int); ok && uid != -1 {
		cmd.Args = append(cmd.Args, "-u", strconv.Itoa(uid))
	}

	// Only pass GID if explicitly set (not -1)
	if gid, ok := options["gid"].(int); ok && gid != -1 {
		cmd.Args = append(cmd.Args, "-g", strconv.Itoa(gid))
	}

	// Only pass home directory if explicitly set and not empty
	if home, ok := options["home"].(string); ok && home != "" {
		cmd.Args = append(cmd.Args, "-d", home)
	}

	if shell, ok := options["shell"].(string); ok && shell != "" {
		cmd.Args = append(cmd.Args, "-s", shell)
	}
	if groups, ok := options["groups"].(string); ok && groups != "" {
		cmd.Args = append(cmd.Args, "-G", groups)
	}

	cmd.Args = append(cmd.Args, username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("usermod failed: %v, output: %s", err, string(output))
	}
	return nil
}

func cliModifyGroup(groupname string, options map[string]interface{}) error {
	cmd := exec.Command("groupmod")

	// Only pass GID if explicitly set (not -1)
	if gid, ok := options["gid"].(int); ok && gid != -1 {
		cmd.Args = append(cmd.Args, "-g", strconv.Itoa(gid))
	}

	cmd.Args = append(cmd.Args, groupname)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("groupmod failed: %v, output: %s", err, string(output))
	}
	return nil
}

// --- Helper for user groups ---
func getUserGroups(username string) ([]string, error) {
	// Parse /etc/group for group membership
	file, err := os.Open("/etc/group")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var groups []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 4 && parts[3] != "" {
			members := strings.Split(parts[3], ",")
			for _, member := range members {
				if member == username {
					groups = append(groups, parts[0])
				}
			}
		}
	}
	return groups, scanner.Err()
}

func canWrite(path string) bool {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// Unix-specific implementations for functions that use Unix syscalls

func umask(mask int) int {
	return syscall.Umask(mask)
}

func chroot(path string) error {
	return syscall.Chroot(path)
}

func canRead(path string) bool {
	return unix.Access(path, unix.R_OK) == nil
}
