# Mozza — Vision & How It Works

**Status:** Living document
**Last updated:** 2026-07-28

---

## The one-sentence version

**Mozza is a deployment compiler.** You describe your app once, in plain English. Mozza either
hands you the file your platform needs — Helm chart, Kubernetes manifests, Docker Compose, Nomad
job — or deploys it for you.

Deploying apps as easy as ordering a pizza. You get it delivered, or you pick it up.

---

## The problem

To ship an app today you must know a format, and the format depends on where you're shipping:

| Where you deploy | What you have to learn |
|---|---|
| Kubernetes | Deployments, Services, Ingress, PVCs, probes, RBAC — ~200 lines of YAML per app |
| Kubernetes, at scale | Helm — the above, plus Go templating, `values.yaml`, chart structure |
| Nomad | HCL job specs, groups, tasks, Consul service stanzas |
| A single box | Docker Compose |

These formats describe **the same handful of ideas**: run this image, expose this port, give it a
disk, connect it to that database, keep two copies alive. The concepts are shared. Only the syntax
differs — and the syntax is where all the effort goes.

Worse, the choice is sticky. Learn Helm and you've married Kubernetes. Write Compose and you can't
scale out. Every format is a one-way door.

## The idea

Separate **what you want** from **how it's expressed.**

```
                    ┌──────────────────┐
   app.mozza  ───▶  │  recipe → plan   │  ──▶  Kubernetes manifests
   (plain English)  │   (the compiler) │  ──▶  Helm chart + values.yaml
                    └──────────────────┘  ──▶  Docker Compose
                                          ──▶  Nomad job spec
                                          ──▶  …
```

One description. Any target. This is how compilers have worked for forty years: one frontend, one
intermediate representation, many backends. Deployment formats never got the same treatment.

**Mozza's `.mozza` recipe is the source. Everything else is a build artifact.**

That last sentence is the whole product. It means:

- A **10-line recipe** goes in your pull request. A human reads it and approves it.
- The **200 lines of YAML** become compiler output. Nobody reviews compiled output.
- **Switching platforms** is changing a flag, not rewriting your infrastructure.
- **You are never locked in** — including to mozza. Mozza emits standard files that work without it.

## Two paths

Everything mozza does resolves into one of two paths. They share the same compiler.

### Path B — "Pick it up" (you get the file)

For engineers who already have a platform, a pipeline, and opinions.

```
mozza compile --target helm
```

You get a chart. You commit it, PR it, and your existing CI deploys it. Mozza never touches your
cluster, never needs credentials, never runs as a service. It's a compiler you invoke, like `tsc`
or `gcc`.

**Why this matters:** it costs nothing to try. One binary, no trust, no install-a-platform step.
And it's the path that proves the compiler is honest — you can read exactly what it produced.

### Path A — "Get it delivered" (mozza deploys it)

For anyone who doesn't want a file at all.

Open the dashboard, pick an app, answer three questions, press **Place Order**. Mozza compiles,
applies, watches it come up, and hands you a working URL and connection details.

**Why this matters:** this is the pizza. No YAML, no kubectl, no jargon. But it only earns trust
because Path B exists to show its work.

---

## How the app works, step by step

### Path B — Pick it up

**1. Start a project**

```bash
mozza init my-app
```

Creates `app.mozza`. Or point mozza at something that already exists:

```bash
mozza import              # Compose file, Dockerfile, Helm values, or a GitHub repo
```

**2. Describe the app**

```
App: my-app

Web:
  from image myorg/web:1.0.0
  open to the public on port 3000
  health check /ready
  run 2 copies
  needs db

Db:
  postgres 16
  20Gi storage
  daily backups
```

Ten lines. Readable by someone who has never seen Kubernetes.

**3. Check what your target supports**

```bash
mozza targets
```

Prints every target and how completely it can express your recipe. Not every idea exists everywhere
— Nomad has no NetworkPolicy, Compose has no RBAC. Mozza tells you *before* you compile, not after
something silently vanished.

**4. Compile**

```bash
mozza compile --target helm -o ./deploy
```

You get real files plus a capability report:

```
✓ Wrote 6 files to ./deploy
  Chart.yaml, values.yaml, templates/deployment.yaml, ...

  2 notes:
  · `daily backups` → emitted as a CronJob
  · `network policy` → not supported by this target, omitted
```

**5. Review and ship**

The generated files go in your repo. The `.mozza` file is what reviewers read. Your existing
pipeline does the rest. Mozza's involvement ends here.

**6. (Optional) Let mozza deploy after all**

```bash
mozza up          # local Docker Compose
mozza deploy      # your Kubernetes cluster
```

Same recipe, same compiler. Only the last step differs.

### Path A — Get it delivered

**1. Open the dashboard**

`mozza serve`, then open the browser. Click **Deploy**.

**2. "What do you want to run?"**

Three ways to answer, all in one step:

- **Pick from the menu** — your saved apps, by friendly name
- **Choose a classic** — WordPress, Ghost, Gitea, Supabase, and the rest of the catalog
- **I have a repo** — paste a GitHub URL; mozza reads the code and infers the recipe

**3. "How do you want it?"**

One screen. App name, where to deploy, and a couple of cards — *Website*, *API + Database*,
*Background Worker*. Toggles for "add a database," "add a cache."

Ports, replica counts, health check paths, CPU limits, storage notation — never shown. Sensible
defaults, hidden behind **Customize more** for anyone who wants them.

Passwords are **generated**, not requested. You are never asked to invent a database password.

**4. "Review & order"**

A receipt. What's being deployed, where, and what it'll cost you in resources. The generated recipe
is there in a collapsed section if you want to look — most people won't.

Press **Place Order**.

**5. Order tracking**

```
Taking your order → Prepping ingredients → In the oven → Quality check → Ready for pickup!
```

Real progress against real stages, with an order number. Kitchen logs are one click away for anyone
who wants them. If something fails, you get a plain-English reason and a suggested fix — not a
stack trace.

**6. Pickup**

Your app's URL, big and clickable. Plus connection details for anything that has them — database
strings, admin tokens, API keys — generated and captured during deploy, presented ready to copy.

---

## What mozza is not

- **Not a hosting provider.** Mozza deploys to *your* cluster or *your* box. There's no mozza cloud.
- **Not a lock-in.** It emits standard files. Delete mozza and your deployment keeps running.
- **Not a Kubernetes replacement.** It's a better way to *write* Kubernetes, and a way to write
  the other formats too.
- **Not a YAML editor with extra steps.** If you find yourself needing to express something the
  recipe can't say, the escape hatch is writing that format directly — not a 40-field form.

---

## Design principles

1. **The recipe is source; everything else is output.** Reviewability is the moat. Any feature that
   makes the recipe less readable is suspect.
2. **A parsed keyword is a promise.** If the DSL accepts a directive, it must produce correct
   behavior on every target that claims to support it — or say plainly that it doesn't. A keyword
   that parses and silently does nothing is a bug with a delay fuse.
3. **Kubernetes is the reference semantics.** Other targets are honest projections from it. Mozza
   does not pretend all targets are equal, and it does not shrink the language to what they share.
4. **Two paths, one compiler.** Every capability must work in both. No feature exists only inside
   the dashboard.
5. **Plain English on the surface, full power underneath.** The recipe never says "StatefulSet."
   The compiler knows when to emit one.
6. **Progressive disclosure, not progressive dumbing-down.** Simple apps stay simple. Hard apps
   remain possible.
7. **Never lose configuration silently.** If a target can't express something, warn loudly and name
   what was dropped.

---

## Where this is going

See `docs/specs/multi-target-compiler.md` for the milestone plan.

Short version: the compiler architecture already exists and already has two backends. The next
milestones name it, prove it's honest, and add Helm and Nomad. Then the pizza path gets built on
top of a compiler people can already trust.
