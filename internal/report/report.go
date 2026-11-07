// Package report turns a scan into tables: by host, by organisation, by
// country, by how the origin was found, and one row per installed package.
// Shares are of source packages, since one source package (one upstream)
// often installs several binary packages.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/officialasishkumar/tw-origin/internal/origin"
	"github.com/officialasishkumar/tw-origin/internal/scan"
)

// Row is one line of a summary table.
type Row struct {
	Key      string  `json:"key"`               // host, organisation or country code
	Org      string  `json:"org,omitempty"`     // host rows: operator
	Country  string  `json:"country,omitempty"` // host and organisation rows
	Basis    string  `json:"basis,omitempty"`   // host rows: how the operator was determined
	Packages int     `json:"source_packages"`
	Binaries int     `json:"installed_packages"`
	Share    float64 `json:"share"` // of all source packages, 0..1
}

// Summary is the aggregated view.
type Summary struct {
	Binaries      int            `json:"installed_packages"`
	Sources       int            `json:"source_packages"`
	WithHost      int            `json:"with_upstream_host"`
	NoURL         int            `json:"no_upstream_url"`
	NoDistURL     int            `json:"no_disturl"`
	Failed        int            `json:"fetch_failed"`
	Unresolved    int            `json:"unresolved_macro"`
	Hosts         []Row          `json:"hosts"`
	Organisations []Row          `json:"organisations"`
	Countries     []Row          `json:"countries"`
	FoundBy       map[string]int `json:"found_by"`
}

// Summarize aggregates a scan.
func Summarize(res *scan.Result) *Summary {
	s := &Summary{Binaries: res.Binaries, Sources: len(res.Packages), FoundBy: map[string]int{}}
	hosts, orgs, ccs := map[string]*Row{}, map[string]*Row{}, map[string]*Row{}
	for _, p := range res.Packages {
		o := p.Origin
		switch o.Kind {
		case scan.KindNoDistURL:
			s.NoDistURL++
		case scan.KindFailed:
			s.Failed++
		case scan.KindUnresolved:
			s.Unresolved++
		case scan.KindNone:
			s.NoURL++
		}
		s.FoundBy[o.Kind]++
		if o.Host == "" {
			continue
		}
		s.WithHost++
		n := len(p.Binaries)
		h := hosts[o.Host]
		if h == nil {
			h = &Row{Key: o.Host, Org: o.Operator.Org, Country: o.Operator.Country, Basis: o.Operator.Basis}
			hosts[o.Host] = h
		}
		h.Packages++
		h.Binaries += n
		orgKey := o.Operator.Org
		if orgKey == "" {
			orgKey = "(unknown operator)"
		}
		r := orgs[orgKey]
		if r == nil {
			r = &Row{Key: orgKey, Country: o.Operator.Country}
			orgs[orgKey] = r
		}
		r.Packages++
		r.Binaries += n
		cc := o.Operator.Country
		if cc == "" {
			cc = "??"
		}
		c := ccs[cc]
		if c == nil {
			c = &Row{Key: cc}
			ccs[cc] = c
		}
		c.Packages++
		c.Binaries += n
	}
	s.Hosts, s.Organisations, s.Countries = rows(hosts, s.Sources), rows(orgs, s.Sources), rows(ccs, s.Sources)
	return s
}

func rows(m map[string]*Row, total int) []Row {
	out := make([]Row, 0, len(m))
	for _, r := range m {
		if total > 0 {
			r.Share = float64(r.Packages) / float64(total)
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Packages != out[j].Packages {
			return out[i].Packages > out[j].Packages
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// Options control the text rendering.
type Options struct {
	Table   bool   // print the per-package table
	Country string // hosts command: only hosts whose operator is in this country
	Width   int    // truncate long organisation names; 0 keeps them whole
}

// Headline is the one-paragraph version of the summary.
func Headline(s *Summary) string {
	var parts []string
	for i, r := range s.Organisations {
		if i >= 6 || r.Share < 0.02 {
			break
		}
		cc := r.Country
		if cc == "" {
			cc = "??"
		}
		parts = append(parts, fmt.Sprintf("%.0f%% %s (%s)", r.Share*100, r.Key, cc))
	}
	line := fmt.Sprintf("%d installed packages built from %d source packages; sources fetched from %d hosts run by %d organisations in %d countries",
		s.Binaries, s.Sources, len(s.Hosts), len(s.Organisations), countriesKnown(s))
	if len(parts) > 0 {
		line += ": " + strings.Join(parts, ", ")
	}
	if rest := s.Sources - s.WithHost; rest > 0 {
		line += fmt.Sprintf("; %d packages without an upstream URL", rest)
	}
	return line + "."
}

func countriesKnown(s *Summary) int {
	n := 0
	for _, r := range s.Countries {
		if r.Key != "??" {
			n++
		}
	}
	return n
}

// Text prints the summary tables and, with o.Table, the package table.
func Text(w io.Writer, res *scan.Result, s *Summary, o Options) {
	fmt.Fprintf(w, "%s\n\n", Headline(s))
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "by host\tsrc pkgs\tshare\tinstalled\toperator\tcc\tbasis")
	for _, r := range s.Hosts {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%d\t%s\t%s\t%s\n", r.Key, r.Packages, pct(r.Share), r.Binaries, trunc(r.Org, o.Width), orQ(r.Country), basis(r.Basis))
	}
	for _, x := range []struct {
		n     int
		label string
	}{{s.NoURL, "(no upstream URL)"}, {s.Unresolved, "(URL host is an unexpanded macro)"}, {s.NoDistURL, "(not built on OBS: no DISTURL)"}, {s.Failed, "(sources not fetched)"}} {
		if x.n > 0 {
			fmt.Fprintf(tw, "%s\t%d\t%s\t\t\t\t\n", x.label, x.n, pct(float64(x.n)/float64(max(s.Sources, 1))))
		}
	}
	tw.Flush()
	fmt.Fprintln(w)

	tw = tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "by organisation\tsrc pkgs\tshare\tinstalled\tcc")
	for _, r := range s.Organisations {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%d\t%s\n", trunc(r.Key, o.Width), r.Packages, pct(r.Share), r.Binaries, orQ(r.Country))
	}
	tw.Flush()
	fmt.Fprintln(w)

	tw = tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "by country\tsrc pkgs\tshare\tinstalled")
	for _, r := range s.Countries {
		name := origin.CountryName(r.Key)
		if r.Key == "??" {
			name = "unknown"
		}
		fmt.Fprintf(tw, "%s  %s\t%d\t%s\t%d\n", r.Key, name, r.Packages, pct(r.Share), r.Binaries)
	}
	tw.Flush()
	fmt.Fprintln(w)
	fmt.Fprintf(w, "found by: %s\n", foundBy(s))
	fmt.Fprintln(w, `"cc" is the organisation's seat for hosts in the built-in table ("known"), and the`)
	fmt.Fprintln(w, `country of the address block's registration for hosts looked up by network ("net").`)

	if o.Table {
		fmt.Fprintln(w)
		PackageTable(w, res, o)
	}
}

// PackageTable prints one row per installed package.
func PackageTable(w io.Writer, res *scan.Result, o Options) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "package\tsource package\thost\toperator\tcc\tfound by")
	type row struct {
		bin string
		p   scan.Package
	}
	var rs []row
	for _, p := range res.Packages {
		for _, b := range p.Binaries {
			rs = append(rs, row{b, p})
		}
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].bin < rs[j].bin })
	for _, r := range rs {
		o2 := r.p.Origin
		host := o2.Host
		if host == "" {
			host = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.bin, r.p.Label(), host, trunc(o2.Operator.Org, o.Width), orQ(o2.Operator.Country), o2.FoundBy)
	}
	tw.Flush()
}

// Hosts prints only the host table, optionally filtered by country.
func Hosts(w io.Writer, s *Summary, o Options) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "host\tsrc pkgs\tshare\tinstalled\toperator\tcc\tbasis")
	n := 0
	for _, r := range s.Hosts {
		if o.Country != "" && !strings.EqualFold(r.Country, o.Country) {
			continue
		}
		n++
		fmt.Fprintf(tw, "%s\t%d\t%s\t%d\t%s\t%s\t%s\n", r.Key, r.Packages, pct(r.Share), r.Binaries, trunc(r.Org, o.Width), orQ(r.Country), basis(r.Basis))
	}
	tw.Flush()
	if o.Country != "" {
		fmt.Fprintf(w, "%d of %d hosts in %s\n", n, len(s.Hosts), strings.ToUpper(o.Country))
	}
}

// Detail prints everything known about one package.
func Detail(w io.Writer, p scan.Package) {
	fmt.Fprintf(w, "%s", p.Label())
	if p.SourceRPM != "" {
		fmt.Fprintf(w, "  (%s)", p.SourceRPM)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  installed:   %s\n", strings.Join(p.Binaries, ", "))
	if p.DistURL != "" {
		fmt.Fprintf(w, "  disturl:     %s\n", p.DistURL)
	}
	if p.WebURL != "" {
		fmt.Fprintf(w, "  obs:         %s\n", p.WebURL)
	}
	if p.SpecURL != "" {
		rev := "pinned to the srcmd5 the RPM was built from"
		if p.Revision == "latest" {
			rev = "that revision is gone; this is the current one"
		}
		fmt.Fprintf(w, "  spec:        %s  (%s)\n", p.SpecURL, rev)
	}
	if p.Error != "" {
		fmt.Fprintf(w, "  error:       %s\n", p.Error)
	}
	if p.SpecPkgName != "" {
		fmt.Fprintf(w, "  Name/Version: %s %s\n", p.SpecPkgName, p.Version)
	}
	if p.URLTag != "" {
		fmt.Fprintf(w, "  URL tag:     %s\n", p.URLTag)
	}
	if len(p.Sources) > 0 {
		fmt.Fprintln(w, "  Source lines:")
		tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
		for _, s := range p.Sources {
			what := "local file"
			switch {
			case s.Host != "":
				what = s.Host
			case s.Unresolved:
				what = "host is an unexpanded macro"
			}
			shown := s.Expanded
			if shown != s.Raw {
				shown += "   <- " + s.Raw
			}
			fmt.Fprintf(tw, "    %s:\t%s\t[%s]\n", s.Tag, shown, what)
		}
		tw.Flush()
	}
	if len(p.Services) > 0 {
		fmt.Fprintln(w, "  _service:")
		for _, o := range p.Services {
			extra := ""
			if o.Revision != "" {
				extra = "  revision " + o.Revision
			}
			if o.Mode != "" {
				extra += "  (mode " + o.Mode + ")"
			}
			fmt.Fprintf(w, "    %s: %s%s\n", o.Service, o.URL, extra)
		}
	}
	if len(p.Hosts) > 1 {
		fmt.Fprintf(w, "  hosts named: %s\n", strings.Join(p.Hosts, ", "))
	}
	o := p.Origin
	fmt.Fprintf(w, "  origin:      ")
	if o.Host == "" {
		fmt.Fprintf(w, "%s\n", o.FoundBy)
		return
	}
	fmt.Fprintf(w, "%s  (from %s)\n", o.Host, o.FoundBy)
	op := o.Operator
	switch op.Basis {
	case origin.BasisKnown:
		fmt.Fprintf(w, "  operator:    %s, %s (%s)  [built-in table]\n", op.Org, origin.CountryName(op.Country), op.Country)
	case origin.BasisNetwork:
		fmt.Fprintf(w, "  operator:    %s (AS%d), %s (%s)  [network registration of %s, not a legal seat]\n", op.Org, op.ASN, origin.CountryName(op.Country), op.Country, op.Addr)
	default:
		fmt.Fprintf(w, "  operator:    unknown (%s)\n", op.Note)
	}
}

// JSON writes the machine-readable report.
func JSON(w io.Writer, res *scan.Result, s *Summary, version string) error {
	out := struct {
		GeneratedBy string `json:"generated_by"`
		*scan.Result
		Summary *Summary `json:"summary"`
	}{"tw-origin " + version, res, s}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func foundBy(s *Summary) string {
	labels := []struct{ kind, label string }{
		{scan.KindSource, "Source URL in the spec"}, {scan.KindService, "_service"}, {scan.KindURLTag, "URL tag only"},
		{scan.KindUnresolved, "unexpanded macro"}, {scan.KindNone, "no URL"}, {scan.KindNoDistURL, "no DISTURL"}, {scan.KindFailed, "not fetched"},
	}
	var parts []string
	for _, l := range labels {
		if n := s.FoundBy[l.kind]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", l.label, n))
		}
	}
	return strings.Join(parts, ", ")
}

func pct(f float64) string {
	if f > 0 && f < 0.005 {
		return "<1%"
	}
	return fmt.Sprintf("%.0f%%", f*100)
}

func orQ(cc string) string {
	if cc == "" {
		return "??"
	}
	return cc
}

func basis(b string) string {
	switch b {
	case origin.BasisKnown:
		return "known"
	case origin.BasisNetwork:
		return "net"
	case "":
		return ""
	}
	return "?"
}

func trunc(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
