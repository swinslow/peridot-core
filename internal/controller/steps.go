// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controller

import (
	"container/list"
	"fmt"

	"github.com/swinslow/peridot-core/pkg/agent"
	pbs "github.com/swinslow/peridot-core/pkg/status"
)

// findStepInSteps finds the step with the given ID within this Steps slice.
// It will also check recursively within concurrent sub-steps. It returns
// a pointer to the Step or nil if not found.
// It will not grab a reader lock, as it assumes that the calling function
// has already grabbed one if needed.
func findStepInSteps(steps []*Step, stepID uint64) *Step {
	for _, step := range steps {
		// check this step
		if step.StepID == stepID {
			return step
		}
		// if concurrent, check this step's sub-steps too
		if step.T == StepTypeConcurrent {
			checkStep := findStepInSteps(step.ConcurrentSteps, stepID)
			if checkStep != nil {
				return checkStep
			}
		}
	}
	return nil
}

// createStepsFromTemplate gets the recursive creation of steps going.
// It discards the nextStepID since we don't need it any longer.
func createStepsFromTemplate(js *JobSet, pendingJSRs *list.List, sts []*StepTemplate) []*Step {
	steps, _ := createStepsFromTemplateHelper(js, pendingJSRs, sts, 1)
	return steps
}

// createStepsFromTemplateHelper recursively creates a set of actual Steps
// within a JobSet based on its JobSetTemplate Steps. It will enqueue new
// pending JSRs where needed for additional JobSets. It returns the created
// Steps as well as the next Step ID to be used so that subsequent recursive
// calls continue to update with unique and ordered Step IDs.
func createStepsFromTemplateHelper(js *JobSet, pendingJSRs *list.List, sts []*StepTemplate, nextStepID uint64) ([]*Step, uint64) {
	steps := []*Step{}

	for _, st := range sts {
		// fill in step details that apply for all step types
		step := &Step{
			T:            st.T,
			JobSetID:     js.JobSetID,
			StepID:       nextStepID,
			StepOrder:    nextStepID,
			RunStatus:    pbs.Status_STARTUP,
			HealthStatus: pbs.Health_OK,
		}
		nextStepID++

		// fill in step details based on type
		switch st.T {
		case StepTypeAgent:
			// ===== AGENT =====
			step.AgentName = st.AgentName

		case StepTypeJobSet:
			// ===== JOBSET =====
			step.SubJobSetTemplateName = st.JSTemplateName
			// create and enqueue new JobSetRequest
			jsr := JobSetRequest{
				TemplateName:    st.JSTemplateName,
				Configs:         map[string]string{},
				ParentJobSetID:  js.JobSetID,
				ParentJobStepID: step.StepID,
			}
			// copy over all config strings from parent JobSet
			for k, v := range js.Configs {
				jsr.Configs[k] = v
			}
			// and submit the request
			pendingJSRs.PushBack(jsr)

		case StepTypeConcurrent:
			// ===== CONCURRENT =====
			step.ConcurrentSteps, nextStepID = createStepsFromTemplateHelper(js, pendingJSRs, st.ConcurrentStepTemplates, nextStepID)
		}

		// and add this step to the steps slice
		steps = append(steps, step)
	}

	// return built steps and next Step ID, to be used for sub-steps
	return steps, nextStepID
}

// retrieveReadySteps walks through a slice of pointers to steps, and returns
// two slices: a slice of pointers to "agent" steps that are ready to run, and
// a slice of pointers to "jobset" steps that have not yet been queued and are
// ready to be added as new JobSetRequests.
// It will recursively read through any "concurrent" steps in order to bubble
// up any "agent" and "jobset" steps that are contained therein.
// It also returns a boolean, which will be set to true if there is some
// failure or error detected which should prevent running any further steps.
func retrieveReadySteps(steps []*Step) ([]*Step, []*Step, bool) {
	// walk through the steps in order, checking whether to proceed and/or
	// whether to add a new step as ready
	for _, step := range steps {
		switch step.RunStatus {
		case pbs.Status_RUNNING:
			// a step is already running. There is nothing more to do for
			// this set of steps until it is completed.
			return nil, nil, false

		case pbs.Status_STOPPED:
			// check whether this step errored out
			if step.HealthStatus == pbs.Health_ERROR {
				// this step failed. We don't want to keep running later
				// steps. This JobSet should be getting an error status
				// and removed from the active list. For now, we'll just
				// return with nothing more to do.
				return nil, nil, true
			}

			// otherwise, no error means keep going past this step
			continue

		case pbs.Status_STARTUP:
			// this step is the one which is ready to run. check its type
			// and figure out which ready steps to add.
			switch step.T {
			case StepTypeAgent:
				return []*Step{step}, nil, false
			case StepTypeJobSet:
				if step.SubJobSetRequestSubmitted {
					// already submitted, so, return without including
					return nil, nil, false
				}
				// not yet submitted, so include it
				return nil, []*Step{step}, false
			case StepTypeConcurrent:
				// for concurrent steps, we now want to pick up EVERY sub-step
				// within this one that is still in startup state, recursing
				// through sub-concurrent steps.
				cAgentSteps, cJobSetSteps := retrieveConcurrentStartupSteps(step.ConcurrentSteps)
				return cAgentSteps, cJobSetSteps, false
			}

		default:
			// some invalid status here; return with nothing to do
			return nil, nil, true
		}
	}

	// if we get here, all steps are either running or stopped. we should
	// just return with nothing to do
	return nil, nil, false
}

// retrieveConcurrentStartupSteps recursively retrieves all steps within
// this one that are in STARTUP state. It returns a slice of "agent" steps
// and a slice of "jobset" steps.
func retrieveConcurrentStartupSteps(steps []*Step) ([]*Step, []*Step) {
	readyAgentSteps := []*Step{}
	readyJobSetSteps := []*Step{}

	for _, step := range steps {
		if step.RunStatus == pbs.Status_STARTUP {
			// this step is ready to run; check its type and figure out
			// where to put it, and/or its sub-steps.
			switch step.T {
			case StepTypeAgent:
				readyAgentSteps = append(readyAgentSteps, step)
			case StepTypeJobSet:
				// only return those that are not yet submitted
				if !step.SubJobSetRequestSubmitted {
					readyJobSetSteps = append(readyJobSetSteps, step)
				}
			case StepTypeConcurrent:
				// recursively retrieve all of its children
				subAgents, subJobSets := retrieveConcurrentStartupSteps(step.ConcurrentSteps)
				for _, aStep := range subAgents {
					readyAgentSteps = append(readyAgentSteps, aStep)
				}
				for _, jsStep := range subJobSets {
					readyJobSetSteps = append(readyJobSetSteps, jsStep)
				}
			}
		}
	}
	return readyAgentSteps, readyJobSetSteps
}

// getFinalStep returns a pointer to the last step for the corresponding steps.
// If it is a concurrent step, it will recurse to point to either an agent or
// a JobSet as its actual final step.
func getFinalStep(steps []*Step) *Step {
	finalStep := steps[len(steps)-1]
	if finalStep.T == StepTypeAgent || finalStep.T == StepTypeJobSet {
		return finalStep
	} else if finalStep.T == StepTypeConcurrent {
		return getFinalStep(finalStep.ConcurrentSteps)
	} else {
		return nil
	}
}

func (c *Controller) getJobSetFinalJobID(js *JobSet) (uint64, error) {
	// find the very last step for this JobSet. If it is a concurrent step,
	// recursively find its actual final step.
	finalStep := getFinalStep(js.Steps)
	if finalStep == nil {
		return 0, fmt.Errorf("could not find final step for JobSet %d", js.JobSetID)
	}

	// if the final step was an agent, just return its job ID
	if finalStep.T == StepTypeAgent {
		if finalStep.AgentJobID == 0 {
			return 0, fmt.Errorf("final step for JobSet %d had ID 0", js.JobSetID)
		}
		return finalStep.AgentJobID, nil
	}

	// or if the final step was another JobSet, go get its job ID
	if finalStep.T == StepTypeJobSet {
		realFinalJobSet, ok := c.jobSets[finalStep.SubJobSetID]
		if !ok {
			return 0, fmt.Errorf("final step for JobSet %d was JobSet with ID %d, no corresponding JobSet found", js.JobSetID, finalStep.SubJobSetID)
		}

		realFinalID, err := c.getJobSetFinalJobID(realFinalJobSet)
		if err != nil {
			return 0, err
		}
		return realFinalID, nil
	}

	// if we got here, the final step didn't have type "agent" or "jobset"
	return 0, fmt.Errorf("final step for JobSet %d had an invalid type", js.JobSetID)
}

type priorStepID struct {
	T           StepType
	agentJobID  uint64
	jobSetSubID uint64
}

// getPriorStepIDs returns a slice of all step Job or JobSet IDs, for all
// "agent" and "jobset" steps prior to the given Step. It will recurse down
// into prior concurrent steps to include those as well.
func getPriorStepIDs(steps []*Step, curStep *Step) []priorStepID {
	priorStepIDs := []priorStepID{}

	// first, find the top-level ID where we should stop
	curTopStep := findTopLevelStepID(steps, curStep.StepID)
	if curTopStep == nil {
		// couldn't find this step, so just bail
		return nil
	}

	// now, walk through until we get to the curTopStep, and add all
	// preceding steps. If we find a concurrent step, roll in all of its
	// steps too.
	for _, step := range steps {
		if step == curTopStep {
			break
		}

		// still prior to current top step; add to prior step IDs
		addPriorStepIDs(priorStepIDs, step)
	}

	return priorStepIDs
}

// findTopLevelStepID returns a pointer to the top-level Step that contains
// the requested Step ID, including looking into concurrent steps if
// necessary. It returns nil if not found.
func findTopLevelStepID(steps []*Step, stepID uint64) *Step {
	for _, step := range steps {
		// if this is the right step, just return it
		if step.StepID == stepID {
			return step
		}

		// or if this is a concurrent step, recurse down into it
		if step.T == StepTypeConcurrent {
			cStep := findTopLevelStepID(step.ConcurrentSteps, stepID)
			if cStep != nil {
				// this concurrent step contains it, so send back ourself
				return step
			}
		}
	}

	// if we get here, it wasn't found
	return nil
}

// addPriorStepIDs adds the given step to priorStepIDs, recursively including
// concurrent steps.
func addPriorStepIDs(priorStepIDs []priorStepID, step *Step) {
	var ps priorStepID

	switch step.T {
	case StepTypeAgent:
		ps.T = StepTypeAgent
		ps.agentJobID = step.AgentJobID

	case StepTypeJobSet:
		ps.T = StepTypeJobSet
		ps.jobSetSubID = step.SubJobSetID

	case StepTypeConcurrent:
		for _, subStep := range step.ConcurrentSteps {
			addPriorStepIDs(priorStepIDs, subStep)
		}
	}
}

// getJobConfigForStep returns the JobConfig corresponding to a given Step.
// It creates and uses predetermined paths for code and SPDX input and output
// directories, based on the preceding step ID(s) and this step's job ID.
// It should NOT grab a lock because it should only be called from a function
// that has already grabbed a lock itself.
func (c *Controller) getJobConfigForStep(step *Step) *agent.JobConfig {
	js, ok := c.jobSets[step.JobSetID]
	if !ok {
		return nil
	}

	codeOutputDir := getCodeOutputDir(c.volPrefix, step.JobSetID, step.AgentJobID, step.AgentName)
	spdxOutputDir := getSpdxOutputDir(c.volPrefix, step.JobSetID, step.AgentJobID, step.AgentName)

	// collect code and spdx paths for all prior steps
	priorStepIDs := getPriorStepIDs(js.Steps, step)

	codeInputs := []*agent.JobConfig_CodeInput{}
	spdxInputs := []*agent.JobConfig_SpdxInput{}

	for _, psid := range priorStepIDs {
		var jobID uint64
		if psid.T == StepTypeAgent {
			jobID = psid.agentJobID
			job, ok := c.jobs[jobID]
			if !ok {
				// problem getting the job; skip it
				continue
			}
			// and actually build the inputs
			source, codePath := getCodeInput(c.volPrefix, step.JobSetID, jobID, job.AgentName)
			codeInputs = append(codeInputs, &agent.JobConfig_CodeInput{
				Source: source,
				Paths:  []string{codePath},
			})

			source, spdxPath := getSpdxInput(c.volPrefix, step.JobSetID, jobID, job.AgentName)
			spdxInputs = append(spdxInputs, &agent.JobConfig_SpdxInput{
				Source: source,
				Paths:  []string{spdxPath},
			})

		} else if psid.T == StepTypeJobSet {
			psJobSet, ok := c.jobSets[psid.jobSetSubID]
			if !ok {
				// problem getting the jobset; skip it
				continue
			}
			jobID, err := c.getJobSetFinalJobID(psJobSet)
			if err != nil {
				// problem getting the jobset's final job id; skip it
				continue
			}
			job, ok := c.jobs[jobID]
			if !ok {
				// problem getting the job; skip it
				continue
			}
			// and actually build the inputs
			source, codePath := getCodeInput(c.volPrefix, psJobSet.JobSetID, jobID, job.AgentName)
			codeInputs = append(codeInputs, &agent.JobConfig_CodeInput{
				Source: source,
				Paths:  []string{codePath},
			})

			source, spdxPath := getSpdxInput(c.volPrefix, psJobSet.JobSetID, jobID, job.AgentName)
			spdxInputs = append(spdxInputs, &agent.JobConfig_SpdxInput{
				Source: source,
				Paths:  []string{spdxPath},
			})
		}
	}

	// build the actual JobConfig
	jc := &agent.JobConfig{
		CodeInputs:    codeInputs,
		CodeOutputDir: codeOutputDir,
		SpdxInputs:    spdxInputs,
		SpdxOutputDir: spdxOutputDir,
		Jkvs:          []*agent.JobConfig_JobKV{},
	}

	// and copy over the config key-values from the JobSet
	for k, v := range js.Configs {
		jkv := &agent.JobConfig_JobKV{
			Key:   k,
			Value: v,
		}
		jc.Jkvs = append(jc.Jkvs, jkv)
	}

	// finally, the config is done!
	return jc
}
