package stackdraft

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// valuesFile is one values-*.yaml overlay and how strongly its name says it
// is meant for a local or development install.
type valuesFile struct {
	path  string // relative to the source root
	score int
}

// overlayScore ranks an overlay by its name. Production and test overlays
// are never picked: they demand digests, real TLS and external databases.
func overlayScore(name string) int {
	n := strings.ToLower(name)
	score := 0
	for word, w := range map[string]int{"local": 3, "dev": 3, "demo": 2, "kind": 2, "k3d": 2, "k3s": 1, "minikube": 2} {
		if strings.Contains(n, word) {
			score += w
		}
	}
	for _, bad := range []string{"prod", "hardened", "test", "digest", "staging", "ci"} {
		if strings.Contains(n, bad) {
			score -= 10
		}
	}
	return score
}

// pickOverlays returns the local-looking overlays, least specific first, so
// the most specific one is applied last and wins, as layered -f files do.
func pickOverlays(root, chartDir string) ([]valuesFile, []valuesFile) {
	var picked, skipped []valuesFile
	files, _ := filepath.Glob(filepath.Join(root, chartDir, "values-*.y*ml"))
	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		v := valuesFile{path: rel, score: overlayScore(filepath.Base(f))}
		if v.score > 0 {
			picked = append(picked, v)
		} else {
			skipped = append(skipped, v)
		}
	}
	sort.SliceStable(picked, func(i, j int) bool { return picked[i].score < picked[j].score })
	return picked, skipped
}

// values is a chart's merged values plus the comments written on its keys,
// which is where charts document the format a value must have.
type values struct {
	merged   map[string]any
	comments map[string]string // dotted path -> comment text
}

func loadValues(root, chartDir string, overlays []valuesFile) values {
	v := values{merged: map[string]any{}, comments: map[string]string{}}
	files := append([]string{filepath.Join(chartDir, "values.yaml")}, pathsOf(overlays)...)
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(root, f)) //nolint:gosec // a values file inside the tree
		if err != nil {
			continue
		}
		var m map[string]any
		if yaml.Unmarshal(b, &m) == nil {
			v.merged = merge(v.merged, m)
		}
		var doc yaml.Node
		if yaml.Unmarshal(b, &doc) == nil && len(doc.Content) == 1 {
			collectComments(doc.Content[0], "", v.comments)
		}
	}
	return v
}

func pathsOf(vs []valuesFile) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.path
	}
	return out
}

func merge(base, over map[string]any) map[string]any {
	for k, ov := range over {
		bm, bok := base[k].(map[string]any)
		om, ook := ov.(map[string]any)
		if bok && ook {
			base[k] = merge(bm, om)
			continue
		}
		base[k] = ov
	}
	return base
}

func collectComments(n *yaml.Node, prefix string, out map[string]string) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		path := key.Value
		if prefix != "" {
			path = prefix + "." + key.Value
		}
		if c := strings.TrimSpace(key.HeadComment + " " + key.LineComment + " " + val.LineComment); c != "" {
			out[path] = c
		}
		collectComments(val, path, out)
	}
}

// imageField is one image a chart is told to run.
type imageField struct {
	path       string // dotted values path of the image block or string
	repository string
	tag        string
	asString   bool // image: "repo:tag" rather than {repository, tag}
	pullPolicy bool // the block has its own pullPolicy key
}

func (f imageField) ref() string {
	if f.tag == "" {
		return f.repository
	}
	return f.repository + ":" + f.tag
}

// shortName is the image's last path segment without a tag or digest.
func (f imageField) shortName() string {
	r := f.repository
	if i := strings.LastIndex(r, "/"); i >= 0 {
		r = r[i+1:]
	}
	return strings.ToLower(r)
}

var imageStringRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*(:[A-Za-z0-9._-]+)?(@sha256:[a-f0-9]{64})?$`)

// findImages walks merged values for {repository, tag} blocks and image
// strings, the two shapes charts use.
func findImages(m map[string]any, prefix string) []imageField {
	// An image under a block that is switched off is never run, so building
	// it would only cost time.
	if en, ok := m["enabled"].(bool); ok && !en {
		return nil
	}
	var out []imageField
	keys := sortedKeys(m)
	for _, k := range keys {
		path := join(prefix, k)
		switch v := m[k].(type) {
		case map[string]any:
			if repo, ok := v["repository"].(string); ok && repo != "" {
				tag, _ := v["tag"].(string)
				_, pp := v["pullPolicy"]
				out = append(out, imageField{path: path, repository: repo, tag: tag, pullPolicy: pp})
				continue
			}
			out = append(out, findImages(v, path)...)
		case string:
			if (k == "image" || strings.HasSuffix(k, "Image")) && strings.ContainsAny(v, "/:") && imageStringRE.MatchString(v) {
				repo, tag := v, ""
				if i := strings.LastIndex(v, ":"); i > strings.LastIndex(v, "/") {
					repo, tag = v[:i], v[i+1:]
				}
				out = append(out, imageField{path: path, repository: repo, tag: tag, asString: true})
			}
		}
	}
	return out
}

// secretField is an empty value that names a credential.
type secretField struct {
	path      string
	generator string
	length    int
	show      bool
	// documented means the chart's own comment says the app fails without
	// it, which is reason enough to generate it before any render.
	documented bool
}

var secretNameRE = regexp.MustCompile(`(?i)(password|passwd|secret|token|apikey|api_key|masterkey|kek|privatekey|encryptionkey|signingkey)$`)

// findSecrets returns empty credential-looking values in parts of the chart
// that are switched on. A value under a block with enabled: false belongs to
// a feature the draft does not turn on, so it is left alone.
func findSecrets(v values) []secretField {
	var out []secretField
	var walk func(m map[string]any, prefix string)
	walk = func(m map[string]any, prefix string) {
		if en, ok := m["enabled"].(bool); ok && !en {
			return
		}
		for _, k := range sortedKeys(m) {
			path := join(prefix, k)
			switch val := m[k].(type) {
			case map[string]any:
				walk(val, path)
			case string:
				if val != "" || !secretNameRE.MatchString(k) || namesASecret(k, v.comments[path]) {
					continue
				}
				out = append(out, classify(path, k, v.comments[path]))
			}
		}
	}
	walk(v.merged, "")
	return out
}

var (
	secretRefKeyRE     = regexp.MustCompile(`(?i)(existing|secretname$|secretref$|pullsecret$|secretkeyref$)`)
	secretRefCommentRE = regexp.MustCompile(`(?i)(name of (an?|the) (existing )?(kubernetes )?secret|existing secret|secret (that )?(holds|contains|containing)|secret name|dockerconfigjson)`)
	documentedRE       = regexp.MustCompile(`(?i)(refuses? to (boot|start)|fail[- ]?fast|fails fast|must (be )?(set|supplied|provided)|no default|installers must|is required)`)
	conditionalRE      = regexp.MustCompile(`(?i)(required|needed|used) (only )?(when|if)`)
)

// namesASecret reports whether a value is the name of a Secret object
// rather than a credential. Generating one would point the chart at a
// Secret that does not exist, and often switches on the feature it serves.
func namesASecret(key, comment string) bool {
	return secretRefKeyRE.MatchString(key) || secretRefCommentRE.MatchString(comment)
}

// classify picks a generator from what the chart says about the value: an
// age key, a base64 key, a password a person types, or random hex.
func classify(path, key, comment string) secretField {
	c := strings.ToLower(comment)
	k := strings.ToLower(key)
	documented := documentedRE.MatchString(comment) && !conditionalRE.MatchString(comment)
	f := classifyGenerator(path, k, c)
	f.documented = documented
	return f
}

func classifyGenerator(path, k, c string) secretField {
	switch {
	case strings.Contains(c, "age-keygen") || strings.Contains(c, "age-secret-key") || regexp.MustCompile(`\bage\b`).MatchString(c):
		return secretField{path: path, generator: "age-key"}
	case strings.Contains(c, "base64"):
		return secretField{path: path, generator: "base64", length: 32}
	case strings.Contains(k, "password") || strings.Contains(k, "passwd"):
		return secretField{path: path, generator: "password", show: strings.Contains(path, "admin") || strings.Contains(path, "bootstrap")}
	default:
		return secretField{path: path, generator: "hex", length: 32}
	}
}

// findStorageClasses returns the paths of storage class values naming a
// specific class, which a local provider may not have.
func findStorageClasses(m map[string]any, prefix string) []string {
	var out []string
	for _, k := range sortedKeys(m) {
		path := join(prefix, k)
		switch v := m[k].(type) {
		case map[string]any:
			out = append(out, findStorageClasses(v, path)...)
		case string:
			if strings.EqualFold(k, "storageClass") || strings.EqualFold(k, "storageClassName") {
				if v != "" && v != "-" {
					out = append(out, path)
				}
			}
		}
	}
	return out
}

// hasNeverPullPolicy reports whether any pull policy is Never, in which case
// even pulled images must be loaded into the nodes.
func hasNeverPullPolicy(m map[string]any) bool {
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			if hasNeverPullPolicy(val) {
				return true
			}
		case string:
			if strings.HasSuffix(strings.ToLower(k), "pullpolicy") && val == "Never" {
				return true
			}
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func join(prefix, k string) string {
	if prefix == "" {
		return k
	}
	return prefix + "." + k
}

// setPath sets a dotted path in a nested map, creating maps on the way.
func setPath(m map[string]any, path string, v any) {
	parts := strings.Split(path, ".")
	for _, p := range parts[:len(parts)-1] {
		next, ok := m[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[p] = next
		}
		m = next
	}
	m[parts[len(parts)-1]] = v
}

// templateFacts are what a chart's templates say about the cluster it needs
// and the values it refuses to render without.
type templateFacts struct {
	apiGroups map[string]bool
	required  []string // "path: message"
	fails     []string
	mentions  string // lowercased template text, for prerequisite markers
}

var (
	requiredRE = regexp.MustCompile(`required\s+"([^"]+)"\s+\(?\s*\.Values\.([A-Za-z0-9_.]+)`)
	failRE     = regexp.MustCompile(`fail\s+"([^"]{8,})"`)
	apiRE      = regexp.MustCompile(`(?m)^apiVersion:\s*([a-z0-9.-]+\.[a-z]+)/`)
)

func readTemplates(root, chartDir string) templateFacts {
	f := templateFacts{apiGroups: map[string]bool{}}
	var all strings.Builder
	_ = filepath.WalkDir(filepath.Join(root, chartDir, "templates"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // best effort over a chart's templates
		}
		b, rerr := os.ReadFile(path) //nolint:gosec // a template inside the tree
		if rerr != nil {
			return nil //nolint:nilerr // best effort
		}
		s := string(b)
		all.WriteString(strings.ToLower(s))
		for _, m := range apiRE.FindAllStringSubmatch(s, -1) {
			f.apiGroups[m[1]] = true
		}
		for _, m := range requiredRE.FindAllStringSubmatch(s, -1) {
			f.required = append(f.required, m[2]+": "+m[1])
		}
		for _, m := range failRE.FindAllStringSubmatch(s, -1) {
			f.fails = append(f.fails, m[1])
		}
		return nil
	})
	f.mentions = all.String()
	sort.Strings(f.required)
	f.required = dedupe(f.required)
	f.fails = dedupe(f.fails)
	return f
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
