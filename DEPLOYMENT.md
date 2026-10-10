# Production deployment preparation

This is a deployment guide, not an automated deployment. It keeps Razorpay in TEST mode and does not request TLS certificates or run migrations for you.

## Readiness and prerequisites

Before exposing the service publicly, obtain a Linux host, domain/DNS, TLS setup, SMTP/transactional email delivery credentials for customer/admin OTP, and private production secrets. OTP delivery requires the `SMTP_*` values in the backend environment. Validate delivery through the provider before enabling registration and recovery flows.

The app currently supports `APP_ENV`, `APP_PORT`, `DATABASE_URL`, `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`, `JWT_SECRET`, `FRONTEND_URL`, `PUBLIC_WEB_URL`, `API_PUBLIC_URL`, `PAYMENT_PROVIDER`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_EMAIL`, `SMTP_FROM_NAME`, `RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET`, and `RAZORPAY_WEBHOOK_SECRET`. SMTP supports implicit TLS on port 465 or mandatory STARTTLS on other ports; authenticated SMTP credentials are required. Frontend build variables are `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_APP_URL`, and optionally `NEXT_PUBLIC_RAZORPAY_KEY_ID`. Only the Razorpay key ID is public. Never put JWT/database/SMTP/Razorpay secret values in `NEXT_PUBLIC_*` variables.

## Environment and TEST payment mode

Copy `.env.production.example` to a private `.env.production` on the host. It is ignored by Git. Replace every placeholder with generated credentials; use URL-encoded reserved characters in `DATABASE_URL`. Set the domain values consistently:

- `APP_ENV=production`
- `PAYMENT_PROVIDER=razorpay`
- `RAZORPAY_MODE=test`
- `FRONTEND_URL=https://YOUR_DOMAIN` (exact browser origin, no trailing slash)
- `PUBLIC_WEB_URL=https://YOUR_DOMAIN`
- `API_PUBLIC_URL=https://YOUR_DOMAIN` (public origin for QR/NFC redirects)
- `NEXT_PUBLIC_APP_URL=https://YOUR_DOMAIN`
- `NEXT_PUBLIC_API_URL=https://YOUR_DOMAIN/api/v1`
- `API_SERVER_URL=http://backend:8080/api/v1` for Next server-side public-card metadata/data fetches inside Compose (server-only, not a `NEXT_PUBLIC_*` value)
- `NEXT_PUBLIC_RAZORPAY_KEY_ID` to the public **TEST** key ID only

Use distinct high-entropy database, Redis, and JWT credentials. `JWT_SECRET` must contain at least 32 non-placeholder characters; the backend refuses to start in production otherwise. The backend also refuses `PAYMENT_PROVIDER=mock` in production. Mock endpoints have an additional local-only check. Razorpay server credentials remain in backend environment only.

Create a JWT secret on the deployment host or through the operator's approved secret manager, for example `openssl rand -base64 48`. Do not commit it, paste it into tickets, or reuse the local development value. Production startup also requires complete non-placeholder SMTP settings; the SMTP server must support STARTTLS (587 or another STARTTLS port) or implicit TLS (465). OTP values and mail credentials are never logged. Delivery errors are returned safely and logged only with the OTP purpose.

## Builds and Docker startup

Run verification before preparing the release:

```powershell
cd backend
go test ./...
go build ./cmd/api
cd ../web
npm ci
npm run build
```

The frontend uses Next.js standalone output. `npm run start` is supported after build; Docker uses the standalone `server.js` runner. The production compose file builds both images and has no source bind mounts. PostgreSQL and Redis have no published ports; application ports bind to host loopback for the host reverse proxy. PostgreSQL and Redis use persistent Docker volumes. Redis is password protected and reachable only on the app bridge network. The app uses both services, so keep the bridge network non-public and secure host firewall access.

1. Copy `.env.production.example` to `.env.production`; fill all values first. Confirm `APP_ENV=production`, `PAYMENT_PROVIDER=razorpay`, `RAZORPAY_MODE=test`.
2. Configure DNS and install Nginx/certbot on the host as described below. Set the real domain in the env file and Nginx config.
3. Build and start:

   ```sh
   docker compose --env-file .env.production -f docker-compose.prod.yml config
   docker compose --env-file .env.production -f docker-compose.prod.yml build
   docker compose --env-file .env.production -f docker-compose.prod.yml up -d
   docker compose --env-file .env.production -f docker-compose.prod.yml ps
   ```

   Inspect the rendered `config` locally before sharing it; compose output includes interpolated secrets. Do not paste it into tickets or logs.

The Docker Compose `postgres:17-alpine` service initializes an empty named volume only. It does not migrate an existing DB or load existing local data. Do not point a new empty production volume at an existing service or run initialization against valuable data without a planned restore/migration.

## Database migrations

The multi-currency pricing API requires migration `000033_global_subscription_prices.sql` before the updated API starts. Confirm `website_pricing_plans.billing_config`, `payments.checkout_country`, and `payments.checkout_snapshot` exist in the target database. Missing `billing_config` makes `/api/v1/website/pricing` return HTTP 500 even though billing history still works. Applying this additive migration preserves existing plan defaults, features, trials, subscriptions, and payment records.

The repository currently has ordered, numbered SQL files through `000033_global_subscription_prices.sql`. It has no migration runner or migration ledger. `000017_create_payment_tables.sql.backup` is a backup file, not a migration; the active migration at that sequence is `000017_create_razorpay_payments.sql`. No reset/drop command is part of deployment.

Before a release, inspect the target schema and migration history, compare each numbered migration, and identify only unapplied files. Take and verify a database backup first. Apply one reviewed migration at a time while monitoring PostgreSQL output; do not blindly replay all SQL files against an existing DB. For a new database, apply the numbered `*.sql` files in ascending order, excluding the `.backup` file. Record the applied filename and checksum in your release/change record until a formal migration ledger is added.

Example of applying one reviewed migration from a host with `psql` configured (replace the file and connection with the secured target):

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f backend/migrations/000001_create_admin_users.sql
```

Repeat only for verified pending migrations. Run through a restricted operator account, not as an unaudited automatic container startup action. Validate schema and application health after each relevant batch. Production migration readiness is manual because migrations are not automatically tracked.

## PostgreSQL backup and restore

Do not run these commands against a production DB without an approved backup window and target confirmation. Backups are not automated by the compose file.

Create a compressed custom-format dump and protect it as sensitive data:

```sh
umask 077
pg_dump --format=custom --no-owner --no-acl --file="digital-card-$(date -u +%Y%m%dT%H%M%SZ).dump" "$DATABASE_URL"
pg_restore --list digital-card-YYYYMMDDTHHMMSSZ.dump
```

Store encrypted copies outside the application host, restrict access, and regularly verify restore into an isolated database. A practical starting retention policy is daily backups for 14 days, weekly for 8 weeks, and monthly for 12 months, adjusted to the business recovery objectives and legal requirements. Monitor backup completion and available storage.

Restore only to an explicitly verified empty/isolated target, preserving the current DB until the restored copy has been validated:

```sh
createdb --maintenance-db="$MAINTENANCE_DATABASE_URL" restore_validation
pg_restore --dbname="$RESTORE_DATABASE_URL" --no-owner --no-acl digital-card-YYYYMMDDTHHMMSSZ.dump
```

`RESTORE_DATABASE_URL` must identify the intended restore database. Avoid `--clean` on production; it drops objects. A cutover/rollback plan should be approved separately.

## Redis

Set `REDIS_ADDR=redis:6379`, a strong `REDIS_PASSWORD`, and `REDIS_DB=0`. Do not publish Redis ports, allow public firewall access, or use an unauthenticated public Redis endpoint. Production compose has no Redis `ports` mapping. Keep the Redis volume private and restrict host access.

## Nginx and HTTPS

The example is [infra/nginx/digital-card-platform.conf](infra/nginx/digital-card-platform.conf). Replace `YOUR_DOMAIN`, review it for the host's Nginx version, and install it only after DNS points to the server. It routes `/api/`, `/q/`, `/n/`, and `/health` to Go and all other paths to Next.js. It preserves `Host`, `X-Forwarded-For`, and `X-Forwarded-Proto`; the frontend proxy includes upgrade headers for Next.js streaming/upgrade compatibility. Security headers are included, with no untested CSP.

For HTTPS, install Nginx and Certbot using the server OS package instructions and open only ports 80/443 at the host firewall. First install a temporary HTTP-only virtual host for `YOUR_DOMAIN` so Nginx config validation succeeds before certificate files exist. After DNS points to this host, obtain certificates and then install/enable the TLS server block in the example. Example operator steps (not run by this project):

```sh
sudo certbot --nginx -d YOUR_DOMAIN -d www.YOUR_DOMAIN
sudo nginx -t
sudo systemctl reload nginx
sudo certbot renew --dry-run
```

Do not request certificates before replacing placeholders and confirming domain ownership/DNS. Verify `https://YOUR_DOMAIN`, `https://YOUR_DOMAIN/api/v1/...`, and `https://YOUR_DOMAIN/health`; the API health route returns 503 if Postgres or Redis is down.

## Auth, secrets, and logs

- Customer and administrator routes keep separate token claims and storage keys. Keep using customer `user_token` and admin `admin_token`; do not share credentials or protected route groups.
- Passwords are bcrypt hashed. OTP codes are hashed in `auth_otp_codes`, rate limited by the OTP service, emailed over TLS/STARTTLS, and are never logged or returned in API responses.
- The current server request logger records request metadata, not request bodies. Do not add body logging or log Authorization headers, cookies, raw passwords, reset tokens, OTPs, webhook signatures, connection strings, or provider secrets.
- Avoid returning raw database errors to clients. Restrict access to container logs because operational errors and identifiers can still be sensitive.
- `.env`, `.env.*`, production env files, dumps, logs, build output, and generated binaries are ignored by Git. Example env files contain placeholders only. Review `git status` and repository history with Git installed before publishing.

## Razorpay TEST verification

Configure test key ID/secret and webhook secret in the backend environment; set only the key ID in the frontend build. Configure the Razorpay TEST webhook URL as:

`https://YOUR_DOMAIN/api/v1/webhooks/razorpay`

Set the webhook secret in the backend `RAZORPAY_WEBHOOK_SECRET` environment variable and subscribe to `payment.captured` and `payment.failed`. If refunds are enabled for the account, also subscribe to `refund.processed`. The API validates the webhook HMAC with the server-side webhook secret, stores processed event IDs for idempotency, and activates subscriptions only after verified payment processing. Run a test payment and check payment, subscription, and webhook records before considering any live-mode change. Never switch to live automatically.

Razorpay cannot send webhooks directly to `localhost`. For local webhook testing, an operator may run an already approved tunnel such as ngrok or cloudflared, then configure the resulting HTTPS URL with `/api/v1/webhooks/razorpay` in the Razorpay TEST dashboard. Do not expose a local machine automatically; keep the tunnel URL and its access controls under operator control.

### Separate LIVE MODE checklist (not enabled)

1. Complete deployed TEST-mode end-to-end checkout, verify, webhook retry/idempotency, failure, refund behavior where configured, and subscription dates.
2. Configure/verify live merchant account and production webhook secret through an approved secret manager.
3. Review payment reconciliation, alerting, support/refund processes, and applicable compliance requirements.
4. Change `PAYMENT_PROVIDER`/credentials/mode only through a separately approved change and staged maintenance window; keep secrets server-side.
5. Confirm no live secret enters Git, frontend variables, build args, logs, or artifacts. This repository intentionally keeps the backend client test-only; code support for live mode needs a separate reviewed change before following any live checklist.

## Health checks and troubleshooting

- Backend: `GET /health` checks PostgreSQL and Redis and returns 503 for dependency failures.
- Compose: `docker compose --env-file .env.production -f docker-compose.prod.yml ps` shows service/health state; inspect logs privately with `docker compose ... logs --tail=100 backend`.
- Backend startup failure: check `APP_ENV`, `JWT_SECRET` length, database URL escaping/connectivity, Redis host/password, and schema version. Never share full environment output.
- Browser API errors: confirm `NEXT_PUBLIC_API_URL` was set to the HTTPS domain at image build, Nginx routes `/api/`, and `FRONTEND_URL` exactly matches the site origin.
- OTP delivery: confirm SMTP host, port, credentials, verified sender address, and TLS support; provider failures return generic API errors and log only the OTP purpose.
- Payments: test mode is intentionally enforced by the current Razorpay client. Missing/mismatched webhook configuration prevents payment finalization; inspect sanitized application logs and provider test dashboard.
- Database migrations: no migration runner/ledger exists. Inspect current schema and apply only verified pending migrations after backup.

## Production release checklist

- [ ] Final customer/admin billing QA passed in deployed Razorpay TEST mode
- [ ] Domain is available and DNS points to the deployment host
- [ ] Server provisioned and firewall limits public access to HTTPS (and HTTP redirect)
- [ ] Production secrets configured outside Git; JWT secret is at least 32 random characters
- [ ] PostgreSQL configured and reachable only over a private network or approved TLS connection
- [ ] Redis configured with authentication and no public port
- [ ] SMTP configured with TLS and OTP delivery tested
- [ ] Database backup schedule and retention configured; restore tested in isolation
- [ ] Pending migrations reviewed and documented
- [ ] Backend tests and build passed
- [ ] Frontend production build passed
- [ ] Nginx routes frontend, `/api/`, `/q/`, `/n/`, and `/health`
- [ ] HTTPS certificate installed and renewal dry run passed
- [ ] Customer authentication tested
- [ ] Administrator authentication and authorization tested
- [ ] OTP email verified for customer and admin recovery flows
- [ ] Razorpay TEST payment, signature verification, and subscription activation tested
- [ ] Razorpay TEST webhook signature and duplicate-event behavior tested
- [ ] `/health` reports service, PostgreSQL, and Redis status
- [ ] Production logs reviewed for credentials, OTPs, tokens, and payment secrets

Do not consider deployment ready until every applicable item is checked and the deployed TEST-mode billing flow has passed. Keep `RAZORPAY_MODE=test`; any LIVE-mode change requires a separate review and operator action.
