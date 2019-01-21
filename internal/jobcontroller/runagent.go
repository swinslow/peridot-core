// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package jobcontroller

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/swinslow/peridot-core/pkg/agent"
	"google.golang.org/grpc"
)

func getErrorUpdate(jobID uint64, err error) JobUpdate {
	return JobUpdate{
		JobID: jobID,
		Status: agent.StatusReport{
			RunStatus:    agent.JobRunStatus_STOPPED,
			HealthStatus: agent.JobHealthStatus_ERROR,
		},
		Err: err,
	}
}

func runJobAgent(ctx context.Context, jobID uint64, ar AgentRef, cfg agent.JobConfig, n *sync.WaitGroup, rc chan<- JobUpdate) {
	defer n.Done()

	log.Printf("===> in runJobAgent\n")

	// connect and get client for each agent server
	conn, err := grpc.Dial(ar.Address, grpc.WithInsecure())
	if err != nil {
		rc <- getErrorUpdate(jobID, fmt.Errorf("could not connect to %s (%s): %v", ar.Name, ar.Address, err))
		return
	}
	defer conn.Close()
	c := agent.NewAgentClient(conn)

	// set up context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	// start NewJob stream
	stream, err := c.NewJob(ctx)
	if err != nil {
		rc <- getErrorUpdate(jobID, fmt.Errorf("could not connect for %s (%s): %v", ar.Name, ar.Address, err))
		return
	}

	// make server call to start job
	startReq := &agent.StartReq{Config: &cfg}
	cm := &agent.ControllerMsg{Cm: &agent.ControllerMsg_Start{Start: startReq}}
	log.Printf("== controller SEND StartReq for jobID %d", jobID)
	err = stream.Send(cm)
	if err != nil {
		rc <- getErrorUpdate(jobID, fmt.Errorf("could not start job for %s (%s): %v", ar.Name, ar.Address, err))
		return
	}

	// set up listener + status updater goroutine
	// until we get past waitc, ONLY the listener goroutine should be
	// updating the job status
	waitc := make(chan interface{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// done with reading
				log.Printf("== controller CLOSING got io.EOF")
				close(waitc)
				return
			}
			if err != nil {
				log.Printf("== controller CLOSING got error")
				rc <- getErrorUpdate(jobID, fmt.Errorf("error for %s (%s): %v", ar.Name, ar.Address, err))
				close(waitc)
				return
			}

			// update status if we got a status report
			switch x := in.Am.(type) {
			case *agent.AgentMsg_Status:
				st := *x.Status
				log.Printf("== controller RECV StatusReport for jobID %d: %#v\n", jobID, st)
				rc <- JobUpdate{
					JobID:  jobID,
					Status: st,
				}
			}
		}
	}()

	// wait until listener loop is done
	// FIXME ordinarily this should probably ping occasionally with a heartbeat
	// FIXME request, and/or eventually exit if we see an error or if a job
	// FIXME hasn't responded for ___ time
	// FIXME also, does CloseSend need to come before we wait for agent to close?
	exiting := false
	for !exiting {
		select {
		case <-waitc:
			stream.CloseSend()
			exiting = true
			// case <-time.After(time.Second * 5):
			// 	// check status and see whether we should continue waiting
			// 	if st.status.RunStatus == agent.JobRunStatus_STOPPED {
			// 		stream.CloseSend()
			// 		exiting = true
			// 	}
		}
	}
}
