package origin

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/officialasishkumar/tw-origin/internal/asn"
)

func TestKnown(t *testing.T) {
	tests := []struct {
		host, org, cc string
		known         bool
	}{
		{"github.com", "GitHub (Microsoft)", "US", true},
		{"codeload.github.com", "GitHub (Microsoft)", "US", true},
		{"raw.githubusercontent.com", "GitHub (Microsoft)", "US", true},
		{"gitlab.com", "GitLab", "US", true},
		{"gitlab.gnome.org", "GNOME Foundation", "US", true}, // more specific than *.gnome.org? no: it is *.gnome.org
		{"gitlab.freedesktop.org", "freedesktop.org (X.Org Foundation)", "US", true},
		{"invent.kde.org", "KDE e.V.", "DE", true},
		{"ftp.gnu.org", "Free Software Foundation", "US", true},
		{"gcc.gnu.org", "Sourceware (Software Freedom Conservancy)", "US", true}, // exact beats *.gnu.org
		{"download.savannah.nongnu.org", "Free Software Foundation", "US", true},
		{"www.kernel.org", "Linux Foundation", "US", true},
		{"files.pythonhosted.org", "Python Software Foundation", "US", true},
		{"crates.io", "Rust Foundation", "US", true},
		{"registry.npmjs.org", "GitHub (Microsoft)", "US", true},
		{"downloads.sourceforge.net", "Slashdot Media (SourceForge)", "US", true},
		{"codeberg.org", "Codeberg e.V.", "DE", true},
		{"download.opensuse.org", "openSUSE (SUSE)", "DE", true},
		{"src.opensuse.org", "openSUSE (SUSE)", "DE", true},
		{"www.cpan.org", "The Perl and Raku Foundation (CPAN)", "US", true},
		{"launchpad.net", "Canonical", "GB", true},
		{"bitbucket.org", "Atlassian", "AU", true},
		{"gitee.com", "OSChina (Gitee)", "CN", true},
		{"ftp.openbsd.org", "OpenBSD Foundation", "CA", true},
		{"GitHub.COM.", "GitHub (Microsoft)", "US", true},
		{"notgithub.com", "", "", false},
		{"example.org", "", "", false},
		{"", "", "", false},
	}
	for _, tt := range tests {
		op, ok := Known(tt.host)
		if ok != tt.known {
			t.Errorf("Known(%q) ok = %v, want %v", tt.host, ok, tt.known)
			continue
		}
		if ok && (op.Org != tt.org || op.Country != tt.cc || op.Basis != BasisKnown) {
			t.Errorf("Known(%q) = %+v, want %s/%s", tt.host, op, tt.org, tt.cc)
		}
	}
}

type fakeResolver map[string][]netip.Addr

func (f fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if a, ok := f[host]; ok {
		return a, nil
	}
	return nil, errors.New("no such host")
}

const tsv = "1.0.0.0\t1.0.0.255\t13335\tUS\tCLOUDFLARENET\n" +
	"195.135.220.0\t195.135.223.255\t29298\tDE\tSUSE-AS SUSE Software Solutions Germany GmbH\n" +
	"203.0.113.0\t203.0.113.255\t0\tNone\tNot routed\n" +
	"2a04:4e42::\t2a04:4e42:ffff:ffff:ffff:ffff:ffff:ffff\t54113\tUS\tFASTLY\n"

func TestClassifierNetworkLookup(t *testing.T) {
	db, err := asn.Parse(strings.NewReader(tsv))
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "hosts.json")
	c := NewClassifier(db, cache)
	c.Resolver = fakeResolver{
		"mirror.example.de": {netip.MustParseAddr("195.135.221.7")},
		"v6only.example":    {netip.MustParseAddr("2a04:4e42::347")},
		"both.example":      {netip.MustParseAddr("2a04:4e42::1"), netip.MustParseAddr("1.0.0.1")},
		"dark.example":      {netip.MustParseAddr("203.0.113.9")},
	}
	ctx := context.Background()
	if op := c.Classify(ctx, "github.com"); op.Basis != BasisKnown {
		t.Errorf("table entry should not be resolved: %+v", op)
	}
	op := c.Classify(ctx, "mirror.example.de")
	if op.Basis != BasisNetwork || op.Country != "DE" || op.ASN != 29298 || !strings.Contains(op.Org, "SUSE") || op.Addr != "195.135.221.7" {
		t.Errorf("mirror.example.de = %+v", op)
	}
	if op := c.Classify(ctx, "v6only.example"); op.ASN != 54113 || op.Country != "US" {
		t.Errorf("v6only.example = %+v", op)
	}
	if op := c.Classify(ctx, "both.example"); op.ASN != 13335 {
		t.Errorf("IPv4 should be preferred: %+v", op)
	}
	if op := c.Classify(ctx, "dark.example"); op.Basis != BasisUnresolved || op.Addr != "203.0.113.9" {
		t.Errorf("unrouted address = %+v", op)
	}
	if op := c.Classify(ctx, "nxdomain.example"); op.Basis != BasisUnresolved || op.Note == "" {
		t.Errorf("unresolvable = %+v", op)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Fatal("cache not written")
	}
	// A fresh offline classifier answers from the cache only.
	c2 := NewClassifier(nil, cache)
	c2.Offline = true
	if op := c2.Classify(ctx, "mirror.example.de"); op.ASN != 29298 {
		t.Errorf("cached answer lost: %+v", op)
	}
	if op := c2.Classify(ctx, "never-seen.example"); op.Basis != BasisUnresolved {
		t.Errorf("offline unknown host = %+v", op)
	}
}
