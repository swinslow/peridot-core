// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controllerrpc

import (
	"context"

	"github.com/swinslow/peridot-core/internal/controller"
	pbc "github.com/swinslow/peridot-core/pkg/controller"
)

// Start corresponds to the Start endpoint for pkg/controller.
func (cs *CServer) Start(ctx context.Context, req *pbc.StartReq) (*pbc.StartResp, error) {
	err := cs.C.Start()
	if err == nil {
		return &pbc.StartResp{Starting: true}, nil
	}

	return &pbc.StartResp{
		Starting: false,
		ErrorMsg: err.Error(),
	}, nil
}

// GetStatus corresponds to the GetStatus endpoint for pkg/controller.
func (cs *CServer) GetStatus(ctx context.Context, req *pbc.GetStatusReq) (*pbc.GetStatusResp, error) {
	runStatus, healthStatus, outputMsg, errorMsg := cs.C.GetStatus()

	return &pbc.GetStatusResp{
		RunStatus:    runStatus,
		HealthStatus: healthStatus,
		OutputMsg:    outputMsg,
		ErrorMsg:     errorMsg,
	}, nil
}

// Stop corresponds to the Stop endpoint for pkg/controller.
func (cs *CServer) Stop(ctx context.Context, req *pbc.StopReq) (*pbc.StopResp, error) {
	cs.C.Stop()
	return &pbc.StopResp{}, nil
}

// AddAgent corresponds to the AddAgent endpoint for pkg/controller.
func (cs *CServer) AddAgent(ctx context.Context, req *pbc.AddAgentReq) (*pbc.AddAgentResp, error) {
	err := cs.C.AddAgent(req.Cfg)
	if err != nil {
		return &pbc.AddAgentResp{
			Success:  false,
			ErrorMsg: err.Error(),
		}, nil
	}
	return &pbc.AddAgentResp{Success: true}, nil
}

// GetAgent corresponds to the GetAgent endpoint for pkg/controller.
func (cs *CServer) GetAgent(ctx context.Context, req *pbc.GetAgentReq) (*pbc.GetAgentResp, error) {
	cfg, err := cs.C.GetAgent(req.Name)
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
func (cs *CServer) GetAllAgents(ctx context.Context, req *pbc.GetAllAgentsReq) (*pbc.GetAllAgentsResp, error) {
	cfgs := cs.C.GetAllAgents()
	return &pbc.GetAllAgentsResp{Cfgs: cfgs}, nil
}

func createStepTemplateFromProtoSteps(inSteps []*pbc.StepTemplate) []*controller.StepTemplate {
	steps := []*controller.StepTemplate{}

	for _, inStep := range inSteps {
		newStep := &controller.StepTemplate{}
		switch x := inStep.S.(type) {
		case *pbc.StepTemplate_Agent:
			newStep.T = controller.StepTypeAgent
			newStep.AgentName = x.Agent.Name
		case *pbc.StepTemplate_Jobset:
			newStep.T = controller.StepTypeJobSet
			newStep.JSTemplateName = x.Jobset.Name
		case *pbc.StepTemplate_Concurrent:
			newStep.T = controller.StepTypeConcurrent
			newStep.ConcurrentStepTemplates = createStepTemplateFromProtoSteps(x.Concurrent.Steps)
		}
		steps = append(steps, newStep)
	}

	return steps
}

func createProtoStepsFromStepTemplate(inSteps []*controller.StepTemplate) []*pbc.StepTemplate {
	steps := []*pbc.StepTemplate{}

	for _, inStep := range inSteps {
		newStep := &pbc.StepTemplate{}
		switch inStep.T {
		case controller.StepTypeAgent:
			newStep.S = &pbc.StepTemplate_Agent{Agent: &pbc.StepAgentTemplate{Name: inStep.AgentName}}
		case controller.StepTypeJobSet:
			newStep.S = &pbc.StepTemplate_Jobset{Jobset: &pbc.StepJobSetTemplate{Name: inStep.JSTemplateName}}
		case controller.StepTypeConcurrent:
			subSteps := createProtoStepsFromStepTemplate(inStep.ConcurrentStepTemplates)
			newStep.S = &pbc.StepTemplate_Concurrent{Concurrent: &pbc.StepConcurrentTemplate{Steps: subSteps}}
		}
		steps = append(steps, newStep)
	}

	return steps
}

// AddJobSetTemplate corresponds to the AddJobSetTemplate endpoint for pkg/controller.
func (cs *CServer) AddJobSetTemplate(ctx context.Context, req *pbc.AddJobSetTemplateReq) (*pbc.AddJobSetTemplateResp, error) {
	// build the jobSetTemplate structure to send to the controller
	name := req.Jst.Name
	steps := createStepTemplateFromProtoSteps(req.Jst.Steps)

	err := cs.C.AddJobSetTemplate(name, steps)
	if err != nil {
		return &pbc.AddJobSetTemplateResp{
			Success:  false,
			ErrorMsg: err.Error(),
		}, nil
	}
	return &pbc.AddJobSetTemplateResp{Success: true}, nil
}

// GetJobSetTemplate corresponds to the GetJobSetTemplate endpoint for pkg/controller.
func (cs *CServer) GetJobSetTemplate(ctx context.Context, req *pbc.GetJobSetTemplateReq) (*pbc.GetJobSetTemplateResp, error) {
	steps, err := cs.C.GetJobSetTemplate(req.Name)
	if err != nil {
		return &pbc.GetJobSetTemplateResp{
			Success:  false,
			ErrorMsg: err.Error(),
		}, nil
	}

	jst := &pbc.JobSetTemplate{
		Name:  req.Name,
		Steps: createProtoStepsFromStepTemplate(steps),
	}
	return &pbc.GetJobSetTemplateResp{
		Success: true,
		Jst:     jst,
	}, nil
}

// GetAllJobSetTemplates corresponds to the GetAllJobSetTemplates endpoint for pkg/controller.
func (cs *CServer) GetAllJobSetTemplates(ctx context.Context, req *pbc.GetAllJobSetTemplatesReq) (*pbc.GetAllJobSetTemplatesResp, error) {
	templates := cs.C.GetAllJobSetTemplates()

	protoTemplates := []*pbc.JobSetTemplate{}
	for name, steps := range templates {
		jst := &pbc.JobSetTemplate{
			Name:  name,
			Steps: createProtoStepsFromStepTemplate(steps),
		}
		protoTemplates = append(protoTemplates, jst)
	}

	return &pbc.GetAllJobSetTemplatesResp{Jsts: protoTemplates}, nil
}

// GetJob corresponds to the GetJob endpoint for pkg/controller.
func (cs *CServer) GetJob(ctx context.Context, req *pbc.GetJobReq) (*pbc.GetJobResp, error) {
	job, err := cs.C.GetJob(req.JobID)
	if err != nil {
		return &pbc.GetJobResp{
			Success:  false,
			ErrorMsg: err.Error(),
		}, nil
	}

	jd := &pbc.JobDetails{
		JobID:           job.JobID,
		JobSetID:        job.JobSetID,
		JobSetStepID:    job.JobSetStepID,
		JobSetStepOrder: job.JobSetStepOrder,
		AgentName:       job.AgentName,
		Cfg:             &job.Cfg,
		St:              &job.Status,
	}
	return &pbc.GetJobResp{
		Success: true,
		Job:     jd,
	}, nil
}

// GetAllJobs corresponds to the GetAllJobs endpoint for pkg/controller.
func (cs *CServer) GetAllJobs(ctx context.Context, req *pbc.GetAllJobsReq) (*pbc.GetAllJobsResp, error) {
	jobs := cs.C.GetAllJobs()

	jds := []*pbc.JobDetails{}

	for _, job := range jobs {
		jd := &pbc.JobDetails{
			JobID:           job.JobID,
			JobSetID:        job.JobSetID,
			JobSetStepID:    job.JobSetStepID,
			JobSetStepOrder: job.JobSetStepOrder,
			AgentName:       job.AgentName,
			Cfg:             &job.Cfg,
			St:              &job.Status,
		}
		jds = append(jds, jd)
	}

	return &pbc.GetAllJobsResp{Jobs: jds}, nil
}

// GetAllJobsForJobSet corresponds to the GetAllJobsForJobSet endpoint for pkg/controller.
func (cs *CServer) GetAllJobsForJobSet(ctx context.Context, req *pbc.GetAllJobsForJobSetReq) (*pbc.GetAllJobsForJobSetResp, error) {
	jobs := cs.C.GetAllJobsForJobSet(req.JobSetID)

	jds := []*pbc.JobDetails{}

	for _, job := range jobs {
		jd := &pbc.JobDetails{
			JobID:           job.JobID,
			JobSetID:        job.JobSetID,
			JobSetStepID:    job.JobSetStepID,
			JobSetStepOrder: job.JobSetStepOrder,
			AgentName:       job.AgentName,
			Cfg:             &job.Cfg,
			St:              &job.Status,
		}
		jds = append(jds, jd)
	}

	return &pbc.GetAllJobsForJobSetResp{Jobs: jds}, nil
}

func createProtoStepsFromSteps(inSteps []*controller.Step) []*pbc.Step {
	steps := []*pbc.Step{}

	for _, inStep := range inSteps {
		newStep := &pbc.Step{
			StepID:       inStep.StepID,
			StepOrder:    inStep.StepOrder,
			RunStatus:    inStep.RunStatus,
			HealthStatus: inStep.HealthStatus,
		}
		switch inStep.T {
		case controller.StepTypeAgent:
			newStep.S = &pbc.Step_Agent{Agent: &pbc.StepAgent{AgentName: inStep.AgentName, JobID: inStep.AgentJobID}}
		case controller.StepTypeJobSet:
			newStep.S = &pbc.Step_Jobset{Jobset: &pbc.StepJobSet{TemplateName: inStep.SubJobSetTemplateName, JobSetID: inStep.SubJobSetID}}
		case controller.StepTypeConcurrent:
			subSteps := createProtoStepsFromSteps(inStep.ConcurrentSteps)
			newStep.S = &pbc.Step_Concurrent{Concurrent: &pbc.StepConcurrent{Steps: subSteps}}
		}
		steps = append(steps, newStep)
	}

	return steps
}

// StartJobSet corresponds to the StartJobSet endpoint for pkg/controller.
func (cs *CServer) StartJobSet(ctx context.Context, req *pbc.StartJobSetReq) (*pbc.StartJobSetResp, error) {
	jobSetID, err := cs.C.StartJobSet(req.JstName, req.Cfgs)
	if err != nil {
		return &pbc.StartJobSetResp{
			Success:  false,
			ErrorMsg: err.Error(),
		}, nil
	}
	return &pbc.StartJobSetResp{
		Success:  true,
		JobSetID: jobSetID,
	}, nil
}

// GetJobSet corresponds to the GetJobSet endpoint for pkg/controller.
func (cs *CServer) GetJobSet(ctx context.Context, req *pbc.GetJobSetReq) (*pbc.GetJobSetResp, error) {
	js, err := cs.C.GetJobSet(req.JobSetID)
	if err != nil {
		return &pbc.GetJobSetResp{
			Success:  false,
			ErrorMsg: err.Error(),
		}, nil
	}

	st := &pbc.JobSetStatusReport{
		RunStatus:      js.RunStatus,
		HealthStatus:   js.HealthStatus,
		TimeStarted:    js.TimeStarted.Unix(),
		TimeFinished:   js.TimeFinished.Unix(),
		OutputMessages: js.OutputMessages,
		ErrorMessages:  js.ErrorMessages,
	}

	steps := createProtoStepsFromSteps(js.Steps)

	jsd := &pbc.JobSetDetails{
		JobSetID:     js.JobSetID,
		TemplateName: js.TemplateName,
		St:           st,
		Steps:        steps,
	}
	return &pbc.GetJobSetResp{
		Success: true,
		JobSet:  jsd,
	}, nil
}

// GetAllJobSets corresponds to the GetAllJobSets endpoint for pkg/controller.
func (cs *CServer) GetAllJobSets(ctx context.Context, req *pbc.GetAllJobSetsReq) (*pbc.GetAllJobSetsResp, error) {
	jss := cs.C.GetAllJobSets()

	jobSets := []*pbc.JobSetDetails{}

	for _, js := range jss {
		st := &pbc.JobSetStatusReport{
			RunStatus:      js.RunStatus,
			HealthStatus:   js.HealthStatus,
			TimeStarted:    js.TimeStarted.Unix(),
			TimeFinished:   js.TimeFinished.Unix(),
			OutputMessages: js.OutputMessages,
			ErrorMessages:  js.ErrorMessages,
		}

		steps := createProtoStepsFromSteps(js.Steps)

		jsd := &pbc.JobSetDetails{
			JobSetID:     js.JobSetID,
			TemplateName: js.TemplateName,
			St:           st,
			Steps:        steps,
		}

		jobSets = append(jobSets, jsd)
	}
	return &pbc.GetAllJobSetsResp{JobSets: jobSets}, nil
}
