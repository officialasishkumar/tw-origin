package rpmdb

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDistURL(t *testing.T) {
	tests := []struct {
		in   string
		want DistURL
		err  bool
	}{
		{
			in:   "obs://build.opensuse.org/openSUSE:Factory/standard/60f9bed512429f6605882ceff8a62883-bash",
			want: DistURL{APIHost: "build.opensuse.org", Project: "openSUSE:Factory", Repository: "standard", SrcMD5: "60f9bed512429f6605882ceff8a62883", Package: "bash"},
		},
		{
			in:   "obs://build.opensuse.org/openSUSE:Factory:ARM/standard/0e1eedc4996b5ca9778ff76611552027-boost:base",
			want: DistURL{APIHost: "build.opensuse.org", Project: "openSUSE:Factory:ARM", Repository: "standard", SrcMD5: "0e1eedc4996b5ca9778ff76611552027", Package: "boost", Flavor: "base"},
		},
		{
			in:   "obs://build.opensuse.org/openSUSE:Factory:ARM/standard/b9746d838e84aada5b7c5c77d38f79b3-000release-packages:openSUSE-release\n",
			want: DistURL{APIHost: "build.opensuse.org", Project: "openSUSE:Factory:ARM", Repository: "standard", SrcMD5: "b9746d838e84aada5b7c5c77d38f79b3", Package: "000release-packages", Flavor: "openSUSE-release"},
		},
		{in: "(none)", err: true},
		{in: "", err: true},
		{in: "obs://build.opensuse.org/openSUSE:Factory/standard/nothex-bash", err: true},
	}
	for _, tt := range tests {
		got, err := ParseDistURL(tt.in)
		if tt.err {
			if err == nil {
				t.Errorf("ParseDistURL(%q): expected error, got %+v", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseDistURL(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDistURL(%q)\n got %+v\nwant %+v", tt.in, got, tt.want)
		}
		if got.String() != strings.TrimSpace(tt.in) {
			t.Errorf("String() = %q, want %q", got.String(), strings.TrimSpace(tt.in))
		}
	}
}

func TestSourceName(t *testing.T) {
	tests := map[string]string{
		"bash-5.3.20-10.1.src.rpm":                        "bash",
		"boost-base-1.92.0-1.1.src.rpm":                   "boost-base",
		"aaa_base-84.87+git20260924.144354a1-2.1.src.rpm": "aaa_base",
		"python-rpm-macros-20260912.881b918-1.1.src.rpm":  "python-rpm-macros",
		"openSUSE-release-20261106-3127.1.nosrc.rpm":      "openSUSE-release",
	}
	for in, want := range tests {
		if got := SourceName(in); got != want {
			t.Errorf("SourceName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseAndGroup(t *testing.T) {
	in := strings.Join([]string{
		"bash\taarch64\tbash-5.3.20-10.1.src.rpm\tobs://build.opensuse.org/openSUSE:Factory:ARM/standard/60f9bed512429f6605882ceff8a62883-bash",
		"bash-sh\tnoarch\tbash-5.3.20-10.1.src.rpm\tobs://build.opensuse.org/openSUSE:Factory:ARM/standard/60f9bed512429f6605882ceff8a62883-bash",
		"gpg-pubkey\t(none)\t(none)\t(none)",
		"boost-license1_92_0\tnoarch\tboost-base-1.92.0-1.1.src.rpm\tobs://build.opensuse.org/openSUSE:Factory:ARM/standard/0e1eedc4996b5ca9778ff76611552027-boost:base",
		"local-thing\taarch64\tlocal-thing-1-1.src.rpm\t(none)",
		"",
	}, "\n")
	bins, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(bins) != 4 {
		t.Fatalf("got %d binaries, want 4 (gpg-pubkey skipped): %+v", len(bins), bins)
	}
	srcs := Group(bins)
	if len(srcs) != 3 {
		t.Fatalf("got %d sources, want 3: %+v", len(srcs), srcs)
	}
	if srcs[0].Name != "bash" || !reflect.DeepEqual(srcs[0].Binaries, []string{"bash", "bash-sh"}) {
		t.Errorf("bash group wrong: %+v", srcs[0])
	}
	if srcs[0].DistURL.SrcMD5 != "60f9bed512429f6605882ceff8a62883" {
		t.Errorf("bash DISTURL not parsed: %+v", srcs[0].DistURL)
	}
	if srcs[1].Name != "boost" || srcs[1].DistURL.Flavor != "base" {
		t.Errorf("multibuild flavor wrong: %+v", srcs[1])
	}
	if srcs[2].Name != "local-thing" || !srcs[2].DistURL.IsZero() {
		t.Errorf("package without DISTURL wrong: %+v", srcs[2])
	}
}
