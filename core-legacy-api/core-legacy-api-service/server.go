package main

import (
	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/lib"

	fiberSec "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2/security"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"

	// memlimit sets memory limit = 0.9 of cgroup memory limit
	_ "github.com/netcracker/qubership-core-lib-go/v3/memlimit"
)

func init() {
	serviceloader.Register(1, &fiberSec.DummyFiberServerSecurityMiddleware{})
	serviceloader.Register(1, &security.DummyToken{})
}

// @title config-server API
// @version     1.0.0
// @description This is the API documentation for the config-server. With the Config
// @description Server you have a central place to manage external properties for applications
// @description across all environments.

//go:generate go run github.com/swaggo/swag/cmd/swag init --generalInfo server.go --parseDependency  --parseGoList=false --parseDepth 2

func main() {
	lib.RunService()
}
