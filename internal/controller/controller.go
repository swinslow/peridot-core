// Package controller is the main JobSet runner for peridot.
// It starts up a JobController, and keeps track of which
// Jobs should run in which order. It exposes functions for
// controller_rpc to trigger actions and to read data.
// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later
package controller

import (
	"sync"

	"github.com/swinslow/peridot-core/internal/jobcontroller"
	pbc "github.com/swinslow/peridot-core/pkg/controller"
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

	// ===== status =====

	// has the Controller been asked to start? (e.g., has StartReq been called?)
	wasStartReq bool

	// has the Controller been asked to stop? (e.g., has StartReq been called?)
	wasStopReq bool

	// is the Controller still running?
	isRunning bool

	// has the Controller encountered any errors? (could still be running)
	isError bool

	// any status output messages
	outputMsg string

	// any status error messages; should only be set if isError == true
	errorMsg string

	// ===== agents =====

	// mapping of agent name to agent ref. this is used to build the
	// agent configuration before we create the JobController.
	agents map[string]pbc.AgentConfig

	// ===== channels =====

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
