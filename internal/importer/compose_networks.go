package importer

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// convertNetworks produces warnings for compose networks that need attention.
func convertNetworks(networks map[string]composeNetworkConfig) []Warning {
	var warnings []Warning
	for name, net := range networks {
		if net.Internal {
			warnings = append(warnings, Warning{
				Feature: "networks",
				Message: fmt.Sprintf(
					"internal network %q — compose keeps its members from reaching anything outside it;"+
						" a recipe's network policy is ingress-only, so that outbound restriction is not carried",
					name),
				Severity: "info",
			})
		}
	}
	return warnings
}

// defaultNetwork is the network compose puts a service on when it names none.
const defaultNetwork = "default"

// explicitNetworks names the networks a service joins that draw a boundary at all,
// which is every network it names except the default one.
//
// Compose's default network needs no declaring and no joining: a service that says
// nothing lands there, and one that writes `networks: [default]` lands in exactly
// the same place. So naming it is not a request to be isolated, and a boundary
// derived as if it were would exclude every service that never wrote the word,
// which compose lets straight through.
func explicitNetworks(svc composeService) []string {
	nets := parseServiceNetworks(svc.Networks)
	explicit := make([]string, 0, len(nets))
	for _, net := range nets {
		if net != defaultNetwork {
			explicit = append(explicit, net)
		}
	}

	return explicit
}

// joinedNetworks names the explicit networks a service joins, in name order and
// each of them once. The list form lets a service name the same network twice; the
// mapping form cannot, and the two spellings have to describe the same file.
func joinedNetworks(svc composeService) []string {
	nets := explicitNetworks(svc)
	sort.Strings(nets)

	return slices.Compact(nets)
}

// joinsDefault reports whether a service names the default network among the ones
// it joins, which explicitNetworks drops and the warnings have to know about.
func joinsDefault(svc composeService) bool {
	return slices.Contains(parseServiceNetworks(svc.Networks), defaultNetwork)
}

// networkMembers maps each explicit network the file's services join to the
// services this file declares on it, in name order.
//
// This is what the `networks:` keys say and nothing more. A service can be on a
// network without naming its key: through `extends`, through `include`, through
// `network_mode`, through a `${VAR}` mozza does not expand, through an override
// file compose loads and mozza is never handed, or because a second key's `name:`
// resolved to the same real network. So a membership read here is a lower bound.
// That is enough to report a boundary and not enough to write one, which is why
// nothing derived from it reaches the recipe.
func networkMembers(cf composeFile) map[string][]string {
	members := map[string][]string{}
	for _, name := range sortedKeys(cf.Services) {
		for _, net := range joinedNetworks(cf.Services[name]) {
			members[net] = append(members[net], name)
		}
	}

	return members
}

// networkWarnings reports every boundary this compose file draws with its networks
// and that the recipe does not carry.
//
// No compose file produces a network policy. The two vocabularies are not the same
// thing: a compose network says who may reach whom, while a recipe's `only accept
// traffic from` is a directed ingress allow-list whose *presence* is a deny: the
// compiled policy admits the components it names and nothing else
// (k8s/networkpolicy.go). So a derived policy is correct only when its list is every
// source compose already allows, and membership read off the `networks:` keys is a
// lower bound rather than that set. A peer set derived from the keys is wrong on a
// whole family of ordinary compose shapes, and every miss is the same outcome: a
// policy that silently black-holes a service while the deploy reports success.
//
// So the boundary is named rather than carried: which network, which services this
// file puts on it, that mozza has not carried it, and the directive that writes it by
// hand. Naming the declared members is true however incomplete the file is; claiming
// they are all of them is what could not be.
//
// One warning per network, because the network is the boundary, then one per
// service on any of them, because the remedy is not. A file with one service is
// silent: its networks keep out nothing that exists here.
func networkWarnings(cf composeFile) []Warning {
	if len(cf.Services) < 2 {
		return nil
	}

	members := networkMembers(cf)

	warnings := make([]Warning, 0, len(members)+len(cf.Services))
	for _, net := range sortedKeys(members) {
		warnings = append(warnings, boundaryWarning(cf, net, members[net]))
	}

	for _, name := range sortedKeys(cf.Services) {
		if len(joinedNetworks(cf.Services[name])) > 0 {
			warnings = append(warnings, remedyWarning(cf, name, members))
		}
	}

	return warnings
}

// boundaryWarning says what one network keeps apart and that the recipe keeps none
// of it. What to write instead belongs to the service rather than the network, and
// is remedyWarning's.
func boundaryWarning(cf composeFile, net string, members []string) Warning {
	subject, keeps := "service", "compose keeps out everything that did not join"
	if len(members) > 1 {
		subject = "services"
		keeps = "compose lets them reach each other and keeps out everything that did not join"
	}

	message := fmt.Sprintf(
		"network %s draws a boundary the recipe does not carry — this file declares %s %s on it,"+
			" and %s", named(net), subject, namedAnd(members), keeps)

	if why, short := unreadMembers(cf, net, members); short {
		message += fmt.Sprintf("; %s, so the services named here may not be all of them", why)
	}

	return Warning{Feature: "networks", Message: message, Severity: "warn"}
}

// remedyWarning is the instruction for one service: the single directive that
// carries every network it joins, or why no directive is recommended for it.
//
// The remedy is grouped by service and not by network, and that is the whole of why
// it exists as its own warning. A slice carries one `only accept traffic from`, and
// the presence of that directive is a deny, so a service on two networks does not
// get a directive per network, it gets one whose list is the union of both. Reported
// per network, the two lists arrive as two instructions that each look complete, and
// following either on its own writes a policy narrower than compose: validate
// passes, the deploy reports success, and the traffic the other list carried is gone
// with nothing anywhere to read. Grouped per service, there is one instruction and
// it can be followed on its own.
//
// A list wider than compose allows is a weaker boundary; a narrower one is an
// outage. So where the declared membership may be short the list is still every peer
// this file records, and the warning says what the directive would leave out.
func remedyWarning(cf composeFile, service string, members map[string][]string) Warning {
	nets := joinedNetworks(cf.Services[service])
	message := fmt.Sprintf("service %s is on %s", named(service), namedAnd(nets))

	// A slice the compiler gives an Ingress must not carry the directive. The ingress
	// controller joins no compose network, so it is in no allow-list read off one, and
	// a policy under such a slice denies it: the pods stay up and the route stays in
	// place while the site goes dark, with nothing anywhere to read. The predicate is
	// the compiler's own ingress guard (k8s/compiler.go) less the conditions an import
	// cannot fail: no compose file names a domain, and a published port is what
	// convertService reads as Public. It is said out loud, because a service passed
	// over in silence is one the reader has to notice the absence of.
	if publishesPort(cf.Services[service]) {
		message += fmt.Sprintf(
			" and publishes a port, so its slice is reachable from outside the cluster and a"+
				" network policy under %s would deny the ingress that serves it — no directive"+
				" is recommended", capitalize(service))

		return Warning{Feature: "networks", Message: message, Severity: "warn"}
	}

	// A key holding a ${VAR} is not a network mozza can name (compose expands it
	// before it reads the project and mozza expands nothing), so the members under it
	// are whoever happened to write the same unexpanded token. Where that is every
	// network a service joins, there is no membership to draw a list from at all.
	// Where it is only some of them, the others are networks with members this file
	// really does declare, and the list drawn from them is true and possibly short,
	// which is what the caveat below is for.
	var short []string
	unnamed := 0
	for _, net := range nets {
		if interpolated(net) {
			unnamed++
		}
		if _, incomplete := unreadMembers(cf, net, members[net]); incomplete {
			short = append(short, net)
		}
	}

	if unnamed == len(nets) {
		message += " — no directive is recommended, because every network it joins is named by a" +
			" `${VAR}` mozza does not expand, and a list drawn from a network mozza cannot name is" +
			" not one to follow"

		return Warning{Feature: "networks", Message: message, Severity: "warn"}
	}

	boundaries := "that boundary"
	if len(nets) > 1 {
		boundaries = "those boundaries"
	}

	message += fmt.Sprintf(
		" — to carry %s by hand, add `only accept traffic from %s` under %s, which is every"+
			" service this file declares on a network with it",
		boundaries, quotedAnd(peers(nets, members)), capitalize(service))

	if len(short) > 0 {
		message += fmt.Sprintf("; %s may have members this file does not declare, and the"+
			" directive denies anything it leaves out", namedAnd(short))
	}

	return Warning{Feature: "networks", Message: message, Severity: "warn"}
}

// peers names every slice the directive for a service on these networks must admit:
// each service this file declares on any of them, the subject included.
//
// The subject is in its own list on purpose. Compose lets a service reach its own
// replicas over a shared network, and mozza gives a multi-copy stateful slice a
// headless service so its pods can find each other, and a directive that left the slice
// off would deny the traffic that service exists to route.
func peers(nets []string, members map[string][]string) []string {
	seen := map[string]bool{}
	var services []string
	for _, net := range nets {
		for _, name := range members[net] {
			if !seen[name] {
				seen[name] = true
				services = append(services, name)
			}
		}
	}
	sort.Strings(services)

	refs := make([]string, 0, len(services))
	for _, name := range services {
		refs = append(refs, sliceRef(name))
	}

	return refs
}

// unreadMembers names something that can put a service on this network without a
// `networks:` key recording it, or reports that nothing in the file does.
//
// Nothing here decides what is emitted (no compose file produces a network policy),
// so a construct missed costs a sentence rather than a service that silently goes
// dark. It is the remedy that needs it: a reader who writes the directive from the
// names above denies whatever these bring in, so where the declared membership may be
// short, the warning says so before recommending it.
//
// Order picks which of several true reasons the reader gets, and never whether one is
// given: this network's own declaration, then a service on it, then the file.
func unreadMembers(cf composeFile, net string, members []string) (string, bool) {
	// The drivers are enumerated the safe way round. `bridge` and `overlay` make a
	// segment of compose's own; everything else either bridges to a segment that
	// already exists (macvlan and ipvlan put a container straight onto the physical
	// network) or is a third-party plugin mozza has no basis to reason about.
	cfg := cf.Networks[net]
	switch {
	case interpolated(net):
		return "this network's name holds a `${VAR}` mozza does not expand, and this may be one of" +
			" the other networks declared here", true

	case cfg.External:
		return "this network is declared `external:`, and its other members live outside this" +
			" compose file", true

	case cfg.Attachable:
		return "this network is declared `attachable:`, and containers compose did not start may" +
			" join it", true

	case !composeCreates(cfg.Driver):
		return fmt.Sprintf("this network's %s driver puts it on a segment compose does not create",
			named(cfg.Driver)), true

	// A network's `name:` is the network compose actually creates, and nothing stops
	// a second key from resolving to the same one, so the services on a network are
	// not necessarily the ones written under the key it is reached by. Last of the
	// per-network reasons: the three above say the members are not only compose's,
	// which is the larger fact when both hold.
	case cfg.Name != "":
		return "this network sets a `name:` of its own, which is the network compose actually" +
			" creates, and a second key can resolve to that same network", true
	}

	// Also on the default network, where compose puts every service that named none,
	// so the services reaching this one are not only the ones under this key.
	for _, name := range members {
		if joinsDefault(cf.Services[name]) {
			return fmt.Sprintf("service %s also joins the default network, where compose puts every"+
				" service that declares no networks of its own", named(name)), true
		}
	}

	return unreadFile(cf)
}

// unreadFile names a construct anywhere in the file through which a service reaches
// another over a network no `networks:` key records, or reports that the file holds
// none. Each of these leaves every membership in the file possibly short, not only
// the one belonging to the service carrying it.
func unreadFile(cf composeFile) (string, bool) {
	if cf.Include != nil {
		return "this file pulls services in with `include`, which mozza does not resolve", true
	}

	for _, name := range sortedKeys(cf.Services) {
		svc := cf.Services[name]

		switch {
		case svc.Extends != nil:
			return fmt.Sprintf("service %s takes its configuration from another with `extends`,"+
				" whose networks mozza does not read", named(name)), true

		// Every value but `none` reaches, because the set that does is open: the
		// compose specification calls the available values platform-specific, and
		// beyond `host` and the `service:` and `container:` forms a bare network name
		// is accepted too and joins that network outright. Measured against docker
		// 29.4.0, `none` is the only value that reaches nothing: even `bridge`
		// reaches a service on a user-defined network at its address.
		case svc.NetworkMode != "" && svc.NetworkMode != "none":
			return fmt.Sprintf("service %s takes its network namespace from `network_mode`, and"+
				" reaches services over networks it never joined by name", named(name)), true

		case svc.ExternalLinks != nil:
			return fmt.Sprintf("service %s declares `external_links`, which reach containers this"+
				" file does not define over a network shared with them", named(name)), true

		case interpolatedNetwork(svc):
			return fmt.Sprintf("service %s joins a network whose name holds a `${VAR}` mozza does"+
				" not expand, and it may be this one", named(name)), true
		}
	}

	// Some other key names the network it creates, and a key that names none still
	// resolves to <project>_<key>, which that one's `name:` is free to be. Telling
	// the two that collide from the two that do not needs the project name, which
	// comes from the directory the file sits in, and the importer is handed content
	// rather than a path.
	if renamed := renamedNetwork(cf); renamed != "" {
		return fmt.Sprintf("network %s sets a `name:` of its own, which is the network compose"+
			" actually creates, and a second key can resolve to that same network",
			named(renamed)), true
	}

	return "", false
}

// composeCreates reports whether a network driver makes a segment holding only what
// compose put on it.
func composeCreates(driver string) bool {
	return driver == "" || driver == "bridge" || driver == "overlay"
}

// interpolated reports whether a network name is one compose would have expanded and
// mozza did not, so the key read here is a literal token rather than the network it
// stands for. Mozza expands ${VAR} nowhere, in any field.
func interpolated(net string) bool {
	return strings.Contains(net, "${")
}

// interpolatedNetwork reports whether a service names any of its networks that way,
// which puts it on a network none of the file's other keys can be held to.
func interpolatedNetwork(svc composeService) bool {
	return slices.ContainsFunc(parseServiceNetworks(svc.Networks), interpolated)
}

// named quotes a compose key the way a sentence quotes one.
//
// Not strconv.Quote: that escapes for Go's lexer, so a network key holding a quote
// character comes back as `net \"with\" quotes`, a backslash the reader has no use
// for in prose that is not a directive. quotedAnd keeps strconv.Quote, because there
// the escaping is what makes the directive parse.
func named(key string) string {
	return `"` + key + `"`
}

// namedAnd renders compose keys as a sentence lists them, joined the way the
// grammar's own lists are so a name reads the same in prose as in the directive.
func namedAnd(keys []string) string {
	quoted := make([]string, 0, len(keys))
	for _, key := range keys {
		quoted = append(quoted, named(key))
	}

	return strings.Join(quoted, " and ")
}

// renamedNetwork names the first network the file declares that carries a `name:`
// of its own, or "" when none does. An undeclared network has no config and so no
// name, which is right: compose rejects a file naming one.
func renamedNetwork(cf composeFile) string {
	for _, net := range sortedKeys(cf.Networks) {
		if cf.Networks[net].Name != "" {
			return net
		}
	}

	return ""
}

// publishesPort reports whether compose publishes one of a service's ports to the
// host, which is what convertService reads as Public and the compiler turns into
// an Ingress. Asked of the compose service rather than the slice it becomes so
// both entry points get the same answer from the same input.
func publishesPort(svc composeService) bool {
	for _, p := range svc.Ports {
		if port, public := parsePort(p); port > 0 && public {
			return true
		}
	}

	return false
}

// sliceRef is the name a directive must use to name the slice a compose service
// becomes. capitalize gives the section header, plan.Build lowercases that
// (plan/build.go convertSlice) and plan.Validate compares what a policy names
// against the result case-sensitively, so the name to write is the lowercased
// header rather than the compose service name. The two differ for a service named
// after a reserved word, where "app" heads a section called "App-svc" and is
// referred to as "app-svc".
func sliceRef(service string) string {
	return strings.ToLower(capitalize(service))
}

// quotedAnd renders names as the recipe grammar's own list: quoted, joined by
// "and".
//
// Every name is quoted, always. The grammar takes an identifier or a string
// (recipe/parser.go parseIdentDirective), and an unquoted name that collides with
// a keyword lexes as that keyword instead: public, secret, image, port, copies,
// domain, health, from, and, traffic and accept all do, and the point of quoting
// unconditionally is that the list of which ones is not worth keeping accurate.
func quotedAnd(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, strconv.Quote(name))
	}

	return strings.Join(quoted, " and ")
}
