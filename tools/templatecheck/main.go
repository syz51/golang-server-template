// templatecheck validates gonew against the current working tree, without a commit.
// Run from the repository root: go run ./tools/templatecheck
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/syz51/golang-server-template/tools/internal/check"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	rootFlag := flag.String("root", ".", "template working tree root")
	skipNested := flag.Bool("skip-nested", false, "skip the generated consumer toolchain check (used by the bounded child run)")
	flag.Parse()
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		return err
	}
	scratch, err := os.MkdirTemp("", "templatecheck-")
	if err != nil {
		return err
	}
	defer check.Cleanup(scratch)
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	sourceModule, err := check.ModulePath(root)
	if err != nil {
		return err
	}
	targetModule := destination(sourceModule)
	version := "v0.0.0-templatecheck"
	if major := check.ModuleMajor(sourceModule); major != "" {
		version = "v" + major + ".0.0-templatecheck"
	}
	proxy := filepath.Join(scratch, "proxy")
	if err := snapshot(root, proxy, sourceModule, version, mod); err != nil {
		return err
	}
	toolbin := filepath.Join(scratch, "bin")
	if err := check.Run(root, []string{"GOBIN=" + toolbin, "GOWORK=off"}, "go", "install", "golang.org/x/tools/cmd/gonew@v0.52.0"); err != nil {
		return err
	}
	proxyURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(proxy)}).String()
	// This cache contains only the synthetic template. Dependencies use the normal
	// cache after gonew has copied the template. The file proxy has no network fallback.
	env := []string{"GOPROXY=" + proxyURL, "GOSUMDB=off", "GONOPROXY=none", "GOPRIVATE=", "GOFLAGS=", "GOMODCACHE=" + filepath.Join(scratch, "modcache"), "GOWORK=off", "GOTOOLCHAIN=local"}
	if err := check.Run(scratch, env, filepath.Join(toolbin, "gonew"), sourceModule+"@"+version, targetModule, "./generated"); err != nil {
		return err
	}
	generated := filepath.Join(scratch, "generated")
	if err := verify(generated, sourceModule, targetModule); err != nil {
		return err
	}
	if !*skipNested {
		if err := check.Run(generated, []string{"GOWORK=off", "GOFLAGS=-mod=readonly"}, "go", "run", "./tools/templatecheck", "-skip-nested"); err != nil {
			return fmt.Errorf("generated consumer toolchain: %w", err)
		}
	}
	for _, args := range [][]string{{"test", "-race", "./..."}, {"vet", "./..."}, {"build", "-o", filepath.Join(scratch, "server"), "./cmd/server"}} {
		if err := check.Run(generated, []string{"GOWORK=off", "GOFLAGS=-mod=readonly"}, "go", args...); err != nil {
			return err
		}
	}
	if err := nativeSmoke(scratch, filepath.Join(scratch, "server")); err != nil {
		return err
	}
	fmt.Println("current-working-tree generation, race tests, vet, build and native smoke passed")
	return nil
}

// Keep the destination distinct when a generated consumer runs this tool again.
func destination(source string) string {
	target := "example.com/generated-server"
	if source == target {
		return "example.com/generated-server-next"
	}
	return target
}

// The module proxy protocol escapes uppercase letters in request paths.
func escapeModulePath(path string) string {
	var escaped strings.Builder
	for _, r := range path {
		if r >= 'A' && r <= 'Z' {
			escaped.WriteByte('!')
			r += 'a' - 'A'
		}
		escaped.WriteRune(r)
	}
	return escaped.String()
}

func excluded(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch part {
		case ".git", ".ssh", ".aws", ".idea", ".vscode", ".cache", ".DS_Store", "tmp", "bin", "build", "dist", "vendor", "node_modules":
			return true
		}
		lower := strings.ToLower(part)
		if (lower == ".env" || strings.HasPrefix(lower, ".env.")) && lower != ".env.example" {
			return true
		}
		if strings.HasPrefix(lower, "coverage") || lower == "credentials" || strings.HasPrefix(lower, "id_rsa") || strings.HasPrefix(lower, "id_ed25519") {
			return true
		}
		for _, suffix := range []string{".pem", ".key", ".p12", ".pfx", ".out", ".test", ".log", ".exe"} {
			if strings.HasSuffix(lower, suffix) {
				return true
			}
		}
	}
	if path == "server" || path == "server.exe" {
		return true
	}
	return false
}

func snapshot(root, proxy, sourceModule, version string, mod []byte) error {
	dir := filepath.Join(proxy, filepath.FromSlash(escapeModulePath(sourceModule)), "@v")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if excluded(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h := &zip.FileHeader{Name: sourceModule + "@" + version + "/" + filepath.ToSlash(rel), Method: zip.Deflate}
		h.SetMode(info.Mode())
		out, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	})
	if err != nil {
		return err
	}
	if err = zw.Close(); err != nil {
		return err
	}
	info, _ := json.Marshal(map[string]string{"Version": version, "Time": "2000-01-01T00:00:00Z"})
	for name, data := range map[string][]byte{version + ".zip": archive.Bytes(), version + ".mod": mod, version + ".info": info, "list": []byte(version + "\n")} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			return err
		}
	}
	return nil
}

func verify(root, sourceModule, targetModule string) error {
	modulePath, err := check.ModulePath(root)
	if err != nil {
		return err
	}
	if modulePath != targetModule {
		return fmt.Errorf("gonew did not rewrite module directive")
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
			s, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if s == sourceModule || strings.HasPrefix(s, sourceModule+"/") {
				return fmt.Errorf("unrewritten import in %s: %s", path, s)
			}
		}
		return nil
	})
}

func nativeSmoke(dir, binary string) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	// Run outside the checkout so local dotenv files cannot affect validation.
	cmd := exec.Command(binary)
	cmd.Dir = dir
	env := []string{}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "APP_") {
			env = append(env, value)
		}
	}
	cmd.Env = append(env, "APP_SERVER_HOST=127.0.0.1", "APP_SERVER_PORT="+strconv.Itoa(port), "APP_APP_ENVIRONMENT=production", "APP_SERVER_MAX_BODY_BYTES=1048576")
	log, err := os.Create(filepath.Join(dir, "server.log"))
	if err != nil {
		return err
	}
	defer log.Close()
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			<-done
		}
		data, _ := os.ReadFile(log.Name())
		fmt.Printf("native server log:\n%s", data)
	}()
	if err := check.Smoke("http://127.0.0.1:" + strconv.Itoa(port)); err != nil {
		return err
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	select {
	case err := <-done:
		waited = true
		if err != nil {
			return fmt.Errorf("SIGTERM exit: %w", err)
		}
	case <-time.After(10 * time.Second):
		return fmt.Errorf("server did not exit after SIGTERM")
	}
	return nil
}
