<div align="center">
    <p>
        <img src="./docs/static/logo.png" width="200" height="200"/>
        <h1 align="center">CalendarSync</h1>
        <b>Stateless CLI tool to sync calendars across different calendaring systems.</b>
    </p>
</div>

# Motivation

As consultants, you may need to use multiple calendars (2-n). Additionally, you
need to keep up with all existing appointments in each of your calendars when
you want to make new appointments. This means you have to check each calendar on
its own. What we wanted to achieve is a single overview over all events in each
of the calendars. Preferably in your primary calendar.

There are some commercial / freemium solutions for this
([reclaim.ai](https://reclaim.ai/),
[SyncThemCalendars](https://syncthemcalendars.com/)), but their privacy policy
is unclear. Calendar data is not only highly interesting personal data (who
participates in which appointment and when?) but also highly interesting from an
industrial espionage/targeted advertising perspective. The two third party
providers get to see the content of the calendar events. In good appointments,
there is a lot of secret and relevant company data in the appointment agenda.

To keep track of all the events, we created `CalendarSync`, which allows the
syncing of events without breaking data protection laws and without exposing
data to a third party.

# How to use

## Installation

Download the newest [release](https://github.com/inovex/CalendarSync/releases) for your platform or install from [the AUR](https://aur.archlinux.org/packages/calendarsync-bin).

**Using [asdf-vm](https://github.com/asdf-vm/asdf)**

You can also install this program using `asdf-vm`.

```sh
asdf plugin add calendarsync
# or
asdf plugin add calendarsync https://github.com/FeryET/asdf-calendarsync.git
## finally
asdf install calendarsync
```
Note: The `asdf` plugin is not managed by inovex, but is provided by a CalendarSync user. inovex assumes no responsibility for proper provisioning.

## First Time Execution

Create a modified `sync.yaml` file based on the content of the `./example.sync.yaml` file.
For the setup of the adapters, take a look at [the docs](docs/adapters.md).
Then, start the app using `CALENDARSYNC_ENCRYPTION_KEY=<YourSecretPassword> ./calendarsync --config sync.yaml` and follow the instructions in the output.

The app will create a file in the execution folder called `auth-storage.yaml`. In this file the OAuth2 Credentials will be saved encrypted by your `$CALENDARSYNC_ENCRYPTION_KEY`.

----

# Configuration

The CalendarSync config file consists of several building blocks:

- `sync` - Controls the timeframe to be synced
- `source` - Controls the source calendar to be synced from
- `sink`- Controls the sink (target) calendar where the events from the source
  calendar are written to
- `transformations` - Controls the transformers applied to the events before
  syncing
- `filters` - Controls filters, which allow events to be excluded from syncing
- `auth` - Controls settings regarding the encrypted auth storage file

## Sync

Should be self-explanatory. Configures the timeframe where to sync events. The
currently only implemented identifiers are `MonthStart` and `MonthEnd`.

```yaml
sync:
  start:
    identifier: MonthStart # 1st of the current month
    offset: -1 # MonthStart -1 month (beginning of last month)
  end:
    identifier: MonthEnd # last day of the current month
    offset: +1 # MonthEnd +1 month (end of next month)
```

## Source

Example:

```yaml
source:
  adapter:
    type: "outlook_http"
    calendar: "[base64-formatstring here]"
    oAuth:
      clientId: "[UUID-format string here]"
      tenantId: "[UUID-format string here]"
```

Configures the Source Adapter, for the adapter configuration, check the
documentation [here](./docs/adapters.md).

### Available Source Adapters

- Google
- Outlook
- [ZEP](https://www.zep.de/en/)

## Sink

Example:

```yaml
sink:
  adapter:
    type: google
    calendar: "target-calendar@group.calendar.google.com"
    oAuth:
      clientId: "[google-oAuth-client-id]"
      clientKey: "[google-oAuth-client-key]"
```

Configures the Sink Adapter, for the adapter configuration, check the
documentation [here](./docs/adapters.md).

### Available Sink Adapters

- Google
- Outlook

## Transformers

Basically, only the time is synced. By means of transformers one can sync
individual further data. Some transformers allow for further configuration using
an additional `config` block, such as the `ReplaceTitle` transformer. Below is a
list of all transformers available. They are applied from top to bottom.

transformerOrder = []string{
"KeepAttendees",
"KeepLocation",
"KeepReminders",
"KeepDescription",
"KeepMeetingLink",
"AddOriginalLink",
"KeepTitle",
"PrefixTitle",
"ReplaceTitle",
}

| **Name**          | **Description**                                                                                                                                                                                                                 | **Configuration**                                 |
|-------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------|
| `KeepAttendees`   | Synchronizes the list of attendees. If `UseEmailAsDisplayName` is set to `true`, the email is used in the attendee list. Do not use when the Outlook Adapter is used as a sink as there is no way to suppress mail invitations. | `config.UseEmailAsDisplayName`, default `false`   |
| `KeepLocation`    | Synchronizes the location of the event.                                                                                                                                                                                         | –                                                 |
| `KeepReminders`   | Synchronizes event reminders.                                                                                                                                                                                                   | –                                                 |
| `KeepDescription` | Synchronizes the description of the event.                                                                                                                                                                                      | –                                                 |
| `KeepMeetingLink` | Adds the meeting link of the original meeting to the description of the event.                                                                                                                                                  | –                                                 |
| `AddOriginalLink` | Adds the link to the original calendar event to the description of the event.                                                                                                                                                   | –                                                 |
| `KeepTitle`       | Synchronizes the event's title. Without this transformer, the title is set to `CalendarSync Event`                                                                                                                              | –                                                 |
| `PrefixTitle`     | Adds the configured prefix to the title.                                                                                                                                                                                        | `config.Prefix`, default `""`                     |
| `ReplaceTitle`    | Replaces the title with the configured string. Does not make sense to be used with `KeepTitle` or `PrefixTitle`                                                                                                                 | `config.NewTitle`, default `"CalendarSync Event"` |

Example configuration:

```yaml
transformations:
  - name: KeepDescription
  - name: KeepLocation
  - name: KeepReminders
  - name: KeepTitle
  - name: PrefixTitle
    config:
      Prefix: "[Sync] "
  - name: KeepMeetingLink
  - name: AddOriginalLink
  - name: KeepAttendees
    config:
      UseEmailAsDisplayName: true
```

## Filters

In some cases events should not be synced. For example, declined events might
create too much noise in the target calendar. These can be filtered by enabling
the corresponding filter.

```yaml
# Filters remove events from being synced due to different criteria
filters:
  # Events where you declined the invitation aren't synced
  - name: DeclinedEvents
  # Events which cover the full day aren't synced
  - name: AllDayEvents
  # Events within the specified timeframe will be retained, while all others will be filtered out.
  # hours are represented in the 24h time format (time is always UTC)
  - name: TimeFrame
    config:
      HourStart: 8
      HourEnd: 17
  # Events within the specified timeframe will be excluded (time is always UTC)
  - name: TimeFilter
    config:
      HourStart: 12
      HourEnd: 13
  # Events where the title matches the ExcludeRegexp (RE2 Regex) aren't synced
  - name: RegexTitle
    config:
      ExcludeRegexp: ".*test"
```

## Auth

In this section you can configure settings regarding the encrypted local auth storage

```yaml
auth:
  storage_mode: yaml # Currently, only yaml is supported
  config:
    # Here you can use the standard unix abbreviation for home directory (~).
    # This works also for Windows systems e.g. ~\calendar-sync\auth-storage.yaml
    path: "./auth-storage.yaml"
```

# Cleaning Up

You just synced a lot of events in your calendar and decide you want to use a
separate calendar for this? Or you want to remove all the synced events
from your calendar?

Use the `--clean` flag to get rid of all the unwanted events. (We leave your
events which weren't synced with CalendarSync alone! :) )

# Web Platform (Cloud)

The repository now contains a production-oriented web platform scaffold:

- `web/` - Next.js frontend with `next-auth` Google sign-in, account-linking UI, rules UI, and run history UI
- `cmd/calendarsync-api` - Go HTTP API for account links, rules, run triggering, and scheduler dispatch
- `cmd/calendarsync-worker` - Go worker process for queued run execution
- `db/migrations/0001_web_platform.sql` - Neon/Postgres schema for users, linked accounts, rules, runs, and encrypted tokens

## Environment Variables

### API (`cmd/calendarsync-api`)

- `DATABASE_URL` - Neon/Postgres DSN
- `GOOGLE_OAUTH_CLIENT_ID`
- `GOOGLE_OAUTH_CLIENT_SECRET`
- `GOOGLE_OAUTH_REDIRECT_URL` - Should point to your web callback route, e.g. `https://your-web-host/oauth/google/callback`
- `OAUTH_STATE_SECRET_B64` - base64 encoded random key for state signing
- `KMS_CRYPTO_KEY` - optional Cloud KMS crypto key resource name (recommended for envelope encryption)
- `CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64` - required fallback if `KMS_CRYPTO_KEY` is not set (32-byte base64 key)

### Worker (`cmd/calendarsync-worker`)

- `DATABASE_URL`
- `GOOGLE_OAUTH_CLIENT_ID`
- `GOOGLE_OAUTH_CLIENT_SECRET`
- `KMS_CRYPTO_KEY` (optional)
- `CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64` (fallback when KMS is not configured)

### Frontend (`web/`)

- `AUTH_SECRET`
- `AUTH_GOOGLE_ID`
- `AUTH_GOOGLE_SECRET`
- `CALENDARSYNC_API_URL` - base URL for `calendarsync-api` (for example `https://api.example.com`)

## Run Locally

1. Apply migration `db/migrations/0001_web_platform.sql` to your Postgres/Neon database.
2. Start API:
   - `go run ./cmd/calendarsync-api`
3. Start worker:
   - `go run ./cmd/calendarsync-worker`
4. Start frontend:
   - `cd web && npm install && npm run dev`

## Deploy to GCP (Cloud Run + Scheduler)

Use the deployment script:

```bash
GOOGLE_OAUTH_CLIENT_ID="<google-oauth-client-id>" \
GOOGLE_OAUTH_CLIENT_SECRET="<google-oauth-client-secret>" \
./deploy/e2e_deploy.sh
```

If you want a custom domain on the web service, pass `WEB_DOMAIN` and use Cloud
Run domain mapping directly. Firebase Hosting is not required for this setup.

```bash
GOOGLE_OAUTH_CLIENT_ID="<google-oauth-client-id>" \
GOOGLE_OAUTH_CLIENT_SECRET="<google-oauth-client-secret>" \
WEB_DOMAIN="calendar-sync.nils.re" \
./deploy/e2e_deploy.sh
```

The script uses project `open-calendar-sync` and will:

- enable required APIs
- create Artifact Registry + secrets
- build and push API/worker/web images
- deploy:
  - Cloud Run service `calendarsync-api`
  - Cloud Run service `calendarsync-web`
  - Cloud Run job `calendarsync-worker`
- create Cloud Scheduler jobs for dispatch and worker execution

Important:

- `NEON_DB` must exist in `.env` or env vars.
- You must configure a valid Google OAuth client and register the final redirect URL printed by the script (format: `<web-url>/oauth/google/callback`).
- When `WEB_DOMAIN` is set, also add `https://<web-domain>/api/auth/callback/google`
  as an authorized redirect URI and `https://<web-domain>` as an authorized JavaScript
  origin in the same Google OAuth client.

## Continuous Deployment from GitHub

This repository includes a GitHub Actions CD workflow at
`.github/workflows/deploy.yaml`. It deploys `calendarsync-api`,
`calendarsync-web`, and `calendarsync-worker` after the `Build` workflow
completes successfully for commits on `main`. Authentication uses Google Cloud
Workload Identity Federation, so no long-lived GCP key is stored in GitHub.

### 1. Bootstrap Workload Identity in GCP

Run:

```bash
chmod +x deploy/setup_github_wif.sh
./deploy/setup_github_wif.sh
```

By default this configures access for:

- project `open-calendar-sync`
- repository `nilsreichardt/CalendarSync-Cloud`
- branch `main`

You can override these with environment variables such as `PROJECT_ID`,
`GITHUB_OWNER`, `GITHUB_REPO`, or `BRANCH`.

The script creates:

- a Workload Identity pool `github-actions`
- an OIDC provider `github`
- a deployer service account `github-deployer@<project>.iam.gserviceaccount.com`
- the IAM bindings required to run Cloud Build and deploy Cloud Run services/jobs

### 2. Configure GitHub Actions variables

Add these repository variables in GitHub (`Settings -> Secrets and variables ->
Actions -> Variables`):

- `GCP_PROJECT_ID`
- `GCP_REGION`
- `GCP_WORKLOAD_IDENTITY_PROVIDER`
- `GCP_SERVICE_ACCOUNT`

The bootstrap script prints the exact values to use.

### 3. Bootstrap the runtime once

The CD workflow reuses `deploy/deploy.sh`, which expects the Cloud Run services
and job to exist already. For the first environment bootstrap, run:

```bash
GOOGLE_OAUTH_CLIENT_ID="<google-oauth-client-id>" \
GOOGLE_OAUTH_CLIENT_SECRET="<google-oauth-client-secret>" \
./deploy/e2e_deploy.sh
```

After that, each successful commit to `main` will build and deploy fresh images
tagged with the GitHub commit SHA.

## Notes

- Sync execution reuses the existing Go sync controller and transformer/filter factory.
- Token material is encrypted before persistence in `encrypted_oauth_tokens`.
- Bidirectional loop prevention still relies on existing metadata/source markers in CalendarSync.

# Trademarks

GOOGLE is a trademark of GOOGLE INC. OUTLOOK is a trademark of Microsoft
Corporation

# Relevant RFCs and Links

[RFC 5545](https://datatracker.ietf.org/doc/html/rfc5545)  Internet Calendaring
and Scheduling Core Object Specification (iCalendar) is used in the Google
calendar API to denote recurrence patterns. CalDav [RFC
4791](https://datatracker.ietf.org/doc/html/rfc4791) uses the dateformat
specified in RFC 5545.

# License

MIT
