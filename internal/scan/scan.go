// Package scan ties the pieces together: installed packages are grouped by
// source package, each source revision's spec and _service are fetched from
// OBS with a bounded worker pool, the upstream host is picked out, and every
// host is attributed to an operator and a country.
package scan

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/officialasishkumar/tw-origin/internal/obs"
	"github.com/officialasishkumar/tw-origin/internal/origin"
	"github.com/officialasishkumar/tw-origin/internal/rpmdb"
	"github.com/officialasishkumar/tw-origin/internal/service"
	"github.com/officialasishkumar/tw-origin/internal/spec"
)

// How the primary host of a package was found.
const (
	KindSource     = "source"     // a Source line with a network URL
	KindService    = "service"    // a _service that clones or downloads from a URL
	KindURLTag     = "url-tag"    // only the URL tag (the home page) names a host
	KindUnresolved = "unresolved" // a Source URL whose host is an unexpanded macro
	KindNone       = "none"       // no upstream URL anywhere
	KindNoDistURL  = "no-disturl" // not built on OBS, nothing to fetch
	KindFailed     = "failed"     // OBS did not answer or had no sources
)

// Origin is the primary upstream location of a source package.
type Origin struct {
	Kind     string          `json:"kind"`
	FoundBy  string          `json:"found_by"` // Source0, "_service obs_scm", "URL tag", ...
	URL      string          `json:"url,omitempty"`
	Host     string          `json:"host,omitempty"`
	Operator origin.Operator `json:"operator"`
}

// Package is the result for one source package.
type Package struct {
	Name      string   `json:"name"`             // OBS package name
	Flavor    string   `json:"flavor,omitempty"` // multibuild flavor
	SourceRPM string   `json:"source_rpm,omitempty"`
	Binaries  []string `json:"binaries"`
	DistURL   string   `json:"disturl,omitempty"`
	Project   string   `json:"project,omitempty"` // project the sources were read from
	SrcMD5    string   `json:"srcmd5,omitempty"`
	Revision  string   `json:"revision,omitempty"` // "srcmd5" or "latest"
	WebURL    string   `json:"web_url,omitempty"`
	SpecURL   string   `json:"spec_url,omitempty"` // public API URL of the spec that was read
	SpecName  string   `json:"spec_name,omitempty"`
	Error     string   `json:"error,omitempty"`

	SpecPkgName string           `json:"spec_pkg_name,omitempty"` // Name: tag
	Version     string           `json:"version,omitempty"`
	URLTag      string           `json:"url_tag,omitempty"`
	Sources     []spec.Source    `json:"sources,omitempty"`
	Services    []service.Origin `json:"services,omitempty"`
	Hosts       []string         `json:"hosts,omitempty"` // every distinct host named by the package
	Origin      Origin           `json:"origin"`
}

// Label is the package name with its flavor.
func (p Package) Label() string {
	if p.Flavor != "" {
		return p.Name + ":" + p.Flavor
	}
	return p.Name
}

// Result is a whole scan.
type Result struct {
	Scanned   time.Time `json:"scanned"`
	Arch      string    `json:"arch"`
	Binaries  int       `json:"binaries"`
	Packages  []Package `json:"packages"`
	Requests  int64     `json:"obs_requests"`
	Retries   int64     `json:"obs_retries"`
	FromCache int       `json:"from_cache"`
}

// Options configure a scan.
type Options struct {
	Client     *obs.Client
	Classifier *origin.Classifier
	Workers    int
	Limit      int                                 // 0: all source packages
	Progress   func(stage string, done, total int) // optional, called from worker goroutines
	Arch       string                              // build arch; "" picks the most common installed arch
	Only       func(s rpmdb.Source) bool           // optional filter on source packages
}

// Run scans the given installed packages.
func Run(ctx context.Context, bins []rpmdb.Binary, o Options) (*Result, error) {
	if o.Workers <= 0 {
		o.Workers = 8
	}
	if o.Progress == nil {
		o.Progress = func(string, int, int) {}
	}
	srcs := rpmdb.Group(bins)
	if o.Only != nil {
		var keep []rpmdb.Source
		for _, s := range srcs {
			if o.Only(s) {
				keep = append(keep, s)
			}
		}
		srcs = keep
	}
	if o.Limit > 0 && len(srcs) > o.Limit {
		srcs = srcs[:o.Limit]
	}
	arch := o.Arch
	if arch == "" {
		arch = commonArch(bins)
	}
	res := &Result{Scanned: time.Now().UTC(), Arch: arch, Binaries: len(bins)}
	if len(srcs) == 0 {
		return res, nil
	}

	// Stage 1: fetch and read the packaging sources.
	pkgs := make([]Package, len(srcs))
	var done, cached int
	var mu sync.Mutex
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range o.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				hit := !srcs[i].DistURL.IsZero() && o.Client.Cached(srcs[i].DistURL)
				pkgs[i] = analyse(ctx, o.Client, srcs[i], arch)
				mu.Lock()
				done++
				if hit {
					cached++
				}
				d := done
				mu.Unlock()
				o.Progress("fetch", d, len(srcs))
			}
		}()
	}
	for i := range srcs {
		select {
		case jobs <- i:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	res.FromCache = cached

	// Stage 2: who operates each host.
	hostIdx := map[string][]int{}
	for i := range pkgs {
		if h := pkgs[i].Origin.Host; h != "" {
			hostIdx[h] = append(hostIdx[h], i)
		}
	}
	hosts := make([]string, 0, len(hostIdx))
	for h := range hostIdx {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	ops := make([]origin.Operator, len(hosts))
	hjobs := make(chan int)
	done = 0
	for range o.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range hjobs {
				ops[i] = o.Classifier.Classify(ctx, hosts[i])
				mu.Lock()
				done++
				d := done
				mu.Unlock()
				o.Progress("resolve", d, len(hosts))
			}
		}()
	}
	for i := range hosts {
		hjobs <- i
	}
	close(hjobs)
	wg.Wait()
	for i, h := range hosts {
		for _, pi := range hostIdx[h] {
			pkgs[pi].Origin.Operator = ops[i]
		}
	}
	res.Packages = pkgs
	if o.Client != nil {
		res.Requests, res.Retries = o.Client.Requests.Load(), o.Client.Retries.Load()
	}
	return res, nil
}

// analyse fetches and reads one source package.
func analyse(ctx context.Context, c *obs.Client, s rpmdb.Source, arch string) Package {
	p := Package{Name: s.Name, SourceRPM: s.SourceRPM, Binaries: s.Binaries, DistURL: s.RawDistURL}
	if s.DistURL.IsZero() {
		p.Origin = Origin{Kind: KindNoDistURL, FoundBy: "no DISTURL"}
		if s.RawDistURL != "" {
			p.Error = "unrecognised DISTURL: " + s.RawDistURL
		}
		return p
	}
	d := s.DistURL
	p.Flavor, p.SrcMD5 = d.Flavor, d.SrcMD5
	p.WebURL = obs.WebURL(d.Project, d.Package)
	src, err := c.Fetch(ctx, d, rpmdb.SourceName(s.SourceRPM))
	if err != nil {
		p.Error = err.Error()
		p.Origin = Origin{Kind: KindFailed, FoundBy: "fetch failed"}
		if errors.Is(err, obs.ErrOffline) {
			p.Origin.FoundBy = "not cached"
		}
		return p
	}
	p.Project, p.Revision, p.SpecName = src.Project, src.Revision, src.SpecName
	p.WebURL = obs.WebURL(src.Project, src.Package)
	rev := d.SrcMD5
	if src.Revision != "srcmd5" {
		rev = ""
	}
	p.SpecURL = c.SourceURL(src.Project, src.Package, src.SpecName, rev)

	sp := spec.ParseWith(src.Spec, spec.Options{Arch: arch, Macros: spec.TumbleweedMacros(arch, d.Flavor)})
	p.SpecPkgName, p.Version, p.URLTag, p.Sources = sp.Name, sp.Version, sp.URL, sp.Sources
	var svc *service.File
	if src.Service != "" {
		if svc, err = service.Parse(src.Service); err != nil {
			p.Error = err.Error()
		} else {
			p.Services = svc.Origins()
		}
	}
	p.Origin = pick(sp, p.Services)
	p.Hosts = distinctHosts(sp, p.Services)
	return p
}

// pick chooses the primary origin: the main tarball's Source URL; else the
// repository a source service clones; else any other Source URL; else the
// URL tag; else a URL whose host could not be expanded; else nothing.
func pick(sp *spec.Spec, services []service.Origin) Origin {
	srcs := append([]spec.Source(nil), sp.Sources...)
	sort.SliceStable(srcs, func(i, j int) bool { return srcs[i].Number < srcs[j].Number })
	if len(srcs) > 0 && srcs[0].Host != "" {
		return Origin{Kind: KindSource, FoundBy: srcs[0].Tag, URL: srcs[0].URL, Host: srcs[0].Host}
	}
	for _, o := range services {
		if o.Host != "" {
			return Origin{Kind: KindService, FoundBy: "_service " + o.Service, URL: o.URL, Host: o.Host}
		}
	}
	for _, s := range srcs {
		if s.Host != "" {
			return Origin{Kind: KindSource, FoundBy: s.Tag, URL: s.URL, Host: s.Host}
		}
	}
	if h := service.HostOf(sp.URL); h != "" && !strings.Contains(h, "%") {
		return Origin{Kind: KindURLTag, FoundBy: "URL tag", URL: sp.URL, Host: h}
	}
	for _, s := range srcs {
		if s.Unresolved {
			return Origin{Kind: KindUnresolved, FoundBy: s.Tag + " (unresolved macro)", URL: s.Expanded}
		}
	}
	return Origin{Kind: KindNone, FoundBy: "no upstream URL"}
}

func distinctHosts(sp *spec.Spec, services []service.Origin) []string {
	seen := map[string]bool{}
	var out []string
	add := func(h string) {
		if h != "" && !strings.Contains(h, "%") && !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	for _, s := range sp.Sources {
		add(s.Host)
	}
	for _, o := range services {
		add(o.Host)
	}
	add(service.HostOf(sp.URL))
	return out
}

// commonArch is the most frequent architecture among the installed packages
// other than noarch: the architecture the specs were built for.
func commonArch(bins []rpmdb.Binary) string {
	counts := map[string]int{}
	for _, b := range bins {
		if b.Arch != "" && b.Arch != "noarch" {
			counts[b.Arch]++
		}
	}
	best, n := "", 0
	for a, c := range counts {
		if c > n || (c == n && a < best) {
			best, n = a, c
		}
	}
	return best
}

// Find returns the source package that built the installed package name,
// or the source package of that name.
func Find(bins []rpmdb.Binary, name string) (rpmdb.Source, error) {
	for _, s := range rpmdb.Group(bins) {
		for _, b := range s.Binaries {
			if b == name {
				return s, nil
			}
		}
	}
	for _, s := range rpmdb.Group(bins) {
		if s.Name == name || rpmdb.SourceName(s.SourceRPM) == name {
			return s, nil
		}
	}
	return rpmdb.Source{}, fmt.Errorf("%s is not installed (neither as a package nor as a source package)", name)
}
