/*
    This file implements an installer for Linux and Windows softwares
    Copyright (C) 2025  Maurice Lambert

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// go build -o installer.exe GoInstaller.go

package main

import (
    "path/filepath"
    "syscall"
    "runtime"
    "strings"
    "os/exec"
    "unsafe"
    "errors"
    "io/fs"
    "embed"
    "fmt"
    "os"
)

const (
    SECURITY_BUILTIN_DOMAIN_RID = 0x00000020
    DOMAIN_ALIAS_RID_ADMINS     = 0x00000220
    SERVICE_RUNNING             = 0x00000004
    SC_MANAGER_CREATE_SERVICE   = 0x00000002
    SERVICE_WIN32_OWN_PROCESS   = 0x00000010
    SERVICE_AUTO_START          = 0x00000002
    SERVICE_ERROR_NORMAL        = 0x00000001
    SERVICE_ALL_ACCESS          = 0x000F01FF
    HKEY_LOCAL_MACHINE          = 0x80000002
    KEY_ALL_ACCESS              = 0xF003F
    REG_EXPAND_SZ               = 2
)

var (
    modAdvapi32               = syscall.NewLazyDLL("advapi32.dll")
    allocateAndInitializeSid  = modAdvapi32.NewProc("AllocateAndInitializeSid")
    checkTokenMembership      = modAdvapi32.NewProc("CheckTokenMembership")
    freeSid                   = modAdvapi32.NewProc("FreeSid")
    openSCManager             = modAdvapi32.NewProc("OpenSCManagerW")
    createService             = modAdvapi32.NewProc("CreateServiceW")
    closeServiceHandle        = modAdvapi32.NewProc("CloseServiceHandle")
    startService              = modAdvapi32.NewProc("StartServiceW")
    regOpenKeyEx              = modAdvapi32.NewProc("RegOpenKeyExW")
    regCloseKey               = modAdvapi32.NewProc("RegCloseKey")
    regQueryValueEx           = modAdvapi32.NewProc("RegQueryValueExW")
    regSetValueEx             = modAdvapi32.NewProc("RegSetValueExW")
    kernel32                  = syscall.NewLazyDLL("kernel32.dll")
    createSymbolicLinkW       = kernel32.NewProc("CreateSymbolicLinkW")

    SECURITY_NT_AUTHORITY     = [6]byte{0, 0, 0, 0, 0, 5}
)

//go:embed data/*
var data_files embed.FS
//go:embed program/*
var program_files embed.FS
//go:embed gui/*
var program_gui_files embed.FS
//go:embed service/*
var service_files embed.FS
const application_name = "${APPLICATION_NAME}"

type File struct {
    filetype string
    path string
    name string
    data []byte
    callback func(string)
}

/*
    The main function to starts the installer.

    1. Check privileges
    2. Create directories
    3. Install/Write files
    4. Run commands
*/
func main() {
    priviliges, err := check_privileges()
    if err != nil || !priviliges {
        fmt.Fprintf(os.Stderr, "This software installer require privileges.\n")
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error checking privileges: %v\n", err)
        }
        os.Exit(5)
    }

    program_directory, data_directory := create_directories()
    process_directories(program_directory, data_directory)

    if runtime.GOOS == "windows" {
        add_to_system_path(program_directory)
    }

    run_commands()

    fmt.Println("Installation completed successfully!")
    os.Exit(0)
}

/*
    This function creates software directories.
*/
func create_directories() (string, string) {
    var program_files_dir, program_data_dir string
    if runtime.GOOS == "windows" {
        program_files_dir = os.Getenv("PROGRAMFILES")
        program_data_dir = os.Getenv("PROGRAMDATA")
    } else {
        program_files_dir = "/usr/local/bin"
        program_data_dir = "/var/lib"
    }

    program_files_dir = filepath.Join(program_files_dir, application_name)
    program_data_dir = filepath.Join(program_data_dir, application_name)
    create_directory(program_files_dir)
    create_directory(program_data_dir)

    return program_files_dir, program_data_dir
}

/*
    This function creates a directory and exit on error.
*/
func create_directory(path string) {
    err := os.MkdirAll(path, os.ModePerm)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", path, err)
        os.Exit(1)
    }
}

/*
    This function calls process_directory function multiples
    times and defines the system directory to use.
*/
func process_directories(program_directory, data_directory string) {
    file := File{}
    file.path = data_directory
    file.filetype = "data"
    process_directory(data_files, file)

    file.path = program_directory
    file.filetype = "program"
    process_directory(program_files, file)

    file.path = program_directory
    file.filetype = "gui"
    if runtime.GOOS == "windows" {
        file.callback = add_to_windows_menu
    }
    process_directory(program_gui_files, file)

    if runtime.GOOS == "windows" {
        file.path = program_directory
        file.callback = create_service
    } else {
        file.path = "/etc/systemd/system/"
    }

    file.filetype = "service"
    process_directory(service_files, file)
}

/*
    This function reads directory from embeded files.
*/
func process_directory(files embed.FS, file File) {
    file_entries, err := files.ReadDir(file.filetype)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading embedded files (%s): %v\n", file.filetype, err)
        return
    }

    for _, entry := range file_entries {
        process_file(files, entry, file)
    }
}


/*
    This function reads file from embeded files.
*/
func process_file(files embed.FS, entry fs.DirEntry, file File) {
    file.name = entry.Name()
    file_path := file.filetype + "/" + file.name

    file_data, err := files.ReadFile(file_path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", file.name, err)
        return
    }
    file.data = file_data

    fullfilepath := write_file(file)

    if file.callback != nil {
        file.callback(fullfilepath)
    }
}

/*
    This function checks if file exists.
*/
func file_exists(file_path string) bool {
    _, err := os.Stat(file_path)
    return !errors.Is(err, os.ErrNotExist)
}

/*
    This function writes the file content or exit on error.
*/
func write_file(file File) string {
    fullfilepath := filepath.Join(file.path, file.name)
    if file.filetype != "data" || !file_exists(fullfilepath) {
        err := os.WriteFile(fullfilepath, file.data, 0755)

        if err != nil {
            fmt.Fprintf(os.Stderr, "Error writing file %s: %v\n", fullfilepath, err)
            os.Exit(2)
        }

        fmt.Printf("Installed: %s\n", fullfilepath)
    } else {
        fmt.Printf("Data file already exists: %s\n", fullfilepath)
    }
    return fullfilepath
}

/*
    This function executes system commands when is
    required for the software install.
*/
func run_commands() {
    var commands []string

    if runtime.GOOS == "windows" {
        commands = []string{${WINDOWS_COMMANDS}} // Insert your Windows commands here
    } else {
        commands = []string{${LINUX_COMMANDS}} // Insert your Linux commands here
    }

    for _, command := range commands {
        var cmd *exec.Cmd
        if runtime.GOOS == "windows" {
            cmd = exec.Command("cmd.exe")
            cmd.SysProcAttr = &syscall.SysProcAttr{
                CmdLine: "C:\\Windows\\System32\\cmd.exe /C " + strings.ReplaceAll(strings.ReplaceAll(command, "^", "^^"), "\"", "^\""),
            }
        } else {
            cmd = exec.Command("sh", "-c", command)
        }

        // err := cmd.Run()
        out, err := cmd.CombinedOutput()

        if err != nil {
            fmt.Fprintf(os.Stderr, "Command error: %v\n", err)
        }

        fmt.Printf("Ouput: %s\n", string(out))
    }
}

/*
    This function checks if process have privileges
    to install the software.
*/
func check_privileges() (bool, error) {
    switch runtime.GOOS {
    case "windows":
        return check_administrator()
    default:
        return check_root()
    }
}

/*
    This function checks for privileges on Windows.
*/
func check_administrator() (bool, error) {
    var sid *syscall.SID
    ret, _, err := allocateAndInitializeSid.Call(
        uintptr(unsafe.Pointer(&SECURITY_NT_AUTHORITY)),
        2,
        uintptr(SECURITY_BUILTIN_DOMAIN_RID),
        uintptr(DOMAIN_ALIAS_RID_ADMINS),
        0, 0, 0, 0, 0, 0,
        uintptr(unsafe.Pointer(&sid)),
    )
    if ret == 0 {
        fmt.Fprintf(os.Stderr, "Error calling AllocateAndInitializeSid: %v\n", err)
        return false, err
    }

    var is_member int32
    ret, _, err = checkTokenMembership.Call(
        0,
        uintptr(unsafe.Pointer(sid)),
        uintptr(unsafe.Pointer(&is_member)),
    )
    ret2, _, err2 := freeSid.Call(uintptr(unsafe.Pointer(sid)))

    if ret == 0 {
        fmt.Fprintf(os.Stderr, "Error checking token membership: %v\n", err)
        return false, err
    }

    if ret2 != 0 {
        fmt.Fprintf(os.Stderr, "Error checking token membership: %v\n", err2)
        return false, err
    }

    return bool(is_member != 0), nil
}

/*
    This function checks for privileges on Linux.
*/
func check_root() (bool, error) {
    return os.Geteuid() == 0, nil
}

/*
    This function creates and starts a service on Windows.
*/
func create_service(executable_path string) {
    service_manager, _, err := openSCManager.Call(0, 0, uintptr(SC_MANAGER_CREATE_SERVICE))
    if service_manager == 0 {
        fmt.Fprintf(os.Stderr, "failed to open Service Control Manager: %v\n", err)
        return
    }

    service_name_pointer, err := syscall.UTF16PtrFromString(application_name)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to generate UTF16 service name: %v\n", err)
        return
    }
    executable_path_pointer, err := syscall.UTF16PtrFromString(executable_path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to generate UTF16 service executable path: %v\n", err)
        return
    }

    service_handle, _, err := createService.Call(
        service_manager,
        uintptr(unsafe.Pointer(service_name_pointer)),
        uintptr(unsafe.Pointer(service_name_pointer)),
        uintptr(SERVICE_ALL_ACCESS),
        uintptr(SERVICE_WIN32_OWN_PROCESS),
        uintptr(SERVICE_AUTO_START),
        uintptr(SERVICE_ERROR_NORMAL),
        uintptr(unsafe.Pointer(executable_path_pointer)),
        0,
        0,
        0,
        0,
        0,
    )
    if service_handle == 0 {
        fmt.Fprintf(os.Stderr, "failed to create service: %v\n", err)
        return
    }

    ret, _, err := startService.Call(service_handle, 0, 0)
    if ret == 0 {
        fmt.Fprintf(os.Stderr, "failed to start service: %v\n", err)
        return
    }

    closeServiceHandle.Call(service_handle)
    closeServiceHandle.Call(service_manager)
    fmt.Printf("Service is running.")
}

/*
    This function adds the program path to the SYSTEM environment variables (for all users).
*/
func add_to_system_path(new_path string) error {
    var handle syscall.Handle
    key := syscall.StringToUTF16Ptr(`SYSTEM\CurrentControlSet\Control\Session Manager\Environment`)
    
    _, _, err := regOpenKeyEx.Call(HKEY_LOCAL_MACHINE, uintptr(unsafe.Pointer(key)), 0, KEY_ALL_ACCESS, uintptr(unsafe.Pointer(&handle)))
    if err != nil && err != syscall.Errno(0) {
        return fmt.Errorf("failed to open registry key: %v", err)
    }
    defer regCloseKey.Call(uintptr(handle))

    var buffer_size uint32
    var value_type uint32
    _, _, err = regQueryValueEx.Call(uintptr(handle), uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Path"))), uintptr(0), uintptr(unsafe.Pointer(&value_type)), uintptr(0), uintptr(unsafe.Pointer(&buffer_size)))
    if err != nil && err != syscall.Errno(0) {
        return fmt.Errorf("Error getting buffer size: %v", err)
    }

    buffer := make([]uint16, buffer_size / 2)
    _, _, err = regQueryValueEx.Call(uintptr(unsafe.Pointer(handle)), uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Path"))), uintptr(0), uintptr(unsafe.Pointer(&value_type)), uintptr((unsafe.Pointer(&buffer[0]))), uintptr(unsafe.Pointer(&buffer_size)))
    if err != nil && err != syscall.Errno(0) {
        return fmt.Errorf("failed to query Path value: %v", err)
    }

    current_path := syscall.UTF16ToString(buffer)
    if current_path[len(current_path)-1] != ';' {
        current_path += ";"
    } else {
        new_path += ";"
    }
    new_path_value := current_path + new_path

    path_ptr := syscall.StringToUTF16Ptr(new_path_value)
    _, _, err = regSetValueEx.Call(uintptr(unsafe.Pointer(handle)), uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Path"))), 0, REG_EXPAND_SZ, uintptr((unsafe.Pointer(path_ptr))), uintptr(uint32(len(new_path_value)*2)))
    if err != nil && err != syscall.Errno(0) {
        return fmt.Errorf("failed to set new Path value: %v", err)
    }

    return nil
}

/*
    This function adds the GUI program to the Windows menu.
*/
func add_to_windows_menu(executable_path string) {
    shortcut_path := os.Getenv("ProgramData") + "\\Microsoft\\Windows\\Start Menu\\Programs\\" + application_name + ".lnk"
    symlink_path_pointer, err := syscall.UTF16PtrFromString(shortcut_path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to get UTF16 symlink path: %v\n", err)
        return
    }
    executable_path_pointer, err := syscall.UTF16PtrFromString(executable_path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to get UTF16 executable path: %v\n", err)
        return
    }

    flags := uint32(0)
    /*if isDir {
        flags = 1 // SYMBOLIC_LINK_FLAG_DIRECTORY
    }*/

    ret, _, err := createSymbolicLinkW.Call(
        uintptr(unsafe.Pointer(symlink_path_pointer)),
        uintptr(unsafe.Pointer(executable_path_pointer)),
        uintptr(flags),
    )

    if ret == 0 {
        fmt.Fprintf(os.Stderr, "failed to generate the symlink: %v\n", err)
        return
    }
}