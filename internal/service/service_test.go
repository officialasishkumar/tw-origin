package service

import (
	"os"
	"path/filepath"
	"testing"
)

func load(t *testing.T, name string) *File {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// helm: obs_scm clone of GitHub plus go_modules vendoring (ignored).
func TestHelm(t *testing.T) {
	f := load(t, "helm_service.xml")
	if len(f.Services) != 5 {
		t.Fatalf("got %d services, want 5", len(f.Services))
	}
	o := f.Origins()
	if len(o) != 1 {
		t.Fatalf("origins = %+v, want one", o)
	}
	if o[0].Service != "obs_scm" || o[0].URL != "https://github.com/helm/helm.git" || o[0].Host != "github.com" || o[0].SCM != "git" || o[0].Revision != "refs/tags/v4.3.1" || o[0].Mode != "manual" {
		t.Errorf("origin = %+v", o[0])
	}
	if f.UsesSourceURLs() {
		t.Error("helm has no download_files service")
	}
}

// fzf: download_files, so the spec's Source URL is the origin.
func TestFzf(t *testing.T) {
	f := load(t, "fzf_service.xml")
	if !f.UsesSourceURLs() {
		t.Error("download_files should mean the Source URLs are used")
	}
	if o := f.Origins(); len(o) != 0 {
		t.Errorf("origins = %+v, want none", o)
	}
}

// crypto-policies: tar_scm from gitlab.com, mode disabled.
func TestCryptoPolicies(t *testing.T) {
	f := load(t, "crypto-policies_service.xml")
	o := f.Origins()
	if len(o) != 1 || o[0].Service != "tar_scm" || o[0].Host != "gitlab.com" || o[0].Mode != "disabled" || o[0].Revision != "cd6043a774abf6e4d17116f1497a2032f5351d49" {
		t.Errorf("origins = %+v", o)
	}
}

func TestDownloadURLAndOddForms(t *testing.T) {
	f, err := Parse(`<services>
  <service name="download_url">
    <param name="protocol">https</param>
    <param name="host">download.example.org</param>
    <param name="path">pub/thing/thing-1.0.tar.gz</param>
  </service>
  <service name="download_url">
    <param name="host">plain.example.org</param>
    <param name="path">/x.tar</param>
  </service>
  <service name="tar_scm" mode="localonly">
    <param name="url">git@github.com:example/thing.git</param>
    <param name="scm">git</param>
  </service>
  <service name="cargo_vendor"><param name="url">https://crates.io/should-be-ignored</param></service>
  <service name="set_version"/>
</services>`)
	if err != nil {
		t.Fatal(err)
	}
	o := f.Origins()
	if len(o) != 3 {
		t.Fatalf("origins = %+v, want 3", o)
	}
	if o[0].URL != "https://download.example.org/pub/thing/thing-1.0.tar.gz" || o[0].Host != "download.example.org" {
		t.Errorf("download_url: %+v", o[0])
	}
	if o[1].URL != "http://plain.example.org/x.tar" || o[1].Host != "plain.example.org" {
		t.Errorf("download_url default protocol: %+v", o[1])
	}
	if o[2].Host != "github.com" {
		t.Errorf("scp-like git address: %+v", o[2])
	}
}

func TestHostOf(t *testing.T) {
	tests := map[string]string{
		"https://github.com/helm/helm.git":            "github.com",
		"HTTPS://User@Example.ORG:8443/path?q=1#frag": "example.org",
		"git://git.kernel.org/pub/scm/x.git":          "git.kernel.org",
		"git@gitlab.com:group/repo.git":               "gitlab.com",
		"ssh://git@codeberg.org/x/y":                  "codeberg.org",
		"not a url":                                   "",
		"":                                            "",
	}
	for in, want := range tests {
		if got := HostOf(in); got != want {
			t.Errorf("HostOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBadXML(t *testing.T) {
	if _, err := Parse("<services><service"); err == nil {
		t.Error("expected an error for truncated XML")
	}
}
