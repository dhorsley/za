//go:build windows && !linux && !freebsd && !openbsd && !netbsd && !dragonfly && !test

package main

import (
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "strconv"
)

func addUser(username string, options map[string]interface{}) error {
    if hasUserManagementCapability() {
        return cliAddUser(username, options)
    }
    return fmt.Errorf("Windows user management requires PowerShell and administrative privileges")
}

func removeUser(username string, options map[string]interface{}) error {
    if hasUserManagementCapability() {
        return cliRemoveUser(username, options)
    }
    return fmt.Errorf("Windows user management requires PowerShell and administrative privileges")
}

func addGroup(groupname string, options map[string]interface{}) error {
    if hasUserManagementCapability() {
        return cliAddGroup(groupname, options)
    }
    return fmt.Errorf("Windows group management requires PowerShell and administrative privileges")
}

func removeGroup(groupname string) error {
    if hasUserManagementCapability() {
        return cliRemoveGroup(groupname)
    }
    return fmt.Errorf("Windows group management requires PowerShell and administrative privileges")
}

func manageGroupMembership(username, groupname, action string) error {
    if hasUserManagementCapability() {
        return cliManageGroupMembership(username, groupname, action)
    }
    return fmt.Errorf("Windows group membership management requires PowerShell and administrative privileges")
}

func modifyUser(username string, options map[string]interface{}) error {
    if hasUserManagementCapability() {
        return cliModifyUser(username, options)
    }
    return fmt.Errorf("Windows user modification requires PowerShell and administrative privileges")
}

func modifyGroup(groupname string, options map[string]interface{}) error {
    if hasUserManagementCapability() {
        return cliModifyGroup(groupname, options)
    }
    return fmt.Errorf("Windows group modification requires PowerShell and administrative privileges")
}

func getUserList() ([]UserInfo, error) {
    cmd := "Get-LocalUser | ConvertTo-Json"
    psCmd := exec.Command("powershell", "-Command", cmd)
    output, err := psCmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("PowerShell Get-LocalUser failed: %v, output: %s", err, string(output))
    }
    var users []UserInfo
    var userArray []map[string]interface{}
    if err := json.Unmarshal(output, &userArray); err == nil {
        for _, userMap := range userArray {
            user := UserInfo{
                Username: getStringValue(userMap, "Name"),
                UID:      getIntValue(userMap, "SID"),
                GID:      getIntValue(userMap, "SID"),
                Home:     getStringValue(userMap, "HomeDirectory"),
                Shell:    getStringValue(userMap, "ScriptPath"),
                Groups:   []string{},
            }
            users = append(users, user)
        }
    } else {
        var userMap map[string]interface{}
        if err := json.Unmarshal(output, &userMap); err == nil {
            user := UserInfo{
                Username: getStringValue(userMap, "Name"),
                UID:      getIntValue(userMap, "SID"),
                GID:      getIntValue(userMap, "SID"),
                Home:     getStringValue(userMap, "HomeDirectory"),
                Shell:    getStringValue(userMap, "ScriptPath"),
                Groups:   []string{},
            }
            users = append(users, user)
        }
    }
    return users, nil
}

func getGroupList() ([]GroupInfo, error) {
    cmd := "Get-LocalGroup | ConvertTo-Json"
    psCmd := exec.Command("powershell", "-Command", cmd)
    output, err := psCmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("PowerShell Get-LocalGroup failed: %v, output: %s", err, string(output))
    }
    var groups []GroupInfo
    var groupArray []map[string]interface{}
    if err := json.Unmarshal(output, &groupArray); err == nil {
        for _, groupMap := range groupArray {
            group := GroupInfo{
                Name:    getStringValue(groupMap, "Name"),
                GID:     getIntValue(groupMap, "SID"),
                Members: []string{},
            }
            groups = append(groups, group)
        }
    } else {
        var groupMap map[string]interface{}
        if err := json.Unmarshal(output, &groupMap); err == nil {
            group := GroupInfo{
                Name:    getStringValue(groupMap, "Name"),
                GID:     getIntValue(groupMap, "SID"),
                Members: []string{},
            }
            groups = append(groups, group)
        }
    }
    return groups, nil
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
    // Check if PowerShell is available and we have admin privileges
    if _, err := exec.LookPath("powershell"); err != nil {
        return false
    }

    // Check if we can run PowerShell commands
    cmd := exec.Command("powershell", "-Command", "Get-LocalUser")
    if err := cmd.Run(); err != nil {
        return false
    }

    return true
}

// --- CLI helpers (not exported) ---

func cliAddUser(username string, options map[string]interface{}) error {
    cmd := "New-LocalUser -Name '" + username + "' -NoPassword"

    // Map Unix-style options to Windows PowerShell equivalents
    if uid, ok := options["uid"].(int); ok && uid != -1 {
        // Windows doesn't have numeric UIDs, but we can set a description
        cmd += " -Description 'UID: " + strconv.Itoa(uid) + "'"
    }

    if gid, ok := options["gid"].(int); ok && gid != -1 {
        // Windows doesn't have numeric GIDs, but we can set a description
        cmd += " -Description 'GID: " + strconv.Itoa(gid) + "'"
    }

    if home, ok := options["home"].(string); ok && home != "" {
        // Windows doesn't have direct home directory setting in New-LocalUser
        // This would need to be set after user creation
        cmd += " -Description 'Home: " + home + "'"
    }

    if shell, ok := options["shell"].(string); ok && shell != "" {
        // Windows doesn't have login shells, but we can set a description
        cmd += " -Description 'Shell: " + shell + "'"
    }

    if groups, ok := options["groups"].(string); ok && groups != "" {
        // Groups are added after user creation, not during creation
        cmd += " -Description 'Groups: " + groups + "'"
    }

    if createHome, ok := options["create_home"].(bool); ok && createHome {
        // Windows doesn't have direct home creation, but we can set a description
        cmd += " -Description 'CreateHome: true'"
    }

    psCmd := exec.Command("powershell", "-Command", cmd)
    output, err := psCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("PowerShell New-LocalUser failed: %v, output: %s", err, string(output))
    }
    return nil
}

func cliRemoveUser(username string, options map[string]interface{}) error {
    cmd := "Remove-LocalUser -Name '" + username + "'"

    // Map Unix-style options to Windows PowerShell equivalents
    if removeHome, ok := options["remove_home"].(bool); ok && removeHome {
        // Windows doesn't have direct home removal, but we can set a description
        cmd += " -Force"
    }

    if removeFiles, ok := options["remove_files"].(bool); ok && removeFiles {
        // Windows doesn't have direct file removal, but we can set a description
        cmd += " -Force"
    }

    psCmd := exec.Command("powershell", "-Command", cmd)
    output, err := psCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("PowerShell Remove-LocalUser failed: %v, output: %s", err, string(output))
    }
    return nil
}

func cliAddGroup(groupname string, options map[string]interface{}) error {
    cmd := "New-LocalGroup -Name '" + groupname + "'"

    // Map Unix-style options to Windows PowerShell equivalents
    if gid, ok := options["gid"].(int); ok && gid != -1 {
        // Windows doesn't have numeric GIDs, but we can set a description
        cmd += " -Description 'GID: " + strconv.Itoa(gid) + "'"
    }

    psCmd := exec.Command("powershell", "-Command", cmd)
    output, err := psCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("PowerShell New-LocalGroup failed: %v, output: %s", err, string(output))
    }
    return nil
}

func cliRemoveGroup(groupname string) error {
    cmd := "Remove-LocalGroup -Name '" + groupname + "'"
    psCmd := exec.Command("powershell", "-Command", cmd)
    output, err := psCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("PowerShell Remove-LocalGroup failed: %v, output: %s", err, string(output))
    }
    return nil
}

func cliManageGroupMembership(username, groupname, action string) error {
    var cmd string
    switch action {
    case "add":
        cmd = "Add-LocalGroupMember -Group '" + groupname + "' -Member '" + username + "'"
    case "remove":
        cmd = "Remove-LocalGroupMember -Group '" + groupname + "' -Member '" + username + "'"
    default:
        return fmt.Errorf("invalid action: %s (use 'add' or 'remove')", action)
    }
    psCmd := exec.Command("powershell", "-Command", cmd)
    output, err := psCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("PowerShell group membership command failed: %v, output: %s", err, string(output))
    }
    return nil
}

func cliModifyUser(username string, options map[string]interface{}) error {
    cmd := "Set-LocalUser -Name '" + username + "'"

    // Map Unix-style options to Windows PowerShell equivalents
    if uid, ok := options["uid"].(int); ok && uid != -1 {
        // Windows doesn't have numeric UIDs, but we can set a description
        cmd += " -Description 'UID: " + strconv.Itoa(uid) + "'"
    }

    if gid, ok := options["gid"].(int); ok && gid != -1 {
        // Windows doesn't have numeric GIDs, but we can set a description
        cmd += " -Description 'GID: " + strconv.Itoa(gid) + "'"
    }

    if home, ok := options["home"].(string); ok && home != "" {
        // Windows doesn't have direct home directory setting, but we can set a description
        cmd += " -Description 'Home: " + home + "'"
    }

    if shell, ok := options["shell"].(string); ok && shell != "" {
        // Windows doesn't have login shells, but we can set a description
        cmd += " -Description 'Shell: " + shell + "'"
    }

    if groups, ok := options["groups"].(string); ok && groups != "" {
        // Groups are managed separately, but we can set a description
        cmd += " -Description 'Groups: " + groups + "'"
    }

    psCmd := exec.Command("powershell", "-Command", cmd)
    output, err := psCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("PowerShell Set-LocalUser failed: %v, output: %s", err, string(output))
    }
    return nil
}

func cliModifyGroup(groupname string, options map[string]interface{}) error {
    cmd := "Set-LocalGroup -Name '" + groupname + "'"

    // Map Unix-style options to Windows PowerShell equivalents
    if gid, ok := options["gid"].(int); ok && gid != -1 {
        // Windows doesn't have numeric GIDs, but we can set a description
        cmd += " -Description 'GID: " + strconv.Itoa(gid) + "'"
    }

    psCmd := exec.Command("powershell", "-Command", cmd)
    output, err := psCmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("PowerShell Set-LocalGroup failed: %v, output: %s", err, string(output))
    }
    return nil
}

// --- JSON helpers ---
func getStringValue(m map[string]interface{}, key string) string {
    if val, ok := m[key]; ok {
        if str, ok := val.(string); ok {
            return str
        }
    }
    return ""
}

func getIntValue(m map[string]interface{}, key string) int {
    if val, ok := m[key]; ok {
        if str, ok := val.(string); ok {
            if i, err := strconv.Atoi(str); err == nil {
                return i
            }
        }
    }
    return 0
}

func canWrite(path string) bool {
    file, err := os.OpenFile(path, os.O_WRONLY, 0)
    if err != nil {
        return false
    }
    file.Close()
    return true
}

// Windows-specific implementations for functions that use Unix syscalls

func umask(mask int) int {
    // Windows doesn't have umask, return 0
    return 0
}

func chroot(path string) error {
    // Windows doesn't have chroot, return error
    return fmt.Errorf("chroot not supported on Windows")
}

func canRead(path string) bool {
    file, err := os.OpenFile(path, os.O_RDONLY, 0)
    if err != nil {
        return false
    }
    file.Close()
    return true
}
