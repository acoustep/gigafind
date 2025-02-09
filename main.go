package main

import (
	"bytes"
	"fmt"
	"github.com/urfave/cli/v2"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var (
	Temp = map[string]float64{}
)

func main() {
	var path string
	var size int
	var debug bool
	var googlechaturl string

	app := &cli.App{
		Name:  "run",
		Usage: "Find files that are large",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "path",
				Aliases:     []string{"p"},
				Usage:       "Search the selected path",
				Destination: &path,
				Value:       ".",
			},
			&cli.IntFlag{
				Name:        "size",
				Aliases:     []string{"s"},
				Usage:       "The size of the files to show in MB. Anything below is ignored",
				Destination: &size,
				Value:       200,
			},
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "verbose output",
				Aliases:     []string{"d"},
				Destination: &debug,
				Value:       false,
			},
			&cli.StringFlag{
				Name:        "googlechat",
				Aliases:     []string{"g"},
				Usage:       "The Google Chat webhook to send results to",
				Destination: &googlechaturl,
				Value:       "",
			},
		},
		Action: func(cCtx *cli.Context) error {
			// Set up a command to run
			sizeToFloat := float64(size)
			cmd := fmt.Sprintf("find %s -type f -not '(' -path '*/.git/*' -or -path '*/node_modules/*' -or -path '*/vendor/*'  -or -path '*/.build/*' -or -path '*/tmp/*' -or -path '*/.*/*' ')' -exec ls -alh {} \\; | sort -hr -k5 | head -n 25", path)
			if debug {
				fmt.Println("[INFO]", cmd)
			}
			stdout, err := exec.Command("bash", "-c", cmd).Output()
			if err != nil {
				fmt.Printf("Failed to execute command: %s %s", err.Error(), cmd)
				return nil
			}

			lines := strings.Split(string(stdout), "\n")
			for _, line := range lines {
				l := strings.Fields(line)
				if len(l) < 2 {
					break
				}
				path := l[8]
				fileSize := l[4]

				sizeToNumber := ConvertFileSizeToMb(fileSize)

				if sizeToNumber >= sizeToFloat {
					Temp[path] = sizeToNumber
				}
			}
			// Print the output

			for size, path := range Temp {
				fmt.Println(size, path)
			}
			SendNotification(googlechaturl, debug)
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func ConvertFileSizeToMb(fileSize string) float64 {
	var sizeToNumber float64
	if strings.Contains(fileSize, "K") {
		fileSize = strings.Replace(fileSize, "K", "", -1)
		sizeToNumber, _ = strconv.ParseFloat(fileSize, 64)
		sizeToNumber = sizeToNumber / 1024

	} else if strings.Contains(fileSize, "M") {
		fileSize = strings.Replace(fileSize, "M", "", -1)
		sizeToNumber, _ = strconv.ParseFloat(fileSize, 64)
		sizeToNumber = sizeToNumber
	} else if strings.Contains(fileSize, "G") {
		fileSize = strings.Replace(fileSize, "G", "", -1)
		sizeToNumber, _ = strconv.ParseFloat(fileSize, 64)
		sizeToNumber = sizeToNumber * 1024
	} else {
		// Bytes
		fileSize = strings.Replace(fileSize, "B", "", -1)
		sizeToNumber, _ = strconv.ParseFloat(fileSize, 64)
		sizeToNumber = sizeToNumber / 1024 / 1024
	}
	return sizeToNumber
}

func SendNotification(googlechaturl string, debug bool) {
	if googlechaturl == "" {
		fmt.Println("[INFO] No Google Chat webhook was provided.")
		return
	}
	if len(Temp) == 0 {
		if debug {
			fmt.Println("[INFO] No results so no Google Chat webhook was sent.")
		}
		return
	}
	json := []byte(`{"text": "test"}`)
	body := bytes.NewBuffer(json)
	client := &http.Client{}
	req, err := http.NewRequest("POST", googlechaturl, body)
	req.Header.Add("Content-Type", "application/json")
	parseFormErr := req.ParseForm()
	if parseFormErr != nil {
		fmt.Println(parseFormErr)
	}

	// Fetch Request
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println("Failure : ", err)
	}
	// Read Response Body
	if debug {
		respBody, _ := io.ReadAll(resp.Body)

		// Display Results
		fmt.Println("response Status : ", resp.Status)
		fmt.Println("response Headers : ", resp.Header)
		fmt.Println("response Body : ", string(respBody))
	}

	return

}
