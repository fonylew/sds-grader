#!/bin/sh
GOOS=darwin GOARCH=arm64 go build -o bin/activity7-macos-arm64 main.go
GOOS=darwin GOARCH=amd64 go build -o bin/activity7-macos-intel main.go
GOOS=linux GOARCH=amd64 go build -o bin/activity7-linux main.go
GOOS=windows GOARCH=amd64 go build -o bin/activity7-windows.exe main.go
chmod +x bin/*
