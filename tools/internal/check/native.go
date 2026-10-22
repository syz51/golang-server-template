package check

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// NativeSmoke runs a built server and verifies HTTP behavior and graceful SIGTERM.
func NativeSmoke(dir, binary string) error {
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
	if err := Smoke("http://127.0.0.1:" + strconv.Itoa(port)); err != nil {
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
