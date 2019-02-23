// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controller

import (
	"time"

	"github.com/swinslow/peridot-core/pkg/agent"
	pbs "github.com/swinslow/peridot-core/pkg/status"
)

// Job is data about a single job, running (or to be run) on one agent.
// it is created within the controller, and its status is updated based
// on broadcasts from the jobcontroller via the jobRecordStream channel.
type Job struct {
	// the job's unique ID
	JobID uint64

	// the jobSet that this job belongs to
	JobSetID uint64

	// the step # within the jobSet that this job corresponds to
	JobSetStepID uint64

	// the step order within the jobSet that this job corresponds to
	JobSetStepOrder uint64

	// the name of the agent running this job
	AgentName string

	// the job's configuration
	Cfg agent.JobConfig

	// the job's current status
	Status agent.StatusReport

	// has this job been submitted to the JobController?
	// an instance of any job should only be submitted once.
	submitted bool
}

// JobSet is a collection of one or more related steps, to be run together as
// a pipeline.
type JobSet struct {
	// the jobSet's unique ID
	JobSetID uint64

	// the name of the jobSetTemplate that this jobSet was built from
	TemplateName string

	// the current run status and health of the jobSet
	RunStatus    pbs.Status
	HealthStatus pbs.Health

	// time started and finished
	TimeStarted  time.Time
	TimeFinished time.Time

	// all of the steps in this jobSet. some steps might be to run
	// additional separate jobSets.
	Steps []*Step

	// key-value configuration for this jobSet
	Configs map[string]string

	// output messages, if any
	OutputMessages string

	// error messages, if any
	ErrorMessages string
}

// Step is a single step within a JobSet. each step must be completed before the
// next one proceeds. a step can be:
// 1) "agent" - represents a single Job run on the specified Agent
// 2) "jobset" - represents a separate JobSet with its own collection of steps
// 3) "concurrent" - represents a collection of steps that can run concurrently
type Step struct {
	// what type of step is this?
	T StepType

	// what jobSet does this step belong to?
	JobSetID uint64

	// what is the unique ID of this step within that jobSet? (will not change)
	StepID uint64

	// what is the ordering of this step within that jobSet? (could change if
	// steps are reordered or new steps are inserted)
	StepOrder uint64

	// what is this step's overall status and health?
	RunStatus    pbs.Status
	HealthStatus pbs.Health

	// "agent" only: what is the corresponding job ID? 0 means not yet assigned
	AgentJobID uint64
	// "agent" only: what is the corresponding agent's name?
	AgentName string

	// "jobset" only: what is the corresponding jobSet ID? 0 means not yet assigned
	SubJobSetID uint64
	// "jobset" only: what is the corresponding jobSet's template name?
	SubJobSetTemplateName string
	// "jobset" only: has a JobSetRequest been submitted yet for this new JobSet?
	SubJobSetRequestSubmitted bool

	// "concurrent" only: what are the concurrent child steps?
	ConcurrentSteps []*Step
}

// StepType is an enum for the different types of steps and StepTemplates.
type StepType int

const (
	// StepTypeAgent is a step that runs a single Job on this agent.
	StepTypeAgent StepType = iota
	// StepTypeJobSet is a step that runs a separate JobSet.
	StepTypeJobSet
	// StepTypeConcurrent is a step that runs multiple sub-steps, which can
	// optionally run concurrently with one another.
	StepTypeConcurrent
)

// JobSetTemplate is a template for creating jobSets.
type JobSetTemplate struct {
	// the template's unique name
	Name string

	// the steps comprising this template
	Steps []*StepTemplate
}

// StepTemplate is a single step within a jobSetTemplate. Its values
// correspond to those of an actual step. StepTemplate is exported so
// that controllerrpc can create templates.
type StepTemplate struct {
	// T specifies what type of step this is
	T StepType

	// AgentName is for "agent" type only: what is the corresponding
	// agent's name?
	AgentName string

	// JSTemplateName is for "jobset" only: what is the name of the
	// corresponding jobSetTemplate?
	JSTemplateName string

	// ConcurrentStepTemplates is for "concurrent" only: what are the
	// templates for the concurrent child steps?
	ConcurrentStepTemplates []*StepTemplate
}

// JobSetRequest is a request to start a new JobSet, based on a
// JobSetTemplate that has already been defined.
type JobSetRequest struct {
	// the name of the requested JobSetTemplate
	TemplateName string

	// the configuration values for this JobSet instance
	Configs map[string]string

	// parent JobSet, if being created as a sub-JobSet
	ParentJobSetID uint64

	// step ID within parent JobSet, if being created as a sub-JobSet
	ParentJobStepID uint64
}
