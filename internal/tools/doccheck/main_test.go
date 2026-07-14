package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckAcceptsDocumentedMetadataBoundary(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "---\ntitle: Project\nstatus: active\n---\n\n# Project\n")
	writeTestFile(t, root, "docs/guide.md", "---\ntitle: Guide\nstatus: draft\nlast_reviewed: 2026-07-13\n---\n\n# Guide\n")
	writeTestFile(t, root, "CONTRIBUTING.md", "# Contributing\n")

	issues, files, err := check(root)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if files != 3 {
		t.Fatalf("files = %d, want 3", files)
	}
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
}

func TestCheckRejectsInvalidDocumentMetadata(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "missing", data: "# Guide\n", want: "missing YAML frontmatter"},
		{name: "unclosed", data: "---\ntitle: Guide\nstatus: active\n", want: "unclosed YAML frontmatter"},
		{name: "missing title", data: "---\nstatus: active\n---\n", want: "frontmatter title is required"},
		{name: "missing status", data: "---\ntitle: Guide\n---\n", want: "frontmatter status is required"},
		{name: "unsupported status", data: "---\ntitle: Guide\nstatus: complete\n---\n", want: `unsupported frontmatter status "complete"`},
		{name: "invalid review date", data: "---\ntitle: Guide\nstatus: active\nlast_reviewed: July 13\n---\n", want: "last_reviewed must use YYYY-MM-DD"},
		{name: "duplicate field", data: "---\ntitle: Guide\ntitle: Other\nstatus: active\n---\n", want: `duplicate frontmatter field "title"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			issues := checkFrontmatter("docs/guide.md", []byte(test.data))
			if !containsIssue(issues, test.want) {
				t.Fatalf("issues = %v, want one containing %q", issues, test.want)
			}
		})
	}
}

func TestCheckRequiresDocumentedBinDirectoryBeforeSourceBuild(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "README.md", "---\ntitle: Project\nstatus: active\n---\n\n```bash\ngo build -o ./bin/cailab ./cmd/cailab\n```\n")

	issues, _, err := check(root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsIssue(issues, "source build writes to ./bin before the code block creates it") {
		t.Fatalf("issues = %v", issues)
	}

	writeTestFile(t, root, "README.md", "---\ntitle: Project\nstatus: active\n---\n\n```bash\nmkdir -p ./bin\ngo build -o ./bin/cailab ./cmd/cailab\n```\n")
	issues, _, err = check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
}

func writeTestFile(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func containsIssue(issues []string, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, want) {
			return true
		}
	}
	return false
}
