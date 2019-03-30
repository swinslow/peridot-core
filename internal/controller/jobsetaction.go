// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controller

import (
	"container/list"
	"fmt"
	"time"

	"github.com/swinslow/peridot-core/internal/jobcontroller"
	pbs "github.com/swinslow/peridot-core/pkg/status"
)

// createNewJobSets walks through the current pendingJSRs queue, and creates
// new JobSets based on the requests found there. It can recursively add new
// JobSetRequests to the end of the queue and it will handle them in turn
// before returning.
func (c *Controller) createNewJobSets() {
	// grab a writer lock
	c.m.Lock()
	defer c.m.Unlock()

	// iterate over the pendingJSRs list, which can grow as we iterate:
	for e := c.pendingJSRs.Front(); e != nil; e = e.Next() {
		// retrieve the next JobSetRequest from the list
		jsr := e.Value.(JobSetRequest)

		var jobSetID uint64
		if jsr.RequestedJobSetID != 0 {
			jobSetID = jsr.RequestedJobSetID
		} else {
			jobSetID = c.nextJobSetID
			c.nextJobSetID++
		}

		// first things first, create a new JobSet entry in our jobSets map
		js := &JobSet{
			JobSetID:     jobSetID,
			TemplateName: jsr.TemplateName,
			RunStatus:    pbs.Status_STARTUP,
			HealthStatus: pbs.Health_OK,
			TimeStarted:  time.Now(),
			// leave TimeFinished as zero value
		}
		c.jobSets[js.JobSetID] = js
		// also add to active JobSet list
		c.activeJobSets[js.JobSetID] = js

		// make sure the TemplateName is a template we actually know about
		jst, ok := c.jobSetTemplates[js.TemplateName]
		if !ok {
			// unknown template; error out
			js.ErrorMessages = fmt.Sprintf("%s is not a known JobSetTemplate name", js.TemplateName)
			js.RunStatus = pbs.Status_STOPPED
			js.HealthStatus = pbs.Health_ERROR
			js.TimeFinished = time.Now()
			return
		}

		// copy over configs from JobSetRequest
		js.Configs = make(map[string]string)
		for k, v := range jsr.Configs {
			js.Configs[k] = v
		}

		// now create steps from template
		js.Steps = createStepsFromTemplate(js, c.pendingJSRs, jst.Steps)

		// finally, if we have a parentJobSetID / parentJobStepID, then
		// this JobSet was created as a step within another JobSet.
		// We should update the parent's step to let it know what the
		// finally-determined JobSet ID was.
		if jsr.ParentJobSetID != 0 {
			// find the parent JobSet
			parentJS, ok := c.jobSets[jsr.ParentJobSetID]
			if !ok {
				// parent JobSet doesn't exist; move controller to error state
				errMsg := fmt.Sprintf("JobSetRequest requested parent ID %d but parent JobSet with that ID does not exist\n", jsr.ParentJobSetID)
				js.RunStatus = pbs.Status_STOPPED
				js.HealthStatus = pbs.Health_ERROR
				js.ErrorMessages += errMsg
				c.runStatus = pbs.Status_STOPPED
				c.healthStatus = pbs.Health_ERROR
				c.errorMsg += errMsg
				return
			}

			// find the right step within the parent JobSet's steps
			stepToUpdate := findStepInSteps(parentJS.Steps, jsr.ParentJobStepID)
			if stepToUpdate == nil {
				// step with this ID wasn't found in parent JobSet;
				// move controller to error state
				errMsg := fmt.Sprintf("JobSetRequest requested Step ID %d in parent JobSet ID %d but parent Step with that ID does not exist\n", jsr.ParentJobStepID, jsr.ParentJobSetID)
				js.RunStatus = pbs.Status_STOPPED
				js.HealthStatus = pbs.Health_ERROR
				js.ErrorMessages += errMsg
				c.runStatus = pbs.Status_STOPPED
				c.healthStatus = pbs.Health_ERROR
				c.errorMsg += errMsg
				return
			}

			// if we get here, we're good to update the parent step's SubJobSetID
			stepToUpdate.SubJobSetID = js.JobSetID
		}

		// and we're done with this one!
	}

	// now, dump and recreate pendingJSRs because these have now been handled
	c.pendingJSRs = list.New()
}

// updateJobStatus updates the status of the Job with the ID noted in the
// JobRecord received from the JobController.
func (c *Controller) updateJobStatus(jr *jobcontroller.JobRecord) {
	// grab a writer lock
	c.m.Lock()
	defer c.m.Unlock()

	// find the job with this ID
	job, ok := c.jobs[jr.JobID]
	if !ok {
		// got a record report for a job we didn't know about; set error
		// status and begin shutting down controller
		c.runStatus = pbs.Status_STOPPED
		c.healthStatus = pbs.Health_ERROR
		c.errorMsg += fmt.Sprintf("received JobRecord status update for Job with ID %d but no such Job found", jr.JobID)
		return
	}

	// update status
	job.Status = jr.Status
}
