package check

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ModulePath lets Go parse the current go.mod, independent of any workspace.
func ModulePath(root string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Path}}")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=readonly")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("read module path: %w", err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" || strings.ContainsAny(path, "\r\n") {
		return "", fmt.Errorf("invalid module path %q", path)
	}
	return path, nil
}

var majorSuffix = regexp.MustCompile(`/v([1-9][0-9]*)$`)
var gopkgMajorSuffix = regexp.MustCompile(`\.v(0|[1-9][0-9]*)(-unstable)?$`)

// ModuleMajor returns an explicit major suffix, or empty for v0/v1 modules.
// gopkg.in uses .vN suffixes, including for v0 and v1.
func ModuleMajor(path string) string {
	pattern := majorSuffix
	if strings.HasPrefix(path, "gopkg.in/") {
		pattern = gopkgMajorSuffix
	}
	if match := pattern.FindStringSubmatch(path); match != nil {
		return match[1]
	}
	return ""
}
