package check

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModulePathUsesCurrentRoot(t *testing.T) {
	scratch, err := os.MkdirTemp("", "modulecheck-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer Cleanup(scratch)
	// Go handles quoted directives and comments; do not assume a byte prefix.
	for _, module := range []string{"example.com/Acme/custom-server", "example.com/generated-server", "example.com/server/v2"} {
		if err := os.WriteFile(filepath.Join(scratch, "go.mod"), []byte("// consumer module\nmodule \""+module+"\" // current root\n"), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := ModulePath(scratch)
		if err != nil || got != module {
			t.Fatalf("ModulePath = %q, %v; want %q", got, err, module)
		}
	}
}
