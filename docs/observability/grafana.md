# Grafana Dashboards

Pre-configured Grafana instance with auto-provisioned datasources and dashboards.

---

## Access

| Property | Value |
|---|---|
| **URL** | [http://localhost:3000](http://localhost:3000) |
| **User** | `admin` |
| **Password** | `admin` |
| **Anonymous access** | Enabled (Viewer role) |

---

## Datasources (Auto-provisioned)

| Name | Type | URL |
|---|---|---|
| Prometheus | Prometheus | `http://prometheus:9090` |
| Loki | Loki | `http://loki:3100` |

Configured in `configs/grafana/provisioning/datasources/datasources.yml`.

---

## EntradasQR Dashboard

A pre-built dashboard is auto-loaded from `configs/grafana/dashboards/entradas-qr.json`.

### Panels

| Panel | Type | Metric | Description |
|---|---|---|---|
| **Request Rate** | Time series | `http_requests_total` | Requests/sec by service |
| **P95 Latency** | Time series | `http_request_duration_seconds` | 95th percentile latency |
| **Error Rate** | Stat | `http_requests_total{status=~"5.."}` | 5xx error percentage |
| **Events Created** | Stat | `events_created_total` | Total events created |
| **Tickets Purchased** | Stat | `tickets_purchased_total` | Total tickets purchased |
| **Validation Results** | Pie chart | `tickets_validated_total` | Valid vs invalid ratio |
| **RabbitMQ Published** | Time series | `rabbitmq_events_published_total` | Events published/sec |
| **RabbitMQ Consumed** | Time series | `rabbitmq_events_consumed_total` | Events consumed/sec |

### Layout

```
┌─────────────────┬─────────────────┬─────────────────┐
│  Request Rate   │   P95 Latency   │   Error Rate    │
├─────────────────┼─────────────────┼─────────────────┤
│ Events Created  │ Tickets Bought  │ Validation Rate │
├─────────────────┴─────────────────┴─────────────────┤
│              RabbitMQ Published / Consumed           │
└─────────────────────────────────────────────────────┘
```

---

## Provisioning Structure

```
configs/grafana/
├── dashboards/
│   └── entradas-qr.json           # Dashboard definition
└── provisioning/
    ├── dashboards/
    │   └── dashboards.yml          # Dashboard provider config
    └── datasources/
        └── datasources.yml         # Prometheus + Loki sources
```

### Dashboard Provider

```yaml
# configs/grafana/provisioning/dashboards/dashboards.yml
apiVersion: 1
providers:
  - name: 'default'
    folder: ''
    type: file
    options:
      path: /var/lib/grafana/dashboards
```

---

## Adding New Panels

1. Open Grafana at `http://localhost:3000`
2. Edit the existing dashboard or create a new one
3. Export the dashboard JSON via **Share → Export → Save to file**
4. Replace `configs/grafana/dashboards/entradas-qr.json` with the exported file
5. Restart Grafana: `docker compose restart grafana`
