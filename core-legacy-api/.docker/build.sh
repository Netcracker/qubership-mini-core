#!/usr/bin/env bash
set -euo pipefail
mkdir -p /reports /out
public_module="$PWD/core-legacy-api-service"
cd "$public_module"
go mod download
go test -v -count=1 -timeout=15m -coverpkg=./... -covermode=atomic \
    -coverprofile=/reports/public-coverage.out ./... 2>&1 | tee /reports/public-unit-tests.log
build_module="$public_module"
if [[ -n "$BOOTSTRAP_DIR" ]]; then
    cp -a "/build-context/$BOOTSTRAP_DIR" /src/private
    build_module=/src/private
    cd "$build_module"
    go mod download
    go test -mod=readonly -v -count=1 -coverpkg=./... -covermode=atomic \
        -coverprofile=/reports/bootstrap-coverage.out ./... 2>&1 | tee /reports/bootstrap-unit-tests.log
    mkdir -p /reports/bootstrap-module
    cp go.mod go.sum /reports/bootstrap-module/
fi
cd "$build_module"
GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -mod=readonly -o /out/core-legacy-api ./cmd/core-legacy-api-service
go version -m /out/core-legacy-api > /reports/build-info.txt
cp "$public_module/application.yaml" "$public_module/docs/swagger.json" "$public_module/docs/swagger.yaml" /out/
if [[ -n "$BOOTSTRAP_DIR" ]]; then
    cp "$build_module"/config/* /out/
fi
chmod 555 /out/core-legacy-api
find /out -type f ! -name core-legacy-api -exec chmod 444 {} +
