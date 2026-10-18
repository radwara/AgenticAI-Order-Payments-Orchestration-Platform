# Postgres

Decision: SPEC-00 ADR-001. PostgreSQL for all transactional data. Managed Postgres in production; CloudNativePG (CNPG) is an optional in-cluster alternative for dev/staging.

Layout: one instance to start with, one database and one role per service (`orders`, `payments`, `inventory`, `pricing`); no shared tables, roles or credentials. Payments moves to its own instance first.

## Dev/staging manifests (`deploy/postgres/`)

- `cluster.yaml`: CNPG `Cluster` with one managed role per service. Roles are `NOSUPERUSER`, cannot create databases or roles and inherit nothing. `pg_hba` allows each role only to its own database over `hostssl` and ends with `host all all all reject`. The Database CRD cannot `REVOKE CONNECT ... FROM PUBLIC`, so that reject is what closes the default PUBLIC privilege in dev/staging.
- `databases.yaml`: one declarative `Database` per service, owned by the role of the same name.
- `pooler.yaml`: PgBouncer 1.24.1 `Pooler` (transaction mode, `max_prepared_statements: 100`). Services connect to `opop-pg-pooler-rw`, never to the instance.
- `externalsecrets.yaml`: one `ExternalSecret` per service mapping a cloud secret (example names `opop/dev/<service>/db`, properties `user`, `password`, `url`, `migration_url`) to the Secret `<service>-db` with keys `DB_URL` (via the pooler), `DB_MIGRATION_URL` (direct to `opop-pg-rw`), `DB_USER`, `DB_PASSWORD` (plus `username`/`password` for the CNPG role). Only names are in git. The Secrets must be in the same namespace as the Cluster and the pods (`secretKeyRef` is namespace-local). A `ClusterSecretStore` named `cloud-secrets` must exist.

Validate with `scripts/validate-postgres.sh` (kubeconform `--strict` against pinned CNPG and External Secrets schemas, isolation assertions, chart assertions). It needs network on the first run to fetch the CRD schemas, then uses a local cache.

## Wiring a service

In the service chart values (library chart `deploy/charts/service-base`):

```yaml
database:
  enabled: true
  secretName: orders-db
```

The pod gets `DB_URL`, `DB_USER` and `DB_PASSWORD` from `secretKeyRef` only; no value is ever rendered in plaintext.

## Managed production setup

- **Instance:** RDS/Aurora, Cloud SQL/AlloyDB or Azure Database for PostgreSQL (provider still open; no code depends on it). Provisioning (Terraform) is out of scope here.
- **Roles and databases:** create the same per-service database and role pairs as above. Run `REVOKE CONNECT ON DATABASE <db> FROM PUBLIC` and grant `CONNECT` only to the owning role. Never give an application role `CREATEDB`, `CREATEROLE` or superuser rights.
- **Credentials:** stored in the cloud secret manager and synced with the same `ExternalSecret` shape. Rotation is a secret update; pods pick it up on restart.

## Private networking
The database has no public endpoint. Place it in private subnets and allow ingress only from the cluster node or pod CIDRs (security group / firewall rule), on port 5432 with TLS required (`sslmode=verify-full` in `DB_URL`). Cluster egress to the pooler or database is allowed by an explicit `NetworkPolicy` rule in the service values, since the chart denies egress by default.

## Pooler

Run PgBouncer (or the cloud proxy, for example RDS Proxy or Cloud SQL Auth Proxy) between services and the database in transaction mode. Size `default_pool_size` per database so that the sum across services stays below the instance `max_connections` with headroom for migrations. Transaction mode rules out session state (session-level advisory locks, `SET` without `LOCAL`); `SKIP LOCKED` queues and ordinary transactions work. Named prepared statements (the pgx and JDBC defaults) fail with "prepared statement does not exist" unless handled:

- PgBouncer >= 1.21 tracks them when `max_prepared_statements` is set (done in `pooler.yaml`; the image is pinned to 1.24.1). Verified only by version, not on a live cluster. Managed proxies differ, so also set the driver side below for production.
- JDBC: `prepareThreshold=0` in the URL. pgx: `default_query_exec_mode=cache_describe` (or `simple_protocol`).

Migrations never use the pooler: they use `DB_MIGRATION_URL`.

## Migration job

Migrations are forward-only, reviewed and run by the CI/CD pipeline, never at pod startup (SPEC-00): Flyway for Java, golang-migrate for Go.

- Run as a Kubernetes `Job` (or pipeline step) before the new Deployment version rolls out, one job per service and database, using that service's own credentials from `<service>-db`, and `DB_MIGRATION_URL` (direct, session-level locks work; enable it in the chart with `database.migration: true`) (the migration role may be a separate owner role with DDL rights; the runtime role then gets only DML).
- The job must be idempotent and safe to retry, with `backoffLimit` small and `ttlSecondsAfterFinished` set; a failed job blocks the rollout.
- Use expand/contract changes so the previous app version keeps working while the migration runs.

## Backups

Enable provider automated backups and point-in-time recovery, retain them per policy and restore-test them regularly. CNPG dev/staging data is disposable and not backed up. Tuning and HA sizing are out of scope.
