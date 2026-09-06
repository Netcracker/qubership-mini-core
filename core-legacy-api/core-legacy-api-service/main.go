package main

import (
	"core-legacy-api/lib"

	fiberSec "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2/security"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
)

func init() {
	serviceloader.Register(1, &fiberSec.DummyFiberServerSecurityMiddleware{})
	serviceloader.Register(1, &security.DummyToken{})
}

func main() {
	lib.RunService()
}
