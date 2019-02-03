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

	// ===== status =====

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

	// ===== jobsets =====

	// mapping of unique ID to all pending, running or completed jobsets.
	// this is the single source of truth for jobset status.
	jobSets map[uint64]*jobSet

	// ===== jobset templates =====

	// mapping of jobset template names to registered templates.
	jobSetTemplates map[string]*jobSetTemplate

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
