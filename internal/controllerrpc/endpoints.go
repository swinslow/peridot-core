// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controllerrpc

import (
	"context"

	"github.com/swinslow/peridot-core/internal/controller"
	pbc "github.com/swinslow/peridot-core/pkg/controller"
)

// AddAgent corresponds to the AddAgent endpoint for pkg/controller.
func (cs *cServer) AddAgent(ctx context.Context, req *pbc.AddAgentReq) (*pbc.AddAgentResp, error) {
	err := cs.c.AddAgent(req.Cfg)
	if err != nil {
		return &pbc.AddAgentResp{
			Success:  false,
			ErrorMsg: err.Error(),
		}, nil
	}
	return &pbc.AddAgentResp{Success: true}, nil
}

// GetAgent corresponds to the GetAgent endpoint for pkg/controller.
func (cs *cServer) GetAgent(ctx context.Context, req *pbc.GetAgentReq) (*pbc.GetAgentResp, error) {
	cfg, err := cs.c.GetAgent(req.Name)
	if err != nil {
		return &pbc.GetAgentResp{
			Success:  false,
			ErrorMsg: err.Error(),
		}, nil
	}
	return &pbc.GetAgentResp{
		Success: true,
		Cfg:     cfg,
	}, nil
}

// GetAllAgents corresponds to the GetAllAgents endpoint for pkg/controller.
func (cs *cServer) GetAllAgents(ctx context.Context, req *pbc.GetAllAgentsReq) (*pbc.GetAllAgentsResp, error) {
	cfgs := cs.c.GetAllAgents()
	return &pbc.GetAllAgentsResp{Cfgs: cfgs}, nil
}

func createStepTemplate(inSteps []*pbc.Step) []*controller.StepTemplate {
	steps := []*controller.StepTemplate{}

	for _, inStep := range inSteps {
		newStep := &controller.StepTemplate{}
		switch x := inStep.S.(type) {
		case *pbc.Step_Agent:
			newStep.T = controller.StepTypeAgent
			newStep.AgentName = x.Agent.Name
		case *pbc.Step_Jobset:
			newStep.T = controller.StepTypeJobSet
			newStep.JSTemplateName = x.Jobset.Name
		case *pbc.Step_Concurrent:
			newStep.T = controller.StepTypeConcurrent
			newStep.ConcurrentStepTemplates = createStepTemplate(x.Concurrent.Steps)
		}
		steps = append(steps, newStep)
	}

	return steps
}

// AddJobSetTemplate corresponds to the AddJobSetTemplate endpoint for pkg/controller.
func (cs *cServer) AddJobSetTemplate(ctx context.Context, req *pbc.AddJobSetTemplateReq) (*pbc.AddJobSetTemplateResp, error) {
	// build the jobSetTemplate structure to send to the controller
	name := req.Jst.Name
	steps := createStepTemplate(req.Jst.Steps)

	err := cs.c.AddJobSetTemplate(name, steps)
	if err != nil {
		return &pbc.AddJobSetTemplateResp{
			Success:  false,
			ErrorMsg: err.Error(),
		}, nil
	}
	return &pbc.AddJobSetTemplateResp{Success: true}, nil
}
