package main

import (
	"fmt"
	"os/exec"
	"strings"
)

var (
	Temp = map[string]string{}
)

func main() {
	// Set up a command to run
	cmd := "find . -type f -not '(' -path '*/.git/*' -or -path '*/node_modules/*' -or -path '*/vendor/*'  -or -path '*/.build/*' -or -path '*/tmp/*' -or -path '*/.*/*' ')' -exec ls -alh {} \\; | sort -hr -k5 | head -n 25"
	stdout, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		fmt.Printf("Failed to execute command: %s %s", err.Error(), cmd)
		return
	}
	// cmd := exec.Command("du", "-h", "-hd1", "/users/mitchellstanley/Code/go")
	//cmd := exec.Command("find", ".", "-type", "f", "-not", "'(' -path '*/.git/*' -or -path '*/node_modules/*' -or -path '*/vendor/*'  -or -path '*/.build/*'  -or -path '*/tmp/*' ')'", "-exec", "ls", "-alh", "{}", "\\;", "|", "sort", "-hr", "-k5", "|", "head", "-n", "25")
	//stdout, err := cmd.Output()

	//if err != nil {
	//	fmt.Println(err.Error())
	//	return
	//}

	// TODO: filter out directories and show files above certain file size
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		l := strings.Fields(line)
		if len(l) < 2 {
			break
		}
		path := l[8]
		size := l[4]
		Temp[path] = size
	}
	// Print the output

	for size, path := range Temp {
		fmt.Println(size, path)
	}
}
