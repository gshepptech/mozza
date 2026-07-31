# Changelog

All notable changes to Mozza will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Mozza is a deployment compiler with two paths: **pick it up** (`mozza compile` hands you the file)
and **get it delivered** (Mozza deploys it for you). The compiler and its two backends already
existed — this release names them and makes the compile-only path a first-class, documented
surface. See [docs/VISION.md](docs/VISION.md).

### Added

- `mozza compile [--target <name>] [-o <dir>]` — compile a recipe to deployment artifacts on disk
  without deploying. Contacts no cluster, reads no kubeconfig, records no deployment state, and
  needs no config file. Default target `kubernetes`, default output directory `.`. An unknown
  target fails before parsing and lists the valid ones
- `mozza targets` — list the registered compile targets with descriptions, marking the default
- `GET /api/v1/targets` — the registered targets as JSON, plus the default target name
- `POST /api/v1/compile` — compile recipe `source` to an optional `target` and return every
  generated file, the summary, and any capability warnings. Applies nothing, so it requires a
  session but not the `deployer` or `admin` role
- `internal/targets` — one place where every built-in target is registered, shared by the CLI and
  the HTTP API, so the two can never disagree about what targets exist
- `docs/VISION.md` — the product vision and design principles
- `docs/specs/multi-target-compiler.md` — the multi-target compiler milestone plan

### Changed

- README reframed around the deployment-compiler model, documenting both paths
- `docs/getting-started.md` covers the compile-only workflow as a first-class path, followed
  start to finish with no cluster and no Docker
- Capability warnings from a compile are now printed by every command that compiles, rather than
  each command deciding for itself whether to surface them
- Environment variable keys now follow the Kubernetes environment variable name rule — letters,
  digits, `_`, `-` and `.`, not starting with a digit — instead of uppercase and underscores only.
  The rule is taken from `k8s.io/apimachinery` rather than restated, so it cannot drift from what a
  cluster will accept, and a rejected key is now explained in words instead of as a regex
- Ports are now held to the range Kubernetes accepts — 1 to 65535 — everywhere a recipe or a
  hand-built plan can carry one: the slice's own port, each named `ports` entry, a sidecar's ports
  and a TCP health check's port. `on port 0 as http` used to validate and then compile to a
  `containerPort: 0` and a Service on `port: 0` that the API server refuses. A rejected port now
  names the offending value and states the range in words — with the line and column when it came
  from a recipe — and reports once instead of following the real complaint with a spurious
  `unknown directive`

### Deprecated

- `mozza deploy --export` — use `mozza compile --target kubernetes` instead. It still works and
  still writes to `./k8s/`, but prints a deprecation notice to stderr. It will be removed in a
  future minor version

### Removed

- The Prometheus recipe's `RETENTION` variable, which never took effect. Retention is set by the
  `--storage.tsdb.retention.time` command-line flag, which a recipe cannot yet express, so the
  control was inert. Retention stays at the Prometheus default of 15 days — the same value the
  catalog advertised — so behaviour is unchanged

### Fixed

- **A value inside an imported file could write recipe directives of its own, and they compiled
  into real Kubernetes manifests.** A Compose service carrying `restart: "always run 5 copies"`
  imported to the line `restart always run 5 copies`, which compiled to a Deployment with
  `replicas: 5` — from a file that asked for one copy and never mentioned copies at all. **This
  needed no newline, and no quote either: the recipe grammar then separated directives by whitespace
  within a section rather than by line, so a single space in the value was enough** — the grammar
  refuses that shape now, below — and the recipe the importer wrote for that file contained no quote
  character anywhere in it. An image tag did the
  same thing the same way (`nginx:1.25 run 7 copies` compiled to `replicas: 7`); an env value could
  close the quotes the importer put around it and carry on (`replicas: 11`); and a volume name
  reached a `# WARNING:` comment, where a newline ended the comment and the next line was live
  again (`replicas: 21`). Each of those compiled clean, with no complaint anywhere along the way.
- **A value the grammar cannot hold is now refused at import rather than escaped, because escaping
  is not something this language offers.** There is no escape sequence in any position: `set Q to
  "a\"b"` fails to parse with `unterminated string literal`, and `set Q to "a\nb"` reaches the
  cluster as the literal characters `a\nb`. So quoting a value that carries structure produces text
  the parser will not read back as the value that went in — it cannot round-trip in any encoding —
  and writing it anyway is exactly what produced the directives above. The import now stops, naming
  the field the value came from (`services.api.restart`, `services.api.environment.EVIL`), which
  byte it could not write, and what that byte would have ended, and it writes no recipe file.
- **The rule is per position, not per character**, because what is structural depends entirely on
  where the value lands. In a bare-word argument every byte that ends a word ends it — a space, a
  tab, a comma, a quote — since the value has to lex to exactly one token. Inside a quoted value
  only a quote or a line ending escapes, so an env value goes on holding a sentence and
  `MESSAGE: "hello world from mozza"` imports byte for byte as it did before. Inside a `#` comment
  nothing short of a line ending escapes at all, so a mount path may hold spaces *and* quotes —
  `data:/var/lib/my "app" data dir` still imports unchanged — where a restart policy may not. A
  single position-blind rule would have had to break both of those to close the unquoted sites.
- Every importer that builds recipe text is covered: the Compose importer at each field it reads;
  the Helm importer, both for the chart name and for the values it reads out of `values.yaml` —
  `image.repository`, `image.tag`, and the port, which it formats from whatever YAML node it finds
  under `service.port` or `containerPort`, and names by the key it actually read; the Dockerfile
  importer, for the port it takes off `EXPOSE` and for the repository name; and the GitHub URL all
  three are reached through, whose owner and repository segments become the chart name, the image
  name and the fallback recipe a repo scan writes when it recognises nothing. That one is guarded
  where the URL is parsed, because `url.Parse` percent-decodes the path and a `%0A` in someone's
  URL arrives as a real newline. Both Compose files in this repository import byte for byte as they
  did before the change.
- **What this does not cover is worth saying plainly, because recipe text is built in places the
  importer never touches.** The framework-detection endpoint assembles a recipe from its own
  answers, the template and marketplace renderers substitute `{{VAR}}` into recipe source with
  request-supplied values, and the dashboard's wizard builds recipe text in the browser. Those
  paths are unchanged, and each is the same defect in a different emitter. What they no longer get
  away with is a directive that does not begin a line, which is what made a single space enough —
  that is closed in the grammar, in the entry below, because it is a change to how every recipe is
  read rather than to how imported ones are written.
- **The recipe language now requires a directive to begin a line, so a value landing mid-line cannot
  become a directive no matter which emitter wrote it.** `restart always run 5 copies` was two
  directives and compiled to `replicas: 5`; it is now `line 5, col 18: unexpected "run" after a
  complete directive — each directive goes on its own line`. Every construct that reads one entry
  per line had the identical hole and each is closed with it — an `Images:` alias pair, two `CRDs:`
  entries and two `before starting` init steps all parsed silently on one line before, and so did a
  second directive after any of the five that end in words they may ignore, such as `update one at a
  time domain evil.example.com`. This is the
  grammar half of the injection above, not a replacement for it: the importer refuses a value at the
  source and names the field it came from, the grammar refuses whatever any other emitter produces.
  **It does not close a value widening the directive it is already inside**, because that damage
  happens within one directive on one line — `depends_on: "db and cache"` still parses to two
  dependencies and a health check of `/healthz every 30s` still sets a 30-second interval, both of
  which stay covered by the import-time refusal. Every recipe in `examples/`,
  `internal/template/recipes/` and the documentation parses to a byte-identical AST.
- **A leftover word is only told to move to its own line when moving it would help.** A word that
  begins no directive is a misspelling or an option the directive does not have, and no line break
  fixes either: `graceful shutdown after 45s` now reads `unexpected "45s" — the directive on this
  line does not accept it, and no directive starts with it` rather than being sent to put `45s` on a
  line of its own.
- **A compose file whose services share a network imported to a recipe that said nothing about the
  boundary, and to an AST `mozza validate` would have rejected.** `ComposeToRecipeAST` put the
  *network's* name into a slice's `only accept traffic from` list, which holds *slice* names, so the
  AST it returned could not survive `plan.Validate` — `network policy references unknown source
  "backend"`. No production caller compiles that AST, so nobody had hit it: `mozza import` writes the
  text path's recipe, and that path emitted no network directive at all, dropping the boundary in
  silence. **Neither path writes a network policy now, and the boundary is reported instead**, on
  both of them. Two services on one `backend` network import with: *network "backend" draws a
  boundary the recipe does not carry — this file declares services "api" and "db" on it, and compose
  lets them reach each other and keeps out everything that did not join*, then, for each of them,
  *service "api" is on "backend" — to carry that boundary by hand, add `only accept traffic from
  "api" and "db"` under Api, which is every service this file declares on a network with it*. The
  names are quoted, so that a service called `public` or `domain` cannot produce a recipe that will
  not parse, and lowercased to the name `plan.Build` gives the slice; every member is in the list
  including the slice the directive goes under, because compose lets a service reach its own replicas
  over a shared network and mozza gives a multi-copy stateful slice a headless service for exactly
  that traffic.
- **One warning per network for the boundary, but one per *service* for the directive that carries
  it**, and that split is load-bearing. A slice carries one `only accept traffic from`, and the
  presence of that directive is a deny — so a service on two networks needs one directive holding the
  union of both, not one per network. Reported per network, a three-tier file handed the reader two
  instructions that each read as complete; following either one alone compiled a policy narrower than
  compose, `mozza validate` passed, the deploy reported success and a tier went dark with nothing
  anywhere to read. Every service on a network now gets exactly one instruction, its list is every
  service this file declares on any network with it, and it is safe to act on by itself.
- **Deriving the policy was tried and abandoned, which is worth stating rather than leaving as a
  gap.** The two vocabularies do not line up: a compose network is an undirected broadcast domain,
  while a recipe's `only accept traffic from` is a directed ingress allow-list whose *presence* is a
  deny. So a derived policy is right only when its list holds every source compose already allows —
  and membership read off the `networks:` keys is a lower bound on that set rather than the set. A
  service joins a network without naming its key through `extends`, `include`, `network_mode`,
  `external_links`, an `external:` or `attachable:` network, a driver that bridges to a segment
  compose did not create, a second key whose `name:` resolves to the same real network, a `${VAR}`
  mozza does not expand, and a `docker-compose.override.yml` that `docker compose up` loads without
  being asked. Every one missed produced a policy that silently black-holes a service while the
  deploy reports success, and the last two cannot be closed at all: mozza expands `${VAR}` in no
  field anywhere, and the server import endpoint is handed compose *content* with no path to look
  beside. Naming what a file declares is true however incomplete that file is; claiming those are all
  of them is what could not be. The boundary is still the user's to have — one line, having been told
  what it should say.
- **Where the declared members may not be all of them, the warning says so before recommending a
  list**, naming what brings the others in: `extends`, `include`, `network_mode` on any value but
  `none`, `external_links`, `external:` (in all three spellings `docker compose config` accepts —
  `true`, `"true"`, and the legacy `{name: legacy_net}`), `attachable:`, a driver other than `bridge`
  or `overlay`, a network's own `name:`, and a service that also joins the default network. Where one
  of a service's networks is in that state, its own instruction repeats the caveat rather than
  leaving it a network away. A service whose every network is named by a `${VAR}` gets no directive at
  all, since a list drawn from a network mozza cannot name is not one to follow. A service that
  publishes a port is offered no directive either, and told why: it is reachable from outside the
  cluster, and a policy under it would deny the ingress that serves it and take a working site dark
  with nothing to read anywhere — said out loud for that service, because one passed over in silence
  is one the reader has to notice the absence of. Two limits are named
  rather than implied. A container that reconfigures its own networking at run time, under
  `privileged: true` or `cap_add: [NET_ADMIN]`, is past what any reading of a compose file can
  account for. And `links:` between services in this file is not treated as reaching: `docker compose
  config` renders a link across two networks that do not touch without adding a network to either, so
  a link is an alias on a network the two already share rather than a path between them.
- **Compose's default network still never becomes a boundary**, however it is spelled — a service
  that writes `networks: [default]` lands exactly where one that writes nothing lands, so it is a
  request for no isolation rather than something to carry. It needs no warning of its own now: each
  network reports what it keeps out, and the services left on the default network are part of that. A
  compose file that declares no networks, and a file with a single service, import byte for byte as
  they did before and say nothing about networks at all.
- **A storage size that was not a quantity passed `mozza validate` and then killed `mozza compile`
  with a Go stack trace.** `each copy needs its own storage of BIGGISH` validated cleanly, reported
  the recipe valid, and then panicked inside `resource.MustParse` — which panics rather than
  returning an error — printing the raw apimachinery regular expression and a goroutine dump at
  someone who had written one wrong word. Three fields were never checked at all: a stateful slice's
  per-copy storage, a database's and a cache's, so `postgres 16, 500K, daily backups` did the same
  thing (only a lowercase `k` is an SI suffix). Every field that becomes a Kubernetes quantity —
  those three plus the CPU and memory limits — is now checked by `mozza validate` against
  `resource.ParseQuantity`, the same rule a cluster applies, so the panic is unreachable from any
  recipe that validates. The value is named and the accepted form explained in words rather than as
  a regular expression; a size still carrying an unrendered `{{STORAGE_SIZE}}` is told that a
  template variable was never substituted, instead of being handed a number-format complaint about a
  value that was never meant to survive rendering; and a negative size is rejected, where
  `postgres 16, -1Gi` used to validate and compile to `storage: -1Gi`. The rule the CPU and memory
  limits were held to was a hand-written regular expression that had drifted from Kubernetes in both
  directions — it rejected `2G`, `1e3`, `500k` and `1Ei`, which Kubernetes accepts — and it was also
  the only thing keeping values the local target cannot render out of a generated
  `docker-compose.yml`, so both quantity converters were brought onto the same rule in the same
  change. The local target's memory limit is now the number Kubernetes means wherever the two used to
  read the same string differently — a suffix Compose spells differently (`limit memory to 500m` was
  500 MiB to Compose and half a byte to Kubernetes), a suffix it does not accept at all
  (`limit memory to 1Ti` reached it verbatim and was refused with `invalid suffix`, the converter
  having no `Ti` mapping), or a size that is not a whole number of bytes, which Compose truncated and
  Kubernetes rounds up (`1.5` was 1 byte and is now 2; `1.1Mi` was 1153433 and is now 1153434).
  Compose's `K`/`M`/`G` are 1024-based, so the existing `Mi`→`M` rename was already byte-exact and
  stays so: `256Mi` still compiles to `256M`, which Compose reads as 268435456
- **Three bundled recipes could not be deployed at all.** `ghost`, `gitea` and `prometheus`
  installed cleanly and then failed to compile, because env-key validation was narrower than
  Kubernetes' own rule and rejected keys as ordinary as Ghost's `database__connection__user`. Every
  recipe in the catalog is now walked through parse, build and validate by `go test`, so a bundled
  recipe that cannot be compiled fails the build rather than the user's first deploy
- **Every HTTP health check compiled to port 80.** A `health check /path` directive names a path but
  no port, and the Kubernetes backend filled that gap with a literal `80` instead of the port the
  slice listens on — so `ghost` probed 80 while serving 2368, `gitea` 3000, `mattermost` 8065 and
  `prometheus` 9090. Liveness and readiness were both affected, so those four recipes rolled out pods
  that could never become ready and hung there. A probe now inherits the slice's port, including one
  declared as a named `ports` entry, and both backends resolve it by the same rule. A check that
  names no port — or nothing to check at all — is now rejected by `mozza validate` rather than
  compiled into a probe aimed at a guess
- **The local target replaced a recipe's probe timings with its own and dropped every check but one,
  without a word.** `health check /healthz every 30s, timeout 9s, wait 25s before starting` compiled
  to `interval: 10s`, `timeout: 5s` and no `start_period` — numbers the author never wrote — and a
  liveness, startup, TCP or command check was discarded outright, so a recipe with three probes
  compiled to one, exited 0 and printed nothing, while the README told the reader that no warning
  means nothing was lost. Compose allows one health check per service, so the local target now emits
  the first readiness, liveness or startup probe Compose can actually run — an HTTP or command check
  of a lesser kind is preferred over a TCP check of a senior one, which Compose cannot run at all
  since the daemon executes a command inside the container rather than opening a socket — and names
  every probe it could not carry in a warning. A check's interval, timeout and initial delay are now
  carried; `start_period` is Compose's closest equivalent to an initial delay rather than the same
  thing, because it keeps probing throughout the window and merely declines to count the failures
  toward `retries`. The 10s/5s/3 numbers survive as the documented defaults for a recipe that states
  no timings. Compose reports a container unhealthy but never restarts it, so a promoted liveness
  check does less locally than on Kubernetes and the warning says so. One deliberate behaviour change
  rides along: for a slice declaring two readiness probes the emitted check now comes from the last
  rather than the first, matching what Kubernetes already compiles. No bundled recipe or example
  declares two readiness probes, so none of them moves
- `mozza import` no longer writes a health check with no port to check. Rejecting unaimed checks
  would otherwise have turned a working import into a dead end: a Compose service with a healthcheck
  but no published `ports:` imported cleanly and then failed to compile. The importer now takes the
  port from the healthcheck URL when Compose publishes none, so a `curl` check on
  `http://localhost:3000/health` yields an `on port 3000` line. Where no port exists anywhere, the
  check is dropped rather than written — reported at import time and recorded as a comment in the
  recipe, both naming what to add to restore it
- **`mozza import` wrote a recipe that mozza itself could not read.** A Compose file with a named
  volume imported to a recipe carrying a `storage 10Gi` line — not a directive the recipe language
  has — so `mozza validate` answered `unknown directive "storage"` and the on-ramp from Compose ended
  at a parse error on the importer's own output. The line was wrong to emit at all rather than merely
  misspelled: a storage size rides on the engine shorthand, as in `postgres 16, 10Gi`, and that
  shorthand is the only thing in the language that sets one, so on a service with no database or
  cache engine there was no size for the compiler to read and it discarded it. A volume the recipe
  cannot carry is now named instead of mistranslated — in a comment beside the slice and on the
  terminal as the import runs — along with what to write to keep it: `each copy needs its own storage
  of 10Gi` deploys the service as a StatefulSet with a volume per copy. The advice also says where
  that volume arrives, because a slice with no engine gives the compiler no path of its own to use
  and the volume is mounted at `/data` whatever path Compose asked for. An engine slice is given one
  volume at its engine's own path regardless of what Compose called it, so a second named volume used
  to vanish in silence — a `postgres` service with a data volume and a backup volume imported to a
  recipe mentioning only the first — and it is now named too. Lastly, an access mode on a volume —
  `:ro`, `:rw`, `:z` and the rest — was being read as part of the mount path, so
  `dbdata:/var/lib/mysql:ro` was recorded as a mount at `/var/lib/mysql:ro`; the mode is now
  separated from the path it qualifies. The same misreading would also have made the new warnings
  fire on volumes that are carried after all, but those warnings and the misreading never met in a
  release — what the released version does with an access mode is carry it into the comment, and
  nothing more
- **A Compose file that `docker compose config` accepts could still stop `mozza import` dead, giving
  the name of a Go type as the reason.** Compose documents two shapes for several fields and mozza
  modelled one of each, so a file written the other way produced no recipe at all and, in its place,
  a line ending `into importer.composeSecretRef` — a type that exists only inside mozza and appears
  nowhere in a Compose file. The shapes that used to abort now import: a secret listed by name under
  a service's `secrets:` rather than spelled out as a mapping; the long forms of `volumes:` and
  `ports:`, which give `source` and `target`, or `target` and `published`, as separate keys; a
  `healthcheck.test` written as one shell command string rather than a list of arguments; a
  `deploy.replicas` written as `"3"` rather than `3`; a network's `internal:` written as `"true"`
  rather than `true`; and a service naming a config under `configs:`, which now imports and is
  reported dropped rather than stopping the file. Each is rewritten, as the file is read, into the
  one shape the importer already handled, so a long-form port and the equivalent `"8080:80"` produce
  the same recipe line and nothing further along the import has learned a second shape. YAML anchors
  and aliases earn their own mention, because sharing a block between services — `<<: *base` — is
  how a Compose file of any size avoids repeating itself, and mozza now follows an alias to the
  value it stands for everywhere it reads the file's structure directly: the port list, the mount
  list, the replica count and a network's `internal` flag. An aliased value is read as the value
  rather than as the name of the anchor, and two services sharing one block import as two slices
  instead of quoting that block's own lines back as errors, once for every service that used it.
  Where mozza genuinely cannot read something the failure is now a sentence, naming the compose
  field by the dotted path `docker compose config` prints for the same fault — `services.web.ports`,
  `networks.backend.internal` — the line the value sits on, and what was expected there: `compose
  services.web.ports (line 5): expected a list of ports such as "8080:80", or a list of entries with
  target and published`. A key given twice says which line the first one was on. Two things are
  worth stating rather than glossing. The top-level `configs:` warning changed for files that
  already imported: it used to say a config was `mapped as environment reference — verify mount
  path`, which was untrue — nothing mapped it, nothing mounted it, and the recipe it described
  contained neither — so it now says the config was dropped and names `mount config "<path>" at
  <target>` as the line to add to the slice that needs it, as a warning rather than an informational
  note, matching every other thing the importer drops. And mozza still does not expand `${VAR}` in a
  Compose file: `deploy.replicas: ${REPLICAS}` now fails with a sentence saying so instead of a YAML
  type error, but it still fails, and `image: ${IMAGE}` still reaches the recipe with the `${IMAGE}`
  in it
- **The CLI now writes to stdout.** Every command's output went to stderr and stdout was always
  empty, so `mozza targets | grep local` matched nothing and `mozza deploy --export > list.txt`
  wrote an empty file. Cobra's `Print*` helpers resolve through `OutOrStderr`, which walks the
  command tree for a writer and falls back to stderr when it finds none — and nothing ever set one.
  Streams are now split by intent: payload on stdout (the `targets` and `status` tables, `logs`
  output), progress and diagnostics on stderr (`compile`'s `wrote …` lines and summary, capability
  warnings, the `--export` notice). `mozza compile > files.txt` stays empty by design — for a
  command whose artifacts are files on disk, the chatter is not the output
- Banner and how-it-works graphics no longer describe Mozza as Kubernetes-only, matching the
  compiler reframe in the README
- Corrected stale README claims: the template catalog holds 15 recipes (documented as 10), and the
  minimum Go version is 1.25 (documented as 1.24, while `go.mod` requires 1.25.0)

## [1.0.0] — 2026-03-19

### Added

- 19 CLI commands: init, up, down, deploy, status, logs, rollback, promote, validate, doctor, serve, operator, import, recipe (search/info/install/deploy/update), connect/disconnect, previews, proxy, version
- Recipe language (.mozza files) with parser supporting full Kubernetes workload types
- Kubernetes compiler: Deployment, StatefulSet, DaemonSet, CronJob, Job, Service, Ingress, PVC, RBAC, HPA, PDB, NetworkPolicy
- Local compiler: Docker Compose generation
- Full deploy pipeline with image pre-validation, readiness polling, auto-rollback
- Web dashboard with 20 pages: overview, applications, deployments, environments, monitoring, marketplace, recipe builder, doctor, clusters, teams, profiles
- Deploy wizard with template catalog, GitHub import, Docker Compose import, framework detection
- Recipe review step — preview compiled manifests before deploying
- Order tracking with pizza-themed deployment status
- Auth with session cookies, bcrypt passwords, 3-role RBAC (viewer/deployer/admin)
- Team-based access control with image aliasing
- 15-recipe marketplace (WordPress, Ghost, Gitea, Uptime Kuma, etc.)
- Git deploy with GitHub webhooks and branch preview deployments
- Real-time monitoring with per-app time-series metrics
- Doctor with 11 diagnostic rules and auto-fix
- Reverse proxy with CertMagic auto-TLS, health-checked routing
- Framework auto-detection (Next.js, Django, Rails, Laravel, Go)
- Docker Compose, Helm, Dockerfile importers
- Environment promotion pipeline (dev → staging → production)

### Infrastructure

- SQLite + PostgreSQL dual-driver database
- AES-GCM encryption for stored secrets
- Prometheus metrics endpoint (opt-in)
- Embedded React UI via go:embed
- TLS support (cert files or self-signed)
