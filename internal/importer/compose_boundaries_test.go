package importer

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"

	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
)

// TestNoImportSetsADomain is the tripwire under the predicate remedyWarning uses to
// decide a slice is reachable from outside the cluster, and so to recommend it no
// directive at all.
//
// The compiler gives a slice an Ingress for `Public || Domain != ""`
// (k8s/compiler.go). publishesPort covers the first half only, and that is
// sufficient for exactly one reason: no import path writes Domain. A compose file
// carries no domain to read, so convertService never sets the field, and the second
// half is unreachable rather than merely unused.
//
// The day one is read (traefik's routing labels, below, are the obvious source), a
// slice would get an Ingress and a recommendation to write the policy that denies the
// controller serving it, and a reader following the advice would take the site dark
// with nothing to read anywhere. That change would be written in convertService,
// nowhere near the predicate. This fails on that commit and names what else has to
// move with it.
func TestNoImportSetsADomain(t *testing.T) {
	const composeFile = `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    networks: [backend]
    labels:
      traefik.enable: "true"
      traefik.http.routers.web.rule: "Host(web.example.com)"
networks:
  backend:
`

	imported, _, err := ComposeToRecipeAST([]byte(composeFile))
	require.NoError(t, err)
	require.Len(t, imported.Slices, 2)

	for _, s := range imported.Slices {
		assert.Empty(t, s.Domain,
			"slice %q carries a domain, so publishesPort is no longer the whole of the compiler's"+
				" ingress guard — remedyWarning has to withhold the directive from a slice with a"+
				" Domain too, or it recommends the policy that denies this slice's ingress", s.Name)
	}
}

// TestFollowingOneRemedyDeniesNothingComposeAllows is what makes the remedy safe to
// act on, and the reason it is grouped by service rather than by network.
//
// A slice carries one `only accept traffic from`, and the presence of that directive
// is a deny. So an instruction naming one network's members is safe only when the
// service is on no other network. Grouped per network, a service on two would get two
// instructions that each read as complete and self-contained, and following either
// one on its own compiles a policy narrower than compose: validate passes, the deploy
// reports success, and a tier goes dark with nothing anywhere to read. No wording
// makes that safe; the instruction has to be whole.
//
// So each instruction is applied here on its own, as a reader would apply it, and the
// policy it compiles to is held against the reach compose gives that service. What
// compose allows is written out below rather than derived, because deriving it from
// the code under test would agree with any answer that code gave.
func TestFollowingOneRemedyDeniesNothingComposeAllows(t *testing.T) {
	// The textbook three tiers: web is public on frontend, api bridges frontend and
	// backend, db is alone with api on backend.
	const composeFile = `
services:
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    networks: [frontend]
  api:
    image: myapi:1.0
    networks: [frontend, backend]
  db:
    image: postgres:16
    networks: [backend]
`

	// Everything sharing at least one network with the service, itself included:
	// compose lets a service reach its own replicas over a network it is on. web is
	// absent because it is public: the compiler gives it an Ingress, and a policy
	// under it would deny the controller serving it, so it is offered no directive.
	reach := map[string][]string{
		"api": {"api", "db", "web"},
		"db":  {"api", "db"},
	}

	text, err := ComposeToRecipe(composeFile)
	require.NoError(t, err)

	_, warnings, err := ComposeToRecipeAST([]byte(composeFile))
	require.NoError(t, err)

	applied := map[string][]string{}
	for _, message := range networksWarnings(warnings) {
		directive, section, ok := recommendation(message)
		if !ok {
			continue
		}

		// One directive, under one section, into the recipe mozza actually wrote,
		// which is the whole of what the reader was told to do.
		parsed, parseErr := recipe.NewParser(
			strings.Replace(text, section+":\n", section+":\n  "+directive+"\n", 1)).Parse()
		require.NoError(t, parseErr, "the directive mozza recommends must parse")

		built, buildErr := plan.Build(parsed)
		require.NoError(t, buildErr)
		require.NoError(t, plan.Validate(built), "and must name slices that exist")

		slice := strings.ToLower(section)
		compiledSlice := compiledFor(t, built, slice)
		require.NotNil(t, compiledSlice.policy, "the directive compiles to a policy")

		applied[slice] = admits(compiledSlice.policy)
	}

	assert.Equal(t, reach, applied,
		"every instruction is one a reader can follow on its own: applied alone, it must"+
			" deny nothing compose allows, and every service compose isolates must get one")
}

// recommended matches the instruction a warning carries, so the test acts on what
// was written rather than on a reconstruction of it.
var recommended = regexp.MustCompile("add `(only accept traffic from [^`]+)` under ([^,]+),")

// recommendation reads the directive and the section out of a warning that carries
// one. A warning that recommends nothing returns false.
func recommendation(message string) (string, string, bool) {
	match := recommended.FindStringSubmatch(message)
	if match == nil {
		return "", "", false
	}

	return match[1], match[2], true
}

// admits reads the components a compiled policy lets through, which is the whole of
// what the slice carrying it is reachable from.
func admits(policy *networkingv1.NetworkPolicy) []string {
	var components []string
	for _, rule := range policy.Spec.Ingress {
		for _, from := range rule.From {
			components = append(components,
				from.PodSelector.MatchLabels["app.kubernetes.io/component"])
		}
	}
	sort.Strings(components)

	return components
}

// TestABoundaryIsReportedForAFileThatCannotBuildYet covers the files the table above
// cannot: a `build:` context imports to a slice with no kind, which fails plan.Build
// for a reason that predates any of this and is filed separately.
//
// The boundary is reported anyway, naming the stub as a member. The user's next move
// is to supply the image by hand, and a warning withheld until the recipe builds
// would be one they never see: the boundary would be gone from under the edit with
// nothing said about it.
func TestABoundaryIsReportedForAFileThatCannotBuildYet(t *testing.T) {
	const composeFile = `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    build: .
    networks: [backend]
networks:
  backend:
`

	imported, warnings, err := ComposeToRecipeAST([]byte(composeFile))
	require.NoError(t, err)

	assert.Equal(t, []string{
		declares("backend", "api", "web"),
		remedy("api", []string{"backend"}, `"api" and "web"`, "Api"),
		remedy("web", []string{"backend"}, `"api" and "web"`, "Web"),
	}, networksWarnings(warnings), "the stub is a member like any other")
	assert.Empty(t, boundaries(imported))

	// The stub is what stops the recipe, and it stops it naming the missing image.
	_, err = plan.Build(imported)
	require.ErrorContains(t, err, `slice "web": could not determine kind`)

	// Supplying the image is the whole of the remedy, and the recipe it produces
	// still carries no policy. The warning is where the boundary was, and stays.
	text, err := ComposeToRecipe(composeFile)
	require.NoError(t, err)

	remedied := strings.Replace(text,
		"  # WARNING: build context detected, needs pre-built image\n",
		"  from image myweb:1.0\n", 1)

	parsed, err := recipe.NewParser(remedied).Parse()
	require.NoError(t, err)

	built, err := plan.Build(parsed)
	require.NoError(t, err)
	require.NoError(t, plan.Validate(built))

	assert.Empty(t, boundaries(parsed))
}
