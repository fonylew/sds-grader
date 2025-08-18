#!/bin/sh

# Get the current directory name
DIR_NAME=$(basename "$PWD")

# Build for different operating systems and architectures
GOOS=darwin GOARCH=arm64 go build -o bin/${DIR_NAME}-macos-arm64 main.go
GOOS=darwin GOARCH=amd64 go build -o bin/${DIR_NAME}-macos-intel main.go
GOOS=linux GOARCH=amd64 go build -o bin/${DIR_NAME}-linux main.go
GOOS=windows GOARCH=amd64 go build -o bin/${DIR_NAME}-windows.exe main.go

# Make the binaries executable
chmod +x bin/*