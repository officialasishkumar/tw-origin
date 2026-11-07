package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func load(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func urlSources(s *Spec) map[string]string {
	out := map[string]string{}
	for _, src := range s.Sources {
		if src.Host != "" {
			out[src.Tag] = src.Host
		}
	}
	return out
}

// The fixtures are the preambles of real openSUSE Factory specs.
func TestRealSpecs(t *testing.T) {
	tw := Options{Arch: "aarch64", Macros: TumbleweedMacros("aarch64", "")}
	tests := []struct {
		file         string
		name, url    string
		version      string
		source0      string // expanded Source/Source0
		host0        string
		urlSourceTag int // number of Source lines that are network URLs
	}{
		{
			file: "bash.spec", name: "bash", url: "https://www.gnu.org/software/bash/bash.html",
			version:      "5.3.%(bash %{_sourcedir}/get_version_number.sh %{_sourcedir})", // shell expansion stays
			source0:      "https://ftp.gnu.org/gnu/bash/bash-5.3.tar.gz",                  // %{bversion}%{bextend} with bextend = %{nil}
			host0:        "ftp.gnu.org",
			urlSourceTag: 2,
		},
		{
			file: "openssl-3.spec", name: "openssl-3", url: "https://www.openssl.org/", version: "3.5.3",
			source0:      "https://www.openssl.org/source/openssl-3.5.3.tar.gz", // %{_rname} inside the host name
			host0:        "www.openssl.org",
			urlSourceTag: 2,
		},
		{
			file: "ncurses.spec", name: "ncurses", url: "https://www.invisible-island.net/ncurses/ncurses.html",
			version:      "6.6.%(bash %{_sourcedir}/get_version_number.sh %{_sourcedir})",
			source0:      "https://invisible-island.net/archives/ncurses/ncurses-6.6.tar.gz", // %global download in the host
			host0:        "invisible-island.net",
			urlSourceTag: 4,
		},
		{
			file: "vim.spec", name: "vim", url: "https://www.vim.org/", version: "9.2.1202",
			source0:      "https://github.com/vim/vim/archive/v9.2.1202.tar.gz#/vim-9.2.1202.tar.gz", // #/ fragment keeps the host
			host0:        "github.com",
			urlSourceTag: 1,
		},
		{
			file: "glibc.spec", name: "glibc", url: "https://www.gnu.org/software/libc/libc.html", version: "2.44",
			source0:      "https://ftp.gnu.org/pub/gnu/glibc/glibc-2.44.tar.xz",
			host0:        "ftp.gnu.org",
			urlSourceTag: 3,
		},
		{
			file: "perl.spec", name: "perl", url: "https://www.perl.org/", version: "5.44.0", // %{?name_suffix} is empty for the main flavor
			source0:      "https://www.cpan.org/src/5.0/perl-5.44.0.tar.xz",
			host0:        "www.cpan.org",
			urlSourceTag: 1,
		},
		{
			file: "zypper.spec", name: "zypper", url: "https://github.com/openSUSE/zypper", version: "1.14.102",
			source0:      "zypper-1.14.102.tar.bz2", // local tarball: no host
			host0:        "",
			urlSourceTag: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			s := ParseWith(load(t, tt.file), tw)
			if s.Name != tt.name {
				t.Errorf("Name = %q, want %q", s.Name, tt.name)
			}
			if s.URL != tt.url {
				t.Errorf("URL = %q, want %q", s.URL, tt.url)
			}
			if s.Version != tt.version {
				t.Errorf("Version = %q, want %q", s.Version, tt.version)
			}
			if len(s.Sources) == 0 {
				t.Fatal("no Source lines found")
			}
			if s.Sources[0].Number != 0 {
				t.Errorf("first Source is %s, want Source0", s.Sources[0].Tag)
			}
			if s.Sources[0].Expanded != tt.source0 {
				t.Errorf("Source0 = %q, want %q", s.Sources[0].Expanded, tt.source0)
			}
			if s.Sources[0].Host != tt.host0 {
				t.Errorf("Source0 host = %q, want %q", s.Sources[0].Host, tt.host0)
			}
			if got := len(urlSources(s)); got != tt.urlSourceTag {
				t.Errorf("%d Source lines with a host, want %d: %v", got, tt.urlSourceTag, urlSources(s))
			}
		})
	}
}

func TestMultibuildFlavor(t *testing.T) {
	text := load(t, "perl.spec")
	s := ParseWith(text, Options{Arch: "x86_64", Macros: TumbleweedMacros("x86_64", "base")})
	if s.Name != "perl-base" {
		t.Errorf("flavor base: Name = %q, want perl-base", s.Name)
	}
	s = ParseWith(text, Options{Arch: "x86_64", Macros: TumbleweedMacros("x86_64", "")})
	if s.Name != "perl" {
		t.Errorf("main flavor: Name = %q, want perl", s.Name)
	}
}

func TestExpandAndClassify(t *testing.T) {
	s := Parse(`
%define pkgname foo
%define forge_host gitlab.freedesktop.org
%bcond_without docs
Name:           python-%{pkgname}
Version:        1.2.3
URL:            https://%{forge_host}/x/%{pkgname}
Source0:        %{url}/-/archive/v%{version}/%{pkgname}-v%{version}.tar.bz2
Source1:        %{name}-%{version}.tar.gz
Source2:        https://files.pythonhosted.org/packages/source/f/%{pkgname}/%{pkgname}-%{version}.tar.gz
Source3:        %{pypi_source}
Source4:        https://%{undefined_host}/thing-%{version}.tar.gz
Source5:        https://user:pw@mirror.example.org:8443/%{nope}/thing.tar.gz
Source6:        ftp://ftp.example.net/pub/%{?with_docs:docs}%{!?with_docs:nodocs}.tar
Source7:        %{?nothing}local.tar
Source8:        file:///usr/share/thing.tar
`)
	if s.Name != "python-foo" || s.Version != "1.2.3" || s.URL != "https://gitlab.freedesktop.org/x/foo" {
		t.Fatalf("tags: %q %q %q", s.Name, s.Version, s.URL)
	}
	want := []struct {
		expanded, host string
		unresolved     bool
	}{
		{"https://gitlab.freedesktop.org/x/foo/-/archive/v1.2.3/foo-v1.2.3.tar.bz2", "gitlab.freedesktop.org", false},
		{"python-foo-1.2.3.tar.gz", "", false},
		{"https://files.pythonhosted.org/packages/source/f/foo/foo-1.2.3.tar.gz", "files.pythonhosted.org", false},
		{"https://files.pythonhosted.org/packages/source/f/foo/foo-1.2.3.tar.gz", "files.pythonhosted.org", false},
		{"https://%{undefined_host}/thing-1.2.3.tar.gz", "", true},
		{"https://user:pw@mirror.example.org:8443/%{nope}/thing.tar.gz", "mirror.example.org", false}, // unresolved path, host still known
		{"ftp://ftp.example.net/pub/docs.tar", "ftp.example.net", false},
		{"local.tar", "", false},
		{"file:///usr/share/thing.tar", "", false},
	}
	if len(s.Sources) != len(want) {
		t.Fatalf("got %d sources, want %d", len(s.Sources), len(want))
	}
	for i, w := range want {
		got := s.Sources[i]
		if got.Expanded != w.expanded || got.Host != w.host || got.Unresolved != w.unresolved {
			t.Errorf("%s: got (%q, host %q, unresolved %v), want (%q, %q, %v)", got.Tag, got.Expanded, got.Host, got.Unresolved, w.expanded, w.host, w.unresolved)
		}
	}
}

func TestConditionals(t *testing.T) {
	text := `
%if 0%{?suse_version} >= 1500
%define host suse.example
%else
%define host other.example
%endif
%if 0%{?fedora}
Source9:        https://fedora.example/x.tar
%endif
%ifarch x86_64 %{ix86}
%define arch_dir intel
%else
%define arch_dir generic
%endif
%if "%{flavor}" == "doc"
Name:           thing-doc
%else
Name:           thing
%endif
%if %{with weird} && (1 + 1 == 2)
Version:        2
%elif %{without weird}
Version:        1
%endif
%if %{some_unknown_macro}
URL:            https://first.example/
%else
URL:            https://second.example/
%endif
Source0:        https://%{host}/%{arch_dir}/%{name}-%{version}.tar.xz
`
	s := ParseWith(text, Options{Arch: "aarch64", Macros: TumbleweedMacros("aarch64", "")})
	if s.Name != "thing" || s.Version != "1" {
		t.Errorf("Name/Version = %q/%q, want thing/1", s.Name, s.Version)
	}
	if s.URL != "https://first.example/" {
		t.Errorf("an unknown condition should take the first branch, got URL %q", s.URL)
	}
	if len(s.Sources) != 1 || s.Sources[0].Expanded != "https://suse.example/generic/thing-1.tar.xz" {
		t.Errorf("Sources = %+v", s.Sources)
	}
	s = ParseWith(text, Options{Arch: "x86_64", Macros: TumbleweedMacros("x86_64", "doc")})
	if s.Name != "thing-doc" || s.Sources[0].Host != "suse.example" || s.Sources[0].Expanded != "https://suse.example/intel/thing-doc-1.tar.xz" {
		t.Errorf("x86_64 doc flavor: %q %+v", s.Name, s.Sources)
	}
}

func TestEvalIf(t *testing.T) {
	tests := []struct {
		expr    string
		val, ok bool
	}{
		{"01699 >= 1500", true, true},
		{"0 >= 1500", false, true},
		{`"" != ""`, false, true},
		{`"base" != "" && "base" != ("@BUILD_" + "FLAVOR@")`, true, true},
		{"!0", true, true},
		{"1 && 0 || 1", true, true},
		{"(1 || 0) && !(2 < 1)", true, true},
		{`"abc" == "abc"`, true, true},
		{"%{unknown}", false, false},
		{"", false, false},
		{"1 +", false, false},
		{"v\"1.2\" == \"1.2\"", true, true},
		{"1.14 > 1", true, true}, // version-looking numbers compare as strings
	}
	for _, tt := range tests {
		val, ok := evalIf(tt.expr)
		if val != tt.val || ok != tt.ok {
			t.Errorf("evalIf(%q) = (%v, %v), want (%v, %v)", tt.expr, val, ok, tt.val, tt.ok)
		}
	}
}
