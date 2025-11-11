## vulntechx

vulntechx is a powerful security tool that automates vulnerability scanning based on technology stack detection. It intelligently processes hosts through tech stack identification and executes targeted scans using tools like Nuclei and httpx.

## 🚀 Key Features

- **🔍 Automated Tech Stack Detection**: Seamlessly integrates with `techx` to identify technologies running on target hosts
- **⚡ Parallel Processing**: Configurable parallel execution (default: 50) for high-performance scanning
- **🎯 Smart Filtering**: Include/exclude specific technologies using comma-separated lists or file inputs
- **📊 Multiple Output Formats**: Save results to files while maintaining real-time console output
- **🛠️ Tool Agnostic**: Works with any security tool that accepts technology tags or file paths
- **🔧 Flexible Input**: Accepts raw domains, host lists, or pre-processed techx JSON output
- **👀 Real-time Monitoring**: Verbose and process flags for detailed debugging and progress tracking

## 📦 Installation

### Option 1: Install via Go
```
go install github.com/rix4uni/vulntechx@latest
```

### Option 2: Download Prebuilt Binaries
```
wget https://github.com/rix4uni/vulntechx/releases/download/v0.0.6/vulntechx-linux-amd64-0.0.6.tgz
tar -xvzf vulntechx-linux-amd64-0.0.6.tgz
rm -rf vulntechx-linux-amd64-0.0.6.tgz
mv vulntechx ~/go/bin/vulntechx
```

Download other platform binaries from [releases page](https://github.com/rix4uni/vulntechx/releases).

### Option 3: Compile from Source
```
git clone --depth 1 https://github.com/rix4uni/vulntechx.git
cd vulntechx; go install
```

## 🔧 Usage
```yaml
                __        __               __
 _   __ __  __ / /____   / /_ ___   _____ / /_   _  __
| | / // / / // // __ \ / __// _ \ / ___// __ \ | |/_/
| |/ // /_/ // // / / // /_ /  __// /__ / / / /_>  <
|___/ \__,_//_//_/ /_/ \__/ \___/ \___//_/ /_//_/|_|

                            Current vulntechx version v0.0.6

vulntechx finds vulnerabilities based on tech stack using nuclei tags or fuzzing with ffuf.

Examples:
  echo "hackerone.com" | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt
  cat subs.txt | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt
  cat techx-output.json | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt

  echo "hackerone.com" | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt
  cat subs.txt | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt
  cat techx-output.json | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt

Usage:
  vulntechx [flags]
  vulntechx [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  httpx       Run httpx scans on multiple hosts in parallel, filtering by technology stack (reads JSON from stdin or runs techx).
  nuclei      Run Nuclei scans on multiple hosts in parallel, filtering by technology stack (reads JSON from stdin or runs techx).

Flags:
  -h, --help      help for vulntechx
  -u, --update    update vulntechx to latest version
  -v, --version   Print the version of the tool and exit.

Use "vulntechx [command] --help" for more information about a command.
```

## 🎯 Quick Start

### Nuclei Scanning
```yaml
echo "hackerone.com" | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech} -es unknown,info,low" --parallel 10 --output nuclei-output.txt
```

### HTTPx Fuzzing
```yaml
echo "hackerone.com" | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 10 --output httpx-output.txt
```

## 📋 Command Reference

### nuclei Command
Run Nuclei scans filtered by technology stack.

**Usage:**
```yaml
vulntechx nuclei --cmd "nuclei [options] -tags {tech}" [flags]
```

**Examples:**
```yaml
# Scan from domain list
cat domains.txt | vulntechx nuclei --cmd "nuclei -duc -t ~/nuclei-templates -tags {tech}" --parallel 20

# Use existing techx JSON output
cat techx-results.json | vulntechx nuclei --cmd "nuclei -tags {tech}" --output results.txt

# Include only specific technologies
cat domains.txt | vulntechx nuclei --include-tech wordpress,joomla --cmd "nuclei -tags {tech}"

# Exclude technologies from file
cat domains.txt | vulntechx nuclei --exclude-tech excluded-techs.txt --cmd "nuclei -tags {tech}"
```

### httpx Command
Run httpx scans with technology-specific path fuzzing.

**Usage:**
```yaml
vulntechx httpx --cmd "httpx [options] -path {tech}" [flags]
```

**Examples:**
```yaml
# Fuzz with technology-specific wordlists
cat subdomains.txt | vulntechx httpx --cmd "httpx -duc -silent -path {tech}" --parallel 15

# Use custom wordlists directory
cat targets.txt | vulntechx httpx --cmd "httpx -path {tech}" --output httpx-results.txt

# Filter technologies during scanning
cat hosts.txt | vulntechx httpx --include-tech jenkins,gitlab --cmd "httpx -path {tech}"
```

## 📊 Command Flags

### Common Flags
- `--cmd string`**: Command template with `{tech}` placeholder (required)
- `--parallel int`**: Number of parallel processes (default: 50)
- `--output string`**: Output file to save results
- `--verbose`**: Enable verbose debugging output
- `--process`**: Show which URLs are being processed

### Technology Filtering Flags
- `--include-tech string`**: Comma-separated list or file of technologies to include
- `--exclude-tech string`**: Comma-separated list or file of technologies to exclude

**Note:** `--include-tech` and `--exclude-tech` cannot be used together.

## 🛠️ How It Works

1. **Input Processing**: Reads hosts from stdin or existing techx JSON output
2. **Tech Detection**: Automatically runs `techx` if JSON isn't provided
3. **Technology Filtering**: Applies include/exclude filters to technologies
4. **Command Execution**: Replaces `{tech}` placeholder in your command template
5. **Parallel Scanning**: Executes scans concurrently with configurable limits
6. **Output Handling**: Saves results to file while displaying real-time progress

## 📁 File Structure & Paths

### Default Wordlist Directory
HTTPx command automatically checks:
- `/root/wordlists/{tech}`
- `/root/wordlists/{tech}.txt`

## Input Formats

vulntechx accepts multiple input formats:

- **Raw domains/hosts**: `echo "example.com" | vulntechx nuclei ...`
- **Domain lists**: `cat domains.txt | vulntechx nuclei ...`
- **techx JSON**: `cat techx-output.json | vulntechx nuclei ...`

## Technology Placeholders

The `{tech}` placeholder in your command template gets replaced with:
- **nuclei**: Comma-separated technology tags
- **httpx**: Path to technology-specific wordlist or inline technology name

## Best Practices

- Start with `--parallel 10` and increase based on system resources
- Use `--verbose` for debugging when first setting up commands
- Combine with `techx` for optimal technology detection
- Use `--output` to save all results for later analysis
- Test commands directly before using with vulntechx

## 🔧 Troubleshooting

- Ensure `techx` is installed and in PATH for automatic tech detection
- Verify your command template works when `{tech}` is manually replaced
- Use `--verbose` to see detailed processing information
- Check that input formats match expected JSON structure when piping techx output
