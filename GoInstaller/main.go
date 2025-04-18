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
    "runtime"
    "os/exec"
    "errors"
    "io/fs"
    "embed"
    "fmt"
    "os"
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

type RegistryKey struct {
    value_name string
    value_data any
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

    if runtime.GOOS == "windows" {
        add_application_source_log(application_name)
        // new_registry_key(`SYSTEM\CurrentControlSet\Services\EventLog\Application\` + application_name, []RegistryKey{{"CustomSource", 1}, {"EventMessageFile", `%SystemRoot%\System32\EventCreate.exe`}, {"TypesSupported", 7}})
    } else {
        create_directory("/var/log/" + application_name)
    }

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
            cmd = execute_windows_command(command)
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
