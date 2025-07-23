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
	"time"
)

var (
	Temp = map[string]int{}
)

func main() {
	var path string
	var host string
	var minimumFileSize string
	var minimumFileCount int
	var debug bool
	var googleChatUrl string

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "print-version",
		Aliases: []string{"V"},
		Usage:   "print only the version",
	}

	app := &cli.App{
		Name:    "run",
		Version: "v0.3.2",
		Usage:   "Find directories with many files or files that are large",
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
			&cli.IntFlag{
				Name:        "minimum-file-count",
				Aliases:     []string{"c"},
				Usage:       "Find directories with at least this many files. Use instead of file size search",
				Destination: &minimumFileCount,
				Value:       500,
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
			ignoredDirectories := cCtx.StringSlice("exclude")
			var ignoredDirectoryBuilder strings.Builder
			for _, ignoredDirectory := range ignoredDirectories {
				ignoredDirectoryBuilder.WriteString(fmt.Sprintf(" -or -path '%s'", ignoredDirectory))
			}

			// Check if we should search by file count or file size
			if minimumFileCount > 0 && minimumFileSize == "200MB" {
				// Search for directories with many files
				cmd := fmt.Sprintf("find %s -type d -not '(' -path '*/.git/*' -or -path '*/node_modules/*' -or -path '*/vendor/*' -or -path '*/.build/*' -or -path '*/tmp/*' -or -path '*/.*/*' %s ')' -exec sh -c 'count=$(find \"$1\" -maxdepth 1 -type f | wc -l); echo \"$count $1\"' _ {} \\; | sort -nr | head -n 25", path, ignoredDirectoryBuilder.String())
				
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
					fields := strings.Fields(line)
					if len(fields) < 2 {
						continue
					}
					fileCount, err := strconv.Atoi(fields[0])
					if err != nil {
						continue
					}
					dirPath := strings.Join(fields[1:], " ")
					
					if fileCount >= minimumFileCount {
						Temp[dirPath] = fileCount
					} else if debug {
						fmt.Printf("[INFO] IGNORED %s: %d files\n", dirPath, fileCount)
					}
				}
				
				// Print the output
				for path, count := range Temp {
					fmt.Printf("Found %s with %d files\n", path, count)
				}
				SendNotification(googleChatUrl, debug, host, "files")
			} else {
				// File and directory size search logic
				preferredUnit := "M"
				if strings.Contains(minimumFileSize, "G") {
					preferredUnit = "G"
				} else if strings.Contains(minimumFileSize, "K") {
					preferredUnit = "K"
				} else if strings.Contains(minimumFileSize, "B") {
					preferredUnit = "B"
				}

				replacer := strings.NewReplacer("K", "", "G", "", "M", "", "B", "")
				minimumFileSizeStripped := replacer.Replace(minimumFileSize)
				minimumFileSizeToInt, err := strconv.ParseInt(minimumFileSizeStripped, 10, 64)

				if err != nil {
					log.Fatalln("Could not parse number")
					return nil
				}

				minimumFileSizeAsFloat := float64(minimumFileSizeToInt)
				
				// Search for large files
				fileCmd := fmt.Sprintf("find %s -type f -not '(' -path '*/.git/*' -or -path '*/node_modules/*' -or -path '*/vendor/*'  -or -path '*/.build/*' -or -path '*/tmp/*' -or -path '*/.*/*' %s ')' -exec ls -alh {} \\;", path, ignoredDirectoryBuilder.String())
				
				// Search for large directories using du
				dirCmd := fmt.Sprintf("find %s -type d -not '(' -path '*/.git/*' -or -path '*/node_modules/*' -or -path '*/vendor/*'  -or -path '*/.build/*' -or -path '*/tmp/*' -or -path '*/.*/*' %s ')' -exec du -sh {} \\;", path, ignoredDirectoryBuilder.String())
				
				// Combine both commands
				combinedCmd := fmt.Sprintf("({ %s; %s; } | sort -hr -k1 | head -n 25)", fileCmd, dirCmd)
				
				if debug {
					fmt.Printf("[INFO] Running `%s`\n", combinedCmd)
				}
				stdout, err := exec.Command("bash", "-c", combinedCmd).Output()
				if err != nil {
					fmt.Printf("Failed to execute command: %s %s", err.Error(), combinedCmd)
					return nil
				}

				lines := strings.Split(string(stdout), "\n")
				for _, line := range lines {
					if strings.TrimSpace(line) == "" {
						continue
					}
					
					fields := strings.Fields(line)
					if len(fields) < 2 {
						continue
					}
					
					var itemPath, sizeStr string
					if len(fields) >= 9 && strings.Contains(line, "-") {
						// This is a file from ls -alh output
						itemPath = fields[8]
						sizeStr = fields[4]
					} else {
						// This is a directory from du -sh output
						sizeStr = fields[0]
						itemPath = strings.Join(fields[1:], " ")
					}

					sizeInPreferredUnit := ConvertFileSizeToPreferredUnit(sizeStr, preferredUnit)

                    if itemPath == "." {
                    // Don't include the current directory
					} else if sizeInPreferredUnit >= minimumFileSizeAsFloat {
						Temp[itemPath] = int(sizeInPreferredUnit)
					} else if debug {
						fmt.Printf("[INFO] IGNORED %s: %.4f%s\n", itemPath, sizeInPreferredUnit, preferredUnit)
					}
				}
				
				// Print the output
				for path, size := range Temp {
					fmt.Printf("Found %s %.2f%s\n", path, float64(size), preferredUnit)
				}
				SendNotification(googleChatUrl, debug, host, preferredUnit)
			}
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

func SendNotification(googleChatUrl string, debug bool, host string, unit string) {
    t := time.Now()
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
// 	text.WriteString("{\"text\": \"")
	 text.WriteString("{\"cards\": [{\"header\": {\"title\": \"Gigafind\", \"subtitle\": \"🔴 ")
	text.WriteString(host)
	 text.WriteString(" - ")
	 text.WriteString(t.Format(time.RFC3339))
	 text.WriteString("\"},")
     text.WriteString("\"sections\": [{ \"widgets\": [{ \"textParagraph\": { \"text\": \"")
	for path, value := range Temp {
		if unit == "files" {
			text.WriteString(fmt.Sprintf("⋅ [file] <b>%s</b> %d %s\n", path, value, unit))
		} else {
			text.WriteString(fmt.Sprintf("⋅ [dir] <b>%s</b> %d%s\n", path, value, unit))
		}
	}
// 	text.WriteString("\"}")
     text.WriteString("\"}}]}]}]}")
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
