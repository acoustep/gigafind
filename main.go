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
	var host string
	var minimumFileSize string
	var debug bool
	var googleChatUrl string

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "print-version",
		Aliases: []string{"V"},
		Usage:   "print only the version",
	}

	app := &cli.App{
		Name:    "run",
		Version: "v0.2.5",
		Usage:   "Find files that are large",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "path",
				Aliases:     []string{"p"},
				Usage:       "Search the selected path",
				Destination: &path,
				Value:       ".",
			},
			&cli.StringFlag{
				Name:        "minimum-file-size",
				Aliases:     []string{"m"},
				Usage:       "The minimum file size of the files to show. Anything below is ignored",
				Destination: &minimumFileSize,
				Value:       "200MB",
			},

			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "verbose output",
				Aliases:     []string{"d"},
				Destination: &debug,
				Value:       false,
			},
			&cli.StringFlag{
				Name:        "google-chat",
				Aliases:     []string{"g"},
				Usage:       "The Google Chat webhook to send results to",
				Destination: &googleChatUrl,
				Value:       "",
			},
			&cli.StringFlag{
				Name:        "host",
				Aliases:     []string{"H"},
				Usage:       "Name of the server to pass to webhooks",
				Destination: &host,
				Value:       "",
			},
			&cli.StringSliceFlag{
				Name:        "exclude",
				Usage:       "Pass multiple excluded directories",
				DefaultText: "--exclude \"*/.git/*\"",
			},
		},
		Action: func(cCtx *cli.Context) error {
			// First lets get the unit type from the minimum file type.
			// This will be anything from byte to Gigabyte
			preferredUnit := "M"
			if strings.Contains(minimumFileSize, "G") {
				preferredUnit = "G"
			} else if strings.Contains(minimumFileSize, "K") {
				preferredUnit = "K"
			} else if strings.Contains(minimumFileSize, "B") {
				preferredUnit = "B"
			}

			// Now we try to get the actual number. We do this by stripping out the unit (e.g. MB)
			// Then parsing as an integer
			replacer := strings.NewReplacer("K", "", "G", "", "M", "", "B", "")
			minimumFileSizeStripped := replacer.Replace(minimumFileSize)
			minimumFileSizeToInt, err := strconv.ParseInt(minimumFileSizeStripped, 10, 64)

			if err != nil {
				log.Fatalln("Could not parse number")
				return nil
			}

			//	// Set up a command to run
			minimumFileSizeAsFloat := float64(minimumFileSizeToInt)
			ignoredDirectories := cCtx.StringSlice("exclude")
			var ignoredDirectoryBuilder strings.Builder
			for _, ignoredDirectory := range ignoredDirectories {
				ignoredDirectoryBuilder.WriteString(fmt.Sprintf(" -or -path '%s'", ignoredDirectory))
			}

			cmd := fmt.Sprintf("find %s -type f -not '(' -path '*/.git/*' -or -path '*/node_modules/*' -or -path '*/vendor/*'  -or -path '*/.build/*' -or -path '*/tmp/*' -or -path '*/.*/*' %s ')' -exec ls -alh {} \\; | sort -hr -k5 | head -n 25", path, ignoredDirectoryBuilder.String())
			if debug {
				fmt.Printf("[INFO] Running `%s`\n", cmd)
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

				fileSizeInPreferredUnit := ConvertFileSizeToPreferredUnit(fileSize, preferredUnit)

				if fileSizeInPreferredUnit >= minimumFileSizeAsFloat {
					Temp[path] = fileSizeInPreferredUnit
				} else {
					fmt.Printf("[INFO] IGNORED %s: %.4f%s\n", path, fileSizeInPreferredUnit, preferredUnit)
				}
			}
			// Print the output
			for path, size := range Temp {
				fmt.Printf("Found %s %.2f%s\n", path, size, preferredUnit)
			}
			SendNotification(googleChatUrl, debug, host, preferredUnit)
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func ConvertFileSizeToPreferredUnit(fileSize string, preferredUnit string) float64 {
	replacer := strings.NewReplacer("K", "", "G", "", "M", "", "B", "")
	fileSizeStripped := replacer.Replace(fileSize)
	fileSizeToNumber, err := strconv.ParseFloat(fileSizeStripped, 64)
	if err != nil {
		log.Fatalln("Could not parse number")
		return 0
	}
	// File is already in the correct unit, return as-is
	if strings.Contains(fileSize, preferredUnit) {
		return fileSizeToNumber
	}

	fileUnit := "M"
	if strings.Contains(fileSize, "G") {
		fileUnit = "G"
	} else if strings.Contains(fileSize, "K") {
		fileUnit = "K"
	} else if strings.Contains(fileSize, "B") {
		fileUnit = "B"
	}

	// There is probably a much better way of doing this.
	switch {
	case fileUnit == "B" && preferredUnit == "K":
		return fileSizeToNumber / 1024
	case fileUnit == "B" && preferredUnit == "M":
		return (fileSizeToNumber / 1024) / 1024
	case fileUnit == "B" && preferredUnit == "G":
		return (fileSizeToNumber / 1024) / 1024 / 1024

	case fileUnit == "K" && preferredUnit == "B":
		return fileSizeToNumber * 1024
	case fileUnit == "K" && preferredUnit == "M":
		return fileSizeToNumber / 1024
	case fileUnit == "K" && preferredUnit == "G":
		return (fileSizeToNumber / 1024) / 1024

	case fileUnit == "M" && preferredUnit == "B":
		return (fileSizeToNumber * 1024) * 1024
	case fileUnit == "M" && preferredUnit == "K":
		return fileSizeToNumber * 1024
	case fileUnit == "M" && preferredUnit == "G":
		return fileSizeToNumber / 1024

	case fileUnit == "G" && preferredUnit == "B":
		return ((fileSizeToNumber * 1024) * 1024) * 1024
	case fileUnit == "G" && preferredUnit == "K":
		return (fileSizeToNumber * 1024) * 1024
	case fileUnit == "G" && preferredUnit == "M":
		return fileSizeToNumber * 1024
	}
	log.Fatalf("Could not convert %s %s %s\n", fileSize, preferredUnit, fileUnit)
	return 0.0
}

func SendNotification(googleChatUrl string, debug bool, host string, preferredUnit string) {
	if googleChatUrl == "" {
		fmt.Println("[INFO] No Google Chat webhook was provided.")
		return
	}
	if len(Temp) == 0 {
		if debug {
			fmt.Println("[INFO] No results so no Google Chat webhook was sent.")
		}
		return
	}
	var text strings.Builder
	text.WriteString("{\"text\": \"")
	if host != "" {
		text.WriteString(host)
		text.WriteString("\n\n")
	}
	for path, size := range Temp {
		text.WriteString(fmt.Sprintf("* %s: %.2f%s\n", path, size, preferredUnit))
	}
	text.WriteString("\"}")
	json := []byte(text.String())
	body := bytes.NewBuffer(json)
	client := &http.Client{}
	req, _ := http.NewRequest("POST", googleChatUrl, body)
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

}
