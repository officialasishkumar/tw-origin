// Package rpmdb lists the installed packages and reads the two tags that
// matter here: SOURCERPM, which groups binary packages by the source package
// they were built from, and DISTURL, which openSUSE's build service writes
// into every RPM and which names the exact source revision it was built from.
package rpmdb

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// QueryFormat is the rpm --qf used by Installed and understood by Parse.
const QueryFormat = `%{NAME}\t%{ARCH}\t%{SOURCERPM}\t%{DISTURL}\n`

// Binary is one installed package.
type Binary struct {
	Name      string `json:"name"`
	Arch      string `json:"arch"`
	SourceRPM string `json:"source_rpm"` // "" when rpm reports (none)
	DistURL   string `json:"disturl"`    // "" when rpm reports (none)
}

// Source is a source package with the binaries built from it.
type Source struct {
	Name       string   // package name on OBS, from DISTURL (falls back to the SOURCERPM name)
	SourceRPM  string   // e.g. bash-5.3.20-10.1.src.rpm
	DistURL    DistURL  // zero when the package has none
	RawDistURL string   // as recorded in the RPM
	Binaries   []string // installed binary package names, sorted
}

// DistURL is a parsed %{DISTURL}, e.g.
// obs://build.opensuse.org/openSUSE:Factory:ARM/standard/60f9bed5...-bash
// or ...-boost:base for a multibuild flavor.
type DistURL struct {
	APIHost    string // build.opensuse.org
	Project    string // openSUSE:Factory:ARM
	Repository string // standard
	SrcMD5     string // 32 hex characters
	Package    string // bash, boost
	Flavor     string // "" or the multibuild flavor (base)
}

// IsZero reports whether the DISTURL was missing.
func (d DistURL) IsZero() bool { return d.SrcMD5 == "" }

// String reassembles the DISTURL.
func (d DistURL) String() string {
	pkg := d.Package
	if d.Flavor != "" {
		pkg += ":" + d.Flavor
	}
	return "obs://" + d.APIHost + "/" + d.Project + "/" + d.Repository + "/" + d.SrcMD5 + "-" + pkg
}

var distURLRe = regexp.MustCompile(`^obs://([^/]+)/([^/]+)/([^/]+)/([0-9a-f]{32})-(.+)$`)

// ParseDistURL splits a DISTURL into its parts.
func ParseDistURL(s string) (DistURL, error) {
	m := distURLRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return DistURL{}, fmt.Errorf("not an OBS DISTURL: %q", s)
	}
	d := DistURL{APIHost: m[1], Project: m[2], Repository: m[3], SrcMD5: m[4], Package: m[5]}
	if pkg, flavor, ok := strings.Cut(m[5], ":"); ok {
		d.Package, d.Flavor = pkg, flavor
	}
	return d, nil
}

var srcRPMRe = regexp.MustCompile(`^(.+)-[^-]+-[^-]+\.(?:no)?src\.rpm$`)

// SourceName strips version, release and suffix from a SOURCERPM value.
func SourceName(srcrpm string) string {
	if m := srcRPMRe.FindStringSubmatch(srcrpm); m != nil {
		return m[1]
	}
	return strings.TrimSuffix(strings.TrimSuffix(srcrpm, ".src.rpm"), ".nosrc.rpm")
}

// Installed queries the rpm database of this machine.
func Installed(ctx context.Context) ([]Binary, error) {
	if _, err := exec.LookPath("rpm"); err != nil {
		return nil, errors.New("rpm not found; run this on an openSUSE system, or pass --from with the output of\n  rpm -qa --qf '" + QueryFormat + "'")
	}
	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rpm", "-qa", "--qf", QueryFormat)
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rpm -qa: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return Parse(&out)
}

// Parse reads lines in QueryFormat. gpg-pubkey pseudo-packages are skipped.
func Parse(r io.Reader) ([]Binary, error) {
	var bins []Binary
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 4 {
			return nil, fmt.Errorf("expected 4 tab-separated fields, got %d: %q", len(f), line)
		}
		b := Binary{Name: f[0], Arch: none(f[1]), SourceRPM: none(f[2]), DistURL: none(f[3])}
		if b.Name == "gpg-pubkey" || (b.SourceRPM == "" && b.DistURL == "") {
			continue
		}
		bins = append(bins, b)
	}
	return bins, sc.Err()
}

func none(s string) string {
	if s == "(none)" {
		return ""
	}
	return s
}

// Group deduplicates binaries by SOURCERPM and returns the source packages
// sorted by name. The DISTURL comes from the first binary that has one;
// binaries without a SOURCERPM become one source each.
func Group(bins []Binary) []Source {
	byKey := map[string]*Source{}
	var order []string
	for _, b := range bins {
		key := b.SourceRPM
		if key == "" {
			key = "\x00" + b.Name
		}
		s := byKey[key]
		if s == nil {
			s = &Source{SourceRPM: b.SourceRPM, Name: SourceName(b.SourceRPM)}
			if b.SourceRPM == "" {
				s.Name = b.Name
			}
			byKey[key] = s
			order = append(order, key)
		}
		s.Binaries = append(s.Binaries, b.Name)
		if s.DistURL.IsZero() && b.DistURL != "" {
			if d, err := ParseDistURL(b.DistURL); err == nil {
				s.DistURL = d
				s.RawDistURL = b.DistURL
				s.Name = d.Package
			} else if s.RawDistURL == "" {
				s.RawDistURL = b.DistURL
			}
		}
	}
	out := make([]Source, 0, len(order))
	for _, k := range order {
		s := byKey[k]
		sort.Strings(s.Binaries)
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].SourceRPM < out[j].SourceRPM
	})
	return out
}
