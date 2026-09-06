FROM artifactorycn.netcracker.com:17003/alpine/base-image:3.16.01

COPY --chown=10001 core-legacy-api-service/bin/core-legacy-api-impl-1.0.0-SNAPSHOT /app/core-legacy-api
COPY --chown=10001 ["core-legacy-api-impl/application.yaml", "core-legacy-api-impl/policies.conf", "/app/"]


CMD ["/app/core-legacy-api"]