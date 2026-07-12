package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "target.md"), "# Target\n")
	writeFile(t, filepath.Join(root, "valid.md"), "# Valid\n\n[Target](target.md)\n")
	issues, files, err := check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 || files != 2 {
		t.Fatalf("check() = issues %v, files %d", issues, files)
	}
}

func TestCheckReportsFormattingAndLinks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "invalid.md"), "# Invalid  \n\n[Missing](missing.md)\n```")
	issues, _, err := check(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(issues, "\n")
	for _, want := range []string{"missing final newline", "trailing whitespace", "unbalanced fenced code blocks", "broken relative link"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("issues %q do not contain %q", joined, want)
		}
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
