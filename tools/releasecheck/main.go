// releasecheck validates canonical release tags against the current module.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/syz51/golang-server-template/tools/internal/check"
)

var semver = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?$`)
var numeric = regexp.MustCompile(`^[0-9]+$`)

func validTag(tag, modulePath string) bool {
	match := semver.FindStringSubmatch(tag)
	if match == nil {
		return false
	}
	major := check.ModuleMajor(modulePath)
	if (major == "" && match[1] != "0" && match[1] != "1") || (major != "" && match[1] != major) {
		return false
	}
	if i := strings.Index(tag, "-"); i >= 0 {
		for _, part := range strings.Split(tag[i+1:], ".") {
			if numeric.MatchString(part) && len(part) > 1 && part[0] == '0' {
				return false
			}
		}
	}
	return true
}
func main() {
	tag := flag.String("tag", "", "release tag, including v prefix")
	root := flag.String("root", ".", "module working tree root")
	flag.Parse()
	modulePath, err := check.ModulePath(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !validTag(*tag, modulePath) {
		fmt.Fprintf(os.Stderr, "invalid release tag %q for module %q; expected vMAJOR.MINOR.PATCH[-PRERELEASE] with matching module major and no build metadata\n", *tag, modulePath)
		os.Exit(1)
	}
	fmt.Println("valid release tag:", *tag)
}
