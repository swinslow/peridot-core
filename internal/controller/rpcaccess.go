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
	// first check whether an agent with this name is already registered
	// grab a reader lock
	c.m.RLocker().Lock()
	_, ok := c.agents[cfg.Name]
	c.m.RLocker().Unlock()

	if ok {
		// an agent already exists in the config with this name; error out
		return fmt.Errorf("agent with name %s is already registered", cfg.Name)
	}

	// name is available, so we'll register it
	// grab a writer lock
	c.m.Lock()
	defer c.m.Unlock()
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
