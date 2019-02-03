// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controller

import (
	"fmt"

	pbc "github.com/swinslow/peridot-core/pkg/controller"
)

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
	for _, cfg := range c.agents {
		// make a copy -- don't return the pointer to the actual record
		// FIXME this might be unnecessary; cfg may already be a copy.
		// FIXME but, not currently sure whether the same memory location
		// FIXME is used for cfg across the entire loop.
		ac := cfg
		cfgs = append(cfgs, &ac)
	}
	c.m.RLocker().Unlock()

	return cfgs
}

func cloneStepTemplate(inSteps []*StepTemplate) ([]*StepTemplate, error) {
	steps := []*StepTemplate{}

	for _, inStep := range inSteps {
		newStep := &StepTemplate{T: inStep.T}
		switch newStep.T {
		case StepTypeAgent:
			if inStep.AgentName == "" {
				return nil, fmt.Errorf("invalid empty string for agent name in steps")
			}
			newStep.AgentName = inStep.AgentName
		case StepTypeJobSet:
			if inStep.JSTemplateName == "" {
				return nil, fmt.Errorf("invalid empty string for jobset name in steps")
			}
			newStep.JSTemplateName = inStep.JSTemplateName
		case StepTypeConcurrent:
			subSteps, err := cloneStepTemplate(inStep.ConcurrentStepTemplates)
			if err != nil {
				return nil, err
			}
			newStep.ConcurrentStepTemplates = subSteps
		}
		steps = append(steps, newStep)
	}

	return steps, nil
}

// AddJobSetTemplate asks the Controller to register a new jobSetTemplate.
// It returns nil if the jobSetTemplate was successfully added, or a non-nil
// error if unsuccessful.
func (c *Controller) AddJobSetTemplate(name string, inSteps []*StepTemplate) error {
	// first, before we grab the lock, let's prepare the actual template
	// structure so we're ready to add it if the name is available
	steps, err := cloneStepTemplate(inSteps)
	if err != nil {
		return err
	}
	jst := &jobSetTemplate{name: name, steps: steps}

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
