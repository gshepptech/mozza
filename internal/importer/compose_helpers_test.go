package importer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	"github.com/gshepptech/mozza/internal/k8s"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
)

// declares is the sentence every network warning opens with: which network, and
// which services this file puts on it. Written out here in full rather than
// assembled from compose.go's own fragments, so that a rewording there fails a test
// instead of travelling silently into the assertion. The clauses a row adds are
// spelled out at the row or built by the two helpers below.
func declares(network string, members ...string) string {
	subject, keeps := "service", "compose keeps out everything that did not join"
	if len(members) > 1 {
		subject = "services"
		keeps = "compose lets them reach each other and keeps out everything that did not join"
	}

	return fmt.Sprintf(
		"network %s draws a boundary the recipe does not carry — this file declares %s %s on it,"+
			" and %s", quotedList(network), subject, quotedList(members...), keeps)
}

// remedy is the whole per-service warning where a directive is recommended: the
// networks the service joins, the one directive that carries all of them, and the
// section it goes under. A row states the allow-list itself, so a peer added or
// dropped fails here rather than being derived from the code under test.
func remedy(service string, nets []string, allow, under string) string {
	boundaries := "that boundary"
	if len(nets) > 1 {
		boundaries = "those boundaries"
	}

	return onNetworks(service, nets) + fmt.Sprintf(
		" — to carry %s by hand, add `only accept traffic from %s` under %s, which is every"+
			" service this file declares on a network with it", boundaries, allow, under)
}

// exempt is the whole per-service warning for a service the compiler gives an
// Ingress: no directive, and why not.
func exempt(service string, nets []string, under string) string {
	return onNetworks(service, nets) + fmt.Sprintf(
		" and publishes a port, so its slice is reachable from outside the cluster and a network"+
			" policy under %s would deny the ingress that serves it — no directive is recommended",
		under)
}

// withheld is the whole per-service warning for a service whose every network is
// named by a token mozza never expanded, so there is no membership to draw a list
// from at all.
func withheld(service string, nets []string) string {
	return onNetworks(service, nets) +
		" — no directive is recommended, because every network it joins is named by a `${VAR}`" +
		" mozza does not expand, and a list drawn from a network mozza cannot name is not one" +
		" to follow"
}

// denies is the clause a remedy carries when one of the service's networks may hold
// members this file never names. It is what stands between a reader and a directive
// that shuts out whatever those are.
func denies(nets ...string) string {
	return fmt.Sprintf("; %s may have members this file does not declare, and the directive"+
		" denies anything it leaves out", quotedList(nets...))
}

// onNetworks opens every per-service warning: the service, and the networks it is on.
func onNetworks(service string, nets []string) string {
	return fmt.Sprintf("service %s is on %s", quotedList(service), quotedList(nets...))
}

// quotedList is how the warnings quote a compose key in a sentence, spelled out here
// rather than borrowed from compose.go so that a change to the quoting there fails an
// assertion instead of travelling into it.
func quotedList(keys ...string) string {
	quoted := make([]string, 0, len(keys))
	for _, key := range keys {
		quoted = append(quoted, `"`+key+`"`)
	}

	return strings.Join(quoted, " and ")
}

// mayBeShort is the clause a row carries when something in the file can put a service
// on the network without a `networks:` key recording it. It is what stands between a
// reader and a hand-written policy that denies whatever that construct brought in.
func mayBeShort(why string) string {
	return "; " + why + ", so the services named here may not be all of them"
}

// networksWarnings picks out what the import said about networks, leaving the
// warnings a row is not about (a build context, a dropped volume) alone.
func networksWarnings(warnings []Warning) []string {
	var said []string
	for _, w := range warnings {
		if w.Feature == "networks" {
			said = append(said, w.Message)
		}
	}

	return said
}

// boundaries reads the boundary every slice carries, keyed by the name plan.Build
// gives the slice. The two paths reach that name differently (the importer
// capitalizes a compose service into a section header, the parser lowercases a
// header it reads back), and the name an AllowFrom entry is checked against is
// the lowercased one, so that is the key both are held to.
//
// A slice carrying no boundary is absent rather than present and empty, because
// BuildNetworkPolicy turns an empty AllowFrom into `ingress: null`: a deny-all,
// and a different answer from no policy at all.
func boundaries(r *recipe.Recipe) map[string][]string {
	got := map[string][]string{}
	for _, s := range r.Slices {
		if s.NetworkPolicy != nil {
			got[strings.ToLower(s.Name)] = s.NetworkPolicy.AllowFrom
		}
	}

	return got
}

// compiled is what `mozza compile --target kubernetes` writes for one slice, as
// far as a boundary is concerned.
type compiled struct {
	policy  *networkingv1.NetworkPolicy
	service *corev1.Service
	ingress *networkingv1.Ingress
}

// compiledFor runs the Kubernetes compiler the way the command does, so a row
// reads the manifests a user would get rather than a reconstruction of them.
func compiledFor(t *testing.T, p *plan.AppPlan, slice string) compiled {
	t.Helper()

	objects, err := k8s.New().CompileObjects(p)
	require.NoError(t, err)

	var got compiled
	for _, object := range objects {
		switch o := object.(type) {
		case *networkingv1.NetworkPolicy:
			if o.Name == slice {
				got.policy = o
			}
		case *corev1.Service:
			if o.Name == slice {
				got.service = o
			}
		case *networkingv1.Ingress:
			if o.Name == slice {
				got.ingress = o
			}
		}
	}

	return got
}
