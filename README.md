<div align="center">
  <p>
    <img width="1480" height="1507" alt="image" src="https://github.com/user-attachments/assets/a704a237-7555-4ec8-8f72-c9cc915b1bee" />
  </p>
  <h1>CalendarSync Cloud</h1>
  <p><b>Sync multiple Google calendars into one view with hosted rules, scheduled runs, and cleanup.</b></p>
  <p>
    <a href="https://calendar-sync.nils.re/">Use the hosted app</a>
  </p>
  <p><b>Free to use.</b></p>
</div>

## What it is

CalendarSync Cloud is a hosted version of CalendarSync focused on a simple use
case: combine events from multiple Google calendars into one destination
calendar without manually duplicating everything yourself.

It is useful if you keep separate personal, work, school, or client calendars
and want one calendar to reflect your availability. The hosted product in this
fork currently supports Google account linking and Google Calendar sync.

## How it works

1. Sign in at [calendar-sync.nils.re](https://calendar-sync.nils.re/).
2. Link one or more Google accounts.
3. Create a sync rule from a source calendar to a target calendar.

Each rule can configure:

- source and target Google accounts/calendars
- full-details sync or busy-only sync
- schedule
- timeframe window
- filters
- transformations
- dry-run mode
- manual runs
- cleanup of previously synced events

## Current capabilities

The current cloud implementation in this repository supports:

- Google sign-in
- multiple linked Google accounts per user
- one-way sync rules
- full-details and busy-only payload modes
- manual runs and scheduled runs
- cleanup runs that remove synced events from the target calendar
- run history with status and error output

This repository is based on the original
[inovex/CalendarSync](https://github.com/inovex/CalendarSync) project. The
original CLI lineage includes additional adapters such as Outlook and ZEP, but
the hosted cloud product in this fork is Google-focused today.

## Privacy and security

- Google OAuth tokens are stored encrypted in the database.
- Application data is backed by Neon/Postgres.
- The cloud deployment targets Google Cloud services.

This README intentionally keeps the claims narrow and factual. If you self-host,
your security posture depends on your own Google Cloud, secret-management, and
database configuration.

## Self-hosting and development

This repository contains the hosted stack as well as the original CLI codebase.
The cloud runtime is split into three main components:

- `web/` - Next.js frontend with Google sign-in and rule management UI
- `cmd/calendarsync-api` - Go API for connections, calendars, rules, runs, and scheduler dispatch
- `cmd/calendarsync-worker` - Go worker for queued sync and cleanup runs

For the cloud version you will need:

- a Neon/Postgres database
- the schema in `db/migrations/0001_web_platform.sql`
- a Google OAuth client
- Google Cloud resources for Cloud Run, Cloud Build, Secret Manager, and Cloud Scheduler

Useful deployment entrypoints:

- `deploy/e2e_deploy.sh` bootstraps a first deployment, creates secrets, and creates scheduler jobs
- `deploy/deploy.sh` rebuilds and redeploys existing services/jobs

The repository's `.env.example` currently includes:

- `CS_GOOGLE_CLIENT_ID`
- `CS_GOOGLE_CLIENT_SECRET`
- `NEON_DB`

For the deployed cloud services, the scripts and binaries also use variables
such as `DATABASE_URL`, `GOOGLE_OAUTH_CLIENT_ID`,
`GOOGLE_OAUTH_CLIENT_SECRET`, `GOOGLE_OAUTH_REDIRECT_URL`,
`SCHEDULER_SHARED_SECRET`, `OAUTH_STATE_SECRET_B64`, and
`CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64`.

The deployment scripts use `gcloud`, and the default hosting path in this fork
is Google Cloud plus Neon.

## Original CLI project

The original CalendarSync project started as a stateless CLI for syncing
calendars across providers via `sync.yaml`. That code is still present in this
repository, but it is no longer the primary entrypoint of this fork.

If you are looking for the original adapter-focused CLI documentation, see:

- [docs/adapters.md](./docs/adapters.md)
- [example.sync.yaml](./example.sync.yaml)
- [docs/systemd-timers.md](./docs/systemd-timers.md)

## Attribution

This fork builds on
[inovex/CalendarSync](https://github.com/inovex/CalendarSync) and adapts it into
a hosted cloud service with a web UI, API, worker, and Google Cloud deployment
scripts.
