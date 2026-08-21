package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/rix4uni/vulntechfinder/banner"
	"github.com/spf13/pflag"
)

// Structure to map the JSON data
type TechData struct {
	Host string   `json:"host"`
	Tech []string `json:"tech"`
}

func main() {
	var (
		cmdStr      string
		verbose     bool
		process     bool
		parallel    int
		output      string
		excludeTech string
		includeTech string
		noResume    bool
		version     bool
		silent      bool
	)

	pflag.StringVar(&cmdStr, "cmd", "", "Command template with {tech} placeholder to execute (e.g. 'nuclei -tags {tech}')")
	pflag.IntVar(&parallel, "parallel", 50, "Number of concurrent parallel processes")
	pflag.StringVar(&output, "output", "", "File path to save the output results")
	pflag.StringVar(&excludeTech, "exclude", "", "Comma-separated list or file path of technologies to exclude")
	pflag.StringVar(&includeTech, "include", "", "Comma-separated list or file path of technologies to include (only these are scanned)")
	pflag.BoolVar(&noResume, "no-resume", false, "Disable resume functionality and start a fresh scan")
	pflag.BoolVar(&process, "process", false, "Display the command being executed for each host")
	pflag.BoolVar(&verbose, "verbose", false, "Enable verbose output for debugging")
	pflag.BoolVar(&silent, "silent", false, "Silent mode.")
	pflag.BoolVar(&version, "version", false, "Print the tool version and exit")

	pflag.Parse()

	if version {
		banner.PrintVersion()
		return
	}

	if !silent {
		banner.PrintBanner()
	}

	if cmdStr == "" {
		fmt.Println("Usage: vulntechfinder --cmd <command> [--parallel N] [--output file]")
		os.Exit(1)
	}

	if parallel <= 0 {
		parallel = 50
	}

	// Parse exclude and include lists (support both comma-separated and file paths)
	excludeList, err := parseTechInput(excludeTech)
	if err != nil {
		fmt.Printf("Error reading exclude-tech input: %s\n", err)
		os.Exit(1)
	}

	includeList, err := parseTechInput(includeTech)
	if err != nil {
		fmt.Printf("Error reading include-tech input: %s\n", err)
		os.Exit(1)
	}

	// Validate that both exclude and include are not used together
	if len(excludeList) > 0 && len(includeList) > 0 {
		fmt.Println("Error: Cannot use both --exclude and --include flags together")
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

	// Open the output file for appending if the --output flag is specified
	var outputFile *os.File
	if output != "" {
		outputFile, err = os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("Error opening output file: %s\n", err)
			os.Exit(1)
		}
		defer outputFile.Close()
	}

	// Resume + interrupt handling
	cwd, _ := os.Getwd()
	resumePath := filepath.Join(cwd, "resume.cfg")
	start := 0
	if noResume {
		_ = deleteResume(resumePath)
		if verbose {
			fmt.Fprintln(os.Stderr, "Starting fresh; resume disabled (--no-resume)")
		}
	} else {
		if s, err := loadResume(resumePath); err == nil {
			start = s
			if start > 0 && verbose {
				fmt.Fprintf(os.Stderr, "Resuming from scanned=%d (skipping %d items)\n", start, start)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	interrupted := false
	go func() {
		<-sigCh
		interrupted = true
		fmt.Fprintln(os.Stderr, "\nInterrupt received. Cancelling pending tasks and saving progress...")
		cancel()
	}()

	if !noResume {
		_ = saveResume(resumePath, start)
	}

	// Peek at the first non-whitespace character to detect if stdin is JSON or raw hosts
	br := bufio.NewReader(os.Stdin)
	var firstByte byte
	var peekErr error

	for {
		b, err := br.ReadByte()
		if err != nil {
			peekErr = err
			break
		}
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			firstByte = b
			_ = br.UnreadByte()
			break
		}
	}

	if peekErr != nil && peekErr != io.EOF {
		fmt.Printf("Error reading stdin: %s\n", peekErr)
		os.Exit(1)
	}

	if peekErr == io.EOF && firstByte == 0 {
		fmt.Println("No input provided on stdin. Provide JSON or pipe host list into this command.")
		os.Exit(1)
	}

	var reader io.Reader
	var techfinderCmd *exec.Cmd

	// Detect if stdin already contains JSON (starts with [ or {). If not, stream into techfinder -silent -json
	if firstByte == '[' || firstByte == '{' {
		if verbose {
			fmt.Println("Detected JSON on stdin — parsing directly.")
		}
		reader = br
	} else {
		if verbose {
			fmt.Println("No JSON detected on stdin — streaming to 'techfinder -silent -json' in real-time.")
		}
		techfinderCmd = exec.CommandContext(ctx, "techfinder", "-silent", "-json")
		techfinderCmd.Stdin = br
		techfinderCmd.Stderr = os.Stderr

		stdoutPipe, err := techfinderCmd.StdoutPipe()
		if err != nil {
			fmt.Printf("Error creating stdout pipe for techfinder: %s\n", err)
			os.Exit(1)
		}

		if err := techfinderCmd.Start(); err != nil {
			// Fallback to sh -c for unix/cygwin/git-bash environments
			techfinderCmd = exec.CommandContext(ctx, "sh", "-c", "techfinder -silent -json")
			techfinderCmd.Stdin = br
			techfinderCmd.Stderr = os.Stderr
			stdoutPipe, err = techfinderCmd.StdoutPipe()
			if err != nil || techfinderCmd.Start() != nil {
				fmt.Printf("Error running techfinder: %s\n", err)
				os.Exit(1)
			}
		}
		reader = stdoutPipe
	}

	decoder := json.NewDecoder(reader)
	type item struct {
		index int
		data  TechData
	}
	items := make(chan item, parallel)
	doneCh := make(chan int, parallel*2)
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, parallel)

	// Producer: decode JSON and send items in real time, skipping first 'start'
	go func() {
		defer close(items)
		if techfinderCmd != nil {
			defer techfinderCmd.Wait()
		}
		idx := 0
		for {
			var td TechData
			if err := decoder.Decode(&td); err == io.EOF {
				break
			} else if err != nil {
				if ctx.Err() != nil {
					return
				}
				if verbose {
					fmt.Printf("Error decoding JSON: %s\n", err)
				}
				break
			}
			if td.Tech == nil {
				if verbose {
					fmt.Printf("Skipping URL with tech field as null: %s\n", td.Host)
				}
				idx++
				continue
			}
			if idx >= start {
				select {
				case items <- item{index: idx, data: td}:
				case <-ctx.Done():
					return
				}
			}
			idx++
		}
	}()

	// Collector to persist contiguous progress
	nextLocal := start
	pending := make(map[int]struct{})
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		for idx := range doneCh {
			pending[idx] = struct{}{}
			for {
				if _, ok := pending[nextLocal]; ok {
					delete(pending, nextLocal)
					nextLocal++
					_ = saveResume(resumePath, nextLocal)
				} else {
					break
				}
			}
		}
	}()

	for it := range items {
		if ctx.Err() != nil {
			break
		}
		td := it.data
		idx := it.index
		wg.Add(1)
		semaphore <- struct{}{}
		go func(techData TechData, index int) {
			defer wg.Done()
			defer func() { <-semaphore }()
			if ctx.Err() != nil {
				return
			}

			// Process tech field with include/exclude logic
			var techs []string
			for _, t := range techData.Tech {
				parts := strings.SplitN(t, ":", 2)
				if len(parts) > 0 {
					tech := strings.TrimSpace(parts[0])
					// Ignore technologies with spaces
					if !strings.Contains(tech, " ") {
						techLower := strings.ToLower(tech)

						// If include list is specified, only include technologies in the list
						if len(includeList) > 0 {
							if contains(includeList, techLower) {
								techs = append(techs, tech)
							}
						} else {
							// Otherwise, use exclude logic only
							if !contains(excludeList, techLower) {
								techs = append(techs, tech)
							}
						}
					}
				}
			}

			// Skip if techs is empty
			if len(techs) == 0 {
				if verbose {
					fmt.Printf("SKIPPED: %s - no matching technologies found\n", techData.Host)
				}
				return
			}

			tech := strings.ToLower(strings.Join(techs, ","))

			var execCmdStr string
			if strings.Contains(cmdStr, "-tc") {
				// Modify to use the -tc format
				var conditions []string
				for _, t := range techs {
					conditions = append(conditions, fmt.Sprintf("contains(to_lower(name),'%s')", strings.ToLower(t)))
				}
				execCmdStr = strings.Replace(cmdStr, "{tech}", fmt.Sprintf("\"%s\"", strings.Join(conditions, " || ")), -1)
			} else if strings.Contains(cmdStr, "-tags") {
				// Use the -tags format as-is
				execCmdStr = strings.Replace(cmdStr, "{tech}", tech, -1)
			} else {
				// Default: replace {tech} as-is
				execCmdStr = strings.Replace(cmdStr, "{tech}", tech, -1)
			}

			if process {
				fmt.Printf("Running: [echo \"%s\" | %s]\n", techData.Host, execCmdStr)
			}

			// Run the command
			cmd := exec.Command("sh", "-c", execCmdStr)
			cmd.Stdin = strings.NewReader(techData.Host)
			stdoutPipe, _ := cmd.StdoutPipe()
			stderrPipe, _ := cmd.StderrPipe()

			if ctx.Err() != nil {
				return
			}
			if err := cmd.Start(); err != nil {
				if verbose {
					fmt.Printf("Error starting command: %s\n", err)
				}
				return
			}

			// Handle the output
			scanner := bufio.NewScanner(io.MultiReader(stdoutPipe, stderrPipe))
			for scanner.Scan() {
				line := scanner.Text()
				fmt.Println(line)

				// Check if the line starts with three sets of square brackets
				parts := strings.Fields(line)
				if len(parts) >= 3 && strings.HasPrefix(parts[0], "[") && strings.HasPrefix(parts[1], "[") && strings.HasPrefix(parts[2], "[") {
					if output != "" {
						// Append the filtered output line to the specified file
						if _, err := outputFile.WriteString(line + "\n"); err != nil && verbose {
							fmt.Printf("Error writing to output file: %s\n", err)
						}
					}
				}
			}

			if err := cmd.Wait(); err != nil && verbose {
				fmt.Printf("Error waiting for command: %s\n", err)
			}
			if ctx.Err() == nil {
				select {
				case doneCh <- index:
				case <-ctx.Done():
				}
			}
		}(td, idx)
	}

	wg.Wait()
	close(doneCh)
	<-collectorDone

	// Cleanup resume file and messaging
	if ctx.Err() == nil {
		_ = deleteResume(resumePath)
	}
	if interrupted {
		fmt.Fprintln(os.Stderr, "Progress saved to resume.cfg. Re-run the same command to resume, or use --no-resume to start over.")
	}
}

// Helper function to parse tech input (supports both comma-separated values and file paths)
func parseTechInput(input string) ([]string, error) {
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

// Utility function to check if a tech is in the exclusion list
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Resume helpers
func loadResume(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "scanned=") {
			val := strings.TrimPrefix(line, "scanned=")
			n, err := strconv.Atoi(strings.TrimSpace(val))
			if err == nil && n >= 0 {
				return n, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, nil
}

func saveResume(path string, scanned int) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	data := []byte(fmt.Sprintf("scanned=%d\n", scanned))
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(path)
	}
	return os.Rename(tmp, path)
}

func deleteResume(path string) error {
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	return nil
}
