# Budget App

A budget-tracking app for a two-person household: personal ledgers per user, plus
a shared ledger for mutual expenses. Built as a Kubernetes/cloud-native course
project — the priority is demonstrating the required architecture cleanly, not
feature completeness.

## Architecture

```
                  ┌──────────────┐
                  │  Mobile app   │  Expo / React Native
                  └──────┬───────┘
                         │  HTTP, JWT bearer token
                         ▼
┌─────────────┐        JWT (RS256)        ┌───────────────┐
│ auth service │──────────verified by────▶│ budget service │
│    (Go)      │        public key         │      (Go)      │
└──────┬───────┘        (no network call)   └───────┬────────┘
       │                                            │
       │           schema: auth.*                  │  schema: budget.*
       └────────────────┬───────────────────────────┘
                         ▼
                  ┌─────────────┐
                  │  PostgreSQL  │
                  └─────────────┘

┌────────────┐   scrapes GET /metrics
│ Prometheus │ ──────────────────────▶  auth, budget
└────────────┘
```

- **auth service** — signup, login, issues JWTs. Owns `auth.households` and
  `auth.users`.
- **budget service** — categories and transactions (personal + shared ledgers).
  Owns `budget.categories` and `budget.transactions`. Never queries the auth
  service's tables or database directly.
- **PostgreSQL** — one instance, two schemas (`auth`, `budget`). Kept as a
  single instance instead of two databases/containers to stay within the
  course's time budget; the schema split still keeps each service's tables
  isolated.
- **Mobile app** — the only client in this project; see
  [Mobile app](#mobile-app) below.
- **Prometheus** — the CNCF component; see
  [CNCF Landscape technology](#cncf-landscape-technology-prometheus) below.

Both services are separate Go modules under `services/auth` and
`services/budget`, each with its own `Dockerfile`, `go.mod`, and embedded
`schema.sql` that is applied automatically on startup (no separate migration
tool — reasonable at this scale, would need revisiting for a real production
system with concurrent deployments).

## Architecture decision: RS256 JWTs, verified locally

The auth service signs JWTs with an RSA private key (RS256). The budget
service verifies incoming tokens using only the corresponding **public** key —
it never calls the auth service to validate a request. This means:

- The budget service can verify auth tokens even if the auth service is
  temporarily down or slow.
- No per-request network hop between the two services just to check "is this
  token valid" — verification is a local, in-process signature check.
- The budget service only ever needs the public key (it cannot mint tokens),
  which limits the blast radius if that key were ever exposed.

The trade-off: token revocation isn't instant. If a user's token needs to be
invalidated before it expires, the budget service has no way to know — there's
no revocation list. Given the course's token lifetime (24h) and scope, this
was accepted deliberately rather than adding a revocation store this early.

The budget service also trusts `user_id` and `household_id` straight from the
verified JWT claims — it never looks them up from the auth database. This
keeps the two services' data fully decoupled: the budget service's schema
only ever stores opaque IDs, never a foreign key into another service's
tables.

## Data model

**auth schema**
- `households(id, name, created_at)`
- `users(id, household_id, email, password_hash, display_name, created_at)`

**budget schema**
- `categories(id, household_id, name, kind[income|expense], created_at)`
- `transactions(id, household_id, user_id, category_id, scope[personal|shared], amount_cents, currency, description, occurred_at, created_at)`

**Ledger rule** (enforced in the budget service, not the database):
- A `personal` transaction is visible to, and can only be edited/deleted by,
  its owner (`user_id`). Other household members get a `404`, not a `403`, so
  the transaction's existence isn't leaked.
- A `shared` transaction is visible to and can be edited/deleted by **any**
  member of the same household — it's the mutual ledger.

Household membership itself is intentionally simple for this course project:
signing up either creates a new household (`household_name`) or joins an
existing one by its ID (`household_id`), with no invite-code verification.
Acceptable for a two-person household demo; a real product would need an
invite flow.

## 12-Factor App

Documenting only the factors actually applied — not all twelve are relevant
at this project's scope, and the table below says which and why.

| Factor | How it's applied |
|---|---|
| **III. Config** | Both services read all configuration — `PORT`, `DATABASE_URL`, `JWT_PRIVATE_KEY_PATH` / `JWT_PUBLIC_KEY_PATH` — from environment variables. Nothing is hardcoded; the same binary/image runs against local Postgres or a clustered one purely by changing env vars. In Kubernetes, those same env vars are populated from Secrets (`k8s/*-deployment.yaml`) instead of a shell export — the mechanism the code relies on doesn't change between local dev and the cluster, only where the values come from. |
| **IV. Backing services** | Postgres is treated as an attached resource, addressed only via the `DATABASE_URL` env var. Locally that URL points at `localhost:5432`; in Kubernetes it points at the `postgres` Service's DNS name instead — swapping the backing instance requires zero code changes, only a different Secret value. |
| **V. Build, release, run** | Each service has a multi-stage `Dockerfile`: a `build` stage compiles the static Go binary, and a separate `distroless` runtime stage contains only the compiled binary — no Go toolchain, shell, or package manager in the image that runs. Configuration (env vars) is injected at run time, never baked into the image. |
| **VI. Processes** | Both services are stateless — no in-memory session or request state. All persistent state lives in Postgres, so any instance can handle any request and the process can be killed/restarted freely. |
| **VII. Port binding** | Each service is self-contained and binds its own port via `net/http`, configurable through `PORT`. No app server is injected by the runtime environment. |
| **IX. Disposability** | Fast startup (single static Go binary, no runtime dependency resolution) and graceful shutdown: both services listen for `SIGTERM`/`SIGINT` and call `http.Server.Shutdown` with a 10s grace period before exiting, so in-flight requests aren't dropped mid-response when Kubernetes terminates a pod. |
| **X. Dev/prod parity** | The same Docker image built and tested locally (`docker run`) is the one loaded into the kind cluster via `kind load docker-image` — no separate "dev build" vs "prod build" of the application code, only env var differences. |
| **XI. Logs** | Both services log unbuffered to stderr via Go's standard `log` package (`log.Printf`/`log.Fatal`) rather than writing to log files — captured directly by `docker logs` / `kubectl logs`, with no in-app log routing or rotation logic. |

Deliberately not exercised at this project's scope:
- **VIII. Concurrency** — every Deployment runs `replicas: 1`; scaling via the
  process model would apply cleanly (both services are stateless, per Factor
  VI) but was never actually exercised, since a two-person household demo has
  no load to justify it.
- **XII. Admin processes** — no one-off admin/management scripts run in the
  application's own environment; schema setup happens automatically on
  service startup instead of via a separate migration command.

## Running locally (pre-Kubernetes)

Postgres as a local container:

```bash
docker run -d --name budget-postgres \
  -e POSTGRES_USER=budget -e POSTGRES_PASSWORD=devpass -e POSTGRES_DB=budgetapp \
  -p 5432:5432 postgres:16-alpine
```

Auth service:

```bash
cd services/auth
DATABASE_URL='postgres://budget:devpass@localhost:5432/budgetapp' \
  JWT_PRIVATE_KEY_PATH=./keys/private.pem PORT=8081 go run .
```

Budget service (separate terminal):

```bash
cd services/budget
DATABASE_URL='postgres://budget:devpass@localhost:5432/budgetapp' \
  JWT_PUBLIC_KEY_PATH=../auth/keys/public.pem PORT=8082 go run .
```

Generating a dev JWT keypair (only needed once, `services/auth/keys/` is
gitignored):

```bash
mkdir -p services/auth/keys
openssl genrsa -out services/auth/keys/private.pem 2048
openssl rsa -in services/auth/keys/private.pem -pubout -out services/auth/keys/public.pem
```

Example requests:

```bash
# create a household + first user
curl -X POST localhost:8081/signup -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"hunter2","display_name":"Alice","household_name":"Alice & Bob"}'

# log in
curl -X POST localhost:8081/login -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"hunter2"}'

# use the returned token against the budget service
curl localhost:8082/transactions -H "Authorization: Bearer <token>"
```

### Testing with Docker instead of `go run`

Both services also run as containers on a shared Docker network, which is
closer to how they'll talk to each other in Kubernetes (via Service DNS
instead of `localhost`):

```bash
docker build -t budget-app/auth:dev services/auth
docker build -t budget-app/budget:dev services/budget

docker network create budget-net
docker network connect budget-net budget-postgres

docker run -d --name auth --network budget-net -p 18081:8080 \
  -e DATABASE_URL=postgres://budget:devpass@budget-postgres:5432/budgetapp \
  -e JWT_PRIVATE_KEY_PATH=/keys/private.pem \
  -v "$(pwd)/services/auth/keys:/keys:ro" \
  budget-app/auth:dev

docker run -d --name budget --network budget-net -p 18082:8080 \
  -e DATABASE_URL=postgres://budget:devpass@budget-postgres:5432/budgetapp \
  -e JWT_PUBLIC_KEY_PATH=/keys/public.pem \
  -v "$(pwd)/services/auth/keys:/keys:ro" \
  budget-app/budget:dev
```

Note: the distroless runtime image runs as a non-root user, so the mounted
`private.pem` needs to be readable by it (`chmod 644` is sufficient for local
dev; in Kubernetes this will be handled by the Secret volume's file mode
instead).

## Kubernetes / kind

All three components run in the `budget-app` namespace of a local `kind`
cluster (cluster name `budget-app`): `postgres` (Deployment + PVC, no
StatefulSet — a single replica with `strategy: Recreate` is enough here),
`auth`, and `budget` (Deployment + Service each). `auth` and `budget` reach
Postgres via the `postgres` ClusterIP Service's DNS name — the same
`postgres:5432` hostname works whether a pod lands on any node in the
cluster, no hardcoded IPs anywhere. `budget` reaches `auth`'s public key the
same way conceptually, except it doesn't even need `auth` to be reachable at
request time, per the RS256-local-verification decision above — the public
key is just mounted from a Secret at startup.

**Secrets**: `postgres-credentials` (DB user/password/db-name plus a
ready-to-use `database-url`) and `jwt-keys` (the RSA keypair) are generated
locally by `scripts/generate-secrets.sh` into `k8s/secrets.yaml`, which is
gitignored and never committed — see `k8s/secrets.example.yaml` for the
shape. `auth` only gets `private.pem` mounted (via the Secret volume's
`items` filter), `budget` only gets `public.pem` — each service can only see
the key material it actually needs.

Deploying from scratch:

```bash
kind create cluster --name budget-app

docker build -t budget-app/auth:dev services/auth
docker build -t budget-app/budget:dev services/budget
kind load docker-image budget-app/auth:dev --name budget-app
kind load docker-image budget-app/budget:dev --name budget-app

bash scripts/generate-secrets.sh

kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/secrets.yaml
kubectl apply -f k8s/postgres-pvc.yaml -f k8s/postgres-deployment.yaml -f k8s/postgres-service.yaml
kubectl apply -f k8s/auth-deployment.yaml -f k8s/auth-service.yaml
kubectl apply -f k8s/budget-deployment.yaml -f k8s/budget-service.yaml
kubectl apply -f k8s/monitoring/prometheus-configmap.yaml \
               -f k8s/monitoring/prometheus-deployment.yaml \
               -f k8s/monitoring/prometheus-service.yaml

kubectl -n budget-app rollout status deployment/postgres
kubectl -n budget-app rollout status deployment/auth
kubectl -n budget-app rollout status deployment/budget
kubectl -n budget-app rollout status deployment/prometheus
```

Then reach either service the same way as the local setup, just via
port-forward instead of a bare port:

```bash
kubectl -n budget-app port-forward svc/auth 8081:80 &
kubectl -n budget-app port-forward svc/budget 8082:80 &
```

### Buffer day: what a full cluster rebuild actually found

Everything above was reached incrementally during development, with fixes
sometimes applied by hand to a live cluster. To check that the repo alone
(not the accumulated state of one long-lived cluster) is enough to reproduce
a working deployment, the whole thing was torn down (`kind delete cluster`)
and rebuilt from scratch, more than once, following only the commands above.
That process surfaced three real, fixed issues:

1. **A PVC delete is not a reliable "clean slate."** Early on, deleting just
   the `postgres-data` PVC (while leaving the rest of the cluster running)
   and recreating it came back bound to a `local-path` volume that still held
   the *previous* PostgreSQL data directory — `POSTGRES_*` env vars from a
   new Secret were silently ignored, since `initdb` only runs against a
   genuinely empty data directory (Postgres logs `"skipping initialization"`
   when this happens). A full `kind delete cluster` + recreate does not have
   this problem — the node's entire filesystem goes with it — so that's the
   reliable way to get a truly blank slate, not a PVC delete on a live
   cluster.

2. **`auth` and `budget` raced each other to create the `pgcrypto` extension.**
   Both services' `schema.sql` runs `CREATE EXTENSION IF NOT EXISTS pgcrypto`
   against the same Postgres instance (the extension is database-wide, not
   schema-scoped). When both pods start within milliseconds of each other on
   a fresh cluster, `IF NOT EXISTS` alone doesn't prevent one of them from
   losing a race on `pg_extension_name_index` and crashing. Fixed by wrapping
   the statement in a `DO $$ ... EXCEPTION WHEN unique_violation THEN NULL;
   END $$;` block in both `schema.sql` files, so the loser of the race treats
   "someone else already created it" as success instead of an error.

3. **The liveness probe could kill a pod before its own retry logic got a
   chance to work.** Both services block in `main()` on a database
   connection (with an internal retry loop of up to ~2 minutes, to tolerate
   Postgres's own cold-start time — including a possible image pull, since
   only the two app images are pre-loaded into kind via `kind load
   docker-image`, not Postgres's) before ever binding `:8080`. The original
   `livenessProbe` started checking after only 5 seconds, got "connection
   refused" repeatedly, and killed the container — restarting it before
   Postgres was even reachable. This is why every Deployment now has a
   `startupProbe` ahead of its readiness/liveness probes: Kubernetes won't
   evaluate liveness at all until the app responds successfully once, so a
   legitimately slow (but not stuck) startup no longer gets treated as a
   crash.

After all three fixes, a completely fresh `kind delete cluster` →
`kind create cluster` → apply-everything cycle came up with **zero pod
restarts** across `postgres`, `auth`, `budget`, and `prometheus`.

## CNCF Landscape technology: Prometheus

Both `auth` and `budget` expose a `GET /metrics` endpoint (via
`prometheus/client_golang`'s `promhttp.Handler()`), and a Prometheus
Deployment in the `budget-app` namespace (`k8s/monitoring/`) scrapes both
every 15s.

**What's exposed beyond the default Go runtime metrics** (`go_*`,
`process_*`): every non-health-check route is wrapped in a small middleware
(`instrument()` in each service's `metrics.go`) that records

- `http_requests_total{method, route, status}` — a counter
- `http_request_duration_seconds{method, route}` — a histogram

The label is the **registered route pattern** (e.g. `/transactions/{id}`),
never the raw request path — using the raw path would make every distinct
transaction ID its own Prometheus label value, and metric cardinality would
grow without bound as the household adds more transactions. This is the
standard pitfall with naive HTTP instrumentation, so it's called out here
deliberately rather than left implicit.

**Why this is useful for this project specifically:** the ledger
authorization rules (personal vs. shared visibility, who can mutate what)
live entirely in application code, not in the database — a bug there fails
silently as a `404` or `403`, not a crash. `http_requests_total` sliced by
`status` surfaces that kind of problem directly (e.g. an unexpected spike in
`403`s on `/transactions/{id}` would suggest the mutation-rights check is
misfiring) without needing to add ad-hoc logging. The latency histogram
gives an early signal if, say, listing shared transactions gets slower as a
household's history grows — something to watch given `GET /transactions`
performs no pagination yet.

**Kept deliberately minimal, consistent with the rest of this project's
scope:**
- Static scrape targets (`auth:80`, `budget:80` via Kubernetes Service DNS)
  instead of `kubernetes_sd_configs` — with exactly two known services,
  Kubernetes service-discovery and the RBAC it requires (a ClusterRole to
  list pods/services) would be pure overhead.
- No persistent volume for Prometheus — metrics history is lost on pod
  restart, which is fine for a course demo and avoids another PVC to manage.
- No Grafana or alerting rules — deferred; Prometheus's own expression
  browser (`kubectl -n budget-app port-forward svc/prometheus 9090:9090`,
  then `localhost:9090`) is enough to demonstrate that scraping and querying
  work end to end.

## Mobile app

`mobile/` is a minimal Expo (React Native + TypeScript) client: an auth
screen (login, plus sign-up with a toggle between creating a new household
and joining an existing one by ID), a transaction list (personal + shared,
pull-to-refresh, with a running balance shown separately per ledger — mixing
personal and shared totals into one number would defeat the point of keeping
them separate), and an add-transaction form (expense/income, personal vs.
shared, optional category with inline category creation). It talks to the
same `auth`/`budget` HTTP APIs used throughout this README — no separate
mobile-specific backend or endpoints.

Deliberately left out of scope, per this project's priorities: navigation
library (three screens are swapped via plain local state in `App.tsx`
instead), offline support, transaction editing/deletion from the UI (the API
supports it; the UI doesn't expose it yet), and any visual polish.

**Android gotcha already fixed:** by default, Android floats the on-screen
keyboard on top of the UI instead of resizing it, so a focused input near the
bottom of a form (e.g. the inline "new category" field) ends up hidden behind
the keyboard with no way to see what's being typed. Fixed with
`"softwareKeyboardLayoutMode": "resize"` in `app.json` plus wrapping both
form screens (`AuthScreen`, `AddTransactionScreen`) in `KeyboardAvoidingView`
+ a scrollable container. This is a config change, not a JS change — it
needs a full restart of `expo start`, not just a reload, to take effect.

**Running it:**

```bash
cd mobile
npm install   # already done if you're continuing this session
npx expo start
```

Then press `w` for web, or scan the QR code with Expo Go on a phone. Before
that works, point `mobile/src/config.ts` at wherever `auth`/`budget` are
actually reachable — the file has the three cases spelled out (same machine,
Android emulator, physical phone over LAN), since "`localhost`" means a
different thing depending on where the app is actually running relative to
the `kubectl port-forward` process.

**Testing on a real phone (iPhone or Android), from a Windows/WSL2 dev
machine:** this needs more than pointing `config.ts` at a LAN IP, because of
how WSL2's networking actually works — worth documenting since it wasn't
obvious and cost real debugging time to work out.

1. Install **Expo Go** on the phone (App Store / Play Store) and put it on
   the same Wi-Fi network as the PC.
2. Find the PC's LAN IP: `ipconfig` in PowerShell, the IPv4 address under the
   Wi-Fi adapter (e.g. `192.168.0.154`).
3. Port-forward with `--address 0.0.0.0`, on ports other than `8081`/`8082` —
   Expo's own dev server defaults to `8081`, which would otherwise collide:
   ```bash
   kubectl -n budget-app port-forward --address 0.0.0.0 svc/auth 18081:80
   kubectl -n budget-app port-forward --address 0.0.0.0 svc/budget 18082:80
   ```
4. **This is the part that actually needs extra work on Windows.** `--address
   0.0.0.0` makes the port-forward listen on all of the *WSL2 VM's* network
   interfaces — it does not, on its own, expose the port on the Windows
   host's real LAN-facing adapter. WSL2's automatic `localhost` forwarding
   only bridges `localhost` on Windows into WSL2; it doesn't bridge the LAN
   IP. Without the next step, the phone will get a hung connection (not even
   a fast "connection refused") when it tries to reach the PC's LAN IP.
   In an **Administrator** PowerShell:
   ```powershell
   wsl hostname -I   # WSL2's internal IP — changes across reboots, re-check if this stops working
   netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=18081 connectaddress=<wsl-ip> connectport=18081
   netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=18082 connectaddress=<wsl-ip> connectport=18082
   New-NetFirewallRule -DisplayName "Budget App Auth" -Direction Inbound -LocalPort 18081 -Protocol TCP -Action Allow
   New-NetFirewallRule -DisplayName "Budget App Budget" -Direction Inbound -LocalPort 18082 -Protocol TCP -Action Allow
   ```
   Verify with `curl http://<LAN-IP>:18081/healthz` from PowerShell itself
   before trying the phone — isolates a network problem from an app problem.
   To remove later or after the WSL2 IP changes: `netsh interface portproxy
   delete v4tov4 listenaddress=0.0.0.0 listenport=18081` (repeat per port),
   then re-add with the current IP.
5. Update `mobile/src/config.ts` with that LAN IP and the `18081`/`18082`
   ports.
6. `cd mobile && npx expo start --tunnel` — `--tunnel` specifically, not the
   default: WSL2 sometimes reports its own internal IP rather than the real
   Wi-Fi IP when Metro builds the QR code, and `--tunnel` (via ngrok) routes
   around that IP-detection problem entirely. This only affects how the phone
   loads the JS bundle — it's unrelated to step 4, which is about the app's
   own network calls once it's running.
7. Scan the QR code (iPhone: Camera app; Android: Expo Go's own scanner).

If login/signup fails with a network error but the app itself loaded, the
network path from step 4 is the first thing to re-check — that error means
the request never left the phone, not that the server rejected it.

The JWT is stored via `expo-secure-store` (not `AsyncStorage`) so a restart
doesn't force a re-login; a `401` from either service (e.g. an expired
token) clears the stored session and drops the user back to the auth screen
automatically, rather than showing a confusing error on every request.
