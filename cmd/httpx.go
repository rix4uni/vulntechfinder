package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// Structure to map the JSON data
type HttpxTechData struct {
	Host  string   `json:"host"`
	Tech  []string `json:"tech"`
	Count int      `json:"count,omitempty"`
}

// httpxCmd represents the httpx command
var httpxCmd = &cobra.Command{
	Use:   "httpx",
	Short: "Run httpx scans on multiple hosts in parallel, filtering by technology stack (reads JSON from stdin or runs techx).",
	Long: `The 'httpx' command reads JSON (objects with {"host":..., "tech":[...]}) from stdin, or if the stdin doesn't contain JSON it will run the external 'techx -silent -json' command (feeding stdin to techx) and consume its JSON output.

Examples:
  echo "hackerone.com" | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt

  cat subs.txt | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt

  cat techx-output.json | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt
`,
	Run: func(cmd *cobra.Command, args []string) {
		httpxCmdStr, _ := cmd.Flags().GetString("cmd")
		verbose, _ := cmd.Flags().GetBool("verbose")
		process, _ := cmd.Flags().GetBool("process")
		parallel, _ := cmd.Flags().GetInt("parallel")
		Output, _ := cmd.Flags().GetString("output")
		excludeTech, _ := cmd.Flags().GetString("exclude-tech")
		includeTech, _ := cmd.Flags().GetString("include-tech")

		if httpxCmdStr == "" {
			fmt.Println("Usage: vulntechx httpx --cmd <httpx command> [--parallel N] [--output file]")
			os.Exit(1)
		}

		if parallel <= 0 {
			parallel = 50
		}

		// Parse exclude and include lists (support both comma-separated and file paths)
		excludeList, err := HttpxparseTechInput(excludeTech)
		if err != nil {
			fmt.Printf("Error reading exclude-tech input: %s\n", err)
			os.Exit(1)
		}

		includeList, err := HttpxparseTechInput(includeTech)
		if err != nil {
			fmt.Printf("Error reading include-tech input: %s\n", err)
			os.Exit(1)
		}

		// Validate that both exclude and include are not used together
		if len(excludeList) > 0 && len(includeList) > 0 {
			fmt.Println("Error: Cannot use both --exclude-tech and --include-tech flags together")
			os.Exit(1)
		}

		if verbose {
			if len(excludeList) > 0 {
				fmt.Printf("Excluding technologies: %v\n", excludeList)
			}
			if len(includeList) > 0 {
				fmt.Printf("Including only these technologies: %v\n", includeList)
			}
		}

		// Read all stdin
		stdinBytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("Error reading stdin: %s\n", err)
			os.Exit(1)
		}

		if len(stdinBytes) == 0 {
			fmt.Println("No input provided on stdin. Provide JSON or pipe host list into this command.")
			os.Exit(1)
		}

		trimmed := strings.TrimSpace(string(stdinBytes))

		var reader io.Reader

		// Detect if stdin already contains JSON (starts with [ or {). If not, run techx -silent -json
		if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
			if verbose {
				fmt.Println("Detected JSON on stdin — parsing directly.")
			}
			reader = strings.NewReader(string(stdinBytes))
		} else {
			if verbose {
				fmt.Println("No JSON detected on stdin — running 'techx -silent -json' and piping stdin to it.")
			}
			// Run techx -silent -json, feeding stdinBytes into its stdin, and capture stdout
			techxCmd := exec.Command("sh", "-c", "techx -silent -json")
			techxCmd.Stdin = strings.NewReader(string(stdinBytes))
			out, err := techxCmd.Output()
			if err != nil {
				fmt.Printf("Error running techx: %s\n", err)
				os.Exit(1)
			}
			reader = strings.NewReader(string(out))
		}

		// Open the output file for appending if the --output flag is specified
		var outputFile *os.File
		if Output != "" {
			outputFile, err = os.OpenFile(Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Printf("Error opening output file: %s\n", err)
				os.Exit(1)
			}
			defer outputFile.Close()
		}

		decoder := json.NewDecoder(reader)
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, parallel) // Limit the number of parallel executions

		for {
			var HttpxtechData HttpxTechData
			if err := decoder.Decode(&HttpxtechData); err == io.EOF {
				break
			} else if err != nil {
				fmt.Printf("Error decoding JSON: %s\n", err)
				os.Exit(1)
			}

			// Skip processing if tech is nil
			if HttpxtechData.Tech == nil {
				if verbose {
					fmt.Printf("Skipping URL with tech field as null: %s\n", HttpxtechData.Host)
				}
				continue
			}

			// Build normalized list of tech names (extract part before ":" and lowercase)
			var normalizedTechs []string
			for _, t := range HttpxtechData.Tech {
				parts := strings.SplitN(t, ":", 2)
				if len(parts) == 0 {
					continue
				}
				techName := strings.TrimSpace(parts[0])
				if techName == "" {
					continue
				}
				// ignore technologies with spaces (same as before)
				if strings.Contains(techName, " ") {
					if verbose {
						fmt.Printf("Ignoring tech with spaces: %q\n", techName)
					}
					continue
				}
				norm := strings.ToLower(techName)
				normalizedTechs = append(normalizedTechs, norm)
			}

			if len(normalizedTechs) == 0 {
				if verbose {
					fmt.Printf("SKIPPED: %s - no valid tech names found\n", HttpxtechData.Host)
				}
				continue
			}

			// For each tech (one httpx run per tech), apply include/exclude and launch job
			for _, tech := range normalizedTechs {
				// Apply include/exclude logic
				if len(includeList) > 0 {
					if !contains(includeList, tech) {
						if verbose {
							fmt.Printf("Skipping tech %s for host %s (not in include list)\n", tech, HttpxtechData.Host)
						}
						continue
					}
				} else if len(excludeList) > 0 {
					if contains(excludeList, tech) {
						if verbose {
							fmt.Printf("Skipping tech %s for host %s (in exclude list)\n", tech, HttpxtechData.Host)
						}
						continue
					}
				}

				wg.Add(1)
				semaphore <- struct{}{} // acquire
				go func(host, techName string) {
					defer wg.Done()
					defer func() { <-semaphore }() // release

					// Build command string for this techName
					var cmdStr string
					if strings.Contains(httpxCmdStr, "-path") {
						// Try candidate paths:
						// 1) techName as provided (maybe user passed "jenkins.txt")
						// 2) ./wordlists/<techName>
						// 3) ./wordlists/<techName>.txt
						// If none exist, fallback to inline techName replacement.
						candidate := techName
						var pathToUse string
						if fileExists(candidate) {
							pathToUse = candidate
							if verbose {
								fmt.Printf("Using existing path for tech %s: %s\n", techName, pathToUse)
							}
						} else {
							wordlistsDir := "/root/wordlists"
							try1 := filepath.Join(wordlistsDir, candidate)
							try2 := try1
							if !strings.HasSuffix(strings.ToLower(try1), ".txt") {
								try2 = try1 + ".txt"
							}
							if fileExists(try1) {
								pathToUse = try1
								if verbose {
									fmt.Printf("Found wordlist path for tech %s: %s\n", techName, pathToUse)
								}
							} else if fileExists(try2) {
								pathToUse = try2
								if verbose {
									fmt.Printf("Found wordlist path for tech %s: %s\n", techName, pathToUse)
								}
							} else {
								// fallback to inline replacement
								pathToUse = techName
								if verbose {
									fmt.Printf("No wordlist found for tech %s; falling back to inline replacement\n", techName)
								}
							}
						}
						cmdStr = strings.Replace(httpxCmdStr, "{tech}", pathToUse, -1)
					} else {
						// Default inline replacement
						cmdStr = strings.Replace(httpxCmdStr, "{tech}", techName, -1)
					}

					if process {
						fmt.Printf("Running httpx for host %s tech %s: [echo \"%s\" | %s]\n", host, techName, host, cmdStr)
					}

					// Execute httpx command for this host/tech
					cmd := exec.Command("sh", "-c", cmdStr)
					cmd.Stdin = strings.NewReader(host)
					stdoutPipe, _ := cmd.StdoutPipe()
					stderrPipe, _ := cmd.StderrPipe()

					if err := cmd.Start(); err != nil {
						if verbose {
							fmt.Printf("Error starting httpx command for %s (%s): %s\n", host, techName, err)
						}
						return
					}

					scanner := bufio.NewScanner(io.MultiReader(stdoutPipe, stderrPipe))
					for scanner.Scan() {
						line := scanner.Text()
						fmt.Println(line)
						if Output != "" {
							if _, err := outputFile.WriteString(line + "\n"); err != nil && verbose {
								fmt.Printf("Error writing to output file: %s\n", err)
							}
						}
					}

					if err := cmd.Wait(); err != nil && verbose {
						fmt.Printf("Error waiting for httpx command for %s (%s): %s\n", host, techName, err)
					}
				}(HttpxtechData.Host, tech)
			}
		}

		wg.Wait() // Wait for all goroutines to finish
	},
}

// Helper function to parse tech input (supports both comma-separated values and file paths)
func HttpxparseTechInput(input string) ([]string, error) {
	if input == "" {
		return []string{}, nil
	}

	// Check if input is a file that exists
	if _, err := os.Stat(input); err == nil {
		// It's a file, read lines from the file
		file, err := os.Open(input)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		var techs []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			tech := strings.TrimSpace(scanner.Text())
			if tech != "" {
				techs = append(techs, strings.ToLower(tech))
			}
		}
		return techs, scanner.Err()
	}

	// Otherwise, treat as comma-separated list
	techs := strings.Split(input, ",")
	for i := range techs {
		techs[i] = strings.TrimSpace(strings.ToLower(techs[i]))
	}
	return techs, nil
}

// Utility function to check if an item is in a slice
func Httpxcontains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// fileExists convenience
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func init() {
	rootCmd.AddCommand(httpxCmd)

	httpxCmd.Flags().StringP("cmd", "c", "", "The httpx command template")
	httpxCmd.Flags().Bool("verbose", false, "Enable verbose output for debugging purposes.")
	httpxCmd.Flags().Bool("process", false, "Show which URL is running on httpx.")
	httpxCmd.Flags().Int("parallel", 50, "Number of parallel processes")
	httpxCmd.Flags().StringP("output", "o", "", "File to save output")
	httpxCmd.Flags().StringP("exclude-tech", "e", "", "Comma-separated list of technologies to exclude, or path to a file with technologies (one per line)")
	httpxCmd.Flags().StringP("include-tech", "i", "", "Comma-separated list of technologies to include (only these will be processed), or path to a file with technologies (one per line)")
}
