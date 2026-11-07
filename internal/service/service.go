// Package service reads an OBS _service file and extracts the upstream
// locations it names. Source services such as obs_scm and tar_scm clone a
// repository given by a "url" parameter; download_url composes one from
// protocol, host and path; download_files and download_assets fetch the
// URLs written in the spec's Source lines and so name no host of their own.
package service

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// Service is one <service> element.
type Service struct {
	Name   string              `json:"name"`
	Mode   string              `json:"mode,omitempty"` // "", manual, disabled, buildtime, localonly, serveronly
	Params map[string][]string `json:"params,omitempty"`
}

// Param returns the first value of a parameter.
func (s Service) Param(name string) string {
	if v := s.Params[name]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// Origin is an upstream location found in a service.
type Origin struct {
	Service  string `json:"service"` // obs_scm, tar_scm, download_url, ...
	Mode     string `json:"mode,omitempty"`
	URL      string `json:"url"`
	Host     string `json:"host,omitempty"`
	SCM      string `json:"scm,omitempty"`      // git, hg, svn, bzr for scm services
	Revision string `json:"revision,omitempty"` // revision/tag parameter when given
}

// File is a parsed _service.
type File struct {
	Services []Service
}

type xmlFile struct {
	Services []struct {
		Name   string `xml:"name,attr"`
		Mode   string `xml:"mode,attr"`
		Params []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:",chardata"`
		} `xml:"param"`
	} `xml:"service"`
}

// Parse reads a _service document.
func Parse(text string) (*File, error) {
	var x xmlFile
	if err := xml.Unmarshal([]byte(text), &x); err != nil {
		return nil, fmt.Errorf("_service: %w", err)
	}
	f := &File{}
	for _, xs := range x.Services {
		s := Service{Name: xs.Name, Mode: xs.Mode, Params: map[string][]string{}}
		for _, p := range xs.Params {
			s.Params[p.Name] = append(s.Params[p.Name], strings.TrimSpace(p.Value))
		}
		f.Services = append(f.Services, s)
	}
	return f, nil
}

// Vendoring services package dependencies (crates, Go modules, npm packages)
// next to the upstream tarball; they are not where the upstream lives.
var vendoring = map[string]bool{
	"cargo_vendor": true, "cargo_audit": true, "go_modules": true, "node_modules": true, "npm": true, "composer": true,
}

// UsesSourceURLs reports whether a service fetches the URLs in the spec's
// Source lines, which makes those lines the real statement of origin.
func (f *File) UsesSourceURLs() bool {
	for _, s := range f.Services {
		switch s.Name {
		case "download_files", "download_assets", "download_src_package":
			return true
		}
	}
	return false
}

// Origins lists the upstream locations in the order the services appear.
func (f *File) Origins() []Origin {
	var out []Origin
	for _, s := range f.Services {
		if vendoring[s.Name] {
			continue
		}
		o := Origin{Service: s.Name, Mode: s.Mode, SCM: s.Param("scm")}
		for _, k := range []string{"revision", "tag", "version"} {
			if v := s.Param(k); v != "" {
				o.Revision = v
				break
			}
		}
		switch {
		case s.Name == "download_url":
			host, path := s.Param("host"), s.Param("path")
			if host == "" {
				continue
			}
			proto := s.Param("protocol")
			if proto == "" {
				proto = "http"
			}
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			o.URL = proto + "://" + host + path
		case s.Param("url") != "":
			o.URL = s.Param("url")
		default:
			continue
		}
		o.Host = HostOf(o.URL)
		out = append(out, o)
	}
	return out
}

// HostOf returns the lower-cased host of a URL, also for scp-like git
// addresses (git@github.com:org/repo.git), or "" when there is none.
func HostOf(u string) string {
	u = strings.TrimSpace(u)
	rest := u
	if i := strings.Index(u, "://"); i >= 0 {
		rest = u[i+3:]
	} else if at := strings.Index(u, "@"); at >= 0 && strings.Contains(u[at:], ":") && !strings.Contains(u[:at], "/") {
		rest = u[at+1:]
		if c := strings.Index(rest, ":"); c >= 0 {
			rest = rest[:c]
		}
	} else {
		return ""
	}
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	if !strings.HasPrefix(rest, "[") {
		if i := strings.LastIndex(rest, ":"); i >= 0 {
			rest = rest[:i]
		}
	}
	return strings.ToLower(strings.TrimSuffix(rest, "."))
}
