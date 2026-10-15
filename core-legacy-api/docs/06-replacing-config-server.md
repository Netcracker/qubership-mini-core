# Replacing config-server

`core-legacy-api` replaces config-server and keeps its REST API paths, the Consul key layout, and the `config-server`
Kubernetes Service, so clients keep the same URLs. A deployment that puts `core-legacy-api` in place of config-server
still has to account for what changes around the API: resource names, Consul access, Helm parameters, removed
endpoints, some API behavior, metrics, health endpoints, and tracing.

The page compares `core-legacy-api` with config-server 4.2.x running with `CONSUL_ENABLED=true`. `core-legacy-api`
reads the data config-server left in Consul as is.

## What to check

| Check | Section |
| --- | --- |
| RBAC bindings, label selectors, scripts, and log and metric filters that use the `config-server` Deployment, pod, container, or ServiceAccount name, or the PodMonitor labels | [Kubernetes resources](#kubernetes-resources) |
| The logical database of config-server in DBaaS | [Kubernetes resources](#kubernetes-resources) |
| Consul ACL binding rules for the `config-server` ServiceAccount, and log level overrides under `logging/<namespace>/` | [Consul access](#consul-access) |
| Helm values that still set removed parameters | [Helm parameters](#helm-parameters) |
| Calls to removed endpoints | [Endpoints](#endpoints) |
| Clients that depend on error codes, locking, defaults, or the exact text of the formatted output | [API behavior](#api-behavior) |
| Dashboards and alerts built on config-server metrics | [Metrics](#metrics) |
| Probes and checks on `/health` and `/health/readiness` | [Health](#health) |
| The tracing collector, the sampling setting, and trace headers | [Tracing](#tracing) |

## Kubernetes resources

The Deployment, its pods and container, the ServiceAccount, the HPA, and the PodMonitor are named after
`SERVICE_NAME`, which defaults to `core-legacy-api`. Anything bound to the old names stops matching: RBAC bindings,
selectors, log and metric filters on `container` or `pod`.

The Service and the email secret keep the config-server names, so the two charts cannot be installed as separate
releases in one namespace. The `config-server` Service selects only `core-legacy-api` pods.

| Resource | config-server | `core-legacy-api` |
| --- | --- | --- |
| Service | `config-server` | `config-server` |
| Email secret | `config-server-email-secret` | `config-server-email-secret` |
| Consul token secret | `config-server-consul-token`, required | `config-server-consul-token`, optional |
| Deployment, pods, container, ServiceAccount, HPA | `config-server` | `core-legacy-api` |
| PodMonitor | `config-server-pod-monitor` | `core-legacy-api-pod-monitor` |
| PodMonitor label `app.kubernetes.io/part-of` | `Cloud-Core` | `platform-monitoring` |
| PodMonitor label `app.kubernetes.io/managed-by` | `saasDeployer` | `platform-monitoring-operator` |
| PodMonitor labels `app.kubernetes.io/processed-by-operator` and `deployment.netcracker.com/sessionId` | Set | Not set |
| Label `app.kubernetes.io/technology` | `java-springboot` | `go` |
| DBaaS `DbPolicy` and secret `dbaas-config-server-credential-secret` | Created | Not created |

The logical database that config-server used in DBaaS is not used by `core-legacy-api`, and nothing deletes it.

## Consul access

Properties are read and written with the static token from the `config-server-consul-token` secret, as in
config-server. `CONSUL_AUTH_MODE` and the other `CONSUL_AUTH_*` parameters apply only to reading log levels. With the
Kubernetes auth method, the service logs in with the `core-legacy-api` ServiceAccount, so a binding rule that matches
the `config-server` ServiceAccount name does not apply to it.

The service reads its log levels only from `logging/<namespace>/core-legacy-api`. Overrides stored under
`logging/<namespace>/config-server` and `logging/<namespace>/application` no longer apply to it. It does not read its
own settings from `config/<namespace>/config-server` or `config/<namespace>/application`; those properties are still
served to clients.

## Helm parameters

The chart schema allows additional properties, so a removed parameter left in the Helm values is ignored without an
error.

| Parameter | Change | Note |
| --- | --- | --- |
| `API_DBAAS_ADDRESS`, `DBAAS_CLUSTER_DBA_CREDENTIALS_USERNAME`, `DBAAS_CLUSTER_DBA_CREDENTIALS_PASSWORD` | Removed | PostgreSQL storage through DBaaS is not supported. |
| `CONSUL_ENABLED` | Removed | Consul is always used, and `CONSUL_URL` is required. |
| `CONSUL_ADMIN_TOKEN`, `CONSUL_PUBLIC_URL` | Removed | Had no effect in config-server. |
| `MEM_ARGS`, `NC_DIAGNOSTIC_MODE`, `NC_DIAGNOSTIC_AGENT_SERVICE` | Removed | JVM options and the Java profiler agent. |
| `TRACING_SAMPLER_PROBABILISTIC` | Removed | Replaced by `TRACING_SAMPLER_RATELIMITING`; see [Tracing](#tracing). |
| `TRACING_SAMPLER_RATELIMITING` | New | Largest number of traces per second, a whole number, default 10. |
| `MEMORY_REQUEST` | New | Memory request of the container. config-server used `MEMORY_LIMIT` for both the request and the limit. |
| `PROBE_STARTUP_*`, `PROBE_LIVENESS_*`, `PROBE_READINESS_*` | New | Probe timings; `-1` keeps the value from the chart template. |

The resource profiles set 80Mi to 100Mi of memory and 100m of CPU for both the request and the limit, where
config-server had 360Mi to 435Mi of memory and a CPU request of 100m with a limit of 2. The HPA target is
`HPA_AVG_CPU_UTILIZATION_TARGET_PERCENT` × `CPU_LIMIT` / `CPU_REQUEST` of the CPU request, so with the profile values
`core-legacy-api` scales out at 75% of 100m per pod, where config-server scaled out at 1500% of 100m.

## Endpoints

All endpoints that read and write properties keep their paths. `/health/liveness` answers as `/health`, and
`/health/readiness` answers as `/readiness`. The endpoints below are gone. A GET request with two or three path
segments matches `/{application}/{profile}[/{label}]` and, while Consul is reachable, returns 200 with the merged
properties of an application named after the first segment. See [Removed API](01-removed-api.md).

| Endpoint | In config-server | Response from `core-legacy-api` |
| --- | --- | --- |
| `POST /api/v1/migrate` | Copied properties from PostgreSQL to Consul | 405 |
| `POST /encrypt`, `POST /decrypt`, `GET /key` | Returned errors: no key was installed | 404 |
| `GET /encrypt/status`, `GET /key/{name}/{profiles}` | `/encrypt/status` returned `NO_KEY` | 200 with properties |
| `POST /encrypt/{name}/{profiles}`, `POST /decrypt/{name}/{profiles}` | Returned errors | 405 |
| `GET /{name}/{profile}/{label}/**` | Did not work | 404 |
| `GET /metrics` | Spring Boot Actuator metrics | 404 |
| `GET /metrics/{name}` | One Actuator metric | 200 with properties |
| `GET /health/{component}` | One Actuator health component | 200 with properties |
| `GET /v3/api-docs` | OpenAPI 3 specification from springdoc | 200 with properties |

New endpoints:

| Endpoint | Returns |
| --- | --- |
| `GET /readiness` | Readiness with the Consul check; see [Health](#health) |
| `GET /api-version` | Supported API versions |
| `GET /swagger-ui/swagger.json` | Swagger 2.0 specification |

## API behavior

| Case | config-server | `core-legacy-api` |
| --- | --- | --- |
| Write of conflicting keys such as `db` and `db.url` | 400 `Property keys with errors`, checked against the whole profile | 201, both keys are stored; see [Conflicting property keys](02-conflicting-property-keys.md) |
| Write to a profile that already stores a key starting with a dot | 400 for any write to that profile | Only the keys in the request are checked |
| Concurrent writes to one application and profile | Lock; after 1 minute, 504 `Aborted write properties ...` | No lock, no 504; see [Why writes take no lock](03-why-writes-take-no-lock.md) |
| Default property deleted through the API | Stays deleted | Restored at the next start; see [Startup initialization](00-startup-initialization.md) |
| Array index above 10000, such as `list[20000]`, in `.json` and `.yaml` | A list padded with `null` | The flat key `list[20000]`; see [Array keys](04-array-keys.md) |
| `list` and `list[0]` in one profile, in `.json` and `.yaml` | 500 | 200, flat keys; see [Array keys](04-array-keys.md) |
| Consul unreachable on a property read | 200 with an empty property source | 500 with a JSON error body |
| `Content-Type` other than JSON on `POST` or `PUT /{application}/{profile}` and on `properties-delete` | 415 | 400 |
| `Accept: application/vnd.spring-cloud.config-server.v2+json` | Response with that `Content-Type` | The same body with `application/json` |
| `GET /applications` | Unordered; also lists applications of namespaces whose name starts with this one, such as `dev2` for `dev` | Sorted; only this namespace |
| `.properties` output | Unordered, array keys last, no final newline | Sorted by key, final newline |
| `.json` output | Unordered, arrays last | Keys in alphabetical order |
| `.yaml` output | Indent of 2, list items at the level of their key | Indent of 4, list items indented |

### Placeholders

Placeholders in `.json`, `.yaml`, and `.properties` output resolve as in config-server except for these cases:

| Case | config-server | `core-legacy-api` |
| --- | --- | --- |
| Nested placeholders such as `${a:${b}}` and `${${x}}` | Resolved | Not resolved |
| Escaped `\${x}` | The literal `${x}` | The value of `x`, with the backslash kept |
| Spaces, as in `${ x }` | Looks up `x` in `.json` and `.yaml`; left unresolved in `.properties` | Looks up ` x ` with the spaces in every format |
| Placeholders in keys of `.properties` output | Resolved | Not resolved; only values are |
| Key `spring.profiles` | Removed from the output | Returned |
| YAML with a single top-level key `document` | The value of `document` without the key | Returned as stored |

## Metrics

Metrics stay at `/prometheus`, but their names and labels change, so dashboards and alerts built on config-server
metrics show no data.

| config-server | `core-legacy-api` |
| --- | --- |
| `http_server_requests_seconds_count` | `http_request_status`, a counter |
| `http_server_requests_seconds_sum` | `http_request_time_sum`, a histogram in seconds with buckets 0.2, 0.8, and 1.0 |
| `http_server_requests_seconds_max` | None |
| Label `uri`, the Spring request mapping | Label `path`, the route, such as `/:application/:profile` |
| Label `method` in upper case | Label `method` in lower case |
| Labels `outcome` and `exception` | None |
| `jvm_*` | None; the Go runtime exports `go_*` |
| Label `container="config-server"`, job `config-server-pod-monitor` | Label `container="core-legacy-api"`, job `core-legacy-api-pod-monitor` |

`http_request_status` and `http_request_time` count only requests that the service completes without an error. A
response that the error handler produces, such as a 400, 404, or 500 with a JSON error body, is not counted. Requests
to `/health`, `/readiness`, and `/prometheus` are counted, where config-server excluded them.

A request rate per endpoint, before and after:

```promql
sum by (uri) (rate(http_server_requests_seconds_count{container="config-server"}[5m]))
sum by (path) (rate(http_request_status{container="core-legacy-api"}[5m]))
```

## Health

| Endpoint | config-server | `core-legacy-api` |
| --- | --- | --- |
| `/health` | Spring Boot Actuator, with components such as `consul` and `diskSpace`: `{"status":...,"components":{...}}` | Always `{"status":"UP"}`, no checks |
| `/health/liveness` | Actuator group with only `livenessState` | The same response as `/health` |
| `/health/readiness` | Actuator group with only `readinessState`: 200 after startup, whatever the state of Consul | The same response as `/readiness`: 503 while Consul is unreachable |
| `/readiness` | None | `{"status":...,"ConsulCheck":{"status":...}}`; `PROBLEM` returns 503, `WARNING` returns 200 |

The chart points the readiness probe at `/readiness` and the liveness and startup probes at `/health`. When Consul
becomes unreachable after startup, pods leave the Service and are not restarted. When Consul is unreachable at
startup, the service exits and the pod restarts until Consul is reachable. A check that used config-server `/health`
to see whether Consul is reachable has to call `/readiness` or `/health/readiness`.

## Tracing

| Setting | config-server | `core-legacy-api` |
| --- | --- | --- |
| Collector | OpenTelemetry OTLP at `http://<TRACING_HOST>:4318/v1/traces` | Zipkin at `http://<TRACING_HOST>:9411/api/v2/spans` |
| Sampling | A share of requests set by `TRACING_SAMPLER_PROBABILISTIC`, default 0.01 | At most `TRACING_SAMPLER_RATELIMITING` traces per second, default 10 |
| Incoming trace headers | B3 and W3C `traceparent` | B3 only; a request with only `traceparent` starts a new trace |
| Requests to probe and metrics paths | Not traced | Traced, except `/health` |
| Service name | `config-server-<namespace>` | `core-legacy-api-<namespace>` |
