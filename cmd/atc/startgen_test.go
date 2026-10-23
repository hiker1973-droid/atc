package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vsfg7/atc/pkg/startgen"
)

var reGenLine = regexp.MustCompile(`atc\.exe ([^"]+)"`)

// Every flag a generated start file passes must be one atc.exe has, or the
// role exits on its first start on a new install.
func TestGeneratedStartFilesUseRealFlags(t *testing.T) {
	defs := flagDefaults(t)
	o := startgen.Defaults()
	o.Carrier = true
	for _, theatre := range []string{"caucasus", "syria", "pg", "germany", "iraq", "kola"} {
		text, err := startgen.RolesText(theatre, theatre, o, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		lines := reGenLine.FindAllStringSubmatch(text, -1)
		if len(lines) < 4 {
			t.Fatalf("%s: %d roles", theatre, len(lines))
		}
		for _, m := range lines {
			for _, tok := range strings.Fields(m[1]) {
				if !strings.HasPrefix(tok, "--") {
					continue
				}
				name, _, _ := strings.Cut(strings.TrimPrefix(tok, "--"), "=")
				if _, ok := defs[name]; !ok {
					t.Errorf("%s: atc.exe has no --%s", theatre, name)
				}
			}
		}
	}
}
