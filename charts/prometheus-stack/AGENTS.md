# prometheus-stack

Grafana dashboards are organized by the sidecar folder annotation. `values.yaml` sets `kube-prometheus-stack.grafana.sidecar.dashboards.folderAnnotation: grafana_folder` and `provider.foldersFromFilesStructure: true`; bundled kube-prometheus-stack dashboards get `grafana_folder: Kubernetes` through `sidecar.dashboards.annotations`.

Custom dashboards live under `dashboards/` as ConfigMaps labeled `grafana_dashboard: "1"`; set `metadata.annotations.grafana_folder` for folder placement. Current convention: curated homelab dashboards in `Homelab`, CloudNativePG dashboard in `Databases`. `dashboards/node-exporter-full.yaml` is intentionally disabled with `grafana_dashboard_disabled: "1"` unless explicitly re-enabled.

`dashboards/townhome-traffic.yaml` tracks public `townhome.theramirez.casa` with blackbox `job="blackbox-townhome"` for Cloudflare Tunnel edge availability/TLS/latency plus origin-side `townhome_http_*` nginx-log-exporter metrics for per-host traffic. cloudflared metrics are not configured here and would be shared-tunnel aggregate only.

As of 2026-07-13, the Townhome owner view has five panels plus collapsed 16-panel Diagnostics. Visitor stats aggregate away pod/instance churn with `max(...)`; stat targets use instant queries. Visitors are distinct client IPs per UTC day, not exact people. The daily trend samples the finalized yesterday gauge daily, so it reports one day late. Prometheus retention is `3d` at `prometheus.prometheusSpec.retention`. Deploy this ConfigMap with `kubectl apply --server-side=true -n monitoring -f dashboards/townhome-traffic.yaml`. Dated live verification: Site `HEALTHY`, today `4`, yesterday `3`; values are not permanent expectations.
