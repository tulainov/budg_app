# Budget App

A budget-tracking app for a two-person household: personal ledgers per user, plus
a shared ledger for mutual expenses. Built as a Kubernetes/cloud-native course
project — the priority is demonstrating the required architecture cleanly, not
feature completeness.

## Architecture

```
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

Documenting only the factors actually applied so far (steps 1–2); this section
grows as later steps add Kubernetes, config-as-env-in-manifests, and metrics.

| Factor | How it's applied |
|---|---|
| **III. Config** | Both services read all configuration — `PORT`, `DATABASE_URL`, `JWT_PRIVATE_KEY_PATH` / `JWT_PUBLIC_KEY_PATH` — from environment variables. Nothing is hardcoded; the same binary/image runs against local Postgres or a clustered one purely by changing env vars. |
| **IV. Backing services** | Postgres is treated as an attached resource, addressed only via the `DATABASE_URL` env var. Swapping the local Postgres container for a different instance (different host, credentials, or even a managed Postgres later) requires no code change. |
| **V. Build, release, run** | Each service has a multi-stage `Dockerfile`: a `build` stage compiles the static Go binary, and a separate `distroless` runtime stage contains only the compiled binary — no Go toolchain, shell, or package manager in the image that runs. Configuration (env vars) is injected at run time, never baked into the image. |
| **VI. Processes** | Both services are stateless — no in-memory session or request state. All persistent state lives in Postgres, so any instance can handle any request and the process can be killed/restarted freely. |
| **VII. Port binding** | Each service is self-contained and binds its own port via `net/http`, configurable through `PORT`. No app server is injected by the runtime environment. |
| **IX. Disposability** | Fast startup (single static Go binary, no runtime dependency resolution) and graceful shutdown: both services listen for `SIGTERM`/`SIGINT` and call `http.Server.Shutdown` with a 10s grace period before exiting, so in-flight requests aren't dropped mid-response when Kubernetes terminates a pod. |
| **X. Dev/prod parity** | The same Docker image built and tested locally (`docker run`) is the one that will be loaded into the kind cluster — no separate "dev build" vs "prod build" of the application code, only env var differences. |
| **XI. Logs** | Both services log unbuffered to stderr via Go's standard `log` package (`log.Printf`/`log.Fatal`) rather than writing to log files — captured directly by `docker logs` / `kubectl logs`, with no in-app log routing or rotation logic. |

Not yet applicable / deferred:
- **VIII. Concurrency** — scaling via the process model (multiple replicas)
  isn't exercised until the Kubernetes step.
- **XII. Admin processes** — no one-off admin/management scripts exist yet;
  schema setup currently happens automatically on service startup.

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

Covered in a later step. `k8s/namespace.yaml`, `k8s/auth-deployment.yaml`, and
`k8s/auth-service.yaml` currently exist as a proof that the
Go → Docker → kind → Deployment/Service toolchain works end to end; the
budget service and Postgres manifests, plus Secrets for DB credentials and
the JWT signing key, come next.

## CNCF Landscape technology

Covered in a later step (Prometheus, scraping `/metrics` on both services).
