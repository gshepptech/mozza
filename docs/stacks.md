# Stacks: local environments for apps that ship their own charts

A recipe is for an app Mozza writes the Kubernetes manifests for. A **stack** is for an app that
already ships Helm charts. In that case the work is not compiling. It is building the environment
those charts expect: a cluster, the operators and CNI they assume, images built from the app's
source, the Secrets the app will not start without, and then the charts themselves.

```bash
mozza stack list                 # stacks and addons Mozza knows
mozza stack plan inventory        # every command `up` would run, nothing executed
mozza stack up inventory          # cluster, images, secrets, addons, charts
mozza stack forward inventory     # reach it on 127.0.0.1 until Ctrl-C
mozza stack secrets inventory     # admin passwords and other values meant for people
mozza stack status inventory      # nodes, Helm releases, pods
mozza stack down inventory        # delete the cluster (only if `up` created it)
```

## Your own app: start with `init`

You do not write a stack from scratch. From the root of your app's repository:

```bash
mozza stack init          # scan the repo, write mozza-stack.yaml
mozza stack plan          # every command up will run, nothing executed
mozza stack up            # bring it up
mozza stack forward       # open it on 127.0.0.1
```

With no stack named, every command uses `./mozza-stack.yaml`. Commit that file, and anyone with
the repository runs `mozza stack up` instead of a folder of scripts.

`init` reads only what the repository declares:

| It finds | It drafts |
|---|---|
| `Chart.yaml` | The Helm component. A repository with several charts gets the one named after the repository; the others are listed for review. |
| `values-*.yaml` | The overlays whose names say local, dev or demo, layered in that order. Production and test overlays are never used. |
| Dockerfiles | A build for each image the chart names, matched by name. The chart's top-level `image` is built by the root Dockerfile. |
| Empty credentials in values | A generated secret for each one the chart cannot do without. The chart's comment says the app fails without it, or `helm template` refuses to render until it is set. Names of existing Secrets are never generated. |
| The rendered manifests | Add-ons for the objects they create (CloudNativePG, cert-manager, Calico), the namespace a chart hardcodes, and the Service to forward. |
| A specific `storageClass` | `${storageClass}`, so the volume binds on kind or k3d. |
| Playbooks | Commented-out Ansible components, with their host groups mapped onto local targets. |

Anything it cannot settle is written as a review list at the top of the file. Examples are an
image from a private registry that no Dockerfile builds, a second chart, or a render that still
fails. The draft always loads, and `init` checks that before writing it.

What `init` cannot know are an app's quirks, such as a container that needs an extra command-line
argument, or a Dockerfile that no longer builds. Those show up on the first `up`, and the fixes
belong in the stack file. See [When an app's own build is broken](#when-an-apps-own-build-is-broken).

A repository with only a `docker-compose.yml` is a recipe's job: `init` points at `mozza import`.

## Local only

Stacks never leave this machine:

- Clusters are kind or k3d. Their API server binds `127.0.0.1`, and so does every `cluster.ports` mapping.
- Mozza refuses to run against a kubeconfig whose server is not a loopback address.
- `forward` listens on `127.0.0.1` only.
- Mozza writes each stack's kubeconfig to `~/.mozza/run/<stack>/kubeconfig`. Your default
  kubeconfig and current context are left alone.

There is no cloud provider and no option to add one.

## What `up` does

Each step is safe to repeat. Re-running `up` after a failure picks up where the last run stopped
instead of starting over.

1. **Images.** Mozza builds each image with docker from the source tree, or pulls it. It skips
   images docker already has unless you pass `--rebuild`. `--image REF=IMAGE` uses an image you
   already have (or can pull) instead of building one.
   - Mozza saves each image for its own platform. If that differs from the nodes', it warns. kind
     nodes on Apple silicon cannot run amd64 images.
2. **Cluster.** Mozza creates the cluster, or reuses one with the same name. Addons can change how
   the cluster is created. For example, the `calico` addon turns off the default CNI and sets the
   pod subnet.
3. **Load.** Mozza saves each image for the node platform and imports it into every node. The
   cluster never needs to reach a registry for app images.
4. **Secrets.** Mozza generates the values, applies them through stdin (never on a command line),
   and stores them in `~/.mozza/run/<stack>/secrets.json` with mode 0600. Later runs reuse the
   stored values. An app that encrypted data with a generated key keeps working.
5. **Components.** Components install in order. Each is a catalog addon or a Helm release plus
   manifests, followed by readiness waits.
   - A release that an interrupted run left `pending-install` is removed first. One stuck
     `pending-upgrade` is rolled back first. Without this, Helm would refuse every later run.
   - A manifest can wait for a CRD before it applies (`waitFor`). Use this for objects whose CRD
     the app's own workload registers once it runs.
6. **Restarts.** When an image is rebuilt under an unchanged tag, the pod spec does not change.
   Mozza records each loaded image's ID and restarts the Deployments, StatefulSets and DaemonSets
   that run any image whose ID changed.

Mozza writes nothing into the app's source tree. A chart with dependencies is copied into the state
directory before `helm dependency build`. A build context that needs trimming is streamed to docker
with the extra excludes applied, so Mozza never writes a `.dockerignore` into the tree.

## Flags

| Flag | Commands | What it does |
|---|---|---|
| `--source PATH` | `up`, `plan` | Build from a different checkout. |
| `--rebuild` | `up` | Build images even if docker already has them. |
| `--image REF=IMAGE` | `up`, `plan` | Use an existing image for REF. Repeatable. IMAGE is pulled if docker lacks it. |
| `--provider kind\|k3d` | all but `list` | Run on the other provider. Commands that act on a running stack reuse the provider its last `up` used. |
| `--forward` | `up` | Forward the stack's services as soon as it is up. |
| `--purge` | `down` | Also delete the stack's state, generated secrets included. |

## Where the source lives

A stack names a default `source.path`. To use a different checkout, either:

- pass `--source ~/src/inventory-feature`, or
- set `MOZZA_<STACK>_SOURCE` (for example `MOZZA_INVENTORY_SOURCE`).

`status`, `forward`, `secrets` and `down` work without the source tree.

## Writing a stack

Stacks are YAML. Mozza looks in `~/.mozza/stacks/<name>.yaml` first, then in the built-in catalog
(`internal/stack/catalog/stacks`). You can also pass a path. Unknown keys are errors.

```yaml
name: myapp                      # lowercase, digits, dashes
description: One line for `mozza stack list`
source:
  path: ~/src/myapp              # relative paths below resolve against this

cluster:
  provider: kind                 # kind | k3d
  nodeImage: kindest/node:v1.33.1  # optional
  workers: 0
  # podSubnet / serviceSubnet: derived from the stack name unless set here
  # or by an addon, so stacks never share the provider's default ranges
  nodeLabels: {lab-pool: "true"}
  ports:                         # published on 127.0.0.1 only
    - {host: 18443, node: 30443}

images:
  - ref: myapp:local             # what the chart is told to run
    build:
      context: .
      dockerfile: deploy/Dockerfile
      target: runtime
      args: {VERSION: dev}
      exclude: ["**/node_modules"]   # on top of the tree's .dockerignore
      rewrite:                       # edits the copy of the Dockerfile sent to docker, never the file
        - why: unpin package releases the mirror no longer serves
          pattern: '(\s[a-z0-9][a-z0-9._+-]*)=[0-9][0-9A-Za-z._+-]*-r[0-9]+'
          replace: '$1'
  - ref: ui:local
    build:
      context: https://github.com/org/ui.git#v1.2.3   # docker clones it; no local checkout needed
  - ref: postgres:16-alpine      # no build: pulled, then loaded

secrets:
  - name: myapp-keys
    namespace: myapp
    data:
      jwt: {generate: hex, length: 32}      # hex | base64 | password | age-key
      admin-password: {generate: password, show: true}
      username: {value: admin}
      token: {fromEnv: MYAPP_TOKEN}         # read at `up`, never stored

components:
  - use: cnpg                    # a catalog addon
    version: 0.29.1              # optional pin
  - name: myapp                  # Helm release name
    namespace: myapp
    manifests:                   # applied before the chart
      - path: deploy/crds.yaml
    helm:
      chart: deploy/helm/myapp   # local path, repo chart (with repo:), or oci://
      valuesFiles: [deploy/helm/myapp/values-dev.yaml]
      timeout: 10m
    values:                      # merged last, like a final -f
      image: {repository: myapp, tag: local, pullPolicy: Never}
      auth:
        jwt: ${secret:myapp/myapp-keys/jwt}   # for charts that want a value, not a Secret
    wait:
      - {resource: deployment/myapp, namespace: myapp, rollout: true}

access:
  - {name: myapp, namespace: myapp, service: myapp, port: 8080, localPort: 18080, health: /healthz}

notes:
  - Sign in as admin with the password from `mozza stack secrets myapp`.
```

Variables:

| Variable | Expands to |
|---|---|
| `${source}` | The source tree. |
| `${stack}`, `${cluster}`, `${context}` | The stack name, cluster name and kube context. |
| `${storageClass}` | The provider's default StorageClass (`standard` on kind, `local-path` on k3d). |
| `${access.<name>.url}` | That access entry's origin, `http://127.0.0.1:<localPort>`. Use it for apps that check the origin a browser calls from. |
| `${env.NAME}` | An environment variable. |
| `${secret:namespace/name/key}` | In Helm values, a value from the stack's secrets, for charts that take a credential as a value. |

An unknown variable is an error.

## Ansible

A component can run a playbook from the app's tree. It runs after the component's manifests and
chart:

```yaml
components:
  - name: configure
    ansible:
      playbook: ansible/site.yml          # relative to the source tree
      dir: ansible                        # where it runs, so ansible.cfg, roles/ and group_vars/ are found
      requirements: ansible/requirements.yml   # optional galaxy requirements
      groups:                             # the playbook's group names -> local targets
        k8s-servers: control_plane
        k8s-workers: workers
        jumphost: runner
      extraVars: {admin_password: "${secret:myapp/keys/admin}"}
      env: {ANSIBLE_STDOUT_CALLBACK: yaml} # ANSIBLE_* overrides the tree's ansible.cfg
      tags: [setup]
      limit: nodes
```

- **Where it runs.** Playbooks run in a container Mozza builds once, with ansible-core 2.21 and the
  `community.docker`, `kubernetes.core`, `ansible.posix` and `community.general` collections.
  Nothing is installed on your machine.
- **What it can reach.** Mozza writes the inventory, and a playbook's own inventory is never used.
  It names only local targets:

  | Target | What it is |
  |---|---|
  | `control_plane`, `workers`, `nodes` | The cluster's node containers, reached through the Docker API. kind nodes are Debian with `python3`; k3d nodes have no Python. |
  | `runner` | The container the playbook runs in. Use it for tasks against the cluster API: `KUBECONFIG` points at the stack's cluster. |

  `localhost` in a playbook is the runner container, not your machine.
- **Playbooks written for VMs** run unchanged where their tasks make sense on a container:
  - Targets connect as root with `become` turned off, which outranks a play's `become: true`.
  - The tree's `remote_user` is overridden.
  - `ask_vault_pass` is switched off, since there is no terminal to answer it.
- **Nothing is written to the tree.** The source tree is mounted read-only. Inventory, extra
  vars, the in-network kubeconfig and installed requirements live in
  `~/.mozza/run/<stack>/ansible/<component>/`.

`examples/stacks/ansible-demo.yaml` runs a playbook against both kinds of target:

```bash
mozza stack up examples/stacks/ansible-demo.yaml
```

## Addons

Addons are the reusable pieces: a CNI, an operator, an ingress controller. A stack refers to one
with `use: <name>`, can pin `version`, and can add `values`. Add your own in
`~/.mozza/addons/<name>.yaml`. An addon there with the same name as a built-in one replaces it.

| Addon | What it installs |
|---|---|
| `calico` | Calico through the Tigera operator. Turns off the cluster's default CNI and sets the pod subnet. |
| `cnpg` | The CloudNativePG operator. |
| `ingress-nginx` | ingress-nginx as NodePort 30080/30443. Pair it with `cluster.ports`. |
| `cert-manager` | cert-manager with CRDs. |
| `metrics-server` | metrics-server with kubelet TLS verification relaxed for kind/k3d. |

An addon is a component with a default `version`. Inside it, `${version}` expands to that version
or to the stack's pin. An addon can declare `needs` (`disableDefaultCNI`, `podSubnet`). Mozza folds
those into the cluster before creating it. If two components need different subnets, `up` stops
with an error.

## Built-in stacks

| Stack | Cluster | Notes |
|---|---|---|
| `inventory` | kind + Calico | Server plus pod labs. The lab provider requires Calico's operator status. SQLite and Mailpit come from the chart's demo and dev overlays. |
| `storefront` | k3d | Server, operator, web UI and PostgreSQL. Generates the KEK, master key and database passwords. Provisioning real clusters still needs cloud credentials. |
| `webapp` | kind + ingress-nginx on 127.0.0.1:80 | Every backend service is built from the controller fork, and the admin UI from its git tag, because the published images are amd64-only. The UIs are at `learn.hf.localhost` and `admin.hf.localhost`; the admin signs in as `admin` / `Sup3r@dmin`. Scenario VMs still need a cloud provider. |

## When an app's own build is broken

A stack can carry a small, named workaround. The app's tree stays untouched:

- **`rewrite`** edits the streamed copy of a Dockerfile. The `inventory` stack uses it to unpin
  Alpine package releases the live mirror has replaced. The `storefront` stack uses it to copy a
  schema file that the web bundle imports from outside `web/`.
- **`exclude`** trims a build context. The `storefront` stack uses it to keep host-built binaries
  and the macOS `node_modules` out of the context, since that tree has no `.dockerignore`.
- **`--image`** skips the build entirely in favour of an image you already have.
- **A git `context`** builds an image whose source is not checked out. The `webapp` stack builds
  its admin UI this way.

Each rewrite reports how many lines it changed, or that it matched nothing. When the app fixes
its build, the workaround shows up in the log as a no-op and can be deleted.

## Known host issues

- **Default cluster ranges already in use.** On a Docker host that already runs other clusters,
  in-cluster traffic to the provider's default Service range can reach a different API server.
  Every system pod then fails TLS with `x509: certificate signed by unknown authority`. Mozza
  derives distinct pod and Service ranges per stack to avoid this. Pin them with
  `cluster.podSubnet` and `cluster.serviceSubnet` if they collide with something.
- **k3d older than v5.9 with Docker Engine 29.** k3d v5.6.3 fails with "stopped returning log
  lines" before the cluster starts. Upgrade k3d, or pass `--provider kind`.
