// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controller

import (
	"fmt"
	"log"
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
	fmt.Println("===> ENTERING runScheduler")
	defer c.m.Unlock()
	defer fmt.Println("===> LEAVING runScheduler")

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
		// if this jobset was still in STARTUP status, it's now running
		if js.RunStatus == pbs.Status_STARTUP {
			js.RunStatus = pbs.Status_RUNNING
		}

		readyAgentSteps := c.getReadyStepsForJobSet(js)
		for _, readyAgent := range readyAgentSteps {
			// ready to submit this as a new Job to run
			jobID := c.nextJobID
			c.nextJobID++

			// update corresponding step with job ID, now that we know it
			readyAgent.AgentJobID = jobID

			// and tell this Step that it is now running
			readyAgent.RunStatus = pbs.Status_RUNNING

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

	// find the corresponding jobSet
	js, ok := c.jobSets[job.JobSetID]
	if !ok {
		// FIXME this shouldn't happen; job with unknown jobSet ID
		log.Fatalf("failed; job ID %d has unknown job set ID %d", job.JobID, job.JobSetID)
	}

	newStatus, newHealth := c.determineStepStatuses(js.Steps)
	if newStatus != pbs.Status_STATUS_SAME {
		js.RunStatus = newStatus
	}
	if newHealth != pbs.Health_HEALTH_SAME {
		js.HealthStatus = newHealth
	}
}

// determineStepStatuses takes a slice of steps and walks through it
// (recursively if needed), looking at what the overall RunStatus and
// HealthStatus should now be. It returns the new statuses.
// It also updates the status/health of concurrent steps as needed.
func (c *Controller) determineStepStatuses(steps []*Step) (pbs.Status, pbs.Health) {
	allStopped := true
	newStatus := pbs.Status_STATUS_SAME
	newHealth := pbs.Health_HEALTH_SAME

	for _, step := range steps {
		// first, if concurrent, get sub-steps' own status and health
		// so we can update the concurrent step itself
		if step.T == StepTypeConcurrent {
			// run recursively on sub-steps
			subStatus, subHealth := c.determineStepStatuses(step.ConcurrentSteps)

			if subStatus != pbs.Status_STATUS_SAME {
				step.RunStatus = subStatus
			}
			if subHealth != pbs.Health_HEALTH_SAME {
				step.HealthStatus = subHealth
			}
		}

		// if jobset, get the separate jobSet's status and health
		if step.T == StepTypeJobSet {
			subJs, ok := c.jobSets[step.SubJobSetID]
			if !ok {
				// FIXME this shouldn't happen; job with unknown jobSet ID
				log.Fatalf("failed; jobset step %d in jobset %d has unknown subJobSet ID %d", step.StepID, step.JobSetID, step.SubJobSetID)
			}
			step.RunStatus = subJs.RunStatus
			step.HealthStatus = subJs.HealthStatus
		}

		// now, evaluate and bubble upwards for this step
		// if it is still running or in startup, check health but go on
		if step.RunStatus != pbs.Status_STOPPED {
			allStopped = false
		}
		// check and update health
		// note degraded, unless we're already in error state
		if step.HealthStatus == pbs.Health_DEGRADED && newHealth != pbs.Health_ERROR {
			newHealth = pbs.Health_DEGRADED
		}
		// and error health means the overall set of steps will be in error
		// and should also stop
		if step.HealthStatus == pbs.Health_ERROR {
			newStatus = pbs.Status_STOPPED
			newHealth = pbs.Health_ERROR
		}
	}

	// finally, decide what to bubble upwards now that we've looked at
	// all of the steps
	if allStopped {
		newStatus = pbs.Status_STOPPED
	}

	return newStatus, newHealth
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
