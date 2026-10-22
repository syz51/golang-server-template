package main

import "testing"

func TestValidTag(t *testing.T) {
	for _, tag := range []string{"v0.0.0", "v1.2.3", "v1.2.3-rc.1", "v1.2.3-alpha-1"} {
		if !validTag(tag, "example.com/server") {
			t.Errorf("rejected %q", tag)
		}
	}
	for _, tag := range []string{"", "1.2.3", "v01.2.3", "v1.2", "v1.2.3.4", "v1.2.3-01", "v1.2.3-rc.01", "v1.2.3-", "v1.2.3+", "v1.2.3\n", "v2.0.0", "v2.0.0-rc.1", "v1.2.3+build.01", "v2.0.0+incompatible"} {
		if validTag(tag, "example.com/server") {
			t.Errorf("accepted %q", tag)
		}
	}
}

func TestModuleMajor(t *testing.T) {
	for _, tc := range []struct {
		module, tag string
		want        bool
	}{
		{"example.com/server/v2", "v2.0.0", true},
		{"example.com/server/v2", "v2.1.0-rc.1", true},
		{"example.com/server/v2", "v1.2.3", false},
		{"example.com/server/v2", "v3.0.0", false},
		{"example.com/server/v2", "v2.0.0+build", false},
		{"example.com/server/v20", "v20.0.0", true},
		{"gopkg.in/yaml.v2", "v2.0.0", true},
		{"gopkg.in/yaml.v1", "v0.1.0", false},
		{"example.com/server", "v0.1.0-beta.1", true},
	} {
		if got := validTag(tc.tag, tc.module); got != tc.want {
			t.Errorf("validTag(%q, %q) = %v, want %v", tc.tag, tc.module, got, tc.want)
		}
	}
}
