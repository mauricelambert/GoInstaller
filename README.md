![GoInstaller Logo](https://mauricelambert.github.io/info/go/code/GoInstaller_small.png "GoInstaller logo")

# GoInstaller

## Description

This repository implements a cross-platform software installer written in Go.

### Features

 - Install software with privileges for all users on the system
 - Install program files
 - Install data files
 - Manage service files
     - *Timer* and *service* files on Linux
     - Executbale files with *service interface*
         - Create service with auto start
         - Start service
 - Run commands after files installations (for exemple to enable/start your service on Linux)

## Requirements

 - Go
 - Go standard library

## Usages

### Step 1: Download

> Download the source code and move in the directory

#### Git

```bash
git clone "https://github.com/mauricelambert/GoInstaller.git"
cd "GoInstaller"
```

#### Wget

```bash
wget https://github.com/mauricelambert/GoInstaller/archive/refs/heads/main.zip
unzip main.zip
cd GoInstaller-main
```

#### cURL

```bash
curl -O https://github.com/mauricelambert/GoInstaller/archive/refs/heads/main.zip
unzip main.zip
cd GoInstaller-main
```

### Step 2: prepare software files

> Create required directories and put your files inside
>> When you don't have any file for a directory add an empty file, minimum one file by directory is required

```bash
mkdir data
mkdir program
mkdir service

mv /path/to/my/data/files data
mv /path/to/my/program/files program
mv /path/to/my/service/files service
```

### Step 3: modify constants

> Modify constants in the source code: application name and commands to run at the end.

### Step 4: Compile your installer

```bash
go build -o installer.exe GoInstaller.go
```

## Links

 - [Github](https://github.com/mauricelambert/PyPePacker)

## License

Licensed under the [GPL, version 3](https://www.gnu.org/licenses/).
