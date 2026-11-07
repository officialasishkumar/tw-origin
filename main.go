// tw-origin answers "where does your distro come from?": for every installed
// package it finds the host the upstream source is fetched from, who operates
// that host, and in which country.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/officialasishkumar/tw-origin/internal/asn"
	"github.com/officialasishkumar/tw-origin/internal/obs"
	"github.com/officialasishkumar/tw-origin/internal/origin"
	"github.com/officialasishkumar/tw-origin/internal/report"
	"github.com/officialasishkumar/tw-origin/internal/rpmdb"
	"github.com/officialasishkumar/tw-origin/internal/scan"
)

var version = "dev"

const usage = `tw-origin: where does your distro come from?

Usage:
  tw-origin scan [flags]              every installed package: upstream host, operator, country
  tw-origin package NAME [flags]      one package in detail: spec URL, Source lines, hosts
  tw-origin hosts [--country CC]      the host table only
  tw-origin version

Flags (all commands):
  --from FILE     read the package list from FILE instead of rpm -qa ("-" for stdin);
                  make it with: rpm -qa --qf '%{NAME}\t%{ARCH}\t%{SOURCERPM}\t%{DISTURL}\n'
  --offline       use only what is cached; never talk to OBS or DNS
  --no-lookup     do not resolve hosts missing from the built-in table
  --workers N     parallel requests to api.opensuse.org (default 8)
  --refresh       ignore cached specs
  --cache DIR     cache directory (default $XDG_CACHE_HOME/tw-origin)
Flags for scan:
  --limit N       only the first N source packages
  --json          write the full report as JSON to stdout
  --no-table      omit the per-package table
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tw-origin:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(usage)
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch args[0] {
	case "scan", "package", "hosts":
		return cmd(ctx, args[0], args[1:])
	case "version":
		fmt.Println("tw-origin", version)
		return nil
	}
	return fmt.Errorf("unknown command %q (try --help)", args[0])
}

func cacheDir() string {
	if x := os.Getenv("XDG_CACHE_HOME"); x != "" {
		return filepath.Join(x, "tw-origin")
	}
	d, err := os.UserCacheDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "tw-origin")
}

func cmd(ctx context.Context, name string, args []string) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	from := fs.String("from", "", "package list file")
	offline := fs.Bool("offline", false, "cache only")
	noLookup := fs.Bool("no-lookup", false, "no DNS/ip2asn lookup")
	workers := fs.Int("workers", 8, "parallel requests")
	refresh := fs.Bool("refresh", false, "ignore cached specs")
	cache := fs.String("cache", cacheDir(), "cache directory")
	limit := fs.Int("limit", 0, "first N source packages")
	jsonOut := fs.Bool("json", false, "JSON output")
	noTable := fs.Bool("no-table", false, "omit the package table")
	country := fs.String("country", "", "hosts: filter by country code")
	var pos []string
	for { // flags before and after the positional argument
		if err := fs.Parse(args); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if name == "package" && len(pos) != 1 {
		return errors.New("usage: tw-origin package NAME")
	}

	var bins []rpmdb.Binary
	var err error
	if *from != "" {
		var r io.Reader = os.Stdin
		if *from != "-" {
			f, err := os.Open(*from)
			if err != nil {
				return err
			}
			defer f.Close()
			r = f
		}
		bins, err = rpmdb.Parse(r)
	} else {
		bins, err = rpmdb.Installed(ctx)
	}
	if err != nil {
		return err
	}
	if len(bins) == 0 {
		return errors.New("no installed packages found")
	}

	client := obs.NewClient(filepath.Join(*cache, "sources"))
	client.Offline, client.Refresh = *offline, *refresh
	cl := origin.NewClassifier(nil, filepath.Join(*cache, "hosts.json"))
	cl.Offline = *offline
	if !*noLookup && !*offline {
		dbPath := filepath.Join(*cache, asn.FileName)
		cl.DBLoader = func(ctx context.Context) (*asn.DB, error) {
			if _, err := os.Stat(dbPath); err != nil {
				if bt := buildtraceDB(); bt != "" {
					dbPath = bt
				} else {
					fmt.Fprintf(os.Stderr, "tw-origin: downloading the IP-to-network database (about 11 MB, once) to %s\n", dbPath)
				}
			}
			return asn.Load(ctx, dbPath, client.UserAgent, true)
		}
	}
	defer cl.Save()

	opts := scan.Options{Client: client, Classifier: cl, Workers: *workers, Limit: *limit}
	if name == "package" {
		src, err := scan.Find(bins, pos[0])
		if err != nil {
			return err
		}
		opts.Only = func(s rpmdb.Source) bool { return s.SourceRPM == src.SourceRPM && s.Name == src.Name }
	}
	srcCount := len(rpmdb.Group(bins))
	status := func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }
	if name != "package" {
		mode := "reading specs from " + client.BaseURL
		if *offline {
			mode = "offline, cache only"
		}
		status("tw-origin: %d installed packages, %d source packages; %s (%d workers, cache %s)", len(bins), srcCount, mode, *workers, *cache)
	}
	start := time.Now()
	last := time.Time{}
	opts.Progress = func(stage string, done, total int) {
		if name == "package" || *jsonOut && !isTerminal(os.Stderr) {
			return
		}
		if done == total || time.Since(last) > time.Second {
			last = time.Now()
			if isTerminal(os.Stderr) {
				fmt.Fprintf(os.Stderr, "\r  %s %d/%d   ", stage, done, total)
				if done == total {
					fmt.Fprintln(os.Stderr)
				}
			} else if done == total || done%25 == 0 {
				status("  %s %d/%d", stage, done, total)
			}
		}
	}
	res, err := scan.Run(ctx, bins, opts)
	if err != nil {
		return err
	}
	if cl.LoadError() != nil {
		status("tw-origin: hosts outside the built-in table stay unclassified: %v", cl.LoadError())
	}
	sum := report.Summarize(res)
	if name != "package" {
		status("  done in %s: %d from cache, %d requests, %d retries", time.Since(start).Round(time.Second), res.FromCache, res.Requests, res.Retries)
		status("")
	}
	switch name {
	case "scan":
		if *jsonOut {
			return report.JSON(os.Stdout, res, sum, version)
		}
		report.Text(os.Stdout, res, sum, report.Options{Table: !*noTable})
	case "hosts":
		report.Hosts(os.Stdout, sum, report.Options{Country: *country})
	case "package":
		if len(res.Packages) == 0 {
			return fmt.Errorf("%s: nothing to report", pos[0])
		}
		if *jsonOut {
			return report.JSON(os.Stdout, res, sum, version)
		}
		report.Detail(os.Stdout, res.Packages[0])
	}
	return nil
}

// buildtraceDB returns buildtrace's cached ip2asn file when it exists, so
// the two tools share one download.
func buildtraceDB() string {
	d, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(d, "buildtrace", asn.FileName)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
