// Package controllerrpc is the gRPC server and endpoints that act
// as an interface between external callers and the main peridot controller.
// It relies on calling the functions exported by the Controller in its
// rpcaccess file.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package controllerrpc

import (
	"log"
	"net"

	"github.com/swinslow/peridot-core/internal/controller"
	pbc "github.com/swinslow/peridot-core/pkg/controller"
	"google.golang.org/grpc"
)

const (
	port = ":8900"
)

type cServer struct {
	c *controller.Controller
}

func runGRPCServer(cs *cServer) {
	// open a socket for listening
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("couldn't open port %v: %v", port, err)
	}

	// create and register new GRPC server for controller
	server := grpc.NewServer()
	pbc.RegisterControllerServer(server, cs)

	// start grpc server
	if err := server.Serve(lis); err != nil {
		log.Fatalf("couldn't start controller GRPC server: %v", err)
	}
}
