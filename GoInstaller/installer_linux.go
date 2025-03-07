/*
    This file implements Linux specific features for GoInstaller
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

//go:build linux
// +build linux

package main

import (
    "os/exec"
    "os"
)

/*
    This function checks for privileges on Linux.
*/
func check_root() (bool, error) {
    return os.Geteuid() == 0, nil
}

/*
    This function adds the GUI program to the Windows menu.
*/
func add_to_windows_menu(executable_path string) {}

/*
    This function adds the program path to the SYSTEM environment variables (for all users).
*/
func add_to_system_path(new_path string) error {
    return nil
}

/*
    This function creates and starts a service on Windows.
*/
func create_service(executable_path string) {}

/*
    This function checks for privileges on Windows.
*/
func check_administrator() (bool, error) {
    return false, nil
}

/*
    This function executes Windows commands.
*/
func execute_windows_command (command string) *exec.Cmd {
    return exec.Command("sh", "-c", command)
}