// Package jobcontroller is the main Job runner for peridot.
// It operates as a set of gRPC clients, with each Agent separately
// running its own gRPC server.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package jobcontroller

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/swinslow/peridot-core/pkg/agent"
)

// jobsData is the main set of data about the Jobs being managed by
// a JobController.
type jobsData struct {
	// cfg is the JobController's own configuration
	cfg Config
	// jobs is where the actual master record of the Job's status lives
	jobs map[uint64]*JobRecord
	// nextJobID will be the next available job ID
	nextJobID uint64
}

// JobController is the main Job runner function. It creates and returns
// three channels (described from the caller's perspective):
// * inJobStream, a write-only channel to submit new JobRequests, which must
//   be closed by the caller
// * inJobUpdateStream, a write-only channel to submit a request for an
//   update of one Job's status given its jobID, or 0 for all Jobs
// * jobRecordStream, a read-only channel with jobRecord updates
// * errc, a read-only channel where an error will be written or else
//   nil if no errors in the controller itself are encountered.
func JobController(ctx context.Context, cfg Config) (chan<- JobRequest, chan<- uint64, <-chan JobRecord, <-chan error) {
	// the caller will own the inJobStream channel and must close it
	inJobStream := make(chan JobRequest)
	// the caller will also own the inJobUpdateStream channel and must close it
	inJobUpdateStream := make(chan uint64)
	// we own the jobRecordStream channel
	jobRecordStream := make(chan JobRecord)
	// we own the errc channel. make it buffered so we can write 1 error
	// without blocking.
	errc := make(chan error, 1)

	js := jobsData{
		cfg:       cfg,
		jobs:      map[uint64]*JobRecord{},
		nextJobID: 1,
	}

	// rc is the response channel for all Job status messages.
	rc := make(chan JobUpdate)

	// n is the WaitGroup used to synchronize agent completion.
	// Each runJob goroutine adds 1 to n when it starts.
	var n sync.WaitGroup
	// Here the JobController itself also adds 1 to n, and this 1 is
	// Done()'d when the JobController's context gets cancelled,
	// signalling the termination of the JobController.
	n.Add(1)

	// start a separate goroutine to wait on the waitgroup until all agents
	// AND the JobController are done, and then close the response channel
	go func() {
		n.Wait()
		close(rc)
	}()

	// now we start a goroutine to listen to channels and waiting for
	// things to happen
	go func() {
		// note that this could introduce a race condition IF the JobController
		// were to receive a cancel signal from context, and decremented n to
		// zero, AND then a new Job were started, which would try to reuse
		// the zeroed waitgroup. To avoid this, we set exiting to true before
		// calling n.Done(), and after exiting is true we don't create any
		// new Jobs.
		exiting := false

		for !exiting {
			select {
			case <-ctx.Done():
				// the JobController has been cancelled and should shut down
				exiting = true
				n.Done()
			case jr := <-inJobStream:
				// the caller has submitted a new JobRequest; create the job
				newJobID := startNewJob(ctx, &js, jr, &n, rc)
				// and broadcast the job record, whether or not it was
				// created successfully
				updateJobRecord(&js, newJobID, nil, jobRecordStream)
			case ju := <-rc:
				// an agent has sent a JobUpdate
				updateJobRecord(&js, ju.JobID, &ju, jobRecordStream)
			case jobID := <-inJobUpdateStream:
				// the caller has submitted a request for a JobRecord update
				// we can get it by sending nil to updateJobRecord
				updateJobRecord(&js, jobID, nil, jobRecordStream)
			}
		}

		// FIXME as we are exiting, do we first need to drain any remaining
		// FIXME updates from rc, and then report out all JobRecords?
	}()

	// finally we return the channels so that the caller can kick things off
	return inJobStream, inJobUpdateStream, jobRecordStream, errc
}

func startNewJob(ctx context.Context, js *jobsData, jr JobRequest, n *sync.WaitGroup, rc chan<- JobUpdate) uint64 {
	log.Printf("===> In startNewJob: jr = %s\n", jr.String())
	// create a new JobRecord and fill it in
	rec := &JobRecord{
		JobID:     js.nextJobID,
		AgentName: jr.AgentName,
		Cfg:       jr.Cfg,
		// fill in default Status data since we haven't talked to the agent yet
		Status: agent.StatusReport{
			RunStatus:    agent.JobRunStatus_STARTUP,
			HealthStatus: agent.JobHealthStatus_OK,
		},
	}
	js.jobs[rec.JobID] = rec

	js.nextJobID++

	// check whether the requested agent name is valid
	ar, ok := js.cfg.Agents[rec.AgentName]
	if !ok {
		log.Printf("===> Error\n")
		// agent name is invalid; set error and bail out
		rec.Err = fmt.Errorf("unknown agent name: %s", rec.AgentName)
		return rec.JobID
	}
	// agent name was valid, we have the AgentRef now
	// time to actually create the job
	n.Add(1)
	go runJobAgent(ctx, rec.JobID, ar, rec.Cfg, n, rc)

	// return new job's ID
	return rec.JobID
}

func updateJobRecord(js *jobsData, jobID uint64, ju *JobUpdate, jobRecordStream chan<- JobRecord) {
	// if ju is nil, we're just sending the original record upon job creation
	if ju != nil {
		// if ju is non-nil, we need to update the record first
		// look up job ID to make sure it's present
		jr, ok := js.jobs[jobID]
		if !ok {
			// if the job ID doesn't exist, we can't do anything with this message
			// but we also don't want to send it out on the stream; just exit
			return
		}
		jr.Status = ju.Status
		jr.Err = ju.Err
	}

	// now we broadcast the updated (or not) record
	// make sure the job with this jobID exists (if ju was nil, we
	// didn't check earlier)
	jr, ok := js.jobs[jobID]
	if !ok {
		// if the job ID doesn't exist, we can't do anything with this message
		// but we also don't want to send it out on the stream; just exit
		return
	}
	jobRecordStream <- *jr
}
