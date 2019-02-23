// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controller

import (
	"fmt"

	pbc "github.com/swinslow/peridot-core/pkg/controller"
	pbs "github.com/swinslow/peridot-core/pkg/status"
)

// Start tries to start the Controller, and returns error message explaining
// why not if it can't.
func (c *Controller) Start() error {
	// do NOT grab a writer lock here, the controller's own Start function
	// will get one and will return the error result to us
	return c.tryToStart()
}

// GetStatus returns the overall status info for the controller as a whole.
func (c *Controller) GetStatus() (pbs.Status, pbs.Health, string, string) {
	c.m.RLocker().Lock()
	defer c.m.RLocker().Unlock()
	return c.runStatus, c.healthStatus, c.outputMsg, c.errorMsg
}

// AddAgent asks the Controller to add the requested new agent,
// prior to starting the JobController. It returns nil if the agent
// is added to the configuration structure for JobController, or a
// non-nil error if unsuccessful.
func (c *Controller) AddAgent(cfg *pbc.AgentConfig) error {
	// grab a writer lock; we cannot unlock after we check on availability
	c.m.Lock()
	defer c.m.Unlock()

	// first check whether an agent with this name is already registered
	_, ok := c.agents[cfg.Name]
	if ok {
		// an agent already exists in the config with this name; error out
		return fmt.Errorf("agent with name %s is already registered", cfg.Name)
	}

	// name is available, so we'll register it
	c.agents[cfg.Name] = *cfg
	return nil
}

// GetAgent returns config information about the Agent with the given name,
// or error if not found. It does not provide status info (e.g., is the
// Agent running?) since that would be better addressed by checking the
// applicable pod's health via Kubernetes.
func (c *Controller) GetAgent(agentName string) (*pbc.AgentConfig, error) {
	// grab a reader lock
	c.m.RLocker().Lock()
	ac, ok := c.agents[agentName]
	c.m.RLocker().Unlock()

	if !ok {
		// no agent found with this name
		return nil, fmt.Errorf("no agent found with name %s", agentName)
	}

	return &ac, nil
}

// GetAllAgents returns the config information for all current agents.
func (c *Controller) GetAllAgents() []*pbc.AgentConfig {
	cfgs := []*pbc.AgentConfig{}

	// grab a reader lock
	c.m.RLocker().Lock()
	defer c.m.RLocker().Unlock()
	for _, cfg := range c.agents {
		// make a copy -- don't return the pointer to the actual record
		// FIXME this might be unnecessary; cfg may already be a copy.
		// FIXME but, not currently sure whether the same memory location
		// FIXME is used for cfg across the entire loop.
		ac := cfg
		cfgs = append(cfgs, &ac)
	}

	return cfgs
}

func cloneStepTemplate(inSteps []*StepTemplate) []*StepTemplate {
	steps := []*StepTemplate{}

	for _, inStep := range inSteps {
		newStep := &StepTemplate{T: inStep.T}
		switch newStep.T {
		case StepTypeAgent:
			newStep.AgentName = inStep.AgentName
		case StepTypeJobSet:
			newStep.JSTemplateName = inStep.JSTemplateName
		case StepTypeConcurrent:
			newStep.ConcurrentStepTemplates = cloneStepTemplate(inStep.ConcurrentStepTemplates)
		}
		steps = append(steps, newStep)
	}

	return steps
}

// AddJobSetTemplate asks the Controller to register a new jobSetTemplate.
// It returns nil if the jobSetTemplate was successfully added, or a non-nil
// error if unsuccessful.
func (c *Controller) AddJobSetTemplate(name string, inSteps []*StepTemplate) error {
	// first, before we grab the lock, let's prepare the actual template
	// structure so we're ready to add it if the name is available
	steps := cloneStepTemplate(inSteps)
	jst := &JobSetTemplate{Name: name, Steps: steps}

	// grab a writer lock; we cannot unlock after we check on availability
	// until we have actually registered the template
	c.m.Lock()
	defer c.m.Unlock()

	// first check whether a template with this name is already registered
	_, ok := c.jobSetTemplates[name]
	if ok {
		// a template is already registered with this name; error out
		return fmt.Errorf("template with name %s is already registered", name)
	}

	// name is available, so we'll register it
	c.jobSetTemplates[name] = jst
	return nil
}

// GetJobSetTemplate requests information about the JobSetTemplate with the given name.
func (c *Controller) GetJobSetTemplate(name string) ([]*StepTemplate, error) {
	// grab a reader lock
	c.m.RLocker().Lock()
	defer c.m.RLocker().Unlock()

	jst, ok := c.jobSetTemplates[name]
	if !ok {
		return nil, fmt.Errorf("no template found with name %s", name)
	}

	// copy the StepTemplate into a separate data structure to return
	steps := cloneStepTemplate(jst.Steps)
	return steps, nil
}

// GetAllJobSetTemplates requests information about all registered JobSetTemplates.
func (c *Controller) GetAllJobSetTemplates() map[string][]*StepTemplate {
	// grab a reader lock
	c.m.RLocker().Lock()
	defer c.m.RLocker().Unlock()

	// copy each StepTemplate into a separate data structure to return
	templates := map[string][]*StepTemplate{}
	for name, jst := range c.jobSetTemplates {
		stepsCopy := cloneStepTemplate(jst.Steps)
		templates[name] = stepsCopy
	}
	return templates
}

// GetJob requests information about the Job with the given ID.
func (c *Controller) GetJob(jobID uint64) (*Job, error) {
	// grab a reader lock
	c.m.RLocker().Lock()
	defer c.m.RLocker().Unlock()

	jd, ok := c.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("no job found with ID %d", jobID)
	}

	// make a copy
	jobDetails := &Job{
		JobID:           jd.JobID,
		JobSetID:        jd.JobSetID,
		JobSetStepID:    jd.JobSetStepID,
		JobSetStepOrder: jd.JobSetStepOrder,
		AgentName:       jd.AgentName,
		Cfg:             jd.Cfg,
		Status:          jd.Status,
	}
	return jobDetails, nil
}

// GetAllJobs requests information about all known Jobs.
func (c *Controller) GetAllJobs() []*Job {
	// grab a reader lock
	c.m.RLocker().Lock()
	defer c.m.RLocker().Unlock()

	jobs := []*Job{}

	for _, jd := range c.jobs {
		// make a copy
		jobDetails := &Job{
			JobID:           jd.JobID,
			JobSetID:        jd.JobSetID,
			JobSetStepID:    jd.JobSetStepID,
			JobSetStepOrder: jd.JobSetStepOrder,
			AgentName:       jd.AgentName,
			Cfg:             jd.Cfg,
			Status:          jd.Status,
		}

		jobs = append(jobs, jobDetails)
	}

	return jobs
}

// GetAllJobsForJobSet requests information about all Jobs within a given JobSet.
func (c *Controller) GetAllJobsForJobSet(jobSetID uint64) []*Job {
	// grab a reader lock
	c.m.RLocker().Lock()
	defer c.m.RLocker().Unlock()

	jobs := []*Job{}

	for _, jd := range c.jobs {
		if jd.JobSetID == jobSetID {
			// make a copy
			jobDetails := &Job{
				JobID:           jd.JobID,
				JobSetID:        jd.JobSetID,
				JobSetStepID:    jd.JobSetStepID,
				JobSetStepOrder: jd.JobSetStepOrder,
				AgentName:       jd.AgentName,
				Cfg:             jd.Cfg,
				Status:          jd.Status,
			}

			jobs = append(jobs, jobDetails)
		}
	}

	return jobs
}

func cloneSteps(inSteps []*Step) []*Step {
	if inSteps == nil {
		return nil
	}

	steps := []*Step{}

	for _, inStep := range inSteps {
		newStep := &Step{
			T:                     inStep.T,
			JobSetID:              inStep.JobSetID,
			StepID:                inStep.StepID,
			StepOrder:             inStep.StepOrder,
			RunStatus:             inStep.RunStatus,
			HealthStatus:          inStep.HealthStatus,
			AgentJobID:            inStep.AgentJobID,
			AgentName:             inStep.AgentName,
			SubJobSetID:           inStep.SubJobSetID,
			SubJobSetTemplateName: inStep.SubJobSetTemplateName,
			ConcurrentSteps:       cloneSteps(inStep.ConcurrentSteps),
		}
		steps = append(steps, newStep)
	}
	return steps
}

// GetJobSet requests information about the JobSet with the given ID.
func (c *Controller) GetJobSet(jobSetID uint64) (*JobSet, error) {
	// grab a reader lock
	c.m.RLocker().Lock()
	defer c.m.RLocker().Unlock()

	js, ok := c.jobSets[jobSetID]
	if !ok {
		return nil, fmt.Errorf("no jobSet found with ID %d", jobSetID)
	}

	// make a copy
	jobSetDetails := &JobSet{
		JobSetID:     js.JobSetID,
		TemplateName: js.TemplateName,
		RunStatus:    js.RunStatus,
		HealthStatus: js.HealthStatus,
		Steps:        cloneSteps(js.Steps),
	}
	// copy Configs one-by-one as well
	jobSetDetails.Configs = map[string]string{}
	for k, v := range js.Configs {
		jobSetDetails.Configs[k] = v
	}
	return jobSetDetails, nil
}

// GetAllJobSets requests information about all JobSets.
func (c *Controller) GetAllJobSets() []*JobSet {
	// grab a reader lock
	c.m.RLocker().Lock()
	defer c.m.RLocker().Unlock()

	jobSets := []*JobSet{}

	for _, js := range c.jobSets {
		// make a copy
		jobSetDetails := &JobSet{
			JobSetID:     js.JobSetID,
			TemplateName: js.TemplateName,
			RunStatus:    js.RunStatus,
			HealthStatus: js.HealthStatus,
			Steps:        cloneSteps(js.Steps),
		}
		// copy Configs one-by-one as well
		jobSetDetails.Configs = map[string]string{}
		for k, v := range js.Configs {
			jobSetDetails.Configs[k] = v
		}

		jobSets = append(jobSets, jobSetDetails)
	}

	return jobSets
}
