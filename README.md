## vulntechfinder

Automated vulnerability scanning based on detected technology stacks

## Installation

### Install via Go
```
go install github.com/rix4uni/vulntechfinder@latest
```

### Download Prebuilt Binaries
```
wget https://github.com/rix4uni/vulntechfinder/releases/download/v0.0.8/vulntechfinder-linux-amd64-0.0.8.tgz
tar -xvzf vulntechfinder-linux-amd64-0.0.8.tgz
rm -rf vulntechfinder-linux-amd64-0.0.8.tgz
mv vulntechfinder ~/go/bin/vulntechfinder
```

Download other platform binaries from the [releases page](https://github.com/rix4uni/vulntechfinder/releases).

### Compile from Source
```
git clone --depth 1 https://github.com/rix4uni/vulntechfinder.git
cd vulntechfinder; go install
```

## Usage
```console
Usage of vulntechfinder:
      --cmd string       Command template with {tech} placeholder to execute (e.g. 'nuclei -tags {tech}')
      --exclude string   Comma-separated list or file path of technologies to exclude
      --include string   Comma-separated list or file path of technologies to include (only these are scanned)
      --no-resume        Disable resume functionality and start a fresh scan
      --output string    File path to save the output results
      --parallel int     Number of concurrent parallel processes (default 50)
      --process          Display the command being executed for each host
      --silent           Silent mode.
      --verbose          Enable verbose output for debugging
      --version          Print the tool version and exit
```

## Usage Examples

### Basic Scanning with Nuclei
```console
echo "hackerone.com" | vulntechfinder --cmd "nuclei -tags {tech}"
```

### Piping Subdomains / Host List
```console
cat subs.txt | vulntechfinder --cmd "nuclei -tags {tech}"
```

### Using techfinder JSON Output
```console
cat techfinder.json | vulntechfinder --cmd "nuclei -tags {tech}"
```

### Include Technologies
```console
cat domains.txt | vulntechfinder --include included-techs.txt --cmd "nuclei -tags {tech}"
```

### Exclude Technologies
```console
cat domains.txt | vulntechfinder --exclude excluded-techs.txt --cmd "nuclei -tags {tech}"
```

### Custom Nuclei Flags
```console
cat subs.txt | vulntechfinder --cmd "nuclei --silent -tags {tech} -es info,low,unknown"
```
