package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
)

// TestAComposeNetworkIsReportedRatherThanCarried pins what a compose network
// imports to, on both paths: no network policy anywhere, and a warning naming the
// boundary that was not carried.
//
// The two vocabularies do not line up. A compose network is an undirected broadcast
// domain (everyone on it reaches everyone else), while a recipe's `only accept
// traffic from` is a directed ingress allow-list whose *presence* is a deny. So a
// derived policy is right only when its list is every source compose already allows,
// and membership read off the `networks:` keys is a lower bound rather than that set.
// `extends`, `include`, `network_mode`, `external_links`, an `external:` or
// `attachable:` network, an exotic driver, a network's `name:`, an unexpanded
// `${VAR}` and an override file compose loads without being asked all put a service
// on a network no key records, and the last two cannot be closed at all, because
// mozza expands nothing anywhere and the server entry point is handed content with no
// path to look beside. Every one missed is a policy that silently black-holes a
// service while the deploy reports success.
//
// So the boundary is named instead. Every row asserts the same two things (that
// neither path writes a directive, and that the warning is exactly what the file
// declares), and the shapes are all kept, because a warning that names the wrong
// thing is the failure that remains once no policy can be written.
//
// A row's warnings come in two kinds, and the split is the point of them. What the
// boundary is belongs to the network, so it is reported once per network; what to
// write instead belongs to the slice, which carries one directive however many
// networks it is on, so it is reported once per service. Following one instruction on
// its own has to be safe, and TestFollowingOneRemedyDeniesNothingComposeAllows is
// where that is held rather than described.
//
// The warnings are pinned in full and in order, so a boundary dropped without one is
// a boundary dropped in silence. Two rows read the compiled manifests rather than the
// recipe, because what a policy would have cost is only visible there.
func TestAComposeNetworkIsReportedRatherThanCarried(t *testing.T) {
	tests := []struct {
		name string
		// input is a file `docker compose config` v5.1.2 accepts.
		input string
		// equivalent is a spelling of the same file that must import identically.
		equivalent string
		check      func(t *testing.T, p *plan.AppPlan)
		// warnings is every networks warning, in full and in order.
		warnings []string
	}{
		{
			// The ordinary file, and the copy a user is most likely to read. It has
			// to be enough on its own to know what to write and where.
			name: "an ordinary shared network is named, with the directive that carries it",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  worker:
    image: myworker:1.0
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "worker"),
				remedy("api", []string{"backend"}, `"api" and "worker"`, "Api"),
				remedy("worker", []string{"backend"}, `"api" and "worker"`, "Worker"),
			},
		},
		{
			// The list inflects with its length, and the allow-list is every member
			// including the slice the directive is written under: compose lets a
			// service reach its own replicas over a shared network.
			name: "three services on one network name all three, each of them included",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  cache:
    image: redis:7
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "cache", "db"),
				remedy("api", []string{"backend"}, `"api" and "cache" and "db"`, "Api"),
				remedy("cache", []string{"backend"}, `"api" and "cache" and "db"`, "Cache"),
				remedy("db", []string{"backend"}, `"api" and "cache" and "db"`, "Db"),
			},
		},
		{
			// The row that keeps the self-peer in the remedy honest. Mozza gives a
			// multi-copy stateful slice a headless service so its pods can find each
			// other, and a directive that left db off its own list would deny the
			// replica traffic that service exists to route. So the list mozza
			// recommends has to contain db. And no policy is compiled either way,
			// which is the whole of what the importer guarantees.
			name: "a service reaching its own replicas is in the list mozza recommends",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    deploy:
      replicas: 3
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db"),
			},
			check: func(t *testing.T, p *plan.AppPlan) {
				t.Helper()

				db := compiledFor(t, p, "db")

				require.NotNil(t, db.service, "a multi-copy stateful slice gets a service")
				assert.Equal(t, "None", db.service.Spec.ClusterIP,
					"it is headless, and headless exists for pod-to-pod discovery")
				assert.Nil(t, db.policy,
					"and nothing compiled beside it can deny what that discovery routes")
			},
		},
		{
			name: "services on networks of their own are two boundaries, not one",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    networks: [frontend]
networks:
  backend:
  frontend:
`,
			warnings: []string{
				declares("backend", "api"),
				declares("frontend", "web"),
				remedy("api", []string{"backend"}, `"api"`, "Api"),
				remedy("web", []string{"frontend"}, `"web"`, "Web"),
			},
		},
		{
			// web bridges two networks that do not touch, and is a member of both.
			// Its one directive is the union of the two, because a slice carries one
			// directive and the presence of that directive is a deny: a list of
			// either network's members alone would shut out the other's.
			name: "a service on several networks gets one directive holding all of them",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  cdn:
    image: mycdn:1.0
    networks: [frontend]
  web:
    image: nginx:1.25
    networks: [frontend, backend]
networks:
  backend:
  frontend:
`,
			warnings: []string{
				declares("backend", "api", "web"),
				declares("frontend", "cdn", "web"),
				remedy("api", []string{"backend"}, `"api" and "web"`, "Api"),
				remedy("cdn", []string{"frontend"}, `"cdn" and "web"`, "Cdn"),
				remedy("web", []string{"backend", "frontend"},
					`"api" and "cdn" and "web"`, "Web"),
			},
		},
		{
			// One service and one boilerplate network, which is most of the compose
			// files that declare one at all. There is nothing here for the network to
			// keep out, so there is no boundary to report and nothing to say.
			name: "a lone service on a boilerplate network imports silently",
			input: `
services:
  web:
    image: nginx:1.25
    networks: [frontend]
networks:
  frontend:
`,
		},
		{
			// Compose puts api on the default network, not on backend, so the two
			// genuinely cannot reach each other. Nothing is derived from the default
			// network (it is where compose puts a service that asked for no
			// isolation), and the partition is reported all the same, by the clause
			// saying what backend keeps out.
			name: "a service that omits networks is what the other network keeps out",
			input: `
services:
  api:
    image: myapi:1.0
  web:
    image: nginx:1.25
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "web"),
				remedy("web", []string{"backend"}, `"web"`, "Web"),
			},
		},
		{
			// `networks: [default]` puts a service exactly where declaring nothing
			// puts it, so the two spellings describe the same file and neither draws
			// a boundary to report.
			name: "naming the default network puts a service where it already was",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [default]
  worker:
    image: myworker:1.0
`,
			equivalent: `
services:
  api:
    image: myapi:1.0
  worker:
    image: myworker:1.0
`,
		},
		{
			// The default network is a real network with real members, and a service
			// that names one of its own as well is on both. So monitoring's members
			// are not everything that reaches api (worker is there too, having
			// declared nothing), and the reader is told before being handed a list
			// that leaves worker out.
			name: "a service also on the default network makes the list a short one",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [default, monitoring]
  metrics:
    image: mymetrics:1.0
    networks: [monitoring]
  worker:
    image: myworker:1.0
networks:
  monitoring:
`,
			warnings: []string{
				declares("monitoring", "api", "metrics") +
					mayBeShort(`service "api" also joins the default network, where compose puts`+
						" every service that declares no networks of its own"),
				remedy("api", []string{"monitoring"}, `"api" and "metrics"`, "Api") +
					denies("monitoring"),
				remedy("metrics", []string{"monitoring"}, `"api" and "metrics"`, "Metrics") +
					denies("monitoring"),
			},
		},
		{
			// Nothing but the default network anywhere, which is a file that asked
			// for no isolation at all.
			name: "a service whose only network is the default one draws no boundary",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [default]
  worker:
    image: myworker:1.0
    networks: [default]
`,
		},
		{
			name: "a file that declares no networks imports as it always did",
			input: `
services:
  api:
    image: myapi:1.0
  db:
    image: postgres:16
`,
		},
		{
			// A published port makes the slice reachable from outside the cluster,
			// and the compiler gives it an Ingress. The ingress controller joins no
			// compose network, so it is in no list read off one. A directive under
			// Web would deny it, the pods would stay up and the site would go dark.
			// So Web is left out of the remedy and told why, rather than the reader
			// noticing a name missing from it.
			name: "a public slice is left out of the remedy and told why",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "web"),
				remedy("api", []string{"backend"}, `"api" and "web"`, "Api"),
				exempt("web", []string{"backend"}, "Web"),
			},
			check: func(t *testing.T, p *plan.AppPlan) {
				t.Helper()

				web := compiledFor(t, p, "web")

				require.NotNil(t, web.ingress, "a public slice keeps its ingress")
				assert.Nil(t, web.policy, "and nothing is compiled that would deny it")
			},
		},
		{
			// Every member is public, so there is nowhere the directive belongs. Each
			// member says so for itself rather than the network reporting an absence:
			// a reader who is told nothing about a service cannot tell being passed
			// over from having nothing to do.
			name: "a network of nothing but public slices says so of every member",
			input: `
services:
  cdn:
    image: mycdn:1.0
    ports: ["9090:90"]
    networks: [edge]
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    networks: [edge]
networks:
  edge:
`,
			warnings: []string{
				declares("edge", "cdn", "web"),
				exempt("cdn", []string{"edge"}, "Cdn"),
				exempt("web", []string{"edge"}, "Web"),
			},
		},
		{
			// An external network's members live in compose projects mozza cannot
			// enumerate, so what the file declares on it is certainly not all of it.
			// All three spellings compose accepts say the same thing.
			name: "an external network says its members live outside this file",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [legacy]
  db:
    image: postgres:16
    networks: [legacy]
networks:
  legacy:
    external: true
`,
			equivalent: `
services:
  api:
    image: myapi:1.0
    networks: [legacy]
  db:
    image: postgres:16
    networks: [legacy]
networks:
  legacy:
    external: "true"
`,
			warnings: []string{
				declares("legacy", "api", "db") +
					mayBeShort("this network is declared `external:`, and its other members live"+
						" outside this compose file"),
				remedy("api", []string{"legacy"}, `"api" and "db"`, "Api") + denies("legacy"),
				remedy("db", []string{"legacy"}, `"api" and "db"`, "Db") + denies("legacy"),
			},
		},
		{
			name: "the legacy external mapping is external too",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [legacy]
  db:
    image: postgres:16
    networks: [legacy]
networks:
  legacy:
    external:
      name: real_legacy_net
`,
			warnings: []string{
				declares("legacy", "api", "db") +
					mayBeShort("this network is declared `external:`, and its other members live"+
						" outside this compose file"),
				remedy("api", []string{"legacy"}, `"api" and "db"`, "Api") + denies("legacy"),
				remedy("db", []string{"legacy"}, `"api" and "db"`, "Db") + denies("legacy"),
			},
		},
		{
			// A network's `name:` is the network compose actually creates, and a
			// second key can resolve to the same one, so the key a service is
			// written under does not settle who else is on the network it names.
			name: "a network renamed by its `name:` may be reached under another key",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
    name: shared_net
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort("this network sets a `name:` of its own, which is the network"+
						" compose actually creates, and a second key can resolve to that same"+
						" network"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// The `name:` is on some other key. A key that declares none still
			// resolves to <project>_<key>, which that one's `name:` is free to be, so
			// the question it raises is about this network as much as its own.
			name: "a network named elsewhere in the file is still a key that can collide",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
  ops:
    name: shared_ops
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort(`network "ops" sets a `+"`name:`"+` of its own, which is the network`+
						" compose actually creates, and a second key can resolve to that same"+
						" network"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// Both hold at once. External wins: that its members live outside the
			// file entirely is the larger fact, and a `name:` only says which key
			// reaches it.
			name: "a network both external and renamed reports the members outside the file",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [legacy]
  db:
    image: postgres:16
    networks: [legacy]
networks:
  legacy:
    external: true
    name: real_legacy_net
`,
			warnings: []string{
				declares("legacy", "api", "db") +
					mayBeShort("this network is declared `external:`, and its other members live"+
						" outside this compose file"),
				remedy("api", []string{"legacy"}, `"api" and "db"`, "Api") + denies("legacy"),
				remedy("db", []string{"legacy"}, `"api" and "db"`, "Db") + denies("legacy"),
			},
		},
		{
			// The mapping form is how a service declares an alias or an address on a
			// network, and it names the same networks the list form does.
			name: "the mapping form of a service's networks joins the same networks",
			input: `
services:
  api:
    image: myapi:1.0
    networks:
      backend:
        aliases: [api-internal]
  db:
    image: postgres:16
    networks:
      backend:
networks:
  backend:
`,
			equivalent: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db"),
			},
		},
		{
			// A service taking its network namespace from another reaches whatever
			// that one reaches, over a network it never joined by name, so it is a
			// source no `networks:` key in the file records.
			name: "a service sharing another's network namespace is a source no key names",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
  sidecar:
    image: busybox:1.36
    network_mode: "service:api"
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort(`service "sidecar" takes its network namespace from `+
						"`network_mode`, and reaches services over networks it never joined by name"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// `none` is the one value that reaches nothing. Measured against docker
			// 29.4.0 every other value reaches something (even `bridge` reaches a
			// service on a user-defined network at its address), so it is the only
			// one that leaves the file's memberships whole.
			name: "a service with no networking at all leaves the memberships whole",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
  job:
    image: myjob:1.0
    network_mode: none
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db"),
			},
		},
		{
			// `extends` merges the base's `networks:` into this service, from this
			// file or one mozza was never handed. Compose renders the short and long
			// forms identically, so mozza must too.
			name: "a service inheriting its networks with `extends` is a member no key names",
			input: `
services:
  base:
    image: mybase:1.0
    networks: [backend]
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    extends:
      service: base
`,
			equivalent: `
services:
  base:
    image: mybase:1.0
    networks: [backend]
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    extends: base
`,
			warnings: []string{
				declares("backend", "api", "base") +
					mayBeShort(`service "web" takes its configuration from another with `+
						"`extends`, whose networks mozza does not read"),
				remedy("api", []string{"backend"}, `"api" and "base"`, "Api") + denies("backend"),
				remedy("base", []string{"backend"}, `"api" and "base"`, "Base") + denies("backend"),
			},
		},
		{
			name: "`extends` from another file is a base mozza cannot read at all",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
  web:
    image: nginx:1.25
    extends:
      file: base.yml
      service: base
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort(`service "web" takes its configuration from another with `+
						"`extends`, whose networks mozza does not read"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// `include` merges another file's services into this project, on these
			// networks. A reader who wrote the directive from the two names here
			// would be denying a service they never saw named.
			name: "a file that includes another may have members it never lists",
			input: `
include:
  - base.yml
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort("this file pulls services in with `include`, which mozza does not"+
						" resolve"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			name: "a service linked to a container outside the project names no key for it",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
    external_links:
      - legacy_db:db
  web:
    image: nginx:1.25
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "web") +
					mayBeShort(`service "api" declares `+"`external_links`"+", which reach"+
						" containers this file does not define over a network shared with them"),
				remedy("api", []string{"backend"}, `"api" and "web"`, "Api") + denies("backend"),
				remedy("web", []string{"backend"}, `"api" and "web"`, "Web") + denies("backend"),
			},
		},
		{
			// `docker run --network` may join an attachable network, so its members
			// are not only this file's services: the same failure as `external:`,
			// reached by a different key. Compose accepts the quoted form and
			// normalises it to the bare flag.
			name: "an attachable network is one containers outside compose may join",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
    attachable: true
`,
			equivalent: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
    attachable: "true"
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort("this network is declared `attachable:`, and containers compose did"+
						" not start may join it"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// macvlan puts a container straight onto the physical network, so what
			// else is on it is nothing a compose file records. The plain network in
			// the same file keeps its own account, which is the per-network scope
			// visible in one row.
			name: "a network on a driver compose does not create is read per network",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [phys]
  db:
    image: postgres:16
    networks: [phys]
  cache:
    image: redis:7
    networks: [ops]
  metrics:
    image: mymetrics:1.0
    networks: [ops]
networks:
  phys:
    driver: macvlan
  ops:
    driver: bridge
`,
			warnings: []string{
				declares("ops", "cache", "metrics"),
				declares("phys", "api", "db") +
					mayBeShort(`this network's "macvlan" driver puts it on a segment compose does`+
						" not create"),
				remedy("api", []string{"phys"}, `"api" and "db"`, "Api") + denies("phys"),
				remedy("cache", []string{"ops"}, `"cache" and "metrics"`, "Cache"),
				remedy("db", []string{"phys"}, `"api" and "db"`, "Db") + denies("phys"),
				remedy("metrics", []string{"ops"}, `"cache" and "metrics"`, "Metrics"),
			},
		},
		{
			// Compose expands ${VAR} before it reads the project and mozza expands
			// nothing anywhere, so this key is a literal token that may be any of the
			// networks declared here. No remedy is offered for it (a list drawn from
			// a network that does not exist is not one to recommend), and the network
			// it may really be is told that a member of it is unaccounted for.
			//
			// This is the shape that ended the derivation: compose resolves all three
			// services onto backend, and any policy written from these keys admits
			// two of them.
			name: "an unexpanded ${VAR} is a network mozza cannot name",
			input: `
services:
  api:
    image: myapi:1.0
    networks: ["${NET:-backend}"]
  cache:
    image: redis:7
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("${NET:-backend}", "api") +
					mayBeShort("this network's name holds a `${VAR}` mozza does not expand, and this"+
						" may be one of the other networks declared here"),
				declares("backend", "cache", "db") +
					mayBeShort(`service "api" joins a network whose name holds a `+
						"`${VAR}`"+" mozza does not expand, and it may be this one"),
				withheld("api", []string{"${NET:-backend}"}),
				remedy("cache", []string{"backend"}, `"cache" and "db"`, "Cache") +
					denies("backend"),
				remedy("db", []string{"backend"}, `"cache" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// A slice name is not always the compose service name: a service called
			// after a recipe keyword gets a suffix, and the directive has to name the
			// slice while the sentence names the service.
			name: "a service named after a reserved word is written as the slice it becomes",
			input: `
services:
  app:
    image: myapp:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "app", "db"),
				remedy("app", []string{"backend"}, `"app-svc" and "db"`, "App-svc"),
				remedy("db", []string{"backend"}, `"app-svc" and "db"`, "Db"),
			},
		},
		{
			// Every name in the directive is quoted, always: the grammar takes an
			// identifier or a string, and an unquoted name colliding with a keyword
			// lexes as that keyword instead.
			name: "a member named after a keyword is still quoted into a directive that parses",
			input: `
services:
  public:
    image: mypublic:1.0
    networks: [backend]
  api:
    image: myapi:1.0
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "public"),
				remedy("api", []string{"backend"}, `"api" and "public"`, "Api"),
				remedy("public", []string{"backend"}, `"api" and "public"`, "Public"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The text path: what mozza import writes to disk, and what the server
			// deploys.
			text, err := ComposeToRecipe(tt.input)
			require.NoError(t, err, "compose accepts this file, so mozza must read it")

			assert.NotContains(t, text, "only accept traffic",
				"no compose file writes a network policy:\n%s", text)

			written, err := recipe.NewParser(text).Parse()
			require.NoError(t, err, "the written recipe must parse:\n%s", text)

			writtenPlan, err := plan.Build(written)
			require.NoError(t, err, "the written recipe must build:\n%s", text)
			require.NoError(t, plan.Validate(writtenPlan),
				"the written recipe must validate:\n%s", text)

			assert.Empty(t, boundaries(written), "what import wrote:\n%s", text)

			// The AST path: what the server reports and the warnings come from.
			imported, warnings, err := ComposeToRecipeAST([]byte(tt.input))
			require.NoError(t, err, "both entry points read the same types, so both must accept it")

			importedPlan, err := plan.Build(imported)
			require.NoError(t, err)
			require.NoError(t, plan.Validate(importedPlan), "the imported AST must validate too")

			assert.Empty(t, boundaries(imported),
				"the AST path has no production caller that compiles it, which is why it is held"+
					" to the same bar: a policy here is a trap for the first one to wire it up")

			assert.Equal(t, tt.warnings, networksWarnings(warnings))

			if tt.equivalent != "" {
				shortText, shortErr := ComposeToRecipe(tt.equivalent)
				require.NoError(t, shortErr)
				assert.Equal(t, shortText, text, "the two shapes name the same thing")

				shortAST, shortWarnings, shortASTErr := ComposeToRecipeAST([]byte(tt.equivalent))
				require.NoError(t, shortASTErr)
				assert.Equal(t, shortAST.Slices, imported.Slices, "both paths agree on both shapes")
				assert.Equal(t, tt.warnings, networksWarnings(shortWarnings),
					"and both shapes report the same boundary")
			}

			if tt.check != nil {
				tt.check(t, importedPlan)
			}
		})
	}
}
