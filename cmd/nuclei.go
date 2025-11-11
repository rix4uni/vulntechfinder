package cmd

import (
  "bufio"
  "encoding/json"
  "fmt"
  "io"
  "io/ioutil"
  "os"
  "os/exec"
  "strings"
  "sync"

  "github.com/spf13/cobra"
)

// Structure to map the JSON data
type TechData struct {
  Host string   `json:"host"`
  Tech []string `json:"tech"`
}

// nucleiCmd represents the nuclei command
var nucleiCmd = &cobra.Command{
  Use:   "nuclei",
  Short: "Run Nuclei scans on multiple hosts in parallel, filtering by technology stack (reads JSON from stdin or runs techx).",
  Long: `The 'nuclei' command reads JSON (objects with {"host":..., "tech":[...]}) from stdin, or if the stdin doesn't contain JSON it will run the external 'techx -silent -json' command (feeding stdin to techx) and consume its JSON output.

Examples:
  echo "hackerone.com" | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt

  cat subs.txt | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt

  cat techx-output.json | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt
`,
  Run: func(cmd *cobra.Command, args []string) {
    nucleiCmdStr, _ := cmd.Flags().GetString("cmd")
    verbose, _ := cmd.Flags().GetBool("verbose")
    process, _ := cmd.Flags().GetBool("process")
    parallel, _ := cmd.Flags().GetInt("parallel")
    Output, _ := cmd.Flags().GetString("output")
    excludeTech, _ := cmd.Flags().GetString("exclude-tech")
    includeTech, _ := cmd.Flags().GetString("include-tech")

    if nucleiCmdStr == "" {
      fmt.Println("Usage: vulntechx nuclei --cmd <nuclei command> [--parallel N] [--output file]")
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
      var techData TechData
      if err := decoder.Decode(&techData); err == io.EOF {
        break
      } else if err != nil {
        fmt.Printf("Error decoding JSON: %s\n", err)
        os.Exit(1)
      }

      // Skip processing if tech is nil
      if techData.Tech == nil {
        if verbose {
          fmt.Printf("Skipping URL with tech field as null: %s\n", techData.Host)
        }
        continue
      }

      wg.Add(1)
      semaphore <- struct{}{} // Acquire a semaphore
      go func(techData TechData) {
        defer wg.Done()
        defer func() { <-semaphore }() // Release the semaphore

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

        var cmdStr string
        if strings.Contains(nucleiCmdStr, "-tc") {
          // Modify to use the -tc format
          var conditions []string
          for _, t := range techs {
            conditions = append(conditions, fmt.Sprintf("contains(to_lower(name),'%s')", strings.ToLower(t)))
          }
          cmdStr = strings.Replace(nucleiCmdStr, "{tech}", fmt.Sprintf("\"%s\"", strings.Join(conditions, " || ")), -1)
        } else if strings.Contains(nucleiCmdStr, "-tags") {
          // Use the -tags format as-is
          cmdStr = strings.Replace(nucleiCmdStr, "{tech}", tech, -1)
        } else {
          // Default: replace {tech} as-is
          cmdStr = strings.Replace(nucleiCmdStr, "{tech}", tech, -1)
        }

        if process {
          fmt.Printf("Running Nuclei: [echo \"%s\" | %s]\n", techData.Host, cmdStr)
        }

        // Run the nuclei command
        cmd := exec.Command("sh", "-c", cmdStr)
        cmd.Stdin = strings.NewReader(techData.Host)
        stdoutPipe, _ := cmd.StdoutPipe()
        stderrPipe, _ := cmd.StderrPipe()

        if err := cmd.Start(); err != nil {
          if verbose {
            fmt.Printf("Error starting nuclei command: %s\n", err)
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
            if Output != "" {
              // Append the filtered output line to the specified file
              if _, err := outputFile.WriteString(line + "\n"); err != nil && verbose {
                fmt.Printf("Error writing to output file: %s\n", err)
              }
            }
          }
        }

        if err := cmd.Wait(); err != nil && verbose {
          fmt.Printf("Error waiting for nuclei command: %s\n", err)
        }

      }(techData)
    }

    wg.Wait() // Wait for all goroutines to finish
  },
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

func init() {
  rootCmd.AddCommand(nucleiCmd)

  nucleiCmd.Flags().StringP("cmd", "c", "", "The nuclei command template")
  nucleiCmd.Flags().Bool("verbose", false, "Enable verbose output for debugging purposes.")
  nucleiCmd.Flags().Bool("process", false, "Show which URL is running on Nuclei.")
  nucleiCmd.Flags().Int("parallel", 50, "Number of parallel processes")
  nucleiCmd.Flags().StringP("output", "o", "", "File to save output")
  nucleiCmd.Flags().StringP("exclude-tech", "e", "", "Comma-separated list of technologies to exclude, or path to a file with technologies (one per line)")
  nucleiCmd.Flags().StringP("include-tech", "i", "", "Comma-separated list of technologies to include (only these will be processed), or path to a file with technologies (one per line)")
}