# Gigafind

Gigafind is a command to help you find files in a directory that are larger than a specified value (Recursively).

## Running in Go 
```
go run main.go path=. --minimum-file-size=1M --debug --host=$hostname --exclude='*/*.txt"
```     

## Compile and run
```
go build
./gigafind path=. --minimum-file-size=1M --debug --host=$hostname --exclude='*/*.txt"
```