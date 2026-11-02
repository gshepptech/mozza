package stack

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// builtin holds the stacks and addons that ship with Mozza. Stacks are the
// app-specific modules; addons are the reusable pieces stacks are built from.
//
//go:embed catalog/stacks/*.yaml catalog/addons/*.yaml
var builtin embed.FS

// Addon is a reusable component definition: a component plus the version its
// URLs and chart default to. Inside an addon, ${version} expands to the
// version in force — the addon's own, or the one the stack pinned with use.
type Addon struct {
	Component   `yaml:",inline"`
	Description string `yaml:"description"`
}

// Catalog finds stacks and addons by name. Definitions in the user's directory
// shadow the built-in ones of the same name, so a person can adjust a shipped
// stack without rebuilding Mozza.
type Catalog struct {
	// UserDir is searched before the built-in catalog. It holds stacks/*.yaml
	// and addons/*.yaml. Empty disables it.
	UserDir string
}

// DefaultCatalog searches $MOZZA_HOME (default ~/.mozza) before the built-ins.
func DefaultCatalog() Catalog {
	return Catalog{UserDir: Home()}
}

// Home is Mozza's per-user directory: $MOZZA_HOME, else ~/.mozza.
func Home() string {
	if h := os.Getenv("MOZZA_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".mozza"
	}
	return filepath.Join(home, ".mozza")
}

// Entry is one catalog listing row.
type Entry struct {
	Name        string
	Description string
	// Origin is "built-in" or the file it came from.
	Origin string
}

// Stacks lists every stack the catalog can resolve by name.
func (c Catalog) Stacks() ([]Entry, error) {
	return c.list("stacks", func(b []byte) (string, string, error) {
		var s Stack
		if err := yaml.Unmarshal(b, &s); err != nil {
			return "", "", err
		}
		return s.Name, s.Description, nil
	})
}

// Addons lists every addon the catalog can resolve by name.
func (c Catalog) Addons() ([]Entry, error) {
	return c.list("addons", func(b []byte) (string, string, error) {
		var a Addon
		if err := yaml.Unmarshal(b, &a); err != nil {
			return "", "", err
		}
		return a.Name, a.Description, nil
	})
}

func (c Catalog) list(kind string, head func([]byte) (string, string, error)) ([]Entry, error) {
	seen := map[string]Entry{}

	builtins, err := fs.Glob(builtin, "catalog/"+kind+"/*.yaml")
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", kind, err)
	}
	for _, p := range builtins {
		b, err := builtin.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", kind, err)
		}
		name, desc, err := head(b)
		if err != nil {
			return nil, fmt.Errorf("list %s: %s: %w", kind, p, err)
		}
		seen[name] = Entry{Name: name, Description: desc, Origin: "built-in"}
	}

	if c.UserDir != "" {
		files, _ := filepath.Glob(filepath.Join(c.UserDir, kind, "*.yaml"))
		for _, p := range files {
			b, err := os.ReadFile(p) //nolint:gosec // the user's own catalog directory
			if err != nil {
				return nil, fmt.Errorf("list %s: %w", kind, err)
			}
			name, desc, err := head(b)
			if err != nil {
				return nil, fmt.Errorf("list %s: %s: %w", kind, p, err)
			}
			seen[name] = Entry{Name: name, Description: desc, Origin: p}
		}
	}

	out := make([]Entry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// read returns the definition named name of the given kind, user directory
// first.
func (c Catalog) read(kind, name string) ([]byte, error) {
	if !nameRE.MatchString(name) {
		return nil, fmt.Errorf("%q is not a valid %s name", name, strings.TrimSuffix(kind, "s"))
	}
	if c.UserDir != "" {
		p := filepath.Join(c.UserDir, kind, name+".yaml")
		if b, err := os.ReadFile(p); err == nil { //nolint:gosec // name is validated above
			return b, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
	}
	b, err := builtin.ReadFile("catalog/" + kind + "/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("no %s named %q (see `mozza stack list`)", strings.TrimSuffix(kind, "s"), name)
	}
	return b, nil
}

// Load resolves ref to a stack. A ref that names an existing file is read from
// disk; anything else is looked up by name in the catalog. The stack is
// validated but not yet resolved — see Resolve.
func (c Catalog) Load(ref string) (*Stack, error) {
	st, statErr := os.Stat(ref)
	if statErr != nil || st.IsDir() {
		b, err := c.read("stacks", ref)
		if err != nil {
			return nil, fmt.Errorf("load stack: %w", err)
		}
		return Parse(b)
	}
	b, err := os.ReadFile(ref) //nolint:gosec // an explicit path the user passed
	if err != nil {
		return nil, fmt.Errorf("load stack: %w", err)
	}
	s, err := Parse(b)
	if err != nil {
		return nil, err
	}
	if abs, aerr := filepath.Abs(ref); aerr == nil {
		s.fileDir = filepath.Dir(abs)
	}
	return s, nil
}

// Parse decodes and validates a stack definition. Unknown keys are errors: a
// misspelled key in a deploy file otherwise vanishes silently and the stack
// comes up subtly different from the one the author wrote.
func Parse(b []byte) (*Stack, error) {
	var s Stack
	if err := decodeStrict(b, &s); err != nil {
		return nil, fmt.Errorf("parse stack: %w", err)
	}
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("stack %q is invalid:\n%w", s.Name, err)
	}
	return &s, nil
}

// addon loads one addon by name.
func (c Catalog) addon(name string) (*Addon, error) {
	b, err := c.read("addons", name)
	if err != nil {
		return nil, err
	}
	var a Addon
	if err := decodeStrict(b, &a); err != nil {
		return nil, fmt.Errorf("addon %s: %w", name, err)
	}
	// An addon's version is its default, not a use-time override, so it is
	// checked as an inline component would be: without it.
	inline := a.Component
	inline.Version = ""
	if err := inline.validate(); err != nil {
		return nil, fmt.Errorf("addon %s: %w", name, err)
	}
	return &a, nil
}

func decodeStrict(b []byte, v any) error {
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
