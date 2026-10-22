package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/syz51/golang-server-template/tools/internal/check"
)

func TestValidVersion(t *testing.T) {
	for _, version := range []string{"v0.1.0", "v1.2.3-rc.1", "v2.0.0", "v1.2.3-0.a-1", strings.Repeat("a", 40)} {
		if !validVersion(version) {
			t.Errorf("rejected immutable version %q", version)
		}
	}
	for _, version := range []string{"", "latest", "main", "HEAD", "master", "v1", "v1.2", "v01.2.3", "v1.2.3-01", "v1.2.3+build", "v1.2.3\n", " v1.2.3", "-version", "abc1234", strings.Repeat("g", 40), "refs/tags/v1.2.3", "v1.2.3/path"} {
		if validVersion(version) {
			t.Errorf("accepted unsafe or noncanonical version %q", version)
		}
	}
}

func TestInvalidInputPrecedesGoInvocation(t *testing.T) {
	// An empty PATH makes any accidental Go/network invocation fail differently.
	t.Setenv("PATH", "")
	for _, args := range [][]string{nil, {"-version", "latest"}, {"-version", "main"}, {"-version", "v1.0.0", "extra"}} {
		if err := run(args); err == nil || !strings.Contains(err.Error(), "-version requires") {
			t.Errorf("run(%q) = %v, want early ref rejection", args, err)
		}
	}
	for _, args := range [][]string{
		{"-version", "v1.0.0", "-commit", ""},
		{"-version", "v1.0.0", "-commit", "abc1234"},
		{"-version", "v1.0.0", "-commit", strings.Repeat("g", 40)},
		{"-version", "v1.0.0", "-commit"},
		{"-version", strings.Repeat("a", 40), "-commit", strings.Repeat("b", 40)},
	} {
		if err := run(args); err == nil || !strings.Contains(err.Error(), "commit") || strings.Contains(err.Error(), "executable file") {
			t.Errorf("run(%q) = %v, want early commit rejection", args, err)
		}
	}
}

func TestPublicSourceIdentity(t *testing.T) {
	const source = "example.com/source"
	commit := strings.Repeat("a", 40)
	const pseudo = "v0.0.0-20261023000000-aaaaaaaaaaaa"
	for _, tc := range []struct {
		name, query, expected, path, resolved, vcs, hash, downloadError string
		wantOK                                                          bool
	}{
		{name: "bound tag", query: "v1.0.0", expected: commit, path: source, resolved: "v1.0.0", vcs: "git", hash: commit, wantOK: true},
		{name: "case insensitive hash", query: "v1.0.0", expected: commit, path: source, resolved: "v1.0.0", vcs: "git", hash: strings.ToUpper(commit), wantOK: true},
		{name: "commit resolves pseudo version", query: commit, expected: commit, path: source, resolved: pseudo, vcs: "git", hash: commit, wantOK: true},
		{name: "commit resolves tag", query: commit, expected: commit, path: source, resolved: "v1.0.0", vcs: "git", hash: commit, wantOK: true},
		{name: "unbound tag", query: "v1.0.0", path: source, resolved: "v1.0.0", wantOK: true},
		{name: "wrong hash", query: "v1.0.0", expected: commit, path: source, resolved: "v1.0.0", vcs: "git", hash: strings.Repeat("b", 40)},
		{name: "short hash", query: "v1.0.0", expected: commit, path: source, resolved: "v1.0.0", vcs: "git", hash: "aaaaaaa"},
		{name: "missing origin", query: "v1.0.0", expected: commit, path: source, resolved: "v1.0.0"},
		{name: "missing hash", query: "v1.0.0", expected: commit, path: source, resolved: "v1.0.0", vcs: "git"},
		{name: "wrong vcs", query: "v1.0.0", expected: commit, path: source, resolved: "v1.0.0", vcs: "hg", hash: commit},
		{name: "wrong path", query: "v1.0.0", path: source + "/other", resolved: "v1.0.0"},
		{name: "wrong tag", query: "v1.0.0", path: source, resolved: "v1.0.1"},
		{name: "missing version", query: commit, expected: commit, path: source, vcs: "git", hash: commit},
		{name: "moving version", query: commit, expected: commit, path: source, resolved: "main", vcs: "git", hash: commit},
		{name: "unresolved commit", query: commit, expected: commit, path: source, resolved: commit, vcs: "git", hash: commit},
		{name: "download error", query: "v1.0.0", path: source, resolved: "v1.0.0", downloadError: "checksum mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metadata := map[string]any{"Path": tc.path, "Version": tc.resolved, "Error": tc.downloadError}
			if tc.vcs != "" || tc.hash != "" {
				metadata["Origin"] = map[string]string{"VCS": tc.vcs, "Hash": tc.hash}
			}
			data, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			resolved, err := verifySourceJSON(strings.NewReader(string(data)), source, tc.query, tc.expected)
			if (err == nil) != tc.wantOK || (tc.wantOK && resolved != tc.resolved) {
				t.Fatalf("resolved=%q, error=%v, want success=%v, version=%q", resolved, err, tc.wantOK, tc.resolved)
			}
		})
	}
	for _, input := range []string{"invalid", `{}`, `{"Path":"example.com/source","Version":"v1.0.0"} {}`, `{"Path":"example.com/source","Version":"v1.0.0"} trailing`} {
		if _, err := verifySourceJSON(strings.NewReader(input), source, "v1.0.0", ""); err == nil {
			t.Errorf("accepted malformed metadata %q", input)
		}
	}
}

func TestRunBindsSourceBeforeGeneration(t *testing.T) {
	const source = "example.com/source"
	commit := strings.Repeat("a", 40)
	const resolved = "v0.0.0-20261023000000-aaaaaaaaaaaa"
	for _, tc := range []struct {
		name, version, expected, hash string
		wantGenerate                  bool
	}{
		{"bound tag", "v1.0.0", commit, commit, true},
		{"implicit commit", commit, "", commit, true},
		{"same explicit commit", commit, strings.ToUpper(commit), commit, true},
		{"tag mismatch", "v1.0.0", commit, strings.Repeat("b", 40), false},
		{"implicit commit mismatch", commit, "", strings.Repeat("b", 40), false},
		{"implicit commit missing origin", commit, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("PUBLICCHECK_TEST_ROOT", root)
			version := resolved
			if !commitPattern.MatchString(tc.version) {
				version = tc.version
			}
			metadata := map[string]any{"Path": source, "Version": version}
			if tc.hash != "" {
				metadata["Origin"] = map[string]string{"VCS": "git", "Hash": tc.hash}
			}
			data, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("PUBLICCHECK_TEST_METADATA", string(data))
			// Stop at generation, recording the actual subprocess arguments and
			// shared cache. No real Go download, server or Docker command runs.
			const fakeGo = `#!/bin/sh
case "$1" in
list) printf '%s\n' 'example.com/source' ;;
mod)
  [ "$2" = download ] && [ "$3" = -json ] || exit 31
  [ "$GOPROXY" = https://proxy.golang.org ] && [ "$GOSUMDB" = sum.golang.org ] && [ "$GOENV" = off ] || exit 32
  printf '%s\n' "$4" "$GOMODCACHE" > "$PUBLICCHECK_TEST_ROOT/download"
  printf '%s\n' "$PUBLICCHECK_TEST_METADATA"
  ;;
install)
  mkdir -p "$GOBIN" || exit 33
  cat > "$GOBIN/gonew" <<'GONEW'
#!/bin/sh
printf '%s\n' "$1" "$GOMODCACHE" > "$PUBLICCHECK_TEST_ROOT/generate"
exit 23
GONEW
  chmod +x "$GOBIN/gonew"
  ;;
*) exit 34 ;;
esac
`
			if err := os.WriteFile(filepath.Join(root, "go"), []byte(fakeGo), 0700); err != nil {
				t.Fatal(err)
			}
			args := []string{"-root", root, "-version", tc.version}
			if tc.expected != "" {
				args = append(args, "-commit", tc.expected)
			}
			err = run(args)
			if err == nil {
				t.Fatal("expected rejection or deliberate generation stop")
			}
			generated, readErr := os.ReadFile(filepath.Join(root, "generate"))
			if !tc.wantGenerate {
				if !strings.Contains(err.Error(), "expected commit") || !os.IsNotExist(readErr) {
					t.Fatalf("identity rejection: error=%v, generation read=%v", err, readErr)
				}
				return
			}
			if !strings.Contains(err.Error(), "generate public consumer") || readErr != nil {
				t.Fatalf("generation: error=%v, record=%v", err, readErr)
			}
			downloaded, err := os.ReadFile(filepath.Join(root, "download"))
			if err != nil {
				t.Fatal(err)
			}
			downloadLines := strings.Split(strings.TrimSpace(string(downloaded)), "\n")
			want := source + "@" + version + "\n" + downloadLines[1] + "\n"
			if string(generated) != want || downloadLines[0] != source+"@"+tc.version || filepath.Base(downloadLines[1]) != "modcache" {
				t.Fatalf("download=%q, generation=%q, want generation=%q", downloaded, generated, want)
			}
		})
	}
}

func TestPublicEnvironmentOverridesLocalBypasses(t *testing.T) {
	scratch := filepath.Join(t.TempDir(), "isolated")
	for key, value := range map[string]string{
		"GOENV": "/private/goenv", "GOPROXY": "file:///private/proxy,direct",
		"GOSUMDB": "off", "GOPRIVATE": "*", "GONOPROXY": "*", "GONOSUMDB": "*", "GOINSECURE": "*",
		"GOWORK": "/private/go.work", "GOFLAGS": "-modfile=/private/go.mod",
		"GOMODCACHE": "/private/cache", "GO111MODULE": "off", "GOTOOLCHAIN": "auto",
	} {
		t.Setenv(key, value)
	}
	// Match check.Run's environment merge: the last entry for each key wins.
	merged := map[string]string{}
	for _, entry := range append(os.Environ(), publicEnv(scratch)...) {
		key, value, _ := strings.Cut(entry, "=")
		merged[key] = value
	}
	for key, want := range map[string]string{
		"GOENV": "off", "GOPROXY": "https://proxy.golang.org", "GOSUMDB": "sum.golang.org",
		"GOPRIVATE": "", "GONOPROXY": "none", "GONOSUMDB": "none", "GOINSECURE": "", "GOWORK": "off",
		"GOFLAGS": "", "GOMODCACHE": filepath.Join(scratch, "modcache"),
		"GO111MODULE": "on", "GOTOOLCHAIN": "local", "TMPDIR": filepath.Join(scratch, "tmp"),
	} {
		if merged[key] != want {
			t.Errorf("%s = %q, want %q", key, merged[key], want)
		}
	}
}

func TestModuleMetadataRejectsReplacementsAndWrongModule(t *testing.T) {
	for _, input := range []string{
		`{"Module":{"Path":"example.com/source"}}`,
		`{"Module":{"Path":"example.com/public-consumer"},"Replace":[{"New":{"Path":"../local"}}]}`,
		`{"Module":{"Path":"example.com/public-consumer"},"Replace":[{"New":{"Path":"example.com/other","Version":"v1.0.0"}}]}`,
		`invalid`,
	} {
		if err := verifyModuleJSON(strings.NewReader(input), "example.com/public-consumer"); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
	if err := verifyModuleJSON(strings.NewReader(`{"Module":{"Path":"example.com/public-consumer"}}`), "example.com/public-consumer"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyConsumerImports(t *testing.T) {
	scratch, err := os.MkdirTemp("", "publiccheck-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer check.Cleanup(scratch)
	const source, target = "example.com/source", "example.com/public-consumer"
	if err := os.WriteFile(filepath.Join(scratch, "go.mod"), []byte("module "+target+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, imp := range []string{source, source + "/internal/config", target + "/internal/config", source + "-other"} {
		if err := os.WriteFile(filepath.Join(scratch, "main.go"), []byte("package main\nimport _ "+strconv.Quote(imp)+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		err := verifyConsumer(scratch, source, target, []string{"GOENV=off", "GOWORK=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly"})
		stale := imp == source || strings.HasPrefix(imp, source+"/")
		if (err != nil) != stale {
			t.Errorf("import %q: %v, stale=%v", imp, err, stale)
		}
	}
}

func TestDestinationDiffersFromSource(t *testing.T) {
	for _, source := range []string{"example.com/source", "example.com/public-consumer", "example.com/public-consumer-next"} {
		if destination(source) == source {
			t.Errorf("destination reused %q", source)
		}
	}
}
