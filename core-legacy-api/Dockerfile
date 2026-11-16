FROM --platform=$BUILDPLATFORM golang:1.27.1@sha256:162be5298a40ed317005c8339c6de4d10d3eef336d66dc8e9259b03ab9d3a6d2 AS build
WORKDIR /app
COPY core-legacy-api-service/ .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o core-legacy-api ./cmd/core-legacy-api-service

FROM ghcr.io/netcracker/qubership-core-base:alpine-2.5.13@sha256:891386ea91d90a1376e12f30b458fc23eb9793afd35f82817c90e4911cddd633 AS run
COPY --chown=10001:0 --chmod=555 --from=build /app/core-legacy-api /app/core-legacy-api
COPY --chown=10001:0 --chmod=444 --from=build /app/application.yaml /app/
COPY --chown=10001:0 --chmod=444 --from=build app/docs/swagger.json /app/
COPY --chown=10001:0 --chmod=444 --from=build app/docs/swagger.yaml /app/

WORKDIR /app
CMD ["/app/core-legacy-api"]