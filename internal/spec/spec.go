// Package spec reads the parts of an RPM spec file that say where the source
// comes from: the Name, Version and URL tags, the Source lines, and the
// %define/%global macros needed to expand them. It is a textual reader, not
// rpm. %if conditions are evaluated when they only involve numbers, strings
// and macros the reader knows (a condition it cannot evaluate is taken as
// true, so the first branch wins); shell and Lua expansions are left in
// place; and a host is still extracted when only the path of a URL remains
// unresolved.
package spec

import (
	"regexp"
	"strconv"
	"strings"
)

// Spec is the result of Parse.
type Spec struct {
	Name    string            // expanded Name tag
	Version string            // expanded Version tag
	URL     string            // expanded URL tag (the project home page, not necessarily the source)
	Sources []Source          // Source lines in file order, from the branches that were taken
	Macros  map[string]string // macros in effect at the end of the preamble, raw bodies
	tags    map[string]string // first value of each tag, raw, lower-cased key
	arch    string
}

// Source is one Source line.
type Source struct {
	Tag        string `json:"tag"`                  // Source, Source0, Source10
	Number     int    `json:"number"`               // 0 for Source and Source0
	Raw        string `json:"raw"`                  // as written
	Expanded   string `json:"expanded"`             // after macro expansion; may still contain %{...}
	URL        string `json:"url,omitempty"`        // Expanded when it is a network URL
	Host       string `json:"host,omitempty"`       // host part, lower-cased
	Unresolved bool   `json:"unresolved,omitempty"` // a URL whose host part still contains a macro
}

// Options tell the reader about the build it should mimic.
type Options struct {
	Arch   string            // build architecture for %ifarch; "" treats every %ifarch as unknown
	Macros map[string]string // predefined macros, e.g. flavor, suse_version
}

// TumbleweedMacros are the distribution macros that specs test most often,
// as openSUSE Factory defines them, plus the multibuild flavor.
func TumbleweedMacros(arch, flavor string) map[string]string {
	m := map[string]string{
		"suse_version": "1699",
		"is_opensuse":  "1",
		"_os":          "linux",
		"_vendor":      "suse",
		"flavor":       flavor,
	}
	if arch != "" {
		m["_arch"] = arch
		m["_target_cpu"] = arch
		m["_build_cpu"] = arch
	}
	return m
}

var (
	tagRe     = regexp.MustCompile(`^\s*([A-Za-z][A-Za-z0-9_]*)\s*:\s*(.*?)\s*$`)
	sourceRe  = regexp.MustCompile(`^[Ss][Oo][Uu][Rr][Cc][Ee]([0-9]*)$`)
	defineRe  = regexp.MustCompile(`^\s*%(?:define|global)\s+([A-Za-z_][A-Za-z0-9_]*)(\([^)]*\))?\s*(.*?)\s*$`)
	undefRe   = regexp.MustCompile(`^\s*%undefine\s+([A-Za-z_][A-Za-z0-9_]*)`)
	bcondRe   = regexp.MustCompile(`^\s*%bcond(_with|_without)?\s+([A-Za-z_][A-Za-z0-9_]*)\s*(\S*)`)
	condRe    = regexp.MustCompile(`^\s*%(if|ifarch|ifnarch|ifos|ifnos|elif|elifarch|elifos|else|endif)\b\s*(.*?)\s*$`)
	bodyRe    = regexp.MustCompile(`^\s*%(prep|build|install|check|files|changelog|pre|post|preun|postun|pretrans|posttrans|trigger\w*|verifyscript|generate_buildrequires|conf)\b`)
	subpkgRe  = regexp.MustCompile(`^\s*%(package|description|sourcelist|patchlist)\b`)
	schemeRe  = regexp.MustCompile(`^\s*([A-Za-z][A-Za-z0-9+.-]*)://([^/?#\s]+)`)
	hostishRe = regexp.MustCompile(`^[A-Za-z0-9%{}?!:_.-]+(\.[A-Za-z0-9%{}?!:_-]+)+(/.*)?$`)
)

// Parse reads a spec file with no predefined macros.
func Parse(text string) *Spec { return ParseWith(text, Options{}) }

type branch struct{ active, taken, parent bool }

// ParseWith reads a spec file as a build with the given options would see it.
func ParseWith(text string, o Options) *Spec {
	s := &Spec{Macros: map[string]string{}, tags: map[string]string{}, arch: o.Arch}
	for k, v := range o.Macros {
		s.Macros[k] = v
	}
	if flavor, ok := o.Macros["flavor"]; ok {
		text = strings.ReplaceAll(text, "@BUILD_FLAVOR@", flavor)
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var stack []branch
	active := func() bool { return len(stack) == 0 || stack[len(stack)-1].active }
	inPreamble := true // Name/Version/URL are only taken from the main preamble
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		for strings.HasSuffix(line, "\\") && i+1 < len(lines) {
			i++
			line = strings.TrimSuffix(line, "\\") + lines[i]
		}
		if m := condRe.FindStringSubmatch(line); m != nil {
			stack = s.conditional(stack, m[1], m[2])
			continue
		}
		if !active() {
			continue
		}
		if bodyRe.MatchString(line) {
			break // %prep and later sections cannot add sources
		}
		if m := defineRe.FindStringSubmatch(line); m != nil {
			if m[2] == "" { // parametric macros cannot be expanded without arguments
				s.Macros[m[1]] = m[3]
			}
			continue
		}
		if m := undefRe.FindStringSubmatch(line); m != nil {
			delete(s.Macros, m[1])
			continue
		}
		if m := bcondRe.FindStringSubmatch(line); m != nil {
			// %bcond_without x and %bcond x 1 mean "with x" by default.
			if m[1] == "_without" || (m[1] == "" && m[3] != "" && s.Expand(m[3]) != "0") {
				s.Macros["with_"+m[2]] = "1"
			}
			continue
		}
		if subpkgRe.MatchString(line) {
			inPreamble = false
			continue
		}
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "%") || strings.HasPrefix(t, "#") {
			continue
		}
		m := tagRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		tag, value := m[1], m[2]
		if sm := sourceRe.FindStringSubmatch(tag); sm != nil {
			n, _ := strconv.Atoi(sm[1])
			s.Sources = append(s.Sources, Source{Tag: tag, Number: n, Raw: value})
			continue
		}
		key := strings.ToLower(tag)
		if !inPreamble && (key == "name" || key == "version" || key == "url") {
			continue
		}
		if _, dup := s.tags[key]; !dup {
			s.tags[key] = value
		}
	}
	s.Name = s.Expand(s.tags["name"])
	s.Version = s.Expand(s.tags["version"])
	s.URL = s.Expand(s.tags["url"])
	if s.URL != "" && !strings.Contains(s.URL, "://") && hostishRe.MatchString(s.URL) {
		s.URL = "https://" + s.URL
	}
	for i := range s.Sources {
		src := &s.Sources[i]
		src.Expanded = s.Expand(src.Raw)
		src.URL, src.Host, src.Unresolved = Classify(src.Expanded)
	}
	return s
}

// conditional applies one %if-family directive to the branch stack.
func (s *Spec) conditional(stack []branch, kw, arg string) []branch {
	parent := len(stack) == 0 || stack[len(stack)-1].active
	test := func() bool {
		switch kw {
		case "if", "elif":
			v, ok := evalIf(s.Expand(arg))
			return !ok || v
		case "ifarch", "elifarch":
			return s.archMatch(arg)
		case "ifnarch":
			return !s.archMatch(arg)
		case "ifos", "elifos":
			return osMatch(arg)
		case "ifnos":
			return !osMatch(arg)
		}
		return true
	}
	switch kw {
	case "if", "ifarch", "ifnarch", "ifos", "ifnos":
		v := test()
		return append(stack, branch{active: parent && v, taken: v, parent: parent})
	case "endif":
		if len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
		return stack
	}
	if len(stack) == 0 { // stray %else/%elif
		return stack
	}
	top := &stack[len(stack)-1]
	switch kw {
	case "else":
		top.active = top.parent && !top.taken
		top.taken = true
	default: // elif, elifarch, elifos
		if top.taken {
			top.active = false
		} else {
			v := test()
			top.active = top.parent && v
			top.taken = v
		}
	}
	return stack
}

func (s *Spec) archMatch(list string) bool {
	if s.arch == "" {
		return true // unknown build: take the first branch
	}
	for _, a := range strings.Fields(s.Expand(list)) {
		if a == s.arch || (a == "%{ix86}" && strings.HasSuffix(s.arch, "86")) || (a == "%{arm}" && strings.HasPrefix(s.arch, "arm")) {
			return true
		}
	}
	return false
}

func osMatch(list string) bool {
	for _, o := range strings.Fields(list) {
		if strings.EqualFold(o, "linux") {
			return true
		}
	}
	return false
}

// Tag returns the raw value of a preamble tag (case-insensitive), "" if absent.
func (s *Spec) Tag(name string) string { return s.tags[strings.ToLower(name)] }

// Classify decides whether a (possibly partially expanded) Source value is a
// network URL. It returns the URL and its host; unresolved is set when the
// value is a URL but its host still contains an unexpanded macro.
func Classify(v string) (url, host string, unresolved bool) {
	v = strings.TrimSpace(v)
	if len(v) > 5 && strings.EqualFold(v[:5], "file:") {
		return "", "", false
	}
	if m := schemeRe.FindStringSubmatch(v); m != nil {
		h := m[2]
		if i := strings.LastIndex(h, "@"); i >= 0 {
			h = h[i+1:]
		}
		if strings.Contains(h, "%") {
			return v, "", true
		}
		h = strings.ToLower(strings.TrimSuffix(h, "."))
		if !strings.HasPrefix(h, "[") { // keep IPv6 literals whole
			if i := strings.LastIndex(h, ":"); i >= 0 {
				h = h[:i]
			}
		}
		return v, h, false
	}
	if strings.Contains(v, "://") || strings.HasPrefix(v, "%{url}") || strings.HasPrefix(v, "%url") || strings.HasPrefix(v, "%{?url}") {
		return v, "", true
	}
	return "", "", false
}

// Expand replaces macros in v using the spec's own definitions and the
// built-in name, version and url macros. Whatever cannot be resolved stays.
func (s *Spec) Expand(v string) string {
	if !strings.Contains(v, "%") {
		return v
	}
	for range 8 {
		out, changed := s.expandOnce(v)
		v = out
		if !changed {
			break
		}
	}
	return strings.ReplaceAll(v, "%%", "%")
}

func (s *Spec) lookup(name string) (string, bool) {
	if v, ok := s.Macros[name]; ok {
		return v, true
	}
	switch name {
	case "nil":
		return "", true
	case "name", "version", "url", "release", "summary", "license":
		if v, ok := s.tags[name]; ok {
			return v, true
		}
	}
	return "", false
}

func (s *Spec) expandOnce(v string) (string, bool) {
	var b strings.Builder
	changed := false
	for i := 0; i < len(v); {
		c := v[i]
		if c != '%' {
			b.WriteByte(c)
			i++
			continue
		}
		if i+1 >= len(v) {
			b.WriteByte(c)
			break
		}
		switch v[i+1] {
		case '%':
			b.WriteString("%%")
			i += 2
		case '{':
			end := matching(v, i+1, '{', '}')
			if end < 0 {
				b.WriteString(v[i:])
				return b.String(), changed
			}
			rep, ok := s.braced(v[i+2 : end])
			if ok {
				b.WriteString(rep)
				changed = true
			} else {
				b.WriteString(v[i : end+1])
			}
			i = end + 1
		case '(':
			end := matching(v, i+1, '(', ')')
			if end < 0 {
				b.WriteString(v[i:])
				return b.String(), changed
			}
			b.WriteString(v[i : end+1]) // shell expansion: left as is
			i = end + 1
		default:
			j := i + 1
			if j < len(v) && v[j] == '?' {
				j++
			}
			k := j
			for k < len(v) && isWord(v[k]) {
				k++
			}
			if k == j {
				b.WriteByte(c)
				i++
				continue
			}
			name := v[j:k]
			val, ok := s.lookup(name)
			switch {
			case ok:
				b.WriteString(val)
				changed = true
			case v[i+1] == '?':
				changed = true // %?undefined expands to nothing
			default:
				b.WriteString(v[i:k])
			}
			i = k
		}
	}
	return b.String(), changed
}

// braced expands the inside of %{...}. ok is false when it must stay as is.
func (s *Spec) braced(inner string) (string, bool) {
	neg, cond := false, false
	if strings.HasPrefix(inner, "!?") {
		neg, cond, inner = true, true, inner[2:]
	} else if strings.HasPrefix(inner, "?") {
		cond, inner = true, inner[1:]
	}
	k := 0
	for k < len(inner) && isWord(inner[k]) {
		k++
	}
	name, rest := inner[:k], inner[k:]
	if name == "" {
		return "", false
	}
	if rest == "" {
		if name == "pypi_source" && !cond {
			return s.pypiSource(nil), true
		}
		val, ok := s.lookup(name)
		switch {
		case cond && neg: // %{!?foo} has no body and expands to nothing
			return "", true
		case cond: // %{?foo}: the value, or nothing
			if ok {
				return val, true
			}
			return "", true
		case ok:
			return val, true
		}
		return "", false
	}
	if strings.HasPrefix(rest, ":") {
		body := rest[1:]
		switch {
		case cond:
			_, ok := s.lookup(name)
			if ok != neg {
				return body, true
			}
			return "", true
		case name == "expand", name == "expr":
			return body, true
		case name == "lower":
			return strings.ToLower(s.Expand(body)), true
		case name == "upper":
			return strings.ToUpper(s.Expand(body)), true
		case name == "shrink":
			return strings.Join(strings.Fields(s.Expand(body)), " "), true
		}
		return "", false // %{lua:...}, %{sub:...} and friends
	}
	if rest[0] == ' ' || rest[0] == '\t' {
		args := strings.Fields(rest)
		switch name {
		case "pypi_source":
			return s.pypiSource(args), true
		case "with", "without", "defined", "undefined":
			if len(args) != 1 {
				return "", false
			}
			key := args[0]
			if name == "with" || name == "without" {
				key = "with_" + key
			}
			_, ok := s.lookup(key)
			if name == "without" || name == "undefined" {
				ok = !ok
			}
			if ok {
				return "1", true
			}
			return "0", true
		}
		return "", false // other parametric macro calls
	}
	return "", false
}

// pypiSource mirrors Fedora's %pypi_source [name [version [ext]]].
func (s *Spec) pypiSource(args []string) string {
	name := strings.TrimPrefix(strings.TrimPrefix(s.Expand(s.tags["name"]), "python-"), "python3-")
	ver, ext := s.Expand(s.tags["version"]), "tar.gz"
	if len(args) > 0 {
		name = s.Expand(args[0])
	}
	if len(args) > 1 {
		ver = s.Expand(args[1])
	}
	if len(args) > 2 {
		ext = s.Expand(args[2])
	}
	if name == "" {
		return "https://files.pythonhosted.org/packages/source/"
	}
	return "https://files.pythonhosted.org/packages/source/" + name[:1] + "/" + name + "/" + name + "-" + ver + "." + ext
}

func matching(v string, open int, lb, rb byte) int {
	depth := 0
	for i := open; i < len(v); i++ {
		switch v[i] {
		case lb:
			depth++
		case rb:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isWord(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
