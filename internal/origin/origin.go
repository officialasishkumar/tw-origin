// Package origin says who operates a host: from a built-in table for the
// forges, registries and foundations that most sources come from, and for
// every other host by resolving its name and looking the address up in the
// ip2asn database, which gives the network's operator and the country the
// address block is registered in.
package origin

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/officialasishkumar/tw-origin/internal/asn"
)

// Basis says how an Operator was determined.
const (
	BasisKnown      = "known host"           // built-in table: the organisation and its seat
	BasisNetwork    = "network registration" // DNS + ip2asn: operator and country of the address block
	BasisUnresolved = "unresolved"           // the name did not resolve, or no database
)

// Operator is the answer for one host.
type Operator struct {
	Org     string `json:"org"`               // organisation, e.g. "GitHub (Microsoft)"
	Country string `json:"country,omitempty"` // ISO 3166-1 alpha-2
	Basis   string `json:"basis"`             // one of the Basis constants
	ASN     uint32 `json:"asn,omitempty"`     // with BasisNetwork
	Network string `json:"network,omitempty"` // AS name as registered, with BasisNetwork
	Addr    string `json:"addr,omitempty"`    // address that was looked up
	Note    string `json:"note,omitempty"`    // why it is unresolved
}

// Known consults only the built-in table.
func Known(host string) (Operator, bool) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	best, bestLen := rule{}, -1
	for _, r := range rules {
		switch {
		case !strings.HasPrefix(r.pattern, "*."):
			if host == r.pattern {
				return Operator{Org: r.org, Country: r.country, Basis: BasisKnown}, true
			}
		default:
			dom := r.pattern[2:]
			if (host == dom || strings.HasSuffix(host, "."+dom)) && len(dom) > bestLen {
				best, bestLen = r, len(dom)
			}
		}
	}
	if bestLen < 0 {
		return Operator{}, false
	}
	return Operator{Org: best.org, Country: best.country, Basis: BasisKnown}, true
}

// Classifier resolves hosts and remembers the answers on disk.
type Classifier struct {
	DB *asn.DB // nil: loaded through DBLoader on first use; unknown hosts stay unresolved without one
	// DBLoader loads the ip2asn database the first time a host has to be
	// looked up, so that a scan whose hosts are all in the table never
	// downloads it.
	DBLoader  func(ctx context.Context) (*asn.DB, error)
	Offline   bool   // never resolve; use the cache only
	CacheFile string // JSON cache of resolved hosts; "" disables it
	TTL       time.Duration
	Resolver  interface {
		LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
	}

	mu      sync.Mutex
	once    sync.Once
	loadErr error
	entries map[string]cached
	dirty   bool
}

// LoadError reports why the database could not be loaded, if it was needed.
func (c *Classifier) LoadError() error { return c.loadErr }

type cached struct {
	Operator Operator  `json:"operator"`
	When     time.Time `json:"when"`
}

// NewClassifier returns a classifier using the system resolver.
func NewClassifier(db *asn.DB, cacheFile string) *Classifier {
	c := &Classifier{DB: db, CacheFile: cacheFile, TTL: 30 * 24 * time.Hour, Resolver: net.DefaultResolver, entries: map[string]cached{}}
	if cacheFile != "" {
		if b, err := os.ReadFile(cacheFile); err == nil {
			json.Unmarshal(b, &c.entries)
		}
	}
	return c
}

// Classify answers for one host. Table entries never touch the network.
func (c *Classifier) Classify(ctx context.Context, host string) Operator {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return Operator{Basis: BasisUnresolved, Note: "no host"}
	}
	if op, ok := Known(host); ok {
		return op
	}
	c.mu.Lock()
	e, ok := c.entries[host]
	c.mu.Unlock()
	if ok {
		fresh := time.Since(e.When) < c.TTL
		if e.Operator.Basis == BasisUnresolved { // failures are retried sooner
			fresh = time.Since(e.When) < 24*time.Hour
		}
		if fresh || c.Offline {
			return e.Operator
		}
	}
	if c.Offline {
		return Operator{Basis: BasisUnresolved, Note: "not resolved before (offline)"}
	}
	op := c.resolve(ctx, host)
	c.mu.Lock()
	c.entries[host] = cached{Operator: op, When: time.Now()}
	c.dirty = true
	c.mu.Unlock()
	return op
}

func (c *Classifier) resolve(ctx context.Context, host string) Operator {
	c.once.Do(func() {
		if c.DB == nil && c.DBLoader != nil {
			c.DB, c.loadErr = c.DBLoader(ctx)
		}
	})
	if c.DB == nil {
		note := "no ip2asn database"
		if c.loadErr != nil {
			note = "ip2asn database: " + c.loadErr.Error()
		}
		return Operator{Basis: BasisUnresolved, Note: note}
	}
	if ctx.Err() != nil {
		return Operator{Basis: BasisUnresolved, Note: "interrupted"}
	}
	lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	addrs, err := c.Resolver.LookupNetIP(lctx, "ip", host)
	if err != nil || len(addrs) == 0 {
		note := "name does not resolve"
		if err != nil {
			note = err.Error()
		}
		return Operator{Basis: BasisUnresolved, Note: note}
	}
	// Prefer IPv4: the v4 table has the better coverage of small operators.
	sort.SliceStable(addrs, func(i, j int) bool { return addrs[i].Unmap().Is4() && !addrs[j].Unmap().Is4() })
	for _, a := range addrs {
		info, ok := c.DB.Lookup(a)
		if !ok || info.ASN == 0 {
			continue
		}
		return Operator{Org: info.Org(), Country: info.Country, Basis: BasisNetwork, ASN: info.ASN, Network: info.Name, Addr: a.String()}
	}
	return Operator{Basis: BasisUnresolved, Addr: addrs[0].String(), Note: "address not in the ip2asn database"}
}

// Save writes the cache when something changed.
func (c *Classifier) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty || c.CacheFile == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.CacheFile), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c.entries, "", " ")
	if err != nil {
		return err
	}
	tmp := c.CacheFile + ".part"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	c.dirty = false
	return os.Rename(tmp, c.CacheFile)
}
