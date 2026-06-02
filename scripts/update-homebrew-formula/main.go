package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type formulaData struct {
	Version     string
	RepoOwner   string
	RepoName    string
	AMD64URL    string
	AMD64SHA256 string
	ARM64URL    string
	ARM64SHA256 string
}

func parseChecksums(input []byte) (map[string]string, error) {
	checksums := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(input))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		fields := bytes.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed checksum line %d: %s", lineNumber, line)
		}
		name := string(fields[1])
		if _, exists := checksums[name]; exists {
			return nil, fmt.Errorf("duplicate checksum entry for %s", name)
		}
		checksums[name] = string(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return checksums, nil
}

func buildFormulaData(tag, owner, repo string, checksums map[string]string) (formulaData, error) {
	tag = strings.TrimSpace(tag)
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	version := strings.TrimPrefix(tag, "v")
	if tag == "" || version == "" {
		return formulaData{}, errors.New("tag is required")
	}
	if owner == "" || repo == "" {
		return formulaData{}, errors.New("owner and repo are required")
	}

	data := formulaData{Version: version, RepoOwner: owner, RepoName: repo}
	for _, item := range []struct {
		goarch string
		url    *string
		sha    *string
	}{
		{goarch: "amd64", url: &data.AMD64URL, sha: &data.AMD64SHA256},
		{goarch: "arm64", url: &data.ARM64URL, sha: &data.ARM64SHA256},
	} {
		name := fmt.Sprintf("ezyl3_%s_darwin_%s.tar.gz", version, item.goarch)
		sum, ok := checksums[name]
		if !ok {
			return formulaData{}, fmt.Errorf("missing checksum for %s", name)
		}
		*item.url = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, name)
		*item.sha = sum
	}
	return data, nil
}

func renderFormula(tmpl string, data formulaData) (string, error) {
	parsed, err := template.New("formula").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := parsed.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func main() {
	var tag, owner, repo, checksumsPath, templatePath, outputPath string
	flag.StringVar(&tag, "tag", "", "release tag, for example v0.1.0")
	flag.StringVar(&owner, "owner", "tlarevo", "GitHub repository owner")
	flag.StringVar(&repo, "repo", "ezyl3", "GitHub repository name")
	flag.StringVar(&checksumsPath, "checksums", "", "path to GoReleaser checksums.txt")
	flag.StringVar(&templatePath, "template", "scripts/templates/ezyl3.rb.tmpl", "formula template path")
	flag.StringVar(&outputPath, "output", "", "formula output path")
	flag.Parse()

	if checksumsPath == "" || outputPath == "" {
		fmt.Fprintln(os.Stderr, "--checksums and --output are required")
		os.Exit(2)
	}
	checksumBytes, err := os.ReadFile(checksumsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	checksums, err := parseChecksums(checksumBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	data, err := buildFormulaData(tag, owner, repo, checksums)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rendered, err := renderFormula(string(tmplBytes), data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(outputPath, []byte(rendered), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
