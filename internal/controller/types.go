// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controller

import (
	pbs "github.com/swinslow/peridot-core/pkg/status"
)

// jobSet is a collection of one or more related steps, to be run together as
// a pipeline.
type jobSet struct {
	// the jobSet's unique ID
	jobSetID uint64

	// the name of the jobSetTemplate that this jobSet was built from
	templateName string

	// the current run status and health of the jobSet
	runStatus    pbs.Status
	healthStatus pbs.Health

	// all of the steps in this jobSet. some steps might be to run
	// additional separate jobSets.
	steps []*step

	// which step are we currently on? should equal 0 if done successfully.
	currentStep uint64

	// key-value configuration for this jobSet
	configs map[string]string
}

// step is a single step within a jobSet. each step must be completed before the
// next one proceeds. a step can be:
// 1) "agent" - represents a single Job run on the specified Agent
// 2) "jobset" - represents a separate JobSet with its own collection of steps
// 3) "concurrent" - represents a collection of steps that can run concurrently
type step struct {
	// what type of step is this?
	t StepType

	// what jobSet does this step belong to?
	jobSetID uint64

	// what is the ordering of this step within that jobSet?
	stepOrder uint32

	// what is this step's overall status and health?
	runStatus    pbs.Status
	healthStatus pbs.Health

	// "agent" only: what is the corresponding job ID? 0 means not yet assigned
	agentJobID uint64
	// "agent" only: what is the corresponding agent's name?
	agentName string

	// "jobset" only: what is the corresponding jobSet ID? 0 means not yet assigned
	subJobSetID uint64
	// "jobset" only: what is the corresponding jobSet's template name?
	subJobSetTemplateName string

	// "concurrent" only: what are the concurrent child steps?
	concurrentSteps []*step
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

// jobSetTemplate is a template for creating jobSets.
type jobSetTemplate struct {
	// the template's unique name
	name string

	// the steps comprising this template
	steps []*StepTemplate
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
