# Building and deploying core-legacy-api

Everything needed to build the public image and deploy its chart is in this
GitHub repository. No GitLab checkout, private module proxy, or corporate image
registry is required. The default entrypoint registers the public dummy security
providers. Deploy behind an appropriate authentication boundary when required.

From the repository root, build and load the image:

```sh
bash core-legacy-api/.docker/build-image.sh --load --tag core-legacy-api:local
```

The multi-stage Dockerfile runs the complete public Go test suite, including
Testcontainers tests against Consul, and compiles the binary. The helper starts
a temporary TLS-enabled Docker daemon for tests, passes its client certificates
as a BuildKit secret, and removes the daemon, its volumes, and certificates when
the build ends. It requires Docker with privileged-container support and
Buildx. No Docker API port is published on the host. To use an existing test
Docker endpoint instead, supply a `test_docker_config` secret archive containing
`ca.pem`, `cert.pem`, `key.pem`, and `environment.sh` to `docker buildx build`.

The `artifact` build target exports test reports and binary dependency metadata.

## Helm deployment and CI integration

The chart is in `helm-templates/core-legacy-api`. Install it with Helm using
`IMAGE_REPOSITORY`, `TAG`, `NAMESPACE`, `CONSUL_URL`, and `DEPLOYMENT_SESSION_ID`,
plus the appropriate resource profile and platform values. The namespace must
have the platform's `topology` ConfigMap. If Consul requires a token, provide the
optional `config-server-consul-token` Secret with a `token` key. The chart
creates the email Secret itself. Disable `MONITORING_ENABLED` when the cluster
does not have the PodMonitor CRD.

GitHub workflows are unchanged. Wiring Docker API access into the existing
image-build workflow and adding Helm deployment automation are deferred.
The Dockerfile's tests require the certificate archive described above.

## Optional alternate bootstrap

The Dockerfile defaults to the public entrypoint. A downstream build can
explicitly set `SOURCE_DIR` to the public service directory within a larger build
context and `BOOTSTRAP_DIR` to a separate Go module in that context. That module
must expose `cmd/core-legacy-api-service`, reference the public implementation,
and provide runtime configuration in `config/`. A local replacement can point to
`../source/core-legacy-api/core-legacy-api-service` inside the build container.
Its tests run before its entrypoint is compiled. The alternate module's code,
dependencies, and configuration stay in the downstream repository. There is no
automatic discovery or download of an alternate bootstrap.
