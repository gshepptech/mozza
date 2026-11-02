package stackdraft

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/gshepptech/mozza/internal/stack"
)

// Options configure a draft.
type Options struct {
	// Name is the stack name; defaults to the source directory's name.
	Name string
	// OutputDir is where the stack file will be written. A source path is
	// recorded relative to it when the source is that directory.
	OutputDir string
	// Render runs `helm template` to learn the chart's real namespace and
	// Services. Off in tests that have no helm.
	Render bool
}

// Result is a drafted stack file and what it could not settle.
type Result struct {
	YAML []byte
	// Review lists what a person must check before `up`; the same text heads
	// the file.
	Review []string
}

// ErrNoChart means the tree ships no Helm chart, which is what a stack
// deploys. A compose-only app is a recipe's job (`mozza import`).
var ErrNoChart = errors.New("no Helm chart found")

// Draft scans root and drafts a stack for the app it holds.
func Draft(root string, opts Options) (*Result, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("draft: %w", err)
	}
	t, err := scan(root)
	if err != nil {
		return nil, fmt.Errorf("draft: scan %s: %w", root, err)
	}
	if len(t.charts) == 0 {
		msg := fmt.Sprintf("%v in %s; a stack installs an app's own chart", ErrNoChart, root)
		if len(t.compose) > 0 {
			msg += fmt.Sprintf(". It has %s — turn that into a recipe with `mozza import %s`", t.compose[0], t.compose[0])
		}
		return nil, errors.New(msg)
	}

	d := &drafter{t: t, opts: opts, values: map[string]any{}}
	d.build()
	out, err := d.render()
	if err != nil {
		return nil, err
	}
	// The draft must load as a stack. A draft that does not parse would
	// hand the person a broken file with no hint of why.
	if _, err := stack.Parse(out); err != nil {
		return nil, fmt.Errorf("draft: internal error, the drafted stack does not parse: %w", err)
	}
	return &Result{YAML: out, Review: d.review}, nil
}

type drafter struct {
	t    *tree
	opts Options

	name, namespace string
	chart           chart
	overlays        []valuesFile
	vals            values
	facts           templateFacts

	images  []stack.Image
	secrets map[string]stack.Value // dotted path -> generator
	addons  []string
	values  map[string]any
	access  *stack.Access
	review  []string
}

func (d *drafter) note(format string, a ...any) {
	d.review = append(d.review, fmt.Sprintf(format, a...))
}

func (d *drafter) build() {
	d.name = d.opts.Name
	if d.name == "" {
		d.name = slug(filepath.Base(d.t.root))
	}

	var others []chart
	d.chart, others = primaryChart(d.t)
	for _, c := range others {
		d.note("Also found chart %q at %s; it is not installed. Add it as a component if the app needs it.", c.name, c.dir)
	}
	d.namespace = slug(d.chart.name)

	var skipped []valuesFile
	d.overlays, skipped = pickOverlays(d.t.root, d.chart.dir)
	for _, s := range skipped {
		d.note("Values overlay %s was not used (its name does not say local or dev).", s.path)
	}
	d.vals = loadValues(d.t.root, d.chart.dir, d.overlays)
	d.facts = readTemplates(d.t.root, d.chart.dir)

	d.matchImages()
	d.draftStorage()

	d.secrets = map[string]stack.Value{}
	candidates := findSecrets(d.vals)
	for _, s := range candidates {
		if s.documented {
			d.addSecret(s)
		}
	}

	rendered := false
	if d.opts.Render {
		rendered = d.renderUntilClean(candidates)
	}
	if !rendered {
		// Without a clean render the template scan is all there is to go on.
		d.detectAddons(d.facts.apiGroups, d.facts.mentions)
		for _, r := range d.facts.required {
			path := strings.SplitN(r, ":", 2)[0]
			if !d.isSet(path) {
				d.note("The chart may require %s and the draft does not set it.", r)
			}
		}
	}
	if d.chart.dependencies > 0 {
		d.note("The chart has %d dependencies; Mozza builds them in its own copy of the chart.", d.chart.dependencies)
	}
}

func (d *drafter) addSecret(s secretField) {
	if _, done := d.secrets[s.path]; done {
		return
	}
	d.secrets[s.path] = stack.Value{Generate: s.generator, Length: s.length, Show: s.show}
	setPath(d.values, s.path, "${secret:"+d.namespaceRef()+"/"+d.secretName()+"/"+s.path+"}")
}

// renderUntilClean renders the chart with the drafted values and, each time
// it refuses for want of a credential the scan found, generates that one
// and renders again. Credentials are added because the chart said it needs
// them, not because of their names: a value for a feature that is off can
// switch the feature on.
func (d *drafter) renderUntilClean(candidates []secretField) bool {
	if d.chart.dependencies > 0 && !dirExists(filepath.Join(d.t.root, d.chart.dir, "charts")) {
		d.note("Could not render the chart: its dependencies are not built here. Check required values and add access entries by hand.")
		return false
	}
	for range 16 {
		out, errText, ok := d.helmTemplate()
		if ok {
			d.readRendered(out)
			return true
		}
		progressed := false
		for _, s := range candidates {
			if _, done := d.secrets[s.path]; done {
				continue
			}
			if mentions(errText, s.path) {
				d.addSecret(s)
				progressed = true
			}
		}
		if !progressed {
			d.note("`helm template` with the drafted values fails, so `up` will too until this is fixed: %s",
				truncate(errText, 300))
			return false
		}
	}
	return false
}

// mentions reports whether a render error names a values path: in full, or
// by its last two segments, which is how chart error messages usually
// refer to a value.
func mentions(errText, path string) bool {
	if strings.Contains(errText, path) {
		return true
	}
	parts := strings.Split(path, ".")
	if len(parts) >= 2 {
		return strings.Contains(errText, strings.Join(parts[len(parts)-2:], "."))
	}
	return false
}

func (d *drafter) isSet(path string) bool {
	if lookup(d.values, path) != nil {
		return true
	}
	v := lookup(d.vals.merged, path)
	s, isString := v.(string)
	return v != nil && (!isString || s != "")
}

func lookup(m map[string]any, path string) any {
	var cur any = m
	for p := range strings.SplitSeq(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[p]
	}
	return cur
}

// matchImages pairs each Dockerfile with the image the chart expects it to
// produce, by name: Dockerfile.web builds "inventory-web", the root Dockerfile
// builds the image named after the repository or chart.
func (d *drafter) matchImages() {
	fields := findImages(d.vals.merged, "")
	repo := strings.ToLower(filepath.Base(d.t.root))
	type pair struct {
		field int
		df    string
		score int
		name  string // the name the built image goes by
	}
	var pairs []pair
	for i, f := range fields {
		for _, df := range d.t.dockerfiles {
			s := matchScore(f.shortName(), dockerfileName(df, repo), repo, strings.ToLower(d.chart.name))
			// A chart's top-level image is the app itself, and the root
			// Dockerfile builds the app, whatever placeholder name the
			// chart still carries (helm create leaves "nginx").
			name := f.shortName()
			if s == 0 && f.path == "image" && df == "Dockerfile" {
				// Named after the app, not the placeholder: tagging a local
				// build "nginx" would shadow the real image.
				s, name = 2, slug(filepath.Base(d.t.root))
			}
			if s > 0 {
				pairs = append(pairs, pair{i, df, s, name})
			}
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].score > pairs[j].score })

	usedField, usedDF := map[int]bool{}, map[string]bool{}
	noIgnore := !fileExists(filepath.Join(d.t.root, ".dockerignore"))
	for _, p := range pairs {
		if usedField[p.field] || usedDF[p.df] {
			continue
		}
		usedField[p.field], usedDF[p.df] = true, true
		f := fields[p.field]
		ref := p.name + ":mozza"
		b := &stack.Build{Context: buildContext(d.t.root, p.df), Dockerfile: p.df}
		if noIgnore {
			b.Exclude = []string{".git", "**/node_modules"}
		}
		d.images = append(d.images, stack.Image{Ref: ref, Build: b})
		if f.asString {
			setPath(d.values, f.path, ref)
		} else {
			setPath(d.values, f.path+".repository", p.name)
			setPath(d.values, f.path+".tag", "mozza")
			if f.pullPolicy {
				setPath(d.values, f.path+".pullPolicy", "IfNotPresent")
			}
		}
	}
	if noIgnore && len(d.images) > 0 {
		d.note("The tree has no .dockerignore, so builds exclude .git and node_modules. Add anything else large (build output, binaries) to exclude.")
	}

	never := hasNeverPullPolicy(d.vals.merged)
	pulled := map[string][]string{} // ref -> values paths
	var order []string
	for i, f := range fields {
		if usedField[i] || f.tag == "" || strings.Contains(f.repository, "{{") {
			continue
		}
		if never {
			// With a Never pull policy the cluster cannot fetch it either.
			d.images = append(d.images, stack.Image{Ref: f.ref()})
			continue
		}
		if publicRegistry(f.repository) {
			continue
		}
		if _, seen := pulled[f.ref()]; !seen {
			order = append(order, f.ref())
		}
		pulled[f.ref()] = append(pulled[f.ref()], f.path)
	}
	for _, ref := range order {
		d.note("Image %s (%s) comes from a registry that may be private and no Dockerfile here builds it. "+
			"If the cluster cannot pull it, add a build or pass --image.", ref, strings.Join(pulled[ref], ", "))
	}
	for _, df := range d.t.dockerfiles {
		if !usedDF[df] {
			d.note("Dockerfile %s builds no image the chart names; add it to images if the app needs it.", df)
		}
	}
}

// publicRegistry reports whether an image comes from a registry anyone can
// pull from: Docker Hub (no host in the reference) and the well-known public
// mirrors. Anything else may need credentials the cluster does not have.
func publicRegistry(repo string) bool {
	first := strings.SplitN(repo, "/", 2)[0]
	if !strings.ContainsAny(first, ".:") || first == "localhost" {
		return !strings.Contains(repo, ".") || first == "docker.io"
	}
	switch first {
	case "docker.io", "registry.k8s.io", "quay.io", "gcr.io", "mcr.microsoft.com", "public.ecr.aws", "registry.gitlab.com":
		return first != "registry.gitlab.com"
	}
	return false
}

func dockerfileName(rel, repo string) string {
	base := filepath.Base(rel)
	switch {
	case base == "Dockerfile":
		if dir := filepath.Dir(rel); dir != "." {
			return strings.ToLower(filepath.Base(dir))
		}
		return repo
	case strings.HasPrefix(base, "Dockerfile."):
		return strings.ToLower(strings.TrimPrefix(base, "Dockerfile."))
	default:
		return strings.ToLower(strings.TrimSuffix(base, ".Dockerfile"))
	}
}

func matchScore(image, df, repo, chartName string) int {
	switch {
	case image == df:
		return 4
	case image == repo+"-"+df || image == chartName+"-"+df || df == repo+"-"+image:
		return 3
	case df == repo && (image == chartName || image == repo):
		return 3
	case len(df) >= 3 && strings.Contains(image, df), len(image) >= 3 && strings.Contains(df, image):
		return 1
	default:
		return 0
	}
}

// buildContext is the directory docker builds from: the Dockerfile's own
// directory when it holds a module of its own, else the repository root,
// which is what Dockerfiles kept in docker/ or deploy/ are written against.
func buildContext(root, df string) string {
	dir := filepath.Dir(df)
	if dir == "." {
		return "."
	}
	for _, marker := range []string{"go.mod", "package.json", "pyproject.toml", "requirements.txt", "Cargo.toml", "pom.xml"} {
		if fileExists(filepath.Join(root, dir, marker)) {
			return dir
		}
	}
	return "."
}

func (d *drafter) secretName() string { return d.name + "-draft" }

// namespaceRef is the namespace generated secrets live in. It is resolved
// after rendering, so it is a placeholder until then.
func (d *drafter) namespaceRef() string { return "@NS@" }

func (d *drafter) draftStorage() {
	for _, p := range findStorageClasses(d.vals.merged, "") {
		setPath(d.values, p, "${storageClass}")
	}
}

// detectAddons maps the API groups the chart creates objects in, and the
// cluster features it refers to, onto catalog addons.
func (d *drafter) detectAddons(apiGroups map[string]bool, mentions string) {
	if apiGroups["postgresql.cnpg.io"] {
		d.addons = append(d.addons, "cnpg")
	}
	if apiGroups["cert-manager.io"] {
		d.addons = append(d.addons, "cert-manager")
	}
	if strings.Contains(mentions, "tigerastatus") || strings.Contains(mentions, "projectcalico.org") {
		d.addons = append([]string{"calico"}, d.addons...)
	}
	if apiGroups["monitoring.coreos.com"] {
		d.note("The chart creates Prometheus Operator objects (ServiceMonitor and similar); turn them off or install the operator first.")
	}
	if strings.Contains(mentions, "kind: ingress") {
		if en, _ := lookup(d.vals.merged, "ingress.enabled").(bool); en {
			d.addons = append(d.addons, "ingress-nginx")
			setPath(d.values, "ingress.className", "nginx")
			d.note("Ingress is enabled; the draft installs ingress-nginx. Map its NodePort with cluster.ports and use hostnames under .localhost.")
		}
	}
}

// helmTemplate renders the chart with the overlays and the drafted values,
// returning the manifests, or helm's complaint when it refuses.
func (d *drafter) helmTemplate() ([]byte, string, bool) {
	tmp, err := os.CreateTemp("", "mozza-draft-values-*.yaml")
	if err != nil {
		return nil, err.Error(), false
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	vals := deepCopy(d.values)
	replaceInMap(vals, "@NS@", d.namespace)
	b, _ := yaml.Marshal(vals)
	_, _ = tmp.Write(b)
	_ = tmp.Close()

	args := []string{"template", d.name, filepath.Join(d.t.root, d.chart.dir), "--namespace", d.namespace}
	for _, o := range d.overlays {
		args = append(args, "-f", filepath.Join(d.t.root, o.path))
	}
	args = append(args, "-f", tmp.Name())
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "helm", args...) //nolint:gosec // argv built here
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, strings.TrimSuffix(msg, " Use --debug flag to render out invalid YAML"), false
	}
	return out, "", true
}

func deepCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			out[k] = deepCopy(sub)
			continue
		}
		out[k] = v
	}
	return out
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

type service struct {
	name, namespace string
	port            int
	portName        string
}

func (d *drafter) readRendered(out []byte) {
	dec := yaml.NewDecoder(bytes.NewReader(out))
	namespaces := map[string]int{}
	apiGroups := map[string]bool{}
	var services []service
	for {
		var obj map[string]any
		if err := dec.Decode(&obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}
		if obj == nil {
			continue
		}
		if av, _ := obj["apiVersion"].(string); strings.Contains(av, "/") {
			apiGroups[strings.SplitN(av, "/", 2)[0]] = true
		}
		meta, _ := obj["metadata"].(map[string]any)
		ns, _ := meta["namespace"].(string)
		if ns != "" {
			namespaces[ns]++
		}
		if obj["kind"] != "Service" {
			continue
		}
		spec, _ := obj["spec"].(map[string]any)
		if spec["clusterIP"] == "None" {
			continue
		}
		name, _ := meta["name"].(string)
		ports, _ := spec["ports"].([]any)
		for _, p := range ports {
			pm, _ := p.(map[string]any)
			port, _ := pm["port"].(int)
			pn, _ := pm["name"].(string)
			services = append(services, service{name: name, namespace: ns, port: port, portName: pn})
		}
	}

	d.detectAddons(apiGroups, strings.ToLower(string(out)))

	// A chart that pins every object to one namespace ignores --namespace;
	// the release and the generated secrets must go there instead.
	if len(namespaces) == 1 {
		for ns := range namespaces {
			if ns != d.namespace {
				d.note("The chart puts everything in namespace %q regardless of the release namespace; the draft uses it.", ns)
				d.namespace = ns
			}
		}
	}

	best, bestScore := service{}, 0
	for _, s := range services {
		if sc := serviceScore(s, d.chart.name); sc > bestScore {
			best, bestScore = s, sc
		}
	}
	if bestScore > 0 {
		ns := best.namespace
		if ns == "" {
			ns = d.namespace
		}
		d.access = &stack.Access{Name: d.name, Namespace: ns, Service: best.name, Port: best.port, LocalPort: localPort(d.name)}
	} else if len(services) > 0 {
		d.note("No Service looked like the app's front door; add an access entry for the one you open.")
	}
}

var (
	frontRE   = regexp.MustCompile(`web|ui|frontend|portal|console|app`)
	backendRE = regexp.MustCompile(`postgres|db|redis|metrics|webhook|mail|grpc|operator|broker|kafka|minio`)
)

func serviceScore(s service, chartName string) int {
	n := strings.ToLower(s.name)
	score := 1
	if frontRE.MatchString(n) {
		score += 3
	}
	if strings.Contains(n, strings.ToLower(chartName)) {
		score++
	}
	switch s.port {
	case 80, 8080, 3000, 8000, 5000, 4200:
		score += 2
	}
	if strings.Contains(strings.ToLower(s.portName), "http") || s.portName == "web" {
		score++
	}
	if backendRE.MatchString(n) {
		score -= 5
	}
	return score
}

func localPort(name string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return 18000 + int(h.Sum32()%900)
}

func slug(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 32 {
		s = strings.Trim(s[:32], "-")
	}
	if s == "" {
		return "app"
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
