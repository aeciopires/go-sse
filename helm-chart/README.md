# go-sse

![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)  ![Version: 2.0.0](https://img.shields.io/badge/Version-2.0.0-informational?style=flat-square)

Helm chart for deploying go-sse Server-Sent Events application (requires Helm 4)

# Prerequisites

- **Helm 4** (Helm 3 is not supported). Check with `helm version` (must print `v4.x`).
- Kubernetes 1.29+.
- Install kubectl, helm and helm-docs following the instructions of the file [REQUIREMENTS.md](../REQUIREMENTS.md) (`make install-deps` installs the pinned versions with [mise](https://mise.jdx.dev)).

# Features

- Liveness (`/livez`), readiness (`/readyz`) and startup probes, with graceful shutdown (`shutdown.delay`, `shutdown.timeout`).
- Hardened pod: non-root user, read-only root filesystem, all capabilities dropped, `RuntimeDefault` seccomp.
- JSON logs on stdout with configurable `logLevel` (default `INFO`), ready for Loki.
- Optional OpenTelemetry export of traces and logs (`otel.enabled`, `otel.endpoint`).
- Optional Prometheus Operator `ServiceMonitor` and Grafana dashboard `ConfigMap` (metrics, logs and traces).

Example enabling the observability integrations:

```bash
helm upgrade --install go-sse go-sse/ -n go-sse --create-namespace \
  --set serviceMonitor.enabled=true \
  --set grafanaDashboard.enabled=true \
  --set otel.enabled=true \
  --set otel.endpoint=http://otel-collector.observability:4318
```

# Installing the Chart

> The [Makefile](Makefile) wraps the commands below: `make lint`, `make template`, `make dry-run`, `make install`, `make uninstall`, `make gen-docs`.

- Access a Kubernetes cluster.

- Change the values according to the need of the environment in ``go-sse/values.yaml`` file. The [Parameters](#parameters) section lists the parameters that can be configured during installation.

- Test the installation with command:

```bash
helm upgrade --install go-sse -f go-sse/values.yaml go-sse/ -n go-sse --create-namespace --dry-run
```

- To install/upgrade the chart with the release name `go-sse`:

```bash
helm upgrade --install go-sse -f go-sse/values.yaml go-sse/ -n go-sse --create-namespace
```

Create a port-forward with the follow command:

```bash
kubectl port-forward svc/go-sse 8080:80 -n go-sse
```

Open the browser and access the URL: http://localhost:8080

## Uninstalling the Chart

To uninstall/delete the `go-sse` deployment:

```bash
helm uninstall go-sse -n go-sse
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Parameters

The following tables lists the configurable parameters of the chart and their default values.

Change the values according to the need of the environment in ``go-sse/values.yaml`` file.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity configurations |
| autoscaling | object | `{"enabled":false,"maxReplicas":20,"minReplicas":1,"targetCPUUtilizationPercentage":80}` | Auto scaling configurations |
| env | list | `[]` | Extra environment variables for the container |
| extraManifests | list | `[]` | Extra arbitrary Kubernetes manifests to deploy within the release |
| fullnameOverride | string | `""` |  |
| grafanaDashboard | object | `{"annotations":{"grafana_folder":"go-sse"},"enabled":false,"labels":{"grafana_dashboard":"1"},"namespace":""}` | Grafana dashboard (metrics, logs and traces) as a ConfigMap discovered by the Grafana sidecar |
| grafanaDashboard.annotations | object | `{"grafana_folder":"go-sse"}` | Annotations, e.g. the Grafana folder used by the sidecar |
| grafanaDashboard.labels | object | `{"grafana_dashboard":"1"}` | Labels used by the Grafana sidecar to discover dashboards |
| grafanaDashboard.namespace | string | `""` | Namespace of the ConfigMap (defaults to the release namespace) |
| image.pullPolicy | string | `"IfNotPresent"` | Pull policy of Docker image |
| image.repository | string | `"aeciopires/go-sse"` | Docker image name |
| image.tag | string | `"2.0.0"` | Docker image tag (defaults to the chart appVersion) |
| imagePullSecrets | list | `[]` | List of dockerconfig secrets names to use when pulling images |
| ingress.annotations | string | `nil` | Ingress annotations |
| ingress.certManagerIssueEmail | string | `"CHANGE_HERE"` | Email for Cert Manager Issue |
| ingress.className | string | `""` |  |
| ingress.createCertManagerIssuer | bool | `false` | Create Cert Manager Issuer |
| ingress.createGcpManagedCertificate | bool | `false` | Create GCP Managed Certificate |
| ingress.enabled | bool | `false` | Enables Ingress |
| ingress.hosts | list | `[]` | Ingress hosts |
| ingress.tls | list | `[]` | Ingress TLS configuration |
| livenessProbe | object | `{"failureThreshold":3,"initialDelaySeconds":0,"path":"/livez","periodSeconds":10,"successThreshold":1,"timeoutSeconds":3}` | Liveness probe: the container is restarted when it fails |
| livenessProbe.failureThreshold | int | `3` | When a probe fails, Kubernetes will try failureThreshold times before giving up. Giving up in case of liveness probe means restarting the container. In case of readiness probe the Pod will be marked Unready |
| livenessProbe.initialDelaySeconds | int | `0` | Number of seconds after the container has started before liveness |
| livenessProbe.path | string | `"/livez"` | Path of liveness check of application |
| livenessProbe.periodSeconds | int | `10` | Specifies that the kubelet should perform a liveness probe every N seconds |
| livenessProbe.successThreshold | int | `1` | Minimum consecutive successes for the probe to be considered successful after having failed |
| livenessProbe.timeoutSeconds | int | `3` | Number of seconds after which the probe times out |
| logLevel | string | `"INFO"` | Log level of the application: DEBUG, INFO, WARN or ERROR. Logs are written to stdout in JSON format. |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` | Node selector configurations |
| otel.enabled | bool | `false` | Enables OTLP export of traces and logs |
| otel.endpoint | string | `""` | OTLP/HTTP endpoint, e.g. an OpenTelemetry Collector or Grafana Alloy (http://otel-collector.observability:4318) |
| otel.resourceAttributes | string | `""` | Extra OTEL resource attributes (comma-separated key=value) |
| otel.serviceName | string | `""` | Service name reported in traces and logs (defaults to the chart name) |
| podAnnotations | object | `{}` | Pod annotations configurations |
| podDisruptionBudget | object | `{"enabled":false,"minAvailable":"25%"}` | Pod Disruption Budget configuration |
| podDisruptionBudget.minAvailable | string | `"25%"` | Minimum available pods |
| podLabels | object | `{}` | Extra pod labels |
| podSecurityContext | object | `{"fsGroup":10001,"runAsGroup":10001,"runAsNonRoot":true,"runAsUser":10001,"seccompProfile":{"type":"RuntimeDefault"}}` | Pod security configurations |
| readinessProbe | object | `{"failureThreshold":3,"initialDelaySeconds":0,"path":"/readyz","periodSeconds":5,"successThreshold":1,"timeoutSeconds":3}` | Readiness probe: the pod receives traffic only while it returns 200 (fails during shutdown) |
| readinessProbe.failureThreshold | int | `3` | When a probe fails, Kubernetes will try failureThreshold times before giving up. Giving up in case of liveness probe means restarting the container. In case of readiness probe the Pod will be marked Unready |
| readinessProbe.initialDelaySeconds | int | `0` | Number of seconds after the container has started before readiness |
| readinessProbe.path | string | `"/readyz"` | Path of readiness check of application |
| readinessProbe.periodSeconds | int | `5` | Specifies that the kubelet should perform a readiness probe every N seconds |
| readinessProbe.successThreshold | int | `1` | Minimum consecutive successes for the probe to be considered successful after having failed |
| readinessProbe.timeoutSeconds | int | `3` | Number of seconds after which the probe times out |
| replicaCount | int | `2` | Number of replicas. Used if autoscaling is false |
| resources | object | `{"limits":{"cpu":"200m","memory":"256Mi"},"requests":{"cpu":"10m","memory":"32Mi"}}` | Requests and limits of pod resources. See: [https://kubernetes.io/docs/concepts/configuration/manage-resources-containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers) |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true}` | Container security context configurations |
| service.annotations | object | `{}` | Annotations of service |
| service.createGcpBackendAndFrontendConfig | bool | `false` | Create GCP Backend and Frontend Config |
| service.port | int | `80` | Port of service in Kubernetes cluster |
| service.type | string | `"NodePort"` | Type of service in Kubernetes cluster |
| serviceMonitor | object | `{"additionalLabels":{},"enabled":false,"interval":"30s","namespace":"go-sse","namespaceSelector":{},"path":"/metrics","scrapeTimeout":"10s"}` | Service monitor configurations |
| shutdown.delay | string | `"5s"` | Time the pod keeps serving after readiness turns false, so endpoints/load balancers stop routing traffic (Go duration) |
| shutdown.timeout | string | `"15s"` | Maximum time for in-flight requests to finish during shutdown (Go duration) |
| sse.eventInterval | string | `"2s"` | Interval between "update" SSE events (Go duration) |
| startupProbe | object | `{"failureThreshold":30,"path":"/livez","periodSeconds":2,"timeoutSeconds":2}` | Startup probe: protects slow starts; liveness/readiness only run after it succeeds. Set to null to disable |
| startupProbe.failureThreshold | int | `30` | Number of failures before the container is restarted |
| startupProbe.path | string | `"/livez"` | Path of the startup check |
| startupProbe.periodSeconds | int | `2` | Specifies that the kubelet should perform the probe every N seconds |
| startupProbe.timeoutSeconds | int | `2` | Number of seconds after which the probe times out |
| terminationGracePeriodSeconds | int | `30` | Seconds Kubernetes waits before killing the pod. Must be greater than shutdown.delay + shutdown.timeout |
| tolerations | list | `[]` | Tolerations configurations |
| updateStrategy | object | `{"rollingUpdate":{"maxSurge":6,"maxUnavailable":0},"type":"RollingUpdate"}` | Update strategy configurations |
