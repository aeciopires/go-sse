#!/usr/bin/env python3
"""Generates go-sse-dashboard.json (Grafana dashboard: metrics, logs, traces).

Edit this file instead of the JSON, then run:

    python3 dashboard/generate.py

Data sources are dashboard variables, so the dashboard can be imported into
any Grafana that has Prometheus, Loki and Tempo data sources.
"""

import json
import pathlib

PROM = {"type": "prometheus", "uid": "${prometheus_ds}"}
LOKI = {"type": "loki", "uid": "${loki_ds}"}
TEMPO = {"type": "tempo", "uid": "${tempo_ds}"}

JOB = 'job=~"$job"'
SVC = '{service_name="$service"}'

_next_id = 0
_y = 0


def _id():
    global _next_id
    _next_id += 1
    return _next_id


def prom(expr, legend="", exemplar=False, instant=False, ref="A"):
    return {
        "datasource": PROM,
        "refId": ref,
        "expr": expr,
        "legendFormat": legend,
        "exemplar": exemplar,
        "instant": instant,
        "range": not instant,
    }


def loki(expr, ref="A", legend="", qtype="range"):
    return {"datasource": LOKI, "refId": ref, "expr": expr, "legendFormat": legend, "queryType": qtype}


def tempo(query, ref="A", limit=20):
    return {
        "datasource": TEMPO,
        "refId": ref,
        "queryType": "traceql",
        "query": query,
        "limit": limit,
        "tableType": "traces",
    }


def row(title):
    global _y
    panel = {
        "type": "row",
        "id": _id(),
        "title": title,
        "collapsed": False,
        "gridPos": {"h": 1, "w": 24, "x": 0, "y": _y},
        "panels": [],
    }
    _y += 1
    return panel


def panel(kind, title, targets, x, w, h, ds=PROM, desc="", unit=None, options=None, field=None):
    defaults = {"unit": unit} if unit else {}
    defaults.update(field or {})
    return {
        "type": kind,
        "id": _id(),
        "title": title,
        "description": desc,
        "datasource": ds,
        "targets": targets,
        "gridPos": {"h": h, "w": w, "x": x, "y": _y},
        "fieldConfig": {"defaults": defaults, "overrides": []},
        "options": options or {},
    }


def advance(h):
    global _y
    _y += h


TS_OPTS = {
    "legend": {"displayMode": "table", "placement": "bottom", "calcs": ["mean", "max", "lastNotNull"]},
    "tooltip": {"mode": "multi", "sort": "desc"},
}
TS_FIELD = {"custom": {"lineWidth": 2, "fillOpacity": 10, "showPoints": "never"}}
STAT_OPTS = {
    "reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
    "colorMode": "background",
    "graphMode": "area",
    "textMode": "auto",
}


def stat(title, target, x, unit=None, desc="", thresholds=None, opts=None):
    field = {
        "color": {"mode": "thresholds"},
        "thresholds": {
            "mode": "absolute",
            "steps": thresholds or [{"color": "purple", "value": None}],
        },
    }
    return panel("stat", title, [target], x, 4, 4, desc=desc, unit=unit,
                 options={**STAT_OPTS, **(opts or {})}, field=field, ds=target["datasource"])


def timeseries(title, targets, x, w, unit=None, desc="", ds=PROM, h=8, field=None):
    f = json.loads(json.dumps(TS_FIELD))
    f.update(field or {})
    return panel("timeseries", title, targets, x, w, h, ds=ds, desc=desc, unit=unit, options=TS_OPTS, field=f)


def build():
    panels = []

    # ------------------------------------------------------------ Overview
    panels.append(row("Overview"))
    panels += [
        stat("Active SSE connections", prom(f"sum(sse_active_connections{{{JOB}}})"), 0,
             desc="Open /sse streams right now (sse_active_connections)."),
        stat("Events sent / s", prom(f"sum(rate(sse_events_sent_total{{{JOB}}}[$__rate_interval]))"), 4,
             unit="ops", desc="SSE events written to clients per second."),
        stat("HTTP requests / s", prom(f'sum(rate(sse_http_requests_total{{{JOB},path!="/metrics"}}[$__rate_interval]))'), 8,
             unit="reqps", desc="Incoming HTTP requests per second (scrapes excluded)."),
        stat("HTTP 5xx ratio",
             prom(f'(sum(rate(sse_http_request_duration_seconds_count{{{JOB},code=~"5.."}}[$__rate_interval])) or vector(0))'
                  f' / clamp_min(sum(rate(sse_http_request_duration_seconds_count{{{JOB}}}[$__rate_interval])), 1e-9)'),
             12, unit="percentunit",
             thresholds=[{"color": "green", "value": None}, {"color": "orange", "value": 0.01}, {"color": "red", "value": 0.05}]),
        stat("Error logs (range)",
             loki(f'sum(count_over_time({SVC} | detected_level="error" [$__range])) or vector(0)', qtype="instant"),
             16, desc="Number of ERROR log lines in the selected time range (Loki).",
             thresholds=[{"color": "green", "value": None}, {"color": "red", "value": 1}],
             opts={"graphMode": "none"}),
        stat("Version", prom(f"sse_build_info{{{JOB}}}", legend="{{version}}", instant=True), 20,
             opts={"textMode": "name", "colorMode": "none", "graphMode": "none"}),
    ]
    advance(4)

    # ------------------------------------------------------------ Metrics
    panels.append(row("Metrics (Prometheus)"))
    panels += [
        timeseries("HTTP request rate by path", [
            prom(f'sum by (path, method) (rate(sse_http_requests_total{{{JOB}}}[$__rate_interval]))', "{{method}} {{path}}"),
        ], 0, 12, unit="reqps"),
        timeseries("HTTP latency p95 by path (with exemplars)", [
            prom(f'histogram_quantile(0.95, sum by (le, path) (rate(sse_http_request_duration_seconds_bucket{{{JOB}}}[$__rate_interval])))',
                 "p95 {{path}}", exemplar=True),
        ], 12, 12, unit="s", desc="Diamonds are exemplars: click one to open the trace in Tempo."),
    ]
    advance(8)
    panels += [
        timeseries("Active SSE connections", [
            prom(f"sum(sse_active_connections{{{JOB}}})", "connections"),
        ], 0, 8),
        timeseries("Events sent rate by type", [
            prom(f"sum by (event_type) (rate(sse_events_sent_total{{{JOB}}}[$__rate_interval]))", "{{event_type}}"),
        ], 8, 8, unit="ops"),
        panel("piechart", "Event types distribution", [
            prom(f"sum by (event_type) (increase(sse_events_sent_total{{{JOB}}}[$__range]))", "{{event_type}}", instant=True),
        ], 16, 8, 8, options={"legend": {"displayMode": "table", "placement": "right", "values": ["value", "percent"]},
                               "pieType": "donut", "reduceOptions": {"calcs": ["lastNotNull"]}}),
    ]
    advance(8)
    panels += [
        timeseries("Event processing duration (with exemplars)", [
            prom(f'histogram_quantile(0.50, sum by (le) (rate(sse_event_duration_seconds_bucket{{{JOB}}}[$__rate_interval])))', "p50", exemplar=True),
            prom(f'histogram_quantile(0.95, sum by (le) (rate(sse_event_duration_seconds_bucket{{{JOB}}}[$__rate_interval])))', "p95", ref="B"),
            prom(f'histogram_quantile(0.99, sum by (le) (rate(sse_event_duration_seconds_bucket{{{JOB}}}[$__rate_interval])))', "p99", ref="C"),
            prom(f'sum(rate(sse_event_duration_seconds_sum{{{JOB}}}[$__rate_interval])) / sum(rate(sse_event_duration_seconds_count{{{JOB}}}[$__rate_interval]))', "avg", ref="D"),
        ], 0, 12, unit="s", desc="Time to encode, write and flush one SSE event."),
        timeseries("SSE session duration", [
            prom(f'histogram_quantile(0.50, sum by (le) (rate(sse_session_duration_seconds_bucket{{{JOB}}}[$__rate_interval])))', "p50"),
            prom(f'histogram_quantile(0.95, sum by (le) (rate(sse_session_duration_seconds_bucket{{{JOB}}}[$__rate_interval])))', "p95", ref="B"),
        ], 12, 12, unit="s", desc="How long clients stay connected (observed on disconnect)."),
    ]
    advance(8)
    panels += [
        timeseries("Go runtime: goroutines", [prom(f"sum by (instance) (go_goroutines{{{JOB}}})", "{{instance}}")], 0, 8),
        timeseries("Go runtime: heap in use", [prom(f"sum by (instance) (go_memstats_heap_inuse_bytes{{{JOB}}})", "{{instance}}")], 8, 8, unit="bytes"),
        timeseries("Process CPU", [prom(f"sum by (instance) (rate(process_cpu_seconds_total{{{JOB}}}[$__rate_interval]))", "{{instance}}")], 16, 8, unit="percentunit"),
    ]
    advance(8)

    # ------------------------------------------------------------ Logs
    panels.append(row("Logs (Loki)"))
    panels.append(timeseries("Log volume by level", [
        loki(f'sum by (detected_level) (count_over_time({SVC} | detected_level=~"$level" [$__auto]))', legend="{{detected_level}}"),
    ], 0, 24, ds=LOKI, h=7, field={"custom": {"drawStyle": "bars", "fillOpacity": 80, "stacking": {"mode": "normal"}, "lineWidth": 1}}))
    advance(7)
    panels.append(panel("logs", "Application logs", [
        loki(f'{SVC} | detected_level=~"$level" |= `$search`'),
    ], 0, 24, 12, ds=LOKI, desc="JSON logs of go-sse. Expand a line and use the trace_id link to open the trace.",
        options={"showTime": True, "wrapLogMessage": True, "sortOrder": "Descending", "enableLogDetails": True,
                 "prettifyLogMessage": False, "dedupStrategy": "none"}))
    advance(12)
    panels.append(panel("logs", "Warnings and errors", [
        loki(f'{SVC} | detected_level=~"warn|error"'),
    ], 0, 24, 8, ds=LOKI, options={"showTime": True, "wrapLogMessage": True, "sortOrder": "Descending", "enableLogDetails": True}))
    advance(8)

    # ------------------------------------------------------------ Traces
    panels.append(row("Traces (Tempo)"))
    panels += [
        timeseries("Span rate by operation (span metrics)", [
            prom('sum by (span_name) (rate(traces_spanmetrics_calls_total{service="$service"}[$__rate_interval]))', "{{span_name}}"),
        ], 0, 12, unit="ops", desc="Generated by Tempo's metrics-generator from the received spans."),
        timeseries("Span latency p95 by operation", [
            prom('histogram_quantile(0.95, sum by (le, span_name) (rate(traces_spanmetrics_latency_bucket{service="$service"}[$__rate_interval])))', "{{span_name}}"),
        ], 12, 12, unit="s"),
    ]
    advance(8)
    panels += [
        panel("table", "Recent traces", [tempo('{resource.service.name="$service"}')], 0, 12, 10, ds=TEMPO,
              desc="Click a trace ID to open it. Each SSE event is its own small trace linked to the connection span."),
        panel("table", "Error traces", [tempo('{resource.service.name="$service" && status=error}')], 12, 12, 10, ds=TEMPO),
    ]
    advance(10)
    panels.append(timeseries("Service graph: request rate (client → server)", [
        prom('sum by (client, server) (rate(traces_service_graph_request_total{server="$service"}[$__rate_interval]))',
             "{{client}} → {{server}}"),
    ], 0, 24, unit="reqps", desc="Tempo service-graph metrics. The full node graph is under Explore → Tempo → Service Graph."))
    advance(8)

    def ds_var(name, label, kind):
        return {"name": name, "label": label, "type": "datasource", "query": kind, "current": {}, "hide": 0,
                "refresh": 1, "regex": "", "includeAll": False, "multi": False}

    return {
        "title": "go-sse - Metrics, Logs and Traces",
        "uid": "go-sse-monitoring",
        "description": "Observability for the go-sse Server-Sent Events application (Prometheus, Loki, Tempo).",
        "tags": ["go-sse", "sse", "prometheus", "loki", "tempo"],
        "timezone": "browser",
        "editable": True,
        "graphTooltip": 1,
        "refresh": "10s",
        "time": {"from": "now-30m", "to": "now"},
        "schemaVersion": 41,
        "version": 1,
        "links": [],
        "annotations": {"list": []},
        "templating": {"list": [
            ds_var("prometheus_ds", "Prometheus", "prometheus"),
            ds_var("loki_ds", "Loki", "loki"),
            ds_var("tempo_ds", "Tempo", "tempo"),
            {"name": "job", "label": "Job", "type": "query", "datasource": PROM,
             "query": {"query": "label_values(sse_build_info, job)", "refId": "job"},
             "definition": "label_values(sse_build_info, job)",
             # No allValue: "All" expands to the jobs exposing sse_build_info only.
             "includeAll": True, "multi": True, "refresh": 2, "current": {}, "hide": 0},
            {"name": "service", "label": "Service name", "type": "textbox", "query": "go-sse",
             "current": {"text": "go-sse", "value": "go-sse"}, "hide": 0},
            {"name": "level", "label": "Log level", "type": "custom", "query": "debug,info,warn,error",
             "includeAll": True, "multi": True, "allValue": ".*",
             "current": {"text": ["All"], "value": ["$__all"]}, "hide": 0},
            {"name": "search", "label": "Log search", "type": "textbox", "query": "",
             "current": {"text": "", "value": ""}, "hide": 0},
        ]},
        "panels": panels,
    }


if __name__ == "__main__":
    out = pathlib.Path(__file__).with_name("go-sse-dashboard.json")
    out.write_text(json.dumps(build(), indent=2) + "\n")
    print(f"wrote {out}")
