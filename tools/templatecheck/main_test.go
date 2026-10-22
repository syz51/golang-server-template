package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/syz51/golang-server-template/tools/internal/check"
)

const sourceModule = "example.com/Acme/custom-server"
const targetModule = "example.com/generated-server"
const version = "v0.0.0-templatecheck"

func TestSnapshotUsesWorkingTreeAndExcludesPrivateFiles(t *testing.T) {
	scratch, err := os.MkdirTemp("", "templatecheck-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer check.Cleanup(scratch)
	root := filepath.Join(scratch, "source")
	for name, body := range map[string]string{"go.mod": "module " + sourceModule + "\n", "untracked/asset.yaml": "current-working-tree", ".env": "secret", ".env.local": "secret", ".env.example": "example", ".git/config": "private", "tmp/server": "binary", "cert.key": "private"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := snapshot(root, filepath.Join(scratch, "proxy"), sourceModule, version, []byte("module "+sourceModule+"\n")); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(filepath.Join(scratch, "proxy", filepath.FromSlash(escapeModulePath(sourceModule)), "@v", version+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	files := map[string]bool{}
	for _, f := range z.File {
		files[f.Name] = true
	}
	prefix := sourceModule + "@" + version + "/"
	for _, name := range []string{"go.mod", "untracked/asset.yaml", ".env.example"} {
		if !files[prefix+name] {
			t.Errorf("missing %s", name)
		}
	}
	if len(files) != 3 {
		t.Fatalf("unexpected archive contents: %v", files)
	}
}

func TestVerifyRejectsStaleImport(t *testing.T) {
	scratch, err := os.MkdirTemp("", "templatecheck-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer check.Cleanup(scratch)
	if err := os.WriteFile(filepath.Join(scratch, "go.mod"), []byte("module "+targetModule+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source := []byte("package example\nimport _ \"" + sourceModule + "/internal/config\"\n")
	if err := os.WriteFile(filepath.Join(scratch, "example.go"), source, 0600); err != nil {
		t.Fatal(err)
	}
	if err := verify(scratch, sourceModule, targetModule); err == nil {
		t.Fatal("accepted stale template import")
	}
	if err := os.WriteFile(filepath.Join(scratch, "example.go"), []byte("package example\nimport _ \""+targetModule+"/internal/config\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verify(scratch, sourceModule, targetModule); err != nil {
		t.Fatal(err)
	}
}

func TestDestinationDiffersFromSource(t *testing.T) {
	for _, source := range []string{sourceModule, targetModule, "example.com/generated-server-next"} {
		if got := destination(source); got == source {
			t.Errorf("destination(%q) reused the source", source)
		}
	}
}
