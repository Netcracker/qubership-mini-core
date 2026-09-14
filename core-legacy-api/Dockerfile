FROM artifactorycn.netcracker.com:17152/netcracker/qubership-core-base:2.4.2

COPY --chown=10001 core-legacy-api-service/bin/core-legacy-api-service-1.0.0-SNAPSHOT /app/core-legacy-api
COPY --chown=10001 ["core-legacy-api-service/application.yaml", "/app/"]


CMD ["/app/core-legacy-api"]