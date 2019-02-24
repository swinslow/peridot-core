// Package controller is the main JobSet runner for peridot.
// It starts up a JobController, and keeps track of which
// Jobs should run in which order. It exposes functions for
// controller_rpc to trigger actions and to read data.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package controller

import (
	"container/list"
	"context"
	"fmt"
	"sync"

	"github.com/swinslow/peridot-core/internal/jobcontroller"
	pbc "github.com/swinslow/peridot-core/pkg/controller"
	pbs "github.com/swinslow/peridot-core/pkg/status"
)

// Controller is the full collection of data about the status of the
// controller and its Jobs, JobSets, etc. It is exported so that
// controller_rpc can access it. However, its data members are
// not exported so that read and start / cancel access must go through
// the exported functions.
type Controller struct {

	// ===== read/write mutex to sync data access =====

	// m is used to synchronize many reader / one writer access to
	// the rest of the data in the Controller struct.
	m *sync.RWMutex

	// ===== configuration =====

	// volume where code and SPDX files live, for building paths
	volPrefix string

	// maximum number of jobs to have running at any one time
	maxJobsRunning int

	// ===== status =====

	// are we open to receive new JobSetRequests via inJobSetStream?
	openForJobSetRequests bool

	// controller's overall run and health status
	runStatus    pbs.Status
	healthStatus pbs.Health

	// any status output messages
	outputMsg string

	// any status error messages; should only be set if isError == true
	errorMsg string

	// ===== agents =====

	// mapping of agent name to agent configuration. this is used to build the
	// agent config before we create the JobController, and should not be updated
	// after Start() is successfully called.
	agents map[string]pbc.AgentConfig

	// ===== jobs =====

	// mapping of unique ID to all pending, running or completed jobs.
	// this is updated based on updates from jobRecordStream.
	jobs map[uint64]*Job

	// jobs that are currently active. value will point to same job that
	// is tracked in main jobs mapping.
	activeJobs map[uint64]*Job

	// ID to be used for the next new Job
	nextJobID uint64

	// ===== jobsets =====

	// mapping of unique ID to all pending, running or completed jobsets.
	// this is the single source of truth for jobset status.
	jobSets map[uint64]*JobSet

	// jobsets that are currently active. value will point to same jobset that
	// is tracked in main jobSets mapping.
	activeJobSets map[uint64]*JobSet

	// ID to be used for the next JobSet
	nextJobSetID uint64

	// pending JobSetRequests that are queued for addition as actual JobSets
	pendingJSRs *list.List

	// ===== jobset templates =====

	// mapping of jobset template names to registered templates.
	jobSetTemplates map[string]*JobSetTemplate

	// ===== channels and contexts =====

	// controllerCancel is the CancelFunc associated with the Controller.
	controllerCancel context.CancelFunc

	// jobControllerCancel is the CancelFunc associated with the JobController.
	jobControllerCancel context.CancelFunc

	// inJobSetStream is created by Controller. The Controller's
	// jobSetProcessingLoop listens on inJobSetStream for requests to start
	// new JobSets. We own this channel and must close it when we're done.
	// and therefore must also be careful to block further writes with new
	// JobSet requests once we are ready to close it.
	inJobSetStream chan JobSetRequest

	// inJobStream is created by JobController. It is used to submit
	// JobRequests to JobController. We own this channel and must close
	// it when we're done.
	inJobStream chan<- jobcontroller.JobRequest

	// inJobUpdateStream is created by JobController. It is used to submit
	// requests for updates on Jobs' statuses. We own this channel and
	// must close it when we're done.
	inJobUpdateStream chan<- uint64

	// jobRecordStream is created by JobController. It receives broadcasts
	// of JobRecord updates. JobController owns this channel and will
	// close it.
	jobRecordStream <-chan jobcontroller.JobRecord

	// errc is created by JobController. It receives broadcasts of any
	// JobController-level errors. JobController owns this channel and will
	// close it.
	errc <-chan error
}

// Config contains configuration values for a newly-created
// Controller.
type Config struct {
	// prefix for volumes
	VolPrefix string

	// maximum number of jobs that can run at once
	MaxJobsRunning int
}

// Init is the initialization function that should be called on a newly
// created Controller, in order to initialize some of its configurations.
func (c *Controller) Init(cfg *Config) {
	c.volPrefix = cfg.VolPrefix

	// perhaps split into sub-categories like long-running jobs,
	// IO-heavy or CPU-heavy or network-heavy jobs, etc.
	c.maxJobsRunning = cfg.MaxJobsRunning
}

// tryToStart tries to start the controller for regular operation. This means:
// (1) starting the JobController; and (2) starting the JobSet processing
// loop. It will return nil on success or an error message if for some reason
// it is unable to start (e.g. if no agents have been previously set).
// Note that all AddAgent calls must occur prior to calling tryToStart.
func (c *Controller) tryToStart() error {
	// grab a writer lock
	c.m.Lock()
	// BE CAREFUL -- not deferring unlock here b/c want to unlock before we
	// start the jobSetProcessorLoop below

	// check whether we have any agents defined; if not, error out
	if len(c.agents) == 0 {
		c.m.Unlock()
		return fmt.Errorf("No agents defined prior to start request")
	}

	// for the time being, we'll manually set the maximum number of
	// jobs that we'll allow to run concurrently
	// as we get more familiar with peridot, this should be configurable,

	// build configuration for JobController
	agents := map[string]jobcontroller.AgentRef{}
	for _, ac := range c.agents {
		address := fmt.Sprintf("%s:%d", ac.Url, ac.Port)
		agents[ac.Name] = jobcontroller.AgentRef{
			Name:    ac.Name,
			Address: address,
		}
	}

	cfg := jobcontroller.Config{Agents: agents}

	// start JobController
	jcCtx, jcCancel := context.WithCancel(context.Background())
	c.jobControllerCancel = jcCancel
	c.inJobStream, c.inJobUpdateStream, c.jobRecordStream, c.errc = jobcontroller.JobController(jcCtx, cfg)

	// create and register the channel for submitting requests to start new JobSets
	c.inJobSetStream = make(chan JobSetRequest)
	c.openForJobSetRequests = true

	// create the list for pending JSR requests
	c.pendingJSRs = list.New()

	// unlocking now
	c.m.Unlock()

	// then start JobSet processing loop
	cCtx, cCancel := context.WithCancel(context.Background())
	c.controllerCancel = cCancel
	go c.jobSetProcessorLoop(cCtx)

	return nil
}

// jobSetProcessorLoop is the main loop for the Controller. It is responsible
// for ensuring that the JobController channels owned by the Controller are
// closed when we are exiting.
func (c *Controller) jobSetProcessorLoop(ctx context.Context) {
	exiting := false

	for !exiting {
		select {
		case <-ctx.Done():
			// the Controller has been cancelled and should shut down
			exiting = true
		case jsr := <-c.inJobSetStream:
			// add the request to the pending queue
			c.pendingJSRs.PushBack(jsr)
			// create new JobSets from the pending queue
			c.createNewJobSets()
			c.runScheduler()
		case jr := <-c.jobRecordStream:
			c.updateJobStatus(&jr)
			c.runScheduler()
		case err := <-c.errc:
			// an error on errc signals a significant problem in either the
			// Controller or the JobController, such as two Jobs that were
			// submitted with the same JobID. The Controller should be moved
			// into an error state and should shut down.
			c.m.Lock()
			c.healthStatus = pbs.Health_ERROR
			c.errorMsg += err.Error() + "\n"
			c.m.Unlock()
			exiting = true
		}

		// if controller status has moved to STOPPED, we should be exiting now
		if c.runStatus == pbs.Status_STOPPED {
			exiting = true
		}

		if !exiting {
			// if we aren't exiting, time to update statuses, run Jobs
			c.runScheduler()
		}
	}

	// once we're here, we're exiting
	// grab a writer lock to make sure no new jobset requests get submitted
	// while we are shutting down
	c.m.Lock()
	c.openForJobSetRequests = false
	c.m.Unlock()
	close(c.inJobSetStream)

	// need to clean up by closing channels we own
	close(c.inJobStream)
	close(c.inJobUpdateStream)

	// tell JobController to shut down also
	c.jobControllerCancel()

	// FIXME do we also need to drain the channels that come from
	// FIXME JobController to ensure they aren't blocked, waiting to be read?
}
