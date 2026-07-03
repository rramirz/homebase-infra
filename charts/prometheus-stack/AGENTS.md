# prometheus-stack

Grafana dashboards are organized by the sidecar folder annotation. `values.yaml` sets `kube-prometheus-stack.grafana.sidecar.dashboards.folderAnnotation: grafana_folder` and `provider.foldersFromFilesStructure: true`; bundled kube-prometheus-stack dashboards get `grafana_folder: Kubernetes` through `sidecar.dashboards.annotations`.

Custom dashboards live under `dashboards/` as ConfigMaps labeled `grafana_dashboard: "1"`; set `metadata.annotations.grafana_folder` for folder placement. Current convention: curated homelab dashboards in `Homelab`, CloudNativePG dashboard in `Databases`. `dashboards/node-exporter-full.yaml` is intentionally disabled with `grafana_dashboard_disabled: "1"` unless explicitly re-enabled.

`dashboards/townhome-traffic.yaml` tracks public `townhome.theramirez.casa` with blackbox `job="blackbox-townhome"` for Cloudflare Tunnel edge availability/TLS/latency plus origin-side `townhome_http_*` nginx-log-exporter metrics for per-host traffic. cloudflared metrics are not configured here and would be shared-tunnel aggregate only.
