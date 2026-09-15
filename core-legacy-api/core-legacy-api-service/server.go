package main

import (
	"github.com/Netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/lib"

	fiberSec "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2/security"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
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

func main() {
	lib.RunService()
}
