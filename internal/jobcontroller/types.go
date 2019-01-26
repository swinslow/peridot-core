// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package jobcontroller

import (
	"fmt"

	"github.com/swinslow/peridot-core/pkg/agent"
)

// Config defines the JobController's own configuration.
type Config struct {
	// Agents defines all Agents that the JobController knows about.
	// It maps the unique Agent instance's name to its AgentRef.
	Agents map[string]AgentRef
}

// String provides a compact string representation of the Config.
func (cfg *Config) String() string {
	agentsStr := ""
	for _, ar := range cfg.Agents {
		agentsStr += fmt.Sprintf("%s => %s; ", ar.Name, ar.Address)
	}
	return fmt.Sprintf("Config{Agents: %s}", agentsStr)
}

// AgentRef defines information about an Agent: its name and where
// it can be found.
type AgentRef struct {
	// Name is the unique ID for this instance of this Agent. It should be
	// unique across all Agent instances in peridot.
	Name string

	// Address is the URL + port combination where this Agent instance
	// can be found.
	Address string
}

// JobRequest defines the metadata needed to start a Job.
type JobRequest struct {
	// AgentName identifies the Agent that is (or was, or will be) running
	// this Job.
	AgentName string

	// Cfg describes the configuration for this Job.
	Cfg agent.JobConfig
}

// String provides a compact string representation of the JobRequest.
func (jreq *JobRequest) String() string {
	return fmt.Sprintf("JobRequest{AgentName: %s, Cfg: %s}", jreq.AgentName, jreq.Cfg.String())
}

// JobRecord defines the full collection of metadata about a Job:
// its Agent, its configuration and its current status.
type JobRecord struct {
	// JobID is the unique ID for this Job. It should be unique across
	// all Jobs in peridot.
	JobID uint64

	// AgentName identifies the Agent that is (or was, or will be) running
	// this Job.
	AgentName string

	// Cfg describes the configuration for this Job.
	Cfg agent.JobConfig

	// Status defines the current status of this Job.
	Status agent.StatusReport

	// Err defines any error messages that have arisen on the controller
	// for this Job. (Agent errors will be found in Status.ErrorMessages.)
	Err error
}

// String provides a compact string representation of the JobRecord.
func (jrec *JobRecord) String() string {
	return fmt.Sprintf("JobRecord{JobID: %d, AgentName: %s, Cfg: %s, Status: %s, Err: %v}", jrec.JobID, jrec.AgentName, jrec.Cfg.String(), jrec.Status.String(), jrec.Err)
}

// JobUpdate defines the messages that a runJob goroutine sends to the
// rc channel.
type JobUpdate struct {
	// JobID is the unique ID for this Job. It should be unique across
	// all Jobs in peridot.
	JobID uint64

	// Status defines the current status of this Job.
	Status agent.StatusReport

	// Err defines any error messages that have arisen on the controller
	// for this Job. (Agent errors will be found in Status.ErrorMessages.)
	Err error
}

// JobShortStatus is a shorter status response for this Job. Full details
// can be seen in JobRecord.
type JobShortStatus struct {
	// JobID is the unique ID for this Job. It should be unique across
	// all Jobs in peridot.
	JobID uint64

	// Run is the Job's current run status.
	Run agent.JobRunStatus

	// Health is the Job's current health status.
	Health agent.JobHealthStatus
}
