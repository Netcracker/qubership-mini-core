Generate OpenAPI with swaggo/swag

Install swag tool:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

Run generator from project root (module containing `server.go`):

```bash
cd core-legacy-api-service
swag init -g server.go
```

This project already includes Swagger annotations in `server.go` and `config/controller.go`. After running `swag init` the generated docs will appear under `docs/` (e.g. `docs/swagger.json`, `docs/swagger.yaml`, `docs/docs.go`).
