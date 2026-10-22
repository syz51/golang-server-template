// publiccheck validates an immutable template version as a public consumer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/syz51/golang-server-template/tools/internal/check"
)

var tagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)
var commitPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
var numericPattern = regexp.MustCompile(`^[0-9]+$`)

func validVersion(version string) bool {
	if commitPattern.MatchString(version) {
		return true
	}
	if !tagPattern.MatchString(version) {
		return false
	}
	if i := strings.IndexByte(version, '-'); i >= 0 {
		for _, part := range strings.Split(version[i+1:], ".") {
			if len(part) > 1 && part[0] == '0' && numericPattern.MatchString(part) {
				return false
			}
		}
	}
	return true
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("publiccheck", flag.ContinueOnError)
	rootFlag := flags.String("root", ".", "source module working tree root")
	version := flags.String("version", "", "required exact vMAJOR.MINOR.PATCH[-PRERELEASE] tag or full 40-character commit")
	commit := flags.String("commit", "", "optional expected full 40-character source commit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	// Reject moving refs before even invoking Go to discover the source module.
	if flags.NArg() != 0 || !validVersion(*version) {
		return fmt.Errorf("-version requires an exact release tag or full 40-character commit; moving refs and positional arguments are not allowed")
	}
	commitSupplied := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "commit" {
			commitSupplied = true
		}
	})
	if commitSupplied && !commitPattern.MatchString(*commit) {
		return fmt.Errorf("-commit requires a full 40-character source commit")
	}
	expected := *commit
	if commitPattern.MatchString(*version) {
		if commitSupplied && !strings.EqualFold(*version, expected) {
			return fmt.Errorf("-commit conflicts with full commit supplied by -version")
		}
		expected = *version
	}
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		return err
	}
	source, err := check.ModulePath(root)
	if err != nil {
		return err
	}
	if !commitPattern.MatchString(*version) {
		major := check.ModuleMajor(source)
		got := tagPattern.FindStringSubmatch(*version)[1]
		if (major == "" && got != "0" && got != "1") || (major != "" && got != major) {
			return fmt.Errorf("version %q does not match module %q major", *version, source)
		}
	}
	scratch, err := os.MkdirTemp("", "publiccheck-")
	if err != nil {
		return err
	}
	defer check.Cleanup(scratch)
	if err := os.Mkdir(filepath.Join(scratch, "tmp"), 0700); err != nil {
		return err
	}
	env := publicEnv(scratch)
	resolved, err := downloadSource(scratch, env, source, *version, expected)
	if err != nil {
		return err
	}
	toolbin := filepath.Join(scratch, "bin")
	installEnv := append(append([]string{}, env...), "GOBIN="+toolbin)
	if err := check.Run(scratch, installEnv, "go", "install", "golang.org/x/tools/cmd/gonew@v0.52.0"); err != nil {
		return fmt.Errorf("install public gonew: %w", err)
	}
	target := destination(source)
	if err := check.Run(scratch, env, filepath.Join(toolbin, "gonew"), source+"@"+resolved, target, "./consumer"); err != nil {
		return fmt.Errorf("generate public consumer: %w", err)
	}
	consumer := filepath.Join(scratch, "consumer")
	// Keep the public settings but let ordinary dependency builds use the normal
	// cache. Generation above always uses the fresh, isolated module cache.
	buildEnv := []string{}
	for _, entry := range env {
		if !strings.HasPrefix(entry, "GOMODCACHE=") && !strings.HasPrefix(entry, "GOFLAGS=") {
			buildEnv = append(buildEnv, entry)
		}
	}
	buildEnv = append(buildEnv, "GOFLAGS=-mod=readonly")
	if err := verifyConsumer(consumer, source, target, buildEnv); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"test", "./..."},
		{"test", "-race", "./..."},
		{"vet", "./..."},
		{"build", "-o", filepath.Join(scratch, "server"), "./cmd/server"},
	} {
		if err := check.Run(consumer, buildEnv, "go", args...); err != nil {
			return fmt.Errorf("public consumer validation: %w", err)
		}
	}
	if err := check.NativeSmoke(scratch, filepath.Join(scratch, "server")); err != nil {
		return err
	}
	if err := check.Run(consumer, buildEnv, "go", "run", "./tools/containercheck"); err != nil {
		return fmt.Errorf("public consumer production container (Docker daemon required): %w", err)
	}
	fmt.Printf("public consumer %s@%s: generation, tests, race, vet, build, native and container smoke passed\n", source, *version)
	return nil
}

// Download and generation share the isolated public module cache. Pass the
// verified canonical version to gonew rather than resolving a commit again.
func downloadSource(root string, env []string, source, version, expected string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "mod", "download", "-json", source+"@"+version)
	cmd.Dir, cmd.Env = root, append(os.Environ(), env...)
	cmd.Stderr = os.Stderr
	out, runErr := cmd.Output()
	resolved, err := verifySourceJSON(strings.NewReader(string(out)), source, version, expected)
	if runErr != nil {
		return "", fmt.Errorf("download public source: %w (metadata: %v)", runErr, err)
	}
	return resolved, err
}

func verifySourceJSON(r io.Reader, source, version, expected string) (string, error) {
	var mod struct {
		Path, Version, Error string
		Origin               *struct{ VCS, Hash string }
	}
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&mod); err != nil {
		return "", fmt.Errorf("parse public source metadata: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", fmt.Errorf("public source metadata must contain exactly one module")
	}
	if mod.Error != "" {
		return "", fmt.Errorf("download public source: %s", mod.Error)
	}
	if mod.Path != source {
		return "", fmt.Errorf("public source path %q does not match %q", mod.Path, source)
	}
	if !validVersion(mod.Version) || commitPattern.MatchString(mod.Version) ||
		(!commitPattern.MatchString(version) && mod.Version != version) {
		return "", fmt.Errorf("unexpected public source version %q for %q", mod.Version, version)
	}
	if expected != "" && (mod.Origin == nil || mod.Origin.VCS != "git" ||
		!commitPattern.MatchString(mod.Origin.Hash) || !strings.EqualFold(mod.Origin.Hash, expected)) {
		return "", fmt.Errorf("public source origin must be git at expected commit %s", expected)
	}
	return mod.Version, nil
}

// GOENV=off prevents persisted go env settings from restoring private proxies or
// checksum exemptions. A sole HTTPS proxy has no direct/VCS or file fallback.
func publicEnv(scratch string) []string {
	return []string{
		"GOENV=off", "GO111MODULE=on", "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local",
		"GOPROXY=https://proxy.golang.org", "GOSUMDB=sum.golang.org",
		"GOPRIVATE=", "GONOPROXY=none", "GONOSUMDB=none", "GOINSECURE=",
		"GOMODCACHE=" + filepath.Join(scratch, "modcache"),
		"TMPDIR=" + filepath.Join(scratch, "tmp"),
		"TMP=" + filepath.Join(scratch, "tmp"), "TEMP=" + filepath.Join(scratch, "tmp"),
	}
}

func destination(source string) string {
	if source == "example.com/public-consumer" {
		return "example.com/public-consumer-next"
	}
	return "example.com/public-consumer"
}

func verifyConsumer(root, source, target string, env []string) error {
	// Go parses the module file without modifying it. Reject replacements before
	// any consumer command can resolve dependencies through a local path.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "mod", "edit", "-json")
	cmd.Dir, cmd.Env = root, append(os.Environ(), env...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("read generated module: %w", err)
	}
	if err := verifyModuleJSON(strings.NewReader(string(out)), target); err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			value, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if value == source || strings.HasPrefix(value, source+"/") {
				return fmt.Errorf("unrewritten public consumer import in %s: %s", path, value)
			}
		}
		return nil
	})
}

func verifyModuleJSON(r io.Reader, target string) error {
	var mod struct {
		Module  struct{ Path string }
		Replace []json.RawMessage
	}
	if err := json.NewDecoder(r).Decode(&mod); err != nil {
		return fmt.Errorf("parse generated module: %w", err)
	}
	if mod.Module.Path != target {
		return fmt.Errorf("gonew did not rewrite module directive to %q", target)
	}
	if len(mod.Replace) != 0 {
		return fmt.Errorf("public consumer must not contain module replacements")
	}
	return nil
}
