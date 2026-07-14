package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidatePackageConfig(t *testing.T) {
	t.Parallel()
	valid := packageConfig{Version: "0.1.0-rc.1", Commit: strings.Repeat("a", 40), Date: time.Unix(1, 0).UTC(), Output: "dist"}
	if err := validatePackageConfig(valid); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	tests := []packageConfig{
		{Version: "v0.1.0", Commit: valid.Commit, Date: valid.Date, Output: valid.Output},
		{Version: "01.1.0", Commit: valid.Commit, Date: valid.Date, Output: valid.Output},
		{Version: "0.1.0-dev..1", Commit: valid.Commit, Date: valid.Date, Output: valid.Output},
		{Version: "0.1.0-dev.01", Commit: valid.Commit, Date: valid.Date, Output: valid.Output},
		{Version: valid.Version, Commit: "not-a-sha", Date: valid.Date, Output: valid.Output},
		{Version: valid.Version, Commit: strings.Repeat("a", 39), Date: valid.Date, Output: valid.Output},
		{Version: valid.Version, Commit: valid.Commit, Date: time.Time{}, Output: valid.Output},
		{Version: valid.Version, Commit: valid.Commit, Date: valid.Date, Output: "."},
	}
	for _, test := range tests {
		if err := validatePackageConfig(test); err == nil {
			t.Errorf("validatePackageConfig(%+v) succeeded", test)
		}
	}
}

func TestArchivesAreDeterministicAndPreserveLayout(t *testing.T) {
	t.Parallel()
	modified := time.Date(2026, 7, 12, 3, 4, 5, 0, time.UTC)
	files := []archiveFile{
		{Name: "cailab_0.1.0_linux_amd64/cailab", Mode: 0o755, Data: []byte("binary")},
		{Name: "cailab_0.1.0_linux_amd64/README.md", Mode: 0o644, Data: []byte("readme")},
	}
	for _, format := range []string{"tar.gz", "zip"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			first := filepath.Join(t.TempDir(), "first."+format)
			second := filepath.Join(t.TempDir(), "second."+format)
			if err := writeArchive(first, format, modified, files); err != nil {
				t.Fatal(err)
			}
			if err := writeArchive(second, format, modified, files); err != nil {
				t.Fatal(err)
			}
			firstData, err := os.ReadFile(first)
			if err != nil {
				t.Fatal(err)
			}
			secondData, err := os.ReadFile(second)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(firstData, secondData) {
				t.Fatal("equivalent archive inputs produced different bytes")
			}
			assertArchive(t, format, first, files, modified)
		})
	}
}

func TestLoadDistributionFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"CHANGELOG.md", "LICENSE", "NOTICE", "README.md", "THIRD_PARTY_NOTICES.md", "third_party/modules.txt"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"README.md", "expected-report.md", "main.go", "policy.json", "prompt.txt"} {
		path := filepath.Join(root, "examples", "external-agent-starter", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"README.md", "scenario.yaml"} {
		path := filepath.Join(root, "examples", "scenario-starter", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"README.md", "github-actions.yml"} {
		path := filepath.Join(root, "examples", "ci", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{
		"docs/08-learning/README.md", "docs/08-learning/adaptation-provenance.md",
		"docs/08-learning/identity-agent-foundations.md", "docs/08-learning/learning-contract.md",
		"learning/catalog.json", "schemas/learning/v1alpha1/learning-catalog.json",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	licensePath := filepath.Join(root, "third_party", "licenses", "example", "LICENSE")
	if err := os.MkdirAll(filepath.Dir(licensePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(licensePath, []byte("license\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := loadDistributionFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{
		"CHANGELOG.md",
		"LICENSE",
		"NOTICE",
		"README.md",
		"THIRD_PARTY_NOTICES.md",
		"docs/08-learning/README.md",
		"docs/08-learning/adaptation-provenance.md",
		"docs/08-learning/identity-agent-foundations.md",
		"docs/08-learning/learning-contract.md",
		"examples/ci/README.md",
		"examples/ci/github-actions.yml",
		"examples/external-agent-starter/README.md",
		"examples/external-agent-starter/expected-report.md",
		"examples/external-agent-starter/main.go",
		"examples/external-agent-starter/policy.json",
		"examples/external-agent-starter/prompt.txt",
		"examples/scenario-starter/README.md",
		"examples/scenario-starter/scenario.yaml",
		"learning/catalog.json",
		"schemas/learning/v1alpha1/learning-catalog.json",
		"third_party/licenses/example/LICENSE",
		"third_party/modules.txt",
	}
	if len(files) != len(wantNames) {
		t.Fatalf("files = %d, want %d", len(files), len(wantNames))
	}
	for index, want := range wantNames {
		if files[index].Name != want || files[index].Mode != 0o644 {
			t.Fatalf("file %d = %+v, want name %q and mode 0644", index, files[index], want)
		}
	}
}

func TestLoadDistributionFilesRequiresLegalBundle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := loadDistributionFiles(root); err == nil || !strings.Contains(err.Error(), "CHANGELOG.md") {
		t.Fatalf("loadDistributionFiles() error = %v, want missing release document", err)
	}
}

func TestRewriteUnbundledMarkdownLinksUsesExactCommit(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("a", 40)
	files := []archiveFile{
		{Name: "LICENSE", Mode: 0o644, Data: []byte("license\n")},
		{Name: "README.md", Mode: 0o644, Data: []byte("[license](LICENSE) [guide](docs/guide.md#start) [anchor](#local) [web](https://example.com)\n")},
	}

	rewritten, err := rewriteUnbundledMarkdownLinks(commit, files)
	if err != nil {
		t.Fatal(err)
	}
	want := "[license](LICENSE) [guide](https://github.com/msinclair25/cailab/blob/" + commit + "/docs/guide.md#start) [anchor](#local) [web](https://example.com)\n"
	if got := string(rewritten[1].Data); got != want {
		t.Fatalf("README = %q, want %q", got, want)
	}
	if got := string(files[1].Data); strings.Contains(got, commit) {
		t.Fatal("rewrite mutated the input archive file")
	}
}

func TestRewriteUnbundledMarkdownLinksRejectsUnsafeTargets(t *testing.T) {
	t.Parallel()
	files := []archiveFile{{Name: "README.md", Mode: 0o644, Data: []byte("[outside](../private.md)\n")}}
	if _, err := rewriteUnbundledMarkdownLinks(strings.Repeat("a", 40), files); err == nil || !strings.Contains(err.Error(), "outside the repository") {
		t.Fatalf("rewrite error = %v, want outside-repository refusal", err)
	}
}

func TestRepositoryDistributionMarkdownLinksResolveOrUseExactCommit(t *testing.T) {
	t.Parallel()
	files, err := loadDistributionFiles(filepath.Clean("../../.."))
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.Repeat("b", 40)
	rewritten, err := rewriteUnbundledMarkdownLinks(commit, files)
	if err != nil {
		t.Fatal(err)
	}

	included := make(map[string]struct{}, len(rewritten))
	for _, file := range rewritten {
		included[filepath.ToSlash(filepath.Clean(file.Name))] = struct{}{}
	}
	for _, file := range rewritten {
		if !strings.EqualFold(filepath.Ext(file.Name), ".md") {
			continue
		}
		for _, match := range markdownLink.FindAllSubmatch(file.Data, -1) {
			target := strings.TrimSpace(string(match[1]))
			if target == "" || strings.HasPrefix(target, "#") || isRemoteMarkdownTarget(target) {
				continue
			}
			pathPart, _, _ := strings.Cut(target, "#")
			resolved := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(file.Name), filepath.FromSlash(pathPart))))
			if _, exists := included[resolved]; !exists {
				t.Errorf("release document %s retains unresolved local link %q", file.Name, target)
			}
		}
	}
}

func TestLoadDistributionFilesRejectsNonRegularDocument(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "CHANGELOG.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := loadDistributionFiles(root); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("loadDistributionFiles() error = %v, want non-regular document", err)
	}
}

func TestWriteChecksumsSortedAndScoped(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	for name, data := range map[string]string{
		"z.zip":             "zip",
		"a.tar.gz":          "tar",
		"release.spdx.json": "sbom",
		"ignored.txt":       "ignored",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	count, err := writeChecksums(directory, "checksums.txt")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("checksum count = %d, want 3", count)
	}
	data, err := os.ReadFile(filepath.Join(directory, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i, want := range []string{"a.tar.gz", "release.spdx.json", "z.zip"} {
		if !strings.HasSuffix(lines[i], "  "+want) {
			t.Fatalf("line %d = %q, want asset %q", i, lines[i], want)
		}
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()
	if err := run(context.Background(), []string{"publish"}, io.Discard); err == nil {
		t.Fatal("run succeeded for unknown command")
	}
}

func assertArchive(t *testing.T, format, path string, want []archiveFile, modified time.Time) {
	t.Helper()
	switch format {
	case "tar.gz":
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			t.Fatal(err)
		}
		defer gzipReader.Close()
		tarReader := tar.NewReader(gzipReader)
		for _, expected := range want {
			header, err := tarReader.Next()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(tarReader)
			if err != nil {
				t.Fatal(err)
			}
			if header.Name != expected.Name || os.FileMode(header.Mode) != expected.Mode || !header.ModTime.Equal(modified) || !bytes.Equal(data, expected.Data) {
				t.Fatalf("tar entry = %+v %q, want %+v %q", header, data, expected, expected.Data)
			}
		}
	case "zip":
		reader, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		if len(reader.File) != len(want) {
			t.Fatalf("zip entries = %d, want %d", len(reader.File), len(want))
		}
		for i, expected := range want {
			entry := reader.File[i]
			stream, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(stream)
			stream.Close()
			if err != nil {
				t.Fatal(err)
			}
			if entry.Name != expected.Name || entry.Mode().Perm() != expected.Mode || !entry.Modified.Equal(modified) || !bytes.Equal(data, expected.Data) {
				t.Fatalf("zip entry = %+v %q, want %+v %q", entry.FileHeader, data, expected, expected.Data)
			}
		}
	default:
		t.Fatalf("unsupported format %q", format)
	}
}
