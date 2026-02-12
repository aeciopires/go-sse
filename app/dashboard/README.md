<!-- TOC -->

- [go-sse Grafana Dashboard](#go-sse-grafana-dashboard)
  - [Dashboard Overview](#dashboard-overview)
    - [Metrics Displayed](#metrics-displayed)
    - [Prometheus Metrics](#prometheus-metrics)
  - [How to Import](#how-to-import)
    - [Using Docker Compose](#using-docker-compose)
    - [Manual Import](#manual-import)
  - [Dashboard Configuration](#dashboard-configuration)
    - [Time Range](#time-range)
    - [Data Source](#data-source)
    - [Panels](#panels)
  - [Customization](#customization)
    - [Example: Modify HTTP Request Threshold](#example-modify-http-request-threshold)
  - [Metrics Explained](#metrics-explained)
    - [sse\_http\_requests\_total](#sse_http_requests_total)
    - [sse\_active\_connections](#sse_active_connections)
    - [sse\_events\_sent\_total](#sse_events_sent_total)
    - [sse\_event\_duration\_seconds](#sse_event_duration_seconds)
  - [Troubleshooting](#troubleshooting)
    - [Dashboard Shows No Data](#dashboard-shows-no-data)
    - [Connection Count Shows 0](#connection-count-shows-0)
    - [Incorrect Time Zone](#incorrect-time-zone)
  - [Exporting and Sharing](#exporting-and-sharing)
  - [Related Documentation](#related-documentation)

<!-- TOC -->

# go-sse Grafana Dashboard

This directory contains a comprehensive Grafana dashboard for monitoring the go-sse application metrics.

## Dashboard Overview

The dashboard visualizes all Prometheus metrics exposed by the go-sse application:

### Metrics Displayed

1. **HTTP Requests Rate (5m window)** - Time series graph showing HTTP requests by path and method
2. **Active SSE Connections** - Gauge showing current number of active SSE connections
3. **Events Sent Rate (5m window)** - Time series showing event sending rate by event type
4. **Average Event Processing Duration** - Stat panel showing mean event processing time
5. **Event Processing Duration Percentiles** - Time series showing p95 and p99 latencies
6. **HTTP Requests by Endpoint** - Stacked bar chart showing request distribution
7. **All HTTP Requests Rate with Statistics** - Detailed time series with mean, max, min, and last values
8. **Event Types Distribution** - Pie chart showing event type breakdown
9. **Total HTTP Requests (5m)** - Stat panel with total request count

### Prometheus Metrics

The dashboard monitors these metrics from go-sse:

- `sse_http_requests_total` - Total HTTP requests counter (labeled by path and method)
- `sse_active_connections` - Active SSE connections gauge (labeled by endpoint)
- `sse_events_sent_total` - Total events sent counter (labeled by event_type)
- `sse_event_duration_seconds` - Event processing duration histogram (labeled by event_type)

## How to Import

### Using Docker Compose

If you're running with docker-compose, the dashboard is automatically configured:

```bash
# Access Grafana
open http://localhost:3000

# Login with default credentials
# Username: admin
# Password: admin

# Go to Dashboards -> Import
# Or use the direct import URL if already uploaded
```

### Manual Import

1. Open Grafana at `http://localhost:3000`
2. Log in with your credentials
3. Click the **+** icon on the left sidebar and select **Import**
4. Either:
   - Paste the JSON from `go-sse-dashboard.json`
   - Upload the file directly
   - Use Grafana's dashboard ID if published to grafana.com

## Dashboard Configuration

### Time Range
- Default: Last 1 hour
- Refreshed every 10 seconds

### Data Source
- Prometheus (must be named "prometheus" in Grafana)

### Panels

All panels are configured with appropriate:
- Legends showing mean, max, min, and last values where relevant
- Color thresholds for alerting visualization
- Tooltips with multi-series support
- Units (seconds for duration metrics)

## Customization

You can customize the dashboard by:

1. **Editing Queries** - Modify PromQL queries in each panel
2. **Adjusting Time Ranges** - Change from 5m to 1h windows as needed
3. **Adding Alerts** - Set alert thresholds on any panel
4. **Changing Visualization** - Switch between chart types

### Example: Modify HTTP Request Threshold

1. Click on the "HTTP Requests Rate (5m window)" panel
2. Edit the query to filter specific paths: `{path="/sse"}`
3. Save the dashboard

## Metrics Explained

### sse_http_requests_total
- **Type**: Counter
- **Labels**: path, method
- **Usage**: Track all incoming HTTP requests
- **PromQL**: `increase(sse_http_requests_total[5m])` for rate

### sse_active_connections
- **Type**: Gauge
- **Labels**: endpoint
- **Usage**: Current number of active SSE connections
- **PromQL**: `sse_active_connections{endpoint="/sse"}`

### sse_events_sent_total
- **Type**: Counter
- **Labels**: event_type (welcome, update)
- **Usage**: Track events sent via SSE
- **PromQL**: `increase(sse_events_sent_total[5m])` for rate

### sse_event_duration_seconds
- **Type**: Histogram with buckets
- **Labels**: event_type
- **Usage**: Measure event processing performance
- **PromQL**: `histogram_quantile(0.95, rate(sse_event_duration_seconds_bucket[5m]))`

## Troubleshooting

### Dashboard Shows No Data
1. Verify Prometheus is scraping go-sse metrics:
   - Visit `http://prometheus:9090/targets`
   - Check if go-sse target is "UP"
2. Confirm metrics are being exported:
   - Visit `http://localhost:8080/metrics`
   - Look for metrics starting with `sse_`
3. Check Grafana data source configuration:
   - Settings -> Data sources -> Prometheus
   - Verify URL is correct: `http://prometheus:9090`

### Connection Count Shows 0
- This is normal if no clients are actively connected
- Open `http://localhost:8080` and click "Connect SSE" to see the gauge update

### Incorrect Time Zone
- Edit dashboard time zone in Grafana settings
- Or adjust PromQL queries to use absolute timestamps

## Exporting and Sharing

To share this dashboard:

1. **Export as JSON**: Click dashboard settings (gear icon) -> Export
2. **Generate URL**: Click share icon and generate public link
3. **Upload to Grafana Cloud**: Use the upload feature

## Related Documentation

- [Prometheus Metrics](../main.go) - Source code with metric definitions
- [Grafana Documentation](https://grafana.com/docs/)
- [PromQL Documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/)
