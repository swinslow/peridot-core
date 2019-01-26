package main

import (
	"context"
	"log"

	"github.com/swinslow/peridot-core/internal/jobcontroller"
	"github.com/swinslow/peridot-core/pkg/agent"
)

func main() {
	// set up JobController configuration
	arIDsearcher := jobcontroller.AgentRef{
		Name:    "idsearcher",
		Address: "localhost:9001",
	}
	cfg := jobcontroller.Config{
		Agents: map[string]jobcontroller.AgentRef{
			"idsearcher": arIDsearcher,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start JobController and get channels
	inJobStream, jobRecordStream, errc := jobcontroller.JobController(ctx, cfg)

	// start a job
	ciPrimary := agent.JobConfig_CodeInput{
		Source: "primary",
		Paths:  []string{"/home/steve/programming/spdx/tools-golang"},
	}
	codeInputs := []*agent.JobConfig_CodeInput{&ciPrimary}
	jobReq := jobcontroller.JobRequest{
		AgentName: "idsearcher",
		Cfg: agent.JobConfig{
			CodeInputs:    codeInputs,
			SpdxOutputDir: "/tmp/report1/",
		},
	}
	inJobStream <- jobReq

	// listen for and print JobRecord notices until the stream is closed
	exiting := false
	for !exiting {
		select {
		case jr, ok := <-jobRecordStream:
			if !ok {
				exiting = true
				break
			}
			log.Printf("record notice: %s\n", jr.String())
			if jr.Status.RunStatus == agent.JobRunStatus_STOPPED {
				exiting = true
			}
		case err := <-errc:
			log.Printf("ERROR received: %v\n", err)
		}
	}
}
