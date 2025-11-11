package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/rix4uni/vulntechx/banner"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "vulntechx",
	Short: "find vulnerabilities based on tech stack using nuclei or ffuf",
	Long: `vulntechx finds vulnerabilities based on tech stack using nuclei tags or fuzzing with ffuf.

Examples:
  echo "hackerone.com" | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt
  cat subs.txt | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt
  cat techx-output.json | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt

  echo "hackerone.com" | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt
  cat subs.txt | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt
  cat techx-output.json | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if the version flag is set
		if v, _ := cmd.Flags().GetBool("version"); v {
			banner.PrintVersion() // Print the version and exit
			return
		}

		// Check if the update flag is set
		if update, _ := cmd.Flags().GetBool("update"); update {
			checkAndUpdateTool()
			return
		}
	},
}

// Function to check for the latest version and update if necessary
func checkAndUpdateTool() {
	currentVersion, err := getCurrentVersion()
	if err != nil {
		fmt.Println("Error fetching the current version:", err)
		os.Exit(1)
	}

	latestVersion, err := getLatestVersion()
	if err != nil {
		fmt.Println("Error fetching the latest version:", err)
		os.Exit(1)
	}

	if latestVersion == currentVersion {
		fmt.Println("There is no latest update; you are using the latest version.")
		return
	}

	fmt.Printf("Updating vulntechx from version %s to %s...\n", currentVersion, latestVersion)
	cmd := exec.Command("go", "install", "github.com/rix4uni/vulntechx@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		fmt.Println("Error updating vulntechx:", err)
		os.Exit(1)
	}

	fmt.Println("vulntechx has been updated to the latest version.")
}

// Function to get the current version by executing 'vulntechx -v'
func getCurrentVersion() (string, error) {
	cmd := exec.Command("vulntechx", "-v")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	// Use regex to find the version string in the output
	re := regexp.MustCompile(`Current vulntechx version (v[0-9]+\.[0-9]+\.[0-9]+)`)
	matches := re.FindStringSubmatch(out.String())
	if len(matches) < 2 {
		return "", fmt.Errorf("current version not found in output")
	}
	return matches[1], nil
}

// Function to get the latest version from the specified URL
func getLatestVersion() (string, error) {
	// Fetch the latest version from the banner.go file
	resp, err := http.Get("https://raw.githubusercontent.com/rix4uni/vulntechx/refs/heads/main/banner/banner.go")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest version: %s", resp.Status)
	}

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Find the version string in the body
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "const version =") {
			// Extract the version
			version := strings.TrimSpace(line[len("const version = "):])
			version = strings.Trim(version, `"`) // Remove quotes
			return version, nil
		}
	}

	return "", fmt.Errorf("version not found in response")
}

func Execute() {
	banner.PrintBanner() // Print banner at the start
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Define flags
	rootCmd.Flags().BoolP("update", "u", false, "update vulntechx to latest version")
	rootCmd.Flags().BoolP("version", "v", false, "Print the version of the tool and exit.")
}
