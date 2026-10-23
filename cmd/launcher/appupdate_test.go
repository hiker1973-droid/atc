package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.2.0", "0.1.0", true}, {"v0.10.0", "0.9.9", true}, {"1.0.0", "1.0.0", false},
		{"0.1.0", "0.2.0", false}, {"0.2.0-beta", "0.1.5", true}, {"junk", "0.1.0", false},
	} {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%s, %s) = %v", c.a, c.b, got)
		}
	}
}

// An installed 0.1.0 sees GitHub's 0.2.0, downloads it, and refuses a
// download whose checksum does not match GitHub's.
func TestAppUpdateCheckAndDownload(t *testing.T) {
	withRoot(t)
	setup := []byte("pretend installer")
	sum := sha256.Sum256(setup)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			fmt.Fprintf(w, `{"tag_name":"v0.2.0","body":"Faster towers.","html_url":"x","assets":[
				{"name":"Readback-Setup.exe","browser_download_url":"%s/dl","size":%d,"digest":"%s"}]}`, srv.URL, len(setup), digest)
		case r.URL.Path == "/dl":
			w.Write(setup)
		}
	}))
	defer srv.Close()
	oldAPI, oldVer := githubAPI, productVersion
	githubAPI, productVersion = srv.URL, "0.1.0"
	t.Cleanup(func() { githubAPI, productVersion = oldAPI, oldVer; appUpd = appUpdateState{} })

	s := checkAppUpdate()
	if !s.Available || s.Latest.Version != "0.2.0" || s.Latest.Notes != "Faster towers." || s.Error != "" {
		t.Fatalf("%+v", s)
	}
	path, err := downloadRelease(s.Latest)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != string(setup) {
		t.Fatal("download differs")
	}
	bad := *s.Latest
	bad.Digest = "sha256:" + strings.Repeat("0", 64)
	if _, err := downloadRelease(&bad); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("bad checksum accepted: %v", err)
	}

	productVersion = "0.2.0"
	if s := checkAppUpdate(); s.Available {
		t.Fatal("0.2.0 offered its own version")
	}
}

// A git checkout (the vSFG-7 rigs) never offers installer updates.
func TestSourceInstallNoAppUpdates(t *testing.T) {
	root := withRoot(t)
	os.Mkdir(root+`\.git`, 0o755)
	old := productVersion
	productVersion = "0.1.0"
	t.Cleanup(func() { productVersion = old })
	if updatesEnabled() {
		t.Fatal("source install would check GitHub releases")
	}
}
