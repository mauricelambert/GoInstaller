/*
    This file implements Windows specific features for GoInstaller
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

//go:build windows
// +build windows

package main

import (
    "os/exec"
    "syscall"
    "strings"
    "unsafe"
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

/*
    This function executes Windows commands.
*/
func execute_windows_command (command string) *exec.Cmd {
    cmd := exec.Command("cmd.exe")
    cmd.SysProcAttr = &syscall.SysProcAttr{
        CmdLine: "C:\\Windows\\System32\\cmd.exe /C " + strings.ReplaceAll(strings.ReplaceAll(command, "^", "^^"), "\"", "^\""),
    }
    return cmd
}

/*
    This function checks for privileges on Linux.
*/
func check_root() (bool, error) {
    return false, nil
}