// Command doccheck validates repository Markdown without external tooling.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var markdownLink = regexp.MustCompile(`\]\(([^)]+)\)`)

func main() {
	root := "."
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./internal/tools/doccheck [root]")
		os.Exit(2)
	}
	if len(os.Args) == 2 {
		root = os.Args[1]
	}
	issues, files, err := check(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doccheck: %v\n", err)
		os.Exit(1)
	}
	if len(issues) > 0 {
		for _, issue := range issues {
			fmt.Fprintln(os.Stderr, issue)
		}
		os.Exit(1)
	}
	fmt.Printf("documentation checks passed (%d Markdown files)\n", files)
}

func check(root string) ([]string, int, error) {
	var issues []string
	files := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".cloudailab", "dist", "coverage":
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		files++
		fileIssues, err := checkFile(path)
		if err != nil {
			return err
		}
		issues = append(issues, fileIssues...)
		return nil
	})
	sort.Strings(issues)
	return issues, files, err
}

func checkFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var issues []string
	if len(data) == 0 || data[len(data)-1] != '\n' {
		issues = append(issues, path+": missing final newline")
	}

	fences := 0
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if strings.HasPrefix(text, "```") {
			fences++
		}
		if strings.HasSuffix(text, " ") || strings.HasSuffix(text, "\t") {
			issues = append(issues, fmt.Sprintf("%s:%d: trailing whitespace", path, line))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	if fences%2 != 0 {
		issues = append(issues, fmt.Sprintf("%s: unbalanced fenced code blocks", path))
	}

	for _, match := range markdownLink.FindAllStringSubmatch(string(data), -1) {
		target := strings.TrimSpace(match[1])
		if target == "" || strings.HasPrefix(target, "#") || isExternal(target) {
			continue
		}
		if index := strings.IndexByte(target, '#'); index >= 0 {
			target = target[:index]
		}
		resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(target)))
		if _, err := os.Stat(resolved); err != nil {
			issues = append(issues, fmt.Sprintf("%s: broken relative link %q (%s)", path, match[1], resolved))
		}
	}
	return issues, nil
}

func isExternal(target string) bool {
	lower := strings.ToLower(target)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:")
}
