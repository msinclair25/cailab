// Command release builds deterministic CloudAILab release archives and checksums.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	identifierPattern = regexp.MustCompile(`^[0-9A-Za-z-]+$`)
	commitPattern     = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	releaseTargets    = []target{
		{GOOS: "linux", GOARCH: "amd64", Format: "tar.gz"},
		{GOOS: "linux", GOARCH: "arm64", Format: "tar.gz"},
		{GOOS: "darwin", GOARCH: "amd64", Format: "tar.gz"},
		{GOOS: "darwin", GOARCH: "arm64", Format: "tar.gz"},
		{GOOS: "windows", GOARCH: "amd64", Format: "zip"},
	}
)

type target struct {
	GOOS   string
	GOARCH string
	Format string
}

type packageConfig struct {
	Version string
	Commit  string
	Date    time.Time
	Output  string
}

type archiveFile struct {
	Name string
	Mode os.FileMode
	Data []byte
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "release: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: release package|checksums|modules [options]")
	}
	switch args[0] {
	case "package":
		fs := flag.NewFlagSet("package", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		version := fs.String("version", "", "semantic release version without a v prefix")
		commit := fs.String("commit", "", "source commit SHA")
		date := fs.String("date", "", "source commit date in RFC 3339 format")
		output := fs.String("output", "dist", "release staging directory")
		if err := fs.Parse(args[1:]); err != nil {
			return fmt.Errorf("parse package flags: %w", err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected package arguments: %s", strings.Join(fs.Args(), " "))
		}
		parsedDate, err := time.Parse(time.RFC3339, *date)
		if err != nil {
			return fmt.Errorf("parse --date: %w", err)
		}
		config := packageConfig{Version: *version, Commit: *commit, Date: parsedDate.UTC(), Output: *output}
		if err := packageRelease(ctx, config); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "packaged %d release targets in %s\n", len(releaseTargets), filepath.Join(config.Output, "packages"))
		return nil
	case "checksums":
		fs := flag.NewFlagSet("checksums", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		directory := fs.String("directory", filepath.Join("dist", "packages"), "directory containing release assets")
		output := fs.String("output", "checksums.txt", "checksum manifest filename within the directory")
		if err := fs.Parse(args[1:]); err != nil {
			return fmt.Errorf("parse checksums flags: %w", err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected checksums arguments: %s", strings.Join(fs.Args(), " "))
		}
		count, err := writeChecksums(*directory, *output)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "wrote %d checksums to %s\n", count, filepath.Join(*directory, *output))
		return nil
	case "modules":
		fs := flag.NewFlagSet("modules", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		packagePath := fs.String("package", "./cmd/cailab", "release binary package")
		if err := fs.Parse(args[1:]); err != nil {
			return fmt.Errorf("parse modules flags: %w", err)
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected modules arguments: %s", strings.Join(fs.Args(), " "))
		}
		modules, err := linkedReleaseModules(ctx, *packagePath)
		if err != nil {
			return err
		}
		for _, module := range modules {
			fmt.Fprintln(stdout, module)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q; expected package, checksums, or modules", args[0])
	}
}

func packageRelease(ctx context.Context, config packageConfig) error {
	if err := validatePackageConfig(config); err != nil {
		return err
	}
	binariesDir := filepath.Join(config.Output, "binaries")
	packagesDir := filepath.Join(config.Output, "packages")
	for _, path := range []string{binariesDir, packagesDir} {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("clean release path %s: %w", path, err)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create release path %s: %w", path, err)
		}
	}

	distributionFiles, err := loadDistributionFiles(".")
	if err != nil {
		return err
	}
	for _, releaseTarget := range releaseTargets {
		if err := buildTarget(ctx, config, releaseTarget, binariesDir, packagesDir, distributionFiles); err != nil {
			return err
		}
	}
	return nil
}

func validatePackageConfig(config packageConfig) error {
	if !validSemanticVersion(config.Version) {
		return fmt.Errorf("invalid semantic --version %q", config.Version)
	}
	if !commitPattern.MatchString(config.Commit) {
		return fmt.Errorf("invalid hexadecimal --commit %q", config.Commit)
	}
	if config.Date.IsZero() {
		return errors.New("--date is required")
	}
	clean := filepath.Clean(config.Output)
	volume := filepath.VolumeName(clean)
	if clean == "." || clean == string(filepath.Separator) || clean == volume+string(filepath.Separator) {
		return fmt.Errorf("unsafe --output %q", config.Output)
	}
	return nil
}

func validSemanticVersion(version string) bool {
	parts := strings.SplitN(version, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return false
	}
	for _, identifier := range core {
		if !validNumericIdentifier(identifier) {
			return false
		}
	}
	if len(parts) == 1 {
		return true
	}
	for _, identifier := range strings.Split(parts[1], ".") {
		if !identifierPattern.MatchString(identifier) {
			return false
		}
		if identifier[0] >= '0' && identifier[0] <= '9' {
			allNumeric := true
			for i := 1; i < len(identifier); i++ {
				if identifier[i] < '0' || identifier[i] > '9' {
					allNumeric = false
					break
				}
			}
			if allNumeric && len(identifier) > 1 && identifier[0] == '0' {
				return false
			}
		}
	}
	return true
}

func validNumericIdentifier(identifier string) bool {
	if identifier == "" || (len(identifier) > 1 && identifier[0] == '0') {
		return false
	}
	for _, character := range identifier {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func loadDistributionFiles(root string) ([]archiveFile, error) {
	required := []string{"CHANGELOG.md", "LICENSE", "NOTICE", "README.md", "THIRD_PARTY_NOTICES.md", "third_party/modules.txt"}
	files := make([]archiveFile, 0, len(required)+16)
	for _, name := range required {
		path := filepath.Join(root, name)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect release document %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("release document is not a regular file: %s", name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read release document %s: %w", name, err)
		}
		files = append(files, archiveFile{Name: name, Mode: 0o644, Data: data})
	}
	licensesRoot := filepath.Join(root, "third_party", "licenses")
	licensesInfo, err := os.Lstat(licensesRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect third-party license directory: %w", err)
	}
	if !licensesInfo.IsDir() || licensesInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("third-party license path is not a regular directory: %s", licensesRoot)
	}
	err = filepath.WalkDir(licensesRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == licensesRoot || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("release license path is not a regular file: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolve release license path %s: %w", path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read release license %s: %w", relative, err)
		}
		files = append(files, archiveFile{Name: filepath.ToSlash(relative), Mode: 0o644, Data: data})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect third-party licenses: %w", err)
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Name < files[right].Name })
	return files, nil
}

func buildTarget(ctx context.Context, config packageConfig, releaseTarget target, binariesDir, packagesDir string, distributionFiles []archiveFile) error {
	rootName := fmt.Sprintf("cailab_%s_%s_%s", config.Version, releaseTarget.GOOS, releaseTarget.GOARCH)
	binaryName := "cailab"
	if releaseTarget.GOOS == "windows" {
		binaryName += ".exe"
	}
	stageDir := filepath.Join(binariesDir, rootName)
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return fmt.Errorf("create stage directory for %s/%s: %w", releaseTarget.GOOS, releaseTarget.GOARCH, err)
	}
	binaryPath := filepath.Join(stageDir, binaryName)
	ldflags := fmt.Sprintf("-s -w -X=main.version=%s -X=main.commit=%s -X=main.date=%s", config.Version, config.Commit, config.Date.Format(time.RFC3339))
	command := exec.CommandContext(ctx, "go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", binaryPath, "./cmd/cailab")
	command.Env = releaseEnvironment(os.Environ(), releaseTarget)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build %s/%s: %w: %s", releaseTarget.GOOS, releaseTarget.GOARCH, err, strings.TrimSpace(string(output)))
	}
	binary, err := os.ReadFile(binaryPath)
	if err != nil {
		return fmt.Errorf("read %s/%s binary: %w", releaseTarget.GOOS, releaseTarget.GOARCH, err)
	}
	files := []archiveFile{{Name: filepath.ToSlash(filepath.Join(rootName, binaryName)), Mode: 0o755, Data: binary}}
	for _, distributionFile := range distributionFiles {
		distributionFile.Name = filepath.ToSlash(filepath.Join(rootName, filepath.FromSlash(distributionFile.Name)))
		files = append(files, distributionFile)
	}
	archivePath := filepath.Join(packagesDir, rootName+"."+releaseTarget.Format)
	if err := writeArchive(archivePath, releaseTarget.Format, config.Date, files); err != nil {
		return fmt.Errorf("archive %s/%s: %w", releaseTarget.GOOS, releaseTarget.GOARCH, err)
	}
	return nil
}

func linkedReleaseModules(ctx context.Context, packagePath string) ([]string, error) {
	if strings.TrimSpace(packagePath) == "" {
		return nil, errors.New("--package is required")
	}
	const moduleTemplate = `{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}}{{end}}{{end}}`
	modules := make(map[string]struct{})
	for _, releaseTarget := range releaseTargets {
		command := exec.CommandContext(ctx, "go", "list", "-mod=readonly", "-deps", "-f", moduleTemplate, packagePath)
		command.Env = releaseEnvironment(os.Environ(), releaseTarget)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		err := command.Run()
		if err != nil {
			return nil, fmt.Errorf("list linked modules for %s/%s: %w: %s", releaseTarget.GOOS, releaseTarget.GOARCH, err, strings.TrimSpace(stderr.String()))
		}
		for _, line := range strings.Split(stdout.String(), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				modules[line] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(modules))
	for module := range modules {
		result = append(result, module)
	}
	sort.Strings(result)
	return result, nil
}

func releaseEnvironment(environment []string, releaseTarget target) []string {
	overrides := map[string]string{
		"CGO_ENABLED": "0",
		"GOARCH":      releaseTarget.GOARCH,
		"GOOS":        releaseTarget.GOOS,
	}
	result := make([]string, 0, len(environment)+len(overrides))
	for _, item := range environment {
		name, _, ok := strings.Cut(item, "=")
		if ok {
			if _, replaced := overrides[name]; replaced {
				continue
			}
		}
		result = append(result, item)
	}
	for _, name := range []string{"CGO_ENABLED", "GOARCH", "GOOS"} {
		result = append(result, name+"="+overrides[name])
	}
	return result
}

func writeArchive(path, format string, modified time.Time, files []archiveFile) (returnErr error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() {
		if err := file.Close(); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("close %s: %w", path, err)
		}
	}()
	switch format {
	case "tar.gz":
		return writeTarGzip(file, modified, files)
	case "zip":
		return writeZip(file, modified, files)
	default:
		return fmt.Errorf("unsupported archive format %q", format)
	}
}

func writeTarGzip(destination io.Writer, modified time.Time, files []archiveFile) error {
	gzipWriter, err := gzip.NewWriterLevel(destination, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	gzipWriter.Header.ModTime = modified
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, file := range files {
		header := &tar.Header{Name: file.Name, Mode: int64(file.Mode.Perm()), Size: int64(len(file.Data)), ModTime: modified, Format: tar.FormatUSTAR}
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header %s: %w", file.Name, err)
		}
		if _, err := tarWriter.Write(file.Data); err != nil {
			return fmt.Errorf("write tar file %s: %w", file.Name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}
	return nil
}

func writeZip(destination io.Writer, modified time.Time, files []archiveFile) error {
	zipWriter := zip.NewWriter(destination)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.Name, Method: zip.Deflate, Modified: modified}
		header.SetMode(file.Mode)
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("write zip header %s: %w", file.Name, err)
		}
		if _, err := writer.Write(file.Data); err != nil {
			return fmt.Errorf("write zip file %s: %w", file.Name, err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("close zip writer: %w", err)
	}
	return nil
}

func writeChecksums(directory, outputName string) (int, error) {
	if filepath.Base(outputName) != outputName || outputName == "." || outputName == "" {
		return 0, fmt.Errorf("invalid checksum output filename %q", outputName)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, fmt.Errorf("read release assets %s: %w", directory, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == outputName {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".tar.gz") || strings.HasSuffix(entry.Name(), ".zip") || strings.HasSuffix(entry.Name(), ".spdx.json") {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return 0, fmt.Errorf("no release assets found in %s", directory)
	}
	sort.Strings(names)
	var manifest strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return 0, fmt.Errorf("read release asset %s: %w", name, err)
		}
		digest := sha256.Sum256(data)
		fmt.Fprintf(&manifest, "%s  %s\n", hex.EncodeToString(digest[:]), name)
	}
	path := filepath.Join(directory, outputName)
	if err := os.WriteFile(path, []byte(manifest.String()), 0o644); err != nil {
		return 0, fmt.Errorf("write checksum manifest %s: %w", path, err)
	}
	return len(names), nil
}
