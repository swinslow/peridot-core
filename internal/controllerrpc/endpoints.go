// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controllerrpc

import (
	"context"

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
