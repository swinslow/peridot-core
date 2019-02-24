package main

import (
	"github.com/swinslow/peridot-core/internal/controller"
	"github.com/swinslow/peridot-core/internal/controllerrpc"
)

func main() {
	// set up Controller configuration
	cfg := &controller.Config{
		VolPrefix:      "/tmp/peridot/",
		MaxJobsRunning: 10,
	}

	// create and initialize Controller
	controller := &controller.Controller{}
	controller.Init(cfg)

	// create the gRPC server object
	cs := &controllerrpc.CServer{C: controller}

	// run the gRPC server until it's done
	controllerrpc.RunGRPCServer(cs)
}
