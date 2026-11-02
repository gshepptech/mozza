// Package stackdraft drafts a stack file from an app's source tree, so a
// person starts `mozza stack` from a file that already names the app's chart,
// images, secrets and prerequisites instead of from an empty page.
//
// It reads what the tree declares and nothing else: Chart.yaml, values files
// and the comments on their keys, chart templates, Dockerfiles, playbooks.
// Whatever it cannot settle from those — which of two charts is the app, an
// image no Dockerfile builds, a value the chart fails without — becomes a
// review note at the top of the draft rather than a guess in its body. The
// draft always parses as a stack; whether the app comes up is learned by
// running it.
package stackdraft

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// skipDirs are never descended into: dependencies, build output, VCS data.
var skipDirs = map[string]bool{ //nolint:gochecknoglobals // immutable lookup table
	"node_modules": true, "vendor": true, "dist": true, "build": true,
	"tmp": true, "testdata": true, "coverage": true, "target": true,
}

const maxDepth = 6

// tree is what a scan found.
type tree struct {
	root        string
	charts      []chart
	dockerfiles []string // relative to root
	playbooks   []playbook
	compose     []string
}

type chart struct {
	dir          string // relative to root
	name         string
	dependencies int
}

type playbook struct {
	path  string   // relative to root
	hosts []string // the host patterns its plays target
}

// scan walks root once and collects every artifact the draft is built from.
func scan(root string) (*tree, error) {
	t := &tree{root: root}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable corner of the tree is skipped, not fatal
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if rel == "." {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || skipDirs[name] || strings.Count(rel, string(filepath.Separator)) >= maxDepth {
				return filepath.SkipDir
			}
			// A chart's charts/ directory holds its dependencies, not apps.
			if name == "charts" && fileExists(filepath.Join(filepath.Dir(path), "Chart.yaml")) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		switch {
		case name == "Chart.yaml":
			if c, ok := readChart(root, path); ok {
				t.charts = append(t.charts, c)
			}
		case isDockerfile(name):
			t.dockerfiles = append(t.dockerfiles, rel)
		case isCompose(name):
			t.compose = append(t.compose, rel)
		case strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml"):
			if p, ok := readPlaybook(path); ok {
				p.path = rel
				t.playbooks = append(t.playbooks, p)
			}
		}
		return nil
	})
	sort.Strings(t.dockerfiles)
	return t, err
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func isDockerfile(name string) bool {
	return name == "Dockerfile" || strings.HasPrefix(name, "Dockerfile.") || strings.HasSuffix(name, ".Dockerfile")
}

func isCompose(name string) bool {
	return name == "compose.yml" || name == "compose.yaml" ||
		(strings.HasPrefix(name, "docker-compose") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")))
}

func readChart(root, path string) (chart, bool) {
	b, err := os.ReadFile(path) //nolint:gosec // walking the tree the user pointed at
	if err != nil {
		return chart{}, false
	}
	var meta struct {
		Name         string `yaml:"name"`
		Type         string `yaml:"type"`
		Dependencies []any  `yaml:"dependencies"`
	}
	if yaml.Unmarshal(b, &meta) != nil || meta.Name == "" || meta.Type == "library" {
		return chart{}, false
	}
	dir, _ := filepath.Rel(root, filepath.Dir(path))
	return chart{dir: dir, name: meta.Name, dependencies: len(meta.Dependencies)}, true
}

// readPlaybook recognises a playbook: a YAML list whose items are plays,
// each a map with a hosts key.
func readPlaybook(path string) (playbook, bool) {
	b, err := os.ReadFile(path) //nolint:gosec // walking the tree the user pointed at
	if err != nil || len(b) > 1<<20 {
		return playbook{}, false
	}
	var plays []map[string]any
	if yaml.Unmarshal(b, &plays) != nil || len(plays) == 0 {
		return playbook{}, false
	}
	var p playbook
	seen := map[string]bool{}
	for _, play := range plays {
		h, ok := play["hosts"]
		if !ok {
			return playbook{}, false
		}
		hosts, _ := h.(string)
		for _, host := range strings.FieldsFunc(hosts, func(r rune) bool { return r == ',' || r == ':' || r == ' ' }) {
			if !seen[host] && !strings.Contains(host, "{{") {
				seen[host] = true
				p.hosts = append(p.hosts, host)
			}
		}
	}
	return p, true
}

// primaryChart picks the chart that is the app: the one named after the
// repository, else the one that is not a prerequisites/CRD/test chart, else
// the first. The others are reported, not dropped silently.
func primaryChart(t *tree) (chart, []chart) {
	repo := strings.ToLower(filepath.Base(t.root))
	best, bestScore := -1, -1<<31
	for i, c := range t.charts {
		score := 0
		n := strings.ToLower(c.name)
		if n == repo {
			score += 4
		}
		if regexp.MustCompile(`prereq|crd|test|example|sample|demo-|webhook`).MatchString(n) {
			score -= 3
		}
		if strings.Contains(c.dir, "deploy") || strings.Contains(c.dir, "helm") || strings.HasPrefix(c.dir, "charts") {
			score++
		}
		score -= len(n) / 20 // prefer the plain name over a decorated one
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	var others []chart
	for i, c := range t.charts {
		if i != best {
			others = append(others, c)
		}
	}
	return t.charts[best], others
}
