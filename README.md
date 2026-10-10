# Digital Card Platform

Customer digital business cards with separate customer and administrator authentication. The application uses Next.js, Go/Chi, PostgreSQL, and Redis.

## Local development

1. Start the local database and cache:

   ```powershell
   docker compose up -d
   ```

2. Configure the backend:

   ```powershell
   Copy-Item backend/.env.example backend/.env
   ```

   Keep `APP_ENV=local` and `PAYMENT_PROVIDER=mock` for local-only demo payments. Set a private `JWT_SECRET` for local use. The example PostgreSQL credentials are development defaults only.

3. Apply any migrations that are not already present in your local database. Review [DEPLOYMENT.md](DEPLOYMENT.md#database-migrations) for the ordered migration list and safe procedure. Do not blindly replay migrations against an existing database.

4. Start the backend and frontend in separate terminals:

   ```powershell
   cd backend
   go run ./cmd/api
   ```

   ```powershell
   cd web
   Copy-Item .env.local.example .env.local
   npm ci
   npm run dev
   ```

   The local frontend is at `http://localhost:3000`, API at `http://localhost:8080`, and API health endpoint at `http://localhost:8080/health`.

5. Admin creation is a one-time operation through `go run ./cmd/seed-admin`. Set the `ADMIN_SEED_*` values in the private backend env file first; remove them afterward.

## Build

```powershell
cd backend
go test ./...
go build ./cmd/api
cd ../web
npm ci
npm run build
```

The Next.js application is configured for standalone output for the production container. Standard `npm run start` remains available after a normal Next build.

## Production

Read [DEPLOYMENT.md](DEPLOYMENT.md) before preparing a server. Production requires explicit secrets, a configured domain, SMTP credentials for OTP delivery, migrations verified against the target database, and Razorpay TEST-mode checkout verification. No live payment mode is enabled by this repository configuration.
