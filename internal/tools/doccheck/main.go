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
	"time"
)

var markdownLink = regexp.MustCompile(`\]\(([^)]+)\)`)

var documentStatuses = map[string]struct{}{
	"accepted":   {},
	"active":     {},
	"deprecated": {},
	"draft":      {},
	"proposed":   {},
	"superseded": {},
}

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
		fileIssues, err := checkFile(root, path)
		if err != nil {
			return err
		}
		issues = append(issues, fileIssues...)
		return nil
	})
	sort.Strings(issues)
	return issues, files, err
}

func checkFile(root, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var issues []string
	if len(data) == 0 || data[len(data)-1] != '\n' {
		issues = append(issues, path+": missing final newline")
	}
	if requiresFrontmatter(root, path) {
		issues = append(issues, checkFrontmatter(path, data)...)
	}

	fences := 0
	inFence := false
	binDirectoryPrepared := false
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if strings.HasPrefix(text, "```") {
			fences++
			inFence = !inFence
			binDirectoryPrepared = false
		} else if inFence {
			trimmed := strings.TrimSpace(text)
			if strings.Contains(trimmed, "mkdir -p ./bin") || (strings.Contains(trimmed, "New-Item") && strings.Contains(trimmed, "bin")) {
				binDirectoryPrepared = true
			}
			if strings.Contains(trimmed, "go build") && strings.Contains(trimmed, "-o ./bin/") && !binDirectoryPrepared {
				issues = append(issues, fmt.Sprintf("%s:%d: source build writes to ./bin before the code block creates it", path, line))
			}
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

func requiresFrontmatter(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	relative = filepath.ToSlash(relative)
	return relative == "README.md" || strings.HasPrefix(relative, "docs/")
}

func checkFrontmatter(path string, data []byte) []string {
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return []string{path + ": missing YAML frontmatter"}
	}

	fields := make(map[string]string)
	closed := false
	for index := 1; index < len(lines); index++ {
		line := lines[index]
		if line == "---" {
			closed = true
			break
		}
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := fields[key]; exists {
			return []string{fmt.Sprintf("%s: duplicate frontmatter field %q", path, key)}
		}
		fields[key] = value
	}
	if !closed {
		return []string{path + ": unclosed YAML frontmatter"}
	}

	var issues []string
	if fields["title"] == "" {
		issues = append(issues, path+": frontmatter title is required")
	}
	status := fields["status"]
	if status == "" {
		issues = append(issues, path+": frontmatter status is required")
	} else if _, allowed := documentStatuses[status]; !allowed {
		issues = append(issues, fmt.Sprintf("%s: unsupported frontmatter status %q", path, status))
	}
	if reviewed := fields["last_reviewed"]; reviewed != "" {
		if _, err := time.Parse("2006-01-02", reviewed); err != nil {
			issues = append(issues, fmt.Sprintf("%s: last_reviewed must use YYYY-MM-DD", path))
		}
	}
	return issues
}

func isExternal(target string) bool {
	lower := strings.ToLower(target)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:")
}
