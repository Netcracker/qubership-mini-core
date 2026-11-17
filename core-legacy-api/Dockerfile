# syntax=docker/dockerfile:1

ARG GO_BUILDER_IMAGE=golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244
ARG RUNTIME_IMAGE=ghcr.io/netcracker/qubership-core-base:alpine-2.5.5@sha256:1e207c43e907165f9c55838bb853465357a28bfd50799cf4aa66b959fe72a836
FROM --platform=$BUILDPLATFORM ${GO_BUILDER_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
ARG SOURCE_DIR=.
# An alternate Go entrypoint is used only when explicitly supplied by the caller.
ARG BOOTSTRAP_DIR=
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GONOPROXY=none \
    GOMAXPROCS=2 GOMEMLIMIT=1GiB GOFLAGS=-p=2
WORKDIR /src/source/core-legacy-api
COPY ${SOURCE_DIR}/ ./
RUN --mount=type=bind,target=/build-context \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,id=core-legacy-api-go-build,target=/root/.cache/go-build \
    --mount=type=secret,id=test_docker_config,required=true \
    sh .docker/with-test-docker.sh bash .docker/build.sh

FROM scratch AS artifact
COPY --from=build /reports/ /test-results/

FROM ${RUNTIME_IMAGE} AS run
WORKDIR /app
COPY --from=build --chown=10001:0 /out/ /app/
USER 10001
CMD ["/app/core-legacy-api"]
