// Package obs fetches the packaging sources (spec file and _service) of a
// package revision from the Open Build Service through its anonymous public
// API, https://api.opensuse.org/public/, and caches them on disk. A DISTURL
// names an immutable source revision (the srcmd5), so a cached answer never
// goes stale.
package obs

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/officialasishkumar/tw-origin/internal/rpmdb"
)

// DefaultBaseURL is the anonymous, read-only API of build.opensuse.org.
const DefaultBaseURL = "https://api.opensuse.org/public"

// ErrNotFound is returned when neither the pinned revision nor any fallback
// exists on the server.
var ErrNotFound = errors.New("not found on OBS")

// ErrOffline is returned in offline mode when the answer is not cached.
var ErrOffline = errors.New("not in the cache (offline)")

// Sources is what the scanner needs from one package revision.
type Sources struct {
	Project   string    `json:"project"`           // project the files were read from
	Package   string    `json:"package"`           // OBS package name
	SrcMD5    string    `json:"srcmd5"`            // revision requested
	Revision  string    `json:"revision"`          // "srcmd5" when the pinned revision was served, "latest" when it had to fall back
	Files     []string  `json:"files"`             // names in the source directory
	SpecName  string    `json:"spec_name"`         // spec file chosen
	Spec      string    `json:"spec"`              // its content
	Service   string    `json:"service,omitempty"` // _service content, if the package has one
	ListedAt  string    `json:"listed_from"`       // API URL of the directory listing that worked
	FetchedAt time.Time `json:"fetched_at"`
}

// Client talks to one OBS instance.
type Client struct {
	BaseURL   string
	UserAgent string
	CacheDir  string // directory for cached revisions; "" disables the cache
	Offline   bool   // answer only from the cache
	Refresh   bool   // ignore cached answers (they are rewritten)
	HTTP      *http.Client
	// Requests counts HTTP requests made; Retries counts the ones repeated
	// after a transient failure.
	Requests, Retries atomic.Int64
}

// NewClient returns a client for api.opensuse.org with sensible timeouts for
// an API that is sometimes slow to answer.
func NewClient(cacheDir string) *Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 90 * time.Second,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
	}
	return &Client{
		BaseURL:   DefaultBaseURL,
		UserAgent: "tw-origin (+https://github.com/officialasishkumar/tw-origin)",
		CacheDir:  cacheDir,
		HTTP:      &http.Client{Transport: tr, Timeout: 3 * time.Minute},
	}
}

// WebURL is the page of a package on build.opensuse.org.
func WebURL(project, pkg string) string {
	return "https://build.opensuse.org/package/show/" + project + "/" + pkg
}

// SourceURL is the public API URL of a source directory or file.
func (c *Client) SourceURL(project, pkg, file, rev string) string {
	u := c.BaseURL + "/source/" + url.PathEscape(project) + "/" + url.PathEscape(pkg)
	if file != "" {
		u += "/" + url.PathEscape(file)
	}
	if rev != "" {
		u += "?rev=" + rev
	}
	return u
}

func (c *Client) cachePath(d rpmdb.DistURL) string {
	if c.CacheDir == "" {
		return ""
	}
	name := d.SrcMD5 + "-" + d.Package
	if d.Flavor != "" {
		name += "--" + d.Flavor
	}
	return filepath.Join(c.CacheDir, strings.ReplaceAll(name, string(os.PathSeparator), "_")+".json")
}

// Cached reports whether Fetch can answer without the network.
func (c *Client) Cached(d rpmdb.DistURL) bool {
	p := c.cachePath(d)
	if p == "" || c.Refresh {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// Fetch returns the spec and _service of the revision named by d. nameHint
// is the source package name from SOURCERPM, used to choose between several
// spec files. The pinned revision is tried in the DISTURL's project, then in
// openSUSE:Factory, then the current Factory sources as a last resort.
func (c *Client) Fetch(ctx context.Context, d rpmdb.DistURL, nameHint string) (*Sources, error) {
	if p := c.cachePath(d); p != "" && !c.Refresh {
		if b, err := os.ReadFile(p); err == nil {
			var s Sources
			if json.Unmarshal(b, &s) == nil && s.Spec != "" {
				return &s, nil
			}
		}
	}
	if c.Offline {
		return nil, ErrOffline
	}
	if d.APIHost != "build.opensuse.org" {
		return nil, fmt.Errorf("sources of %s are not on a public OBS instance", d.APIHost)
	}
	type attempt struct{ project, rev, label string }
	attempts := []attempt{{d.Project, d.SrcMD5, "srcmd5"}}
	if d.Project != "openSUSE:Factory" {
		attempts = append(attempts, attempt{"openSUSE:Factory", d.SrcMD5, "srcmd5"})
	}
	attempts = append(attempts, attempt{"openSUSE:Factory", "", "latest"})
	var lastErr error
	for _, a := range attempts {
		s, err := c.fetchFrom(ctx, a.project, d.Package, a.rev, d.Flavor, nameHint)
		if err == nil {
			s.SrcMD5, s.Revision = d.SrcMD5, a.label
			s.FetchedAt = time.Now().UTC()
			c.store(d, s)
			return s, nil
		}
		lastErr = err
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) store(d rpmdb.DistURL, s *Sources) {
	p := c.cachePath(d)
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	tmp := p + ".part"
	if err := os.WriteFile(tmp, b, 0o644); err == nil {
		os.Rename(tmp, p)
	}
}

type directory struct {
	Name    string  `xml:"name,attr"`
	SrcMD5  string  `xml:"srcmd5,attr"`
	Entries []entry `xml:"entry"`
}

type entry struct {
	Name string `xml:"name,attr"`
}

func (c *Client) fetchFrom(ctx context.Context, project, pkg, rev, flavor, nameHint string) (*Sources, error) {
	listURL := c.SourceURL(project, pkg, "", rev)
	body, err := c.get(ctx, listURL)
	if err != nil {
		return nil, err
	}
	dir, err := parseDirectory(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", listURL, err)
	}
	files := dir.names()
	if !hasSuffix(files, ".spec") && contains(files, "_link") {
		// A plain link: the files live in the link target; ask for the expanded view.
		expanded := listURL + sep(listURL) + "expand=1"
		if body, err = c.get(ctx, expanded); err == nil {
			if dir, err = parseDirectory(body); err == nil {
				listURL, files = expanded, dir.names()
			}
		}
	}
	specName := chooseSpec(files, pkg, flavor, nameHint)
	if specName == "" {
		return nil, fmt.Errorf("%s: no spec file among %d files", listURL, len(files))
	}
	spec, err := c.get(ctx, c.SourceURL(project, pkg, specName, rev))
	if err != nil {
		return nil, err
	}
	s := &Sources{Project: project, Package: pkg, Files: files, SpecName: specName, Spec: string(spec), ListedAt: listURL}
	if contains(files, "_service") {
		svc, err := c.get(ctx, c.SourceURL(project, pkg, "_service", rev))
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
		s.Service = string(svc)
	}
	return s, nil
}

func sep(u string) string {
	if strings.Contains(u, "?") {
		return "&"
	}
	return "?"
}

func parseDirectory(b []byte) (*directory, error) {
	var d directory
	if err := xml.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("unexpected answer (not a directory listing): %w", err)
	}
	return &d, nil
}

func (d *directory) names() []string {
	out := make([]string, 0, len(d.Entries))
	for _, e := range d.Entries {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}

// chooseSpec picks the spec file that built this package: the multibuild
// flavor's own spec when it has one, otherwise the package's, otherwise the
// one named after the source RPM, otherwise the only one there is.
func chooseSpec(files []string, pkg, flavor, nameHint string) string {
	var specs []string
	for _, f := range files {
		if strings.HasSuffix(f, ".spec") {
			specs = append(specs, f)
		}
	}
	for _, want := range []string{flavor, pkg, nameHint} {
		if want != "" && contains(specs, want+".spec") {
			return want + ".spec"
		}
	}
	if len(specs) > 0 {
		return specs[0]
	}
	return ""
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func hasSuffix(list []string, suf string) bool {
	for _, x := range list {
		if strings.HasSuffix(x, suf) {
			return true
		}
	}
	return false
}

// get fetches a URL with retries on transient failures (connection errors,
// 429 and 5xx), backing off exponentially and honouring Retry-After.
func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	const attempts = 5
	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			c.Retries.Add(1)
		}
		body, retryAfter, err := c.once(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if errors.Is(err, ErrNotFound) || ctx.Err() != nil {
			return nil, err
		}
		var te *transientError
		if !errors.As(err, &te) {
			return nil, err
		}
		delay := time.Duration(float64(2*time.Second) * float64(int(1)<<i) * (0.5 + rand.Float64()))
		if retryAfter > delay {
			delay = retryAfter
		}
		if delay > 60*time.Second {
			delay = 60 * time.Second
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("giving up after %d attempts: %w", attempts, lastErr)
}

type transientError struct{ err error }

func (e *transientError) Error() string { return e.err.Error() }
func (e *transientError) Unwrap() error { return e.err }

func (c *Client) once(ctx context.Context, u string) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/xml, text/plain, */*")
	c.Requests.Add(1)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		return nil, 0, &transientError{err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, 0, &transientError{fmt.Errorf("reading %s: %w", u, err)}
	}
	switch {
	case resp.StatusCode == http.StatusOK:
		return body, 0, nil
	case resp.StatusCode == http.StatusNotFound:
		return nil, 0, fmt.Errorf("%w: %s", ErrNotFound, u)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		var ra time.Duration
		if s := resp.Header.Get("Retry-After"); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				ra = time.Duration(n) * time.Second
			} else if t, err := http.ParseTime(s); err == nil {
				ra = time.Until(t)
			}
		}
		return nil, ra, &transientError{fmt.Errorf("GET %s: %s", u, resp.Status)}
	}
	return nil, 0, fmt.Errorf("GET %s: %s", u, resp.Status)
}
