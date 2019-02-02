// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package jobcontroller

import (
	"log"
	"net"

	"github.com/swinslow/peridot-core/pkg/controller"
	"google.golang.org/grpc"
)

const (
	port = ":8900"
)

type jcServer struct {
	inJobStream       chan<- JobRequest
	inJobUpdateStream chan<- uint64
	jobRecordStream   <-chan JobRecord
	errc              <-chan error
}

func runGRPCServer(jcs *jcServer) {
	// open a socket for listening
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("couldn't open port %v: %v", port, err)
	}

	// create and register new GRPC server for controller
	server := grpc.NewServer()
	controller.RegisterControllerServer(server, jcs)

	// start grpc server
	if err := server.Serve(lis); err != nil {
		log.Fatalf("couldn't start controller GRPC server: %v", err)
	}
}
