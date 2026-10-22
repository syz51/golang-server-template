// containercheck builds the working tree using ordinary Docker and checks the
// shell-free final image. Run: go run ./tools/containercheck [-image TAG]
// A supplied image is inspected and tested, never removed; without -image the
// tool builds and removes its uniquely named image as well as its container.
package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/syz51/golang-server-template/tools/internal/check"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func docker(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	if len(args) > 0 && args[0] == "logs" {
		out = append(out, stderr.Bytes()...)
	}
	return strings.TrimSpace(string(out)), nil
}
func run() error {
	imageFlag := flag.String("image", "", "existing image to validate (default: build current working tree)")
	root := flag.String("root", ".", "Docker build context")
	flag.Parse()
	scratch, err := os.MkdirTemp("", "containercheck-")
	if err != nil {
		return err
	}
	defer check.Cleanup(scratch)
	id := filepath.Base(scratch)
	image := *imageFlag
	if image == "" {
		image = "templatecheck:" + strings.ToLower(id)
		defer func() {
			if _, err := docker("image", "rm", "-f", image); err != nil {
				fmt.Fprintln(os.Stderr, "image cleanup:", err)
			}
		}()
		if err := check.Run(*root, nil, "docker", "build", "--force-rm", "-t", image, "."); err != nil {
			return err
		}
	}
	inspect, err := docker("image", "inspect", image)
	if err != nil {
		return err
	}
	var images []struct {
		Config struct {
			User, WorkingDir string
			Env              []string
		}
	}
	if err := json.Unmarshal([]byte(inspect), &images); err != nil {
		return err
	}
	if len(images) != 1 {
		return fmt.Errorf("expected one image")
	}
	config := images[0].Config
	if config.User != "65532" && config.User != "65532:65532" && config.User != "nonroot" && config.User != "nonroot:nonroot" {
		return fmt.Errorf("unexpected runtime user %q", config.User)
	}
	if config.WorkingDir != "/app" {
		return fmt.Errorf("unexpected WORKDIR %q", config.WorkingDir)
	}
	production := false
	for _, env := range config.Env {
		if env == "APP_APP_ENVIRONMENT=production" {
			production = true
		}
	}
	if !production {
		return fmt.Errorf("image does not explicitly default to production")
	}
	// Register cleanup before create: even a partially failed create is removed.
	defer func() { _, _ = docker("rm", "-f", id) }()
	container, err := docker("create", "--name", id, "-p", "127.0.0.1::8080", "-e", "APP_SERVER_HOST=0.0.0.0", "-e", "APP_SERVER_PORT=8080", "-e", "APP_SERVER_MAX_BODY_BYTES=1048576", image)
	if err != nil {
		return err
	}
	if err := inspectFilesystem(container, scratch); err != nil {
		return err
	}
	if _, err := docker("start", container); err != nil {
		return err
	}
	defer func() { logs, _ := docker("logs", container); fmt.Printf("container log:\n%s\n", logs) }()
	port, err := docker("port", container, "8080/tcp")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(port, "127.0.0.1:") {
		return fmt.Errorf("unexpected published port %q", port)
	}
	if err := check.Smoke("http://" + port); err != nil {
		return err
	}
	if _, err := docker("stop", "--signal", "SIGTERM", "--timeout", "10", container); err != nil {
		return err
	}
	state, err := docker("inspect", "--format", "{{.State.ExitCode}} {{.State.Running}}", container)
	if err != nil {
		return err
	}
	if state != "0 false" {
		return fmt.Errorf("SIGTERM state %q, want exit 0 and stopped", state)
	}
	fmt.Println("container nonroot, production, filesystem, HTTP and SIGTERM checks passed")
	return nil
}

func inspectFilesystem(container, scratch string) error {
	archive := filepath.Join(scratch, "rootfs.tar")
	if _, err := docker("export", "-o", archive, container); err != nil {
		return err
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	license := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(h.Name, "./")
		name = strings.TrimPrefix(name, "/")
		for _, part := range strings.Split(name, "/") {
			if part == ".git" || part == ".env" || strings.HasPrefix(part, ".env.") {
				return fmt.Errorf("unexpected sensitive image path %s", name)
			}
		}
		switch filepath.Base(name) {
		case "sh", "bash", "ash", "dash", "zsh", "ksh", "fish", "busybox":
			return fmt.Errorf("unexpected shell in final image: %s", name)
		}
		if name == "licenses/LICENSE" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(data)) == 0 {
				return fmt.Errorf("empty LICENSE")
			}
			license = true
		}
	}
	if !license {
		return fmt.Errorf("missing /licenses/LICENSE")
	}
	return nil
}
