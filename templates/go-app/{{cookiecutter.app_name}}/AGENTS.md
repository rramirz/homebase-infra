# {{ cookiecutter.app_name }}

- Go app, chart in `chart/`, deploy via `task deploy` (Helm only).
- DB (`use_db={{ cookiecutter.use_db }}`): Crossplane `DB` claim on `home-db`; see homebase-infra `docs/runbooks/app-database.md`.
- Images: `--platform linux/amd64`. LAN/VPN only; never Cloudflare Tunnel without review.
- Keep `ARCHITECTURE.md` updated.
