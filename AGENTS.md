# Agents

For interaction with Google Cloud services, use `gcloud` commands (e.g. deploy Cloud Run services, create Cloud Scheduler jobs, etc.). For every `gcloud` command, use the following flags: `--project open-calendar-sync` to make sure you're using the correct project and account.

The environment is already set up, so no need to run `./deploy/e2e_deploy.sh` again.

For interaction with Neon, the connection string is stored in the `.env` file as `NEON_DB`.

For interaction with Vercel, use `vercel` commands (e.g. deploy the frontend, create a new project, etc.).