// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controller

import (
	"fmt"
	"time"

	"github.com/swinslow/peridot-core/internal/jobcontroller"
	"github.com/swinslow/peridot-core/pkg/agent"
	pbs "github.com/swinslow/peridot-core/pkg/status"
)

// runScheduler is the main "decider" within the Controller.
// It walks through active Jobs and JobSets, decides whether to update them,
// and decides whether to start new Jobs based on the current overall state.
func (c *Controller) runScheduler() {
	// grab a writer lock
	c.m.Lock()
	fmt.Printf("===> ENTERING runScheduler")
	defer c.m.Unlock()
	defer fmt.Printf("===> LEAVING runScheduler")

	// first, remove any stopped jobs from the active list, and update
	// corresponding JobSets' statuses
	for jobID, job := range c.activeJobs {
		if job.Status.RunStatus == agent.JobRunStatus_STOPPED {
			// tell the job's JobSet to update its own status
			c.updateJobSetStatusForJob(job)

			// and remove the job from the activeJobs map since it's done
			delete(c.activeJobs, jobID)
		}
	}

	// next, remove any stopped jobSets from the active list
	for jobSetID, js := range c.activeJobSets {
		if js.RunStatus == pbs.Status_STOPPED {
			delete(c.activeJobSets, jobSetID)
		}
	}

	// now, see if we're already at capacity for maximum number of running
	// jobs. If we are, return early without checking for new jobs to add.
	if len(c.activeJobs) >= c.maxJobsRunning {
		return
	}

	// we have capacity for new jobs. start walking through the active
	// jobSets, check for ready jobs and add them as we go.
	for _, js := range c.activeJobSets {
		readyAgentSteps := c.getReadyStepsForJobSet(js)
		for _, readyAgent := range readyAgentSteps {
			// ready to submit this as a new Job to run
			jobID := c.nextJobID
			c.nextJobID++

			// create the Job's configuration
			cfg := c.getJobConfigForStep(readyAgent)

			// create a Job to store data within the controller
			job := &Job{
				JobID:           jobID,
				JobSetID:        readyAgent.JobSetID,
				JobSetStepID:    readyAgent.StepID,
				JobSetStepOrder: readyAgent.StepOrder,
				AgentName:       readyAgent.AgentName,
				Cfg:             *cfg,
				Status: agent.StatusReport{
					RunStatus:    agent.JobRunStatus_STARTUP,
					HealthStatus: agent.JobHealthStatus_OK,
					TimeStarted:  time.Now().Unix(),
				},
			}

			// add it to the main jobs and active jobs maps
			c.jobs[jobID] = job
			c.activeJobs[jobID] = job

			// update corresponding step with job ID, now that we know it
			readyAgent.AgentJobID = jobID

			// now, create a JobRequest
			// we do this _after_ adding to main jobs / active jobs maps
			// so that the controller will already know about them, whenever
			// the jobcontroller gets back to us with status updates
			jr := jobcontroller.JobRequest{
				JobID:     jobID,
				AgentName: readyAgent.AgentName,
				Cfg:       *cfg,
			}

			// submit it to the channel
			c.inJobStream <- jr

			// finally, check and see whether we're now at max jobs running
			// and if we are, time to stop
			if len(c.activeJobs) >= c.maxJobsRunning {
				return
			}
		}
	}
}

// updateJobSetStatusForJob updates the status of the JobSet containing the
// given Job, based on the current run and health status of that Job.
// It does not grab a lock, as runScheduler has already grabbed one and
// no other function should be calling updateJobSetStatusForJob.
func (c *Controller) updateJobSetStatusForJob(job *Job) {
	// don't grab a writer lock; runScheduler already has one

	// ===== FIXME COMPLETE =====
	// ===== FIXME BE SURE TO HANDLE ERROR STATUS APPROPRIATELY =====
}

// getReadyStepsForJobSet takes a JobSet and returns a slice of pointers
// to steps that are ready to run. The returned steps should only include
// steps that can be turned into Jobs, e.g. steps with type "agent".
// If a step with type "jobset" is ready to run, it should not be included
// in the returned steps; instead, a JobSetRequest should be submitted for
// it if one has not yet been submitted.
// If a step with type "concurrent" is ready to run, it should not be included
// in the returned steps; instead, its children (potentially including more
// sub-concurrent steps) should be handled as described above and included
// in the returned steps if they are of type "agent".
func (c *Controller) getReadyStepsForJobSet(js *JobSet) []*Step {
	readyAgentSteps, readyJobSetSteps, problem := retrieveReadySteps(js.Steps)

	if problem {
		// some problem occurred; return and don't provide any ready steps
		return nil
	}

	// create JobSetRequests for each JobSet that is ready
	if c.openForJobSetRequests {
		for _, jsStep := range readyJobSetSteps {
			// get parent JobSet so we can reuse its configs
			parentJobSetID := jsStep.JobSetID
			parentJobSet, ok := c.jobSets[parentJobSetID]
			if !ok {
				// problem finding parent job set; skip this one
				continue
			}

			jsr := JobSetRequest{
				TemplateName:    jsStep.SubJobSetTemplateName,
				Configs:         parentJobSet.Configs,
				ParentJobSetID:  parentJobSetID,
				ParentJobStepID: jsStep.StepID,
			}
			// add directly to pendingJSRs list; don't send through channel
			// because this is the same goroutine that would need to read
			// from that channel
			c.pendingJSRs.PushBack(jsr)

			// and mark this one as submitted
			jsStep.SubJobSetRequestSubmitted = true
		}
	}

	// now, return the agent steps that are ready to run
	return readyAgentSteps
}

// // getReadyJobs returns a slice of all Jobs within this JobSet that are
// // ready to be run, if any. It returns nil if no Jobs are currently ready,
// // and returns a non-nil error if a preceding Job has errored out.
// //func (c *Controller) getReadyJobs(js *JobSet) []*Job

// // runCoordinator tries to take further action on this JobSet if
// // more is currently possible. For example, if it can proceed to the
// // next step, then it will do so and will submit the corresponding Job
// // to the JobController.
// func (c *Controller) runCoordinator(js *JobSet) {
// 	// check (and if needed, update) the current step's status
// 	step := findStepInSteps(js.Steps, js.CurrentStep)
// 	if step == nil {
// 		// there's a problem: CurrentStep points to an invalid step ID
// 		return
// 	}
// }
