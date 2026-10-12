# qubership-mini-core

`core-legacy-api` serves the config-server REST API and stores the properties in Consul. A service that reads its
configuration from config-server can read it from `core-legacy-api` at the same paths. The differences are listed in
[API differences](#api-differences).

## How properties are stored

Each application and profile is a Consul KV prefix: `config/<namespace>/<application>/` for the `default` profile and
`config/<namespace>/<application>,<profile>/` for any other profile. The `global` application, whose properties every
application receives, is stored as `config/<namespace>/application/`.

On the first start, the service writes default properties for `global`, `tenant-manager`, and `dmp-tenant-activator`,
and copies selected properties from a baseline config-server when `BASELINE_PROJ` is set. See
[Startup initialization](core-legacy-api/docs/00-startup-initialization.md).

## API differences

| Page | Covers |
| --- | --- |
| [Removed API](core-legacy-api/docs/01-removed-api.md) | Endpoints that config-server has and `core-legacy-api` does not, and the supported endpoints |
| [Conflicting property keys](core-legacy-api/docs/02-conflicting-property-keys.md) | Keys such as `db` and `db.url` stored side by side |
| [Why writes take no lock](core-legacy-api/docs/03-why-writes-take-no-lock.md) | Concurrent writes to one application and profile |
| [Array keys](core-legacy-api/docs/04-array-keys.md) | Keys such as `list[0]` in the JSON and YAML formats |
| [Request and response samples](core-legacy-api/docs/05-request-response-samples.md) | One request and response per endpoint |

## Deployment parameters

The Helm chart is in [`core-legacy-api/helm-templates/core-legacy-api`](core-legacy-api/helm-templates/core-legacy-api).
[`values.schema.json`](core-legacy-api/helm-templates/core-legacy-api/values.schema.json) describes every parameter.
The parameters below configure the service itself.

| Parameter | Default | Purpose |
| --- | --- | --- |
| `CONSUL_URL` | none | URL of the Consul server that stores the properties. The service does not start without it. |
| `BASELINE_PROJ` | none | Namespace of a config-server whose `global` properties `tenant.default.id` and `bss.tenant.default-id` are copied to `global` on the first start. Unset skips the copy. |
| `LOG_LEVEL` | `info` | Log level. |
| `TRACING_ENABLED` | `false` | Sends traces to `TRACING_HOST`. |
| `TRACING_HOST` | `nc-diagnostic-agent` | Tracing collector host. |
| `TRACING_SAMPLER_PROBABILISTIC` | `0.01` | Share of requests that are traced. |
| `HTTP_BUFFER_HEADER_MAX_SIZE` | `10240` | Largest request header the service accepts, in bytes. Also written to `global` as `http.buffer.header.max.size`. |

These parameters only set default properties on the first start. The properties go to `global`, and
`openshift.server.url` also goes to `tenant-manager`, where it takes precedence over `global`:

| Parameter | Default | Property |
| --- | --- | --- |
| `CLOUD_PUBLIC_HOST` | none | Host part of `apigateway.external.public.url` and `openshift.server.url` |
| `CLOUD_PRIVATE_HOST` | `CLOUD_PUBLIC_HOST` | Host part of `apigateway.external.private.url` and `openshift.server.internal.url` |
| `CLOUD_API_HOST` | `CLOUD_PUBLIC_HOST` | Host part of `openshift.server.url` |
| `CLOUD_API_PORT` | `6443` | Port of `openshift.server.url` and `openshift.server.internal.url` |
| `CLOUD_PROTOCOL` | `https` | Scheme of the gateway and `openshift.server` URLs |
| `PUBLIC_GATEWAY_URL` | `<protocol>://public-gateway-<namespace>.<public host>` | `apigateway.external.public.url` |
| `PRIVATE_GATEWAY_URL` | `<protocol>://private-gateway-<namespace>.<private host>` | `apigateway.external.private.url` |
| `CLOUD_ADMIN_EMAIL` | a test address in `values.yaml` | `mail.cloudAdminEmail` |
| `EMAIL_HOST` | `mail.example.com` | `mail.server.host` |
| `EMAIL_FROM` | a test address in `values.yaml` | `mail.fromEmail` |
| `EMAIL_USER` | none | `mail.server.user`; unset writes `admin` and sets `mail.server.auth` to `false` |
| `EMAIL_PASSWORD` | none | `mail.server.password`; unset writes `admin` and sets `mail.server.auth` to `false` |
| `ALLOWED_HEADERS` | none | `headers.allowed`; unset leaves the property out |
