package main

import (
	"context"
	"log"

	"github.com/swinslow/peridot-core/internal/jobcontroller"
	"github.com/swinslow/peridot-core/pkg/agent"
)

func runRetrieve(inJobStream chan<- jobcontroller.JobRequest, jobRecordStream <-chan jobcontroller.JobRecord, errc <-chan error) {
	// retrieve some code
	jobReq := jobcontroller.JobRequest{
		AgentName: "retrieve-github",
		Cfg: agent.JobConfig{
			CodeOutputDir: "/tmp/code1",
			Jkvs: []*agent.JobConfig_JobKV{
				&agent.JobConfig_JobKV{Key: "org", Value: "fossology"},
				&agent.JobConfig_JobKV{Key: "repo", Value: "atarashi"},
				&agent.JobConfig_JobKV{Key: "branch", Value: "feat/ngram"},
			},
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

func runIdsearch(inJobStream chan<- jobcontroller.JobRequest, jobRecordStream <-chan jobcontroller.JobRecord, errc <-chan error) {
	// start a job
	ciPrimary := agent.JobConfig_CodeInput{
		Source: "primary",
		Paths:  []string{"/tmp/code1"},
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

func main() {
	// set up JobController configuration
	arIDsearcher := jobcontroller.AgentRef{
		Name:    "idsearcher",
		Address: "localhost:9001",
	}
	arRetrieveGithub := jobcontroller.AgentRef{
		Name:    "retrieve-github",
		Address: "localhost:9002",
	}
	cfg := jobcontroller.Config{
		Agents: map[string]jobcontroller.AgentRef{
			"idsearcher":      arIDsearcher,
			"retrieve-github": arRetrieveGithub,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start JobController and get channels
	inJobStream, jobRecordStream, errc := jobcontroller.JobController(ctx, cfg)

	// run the retriever
	runRetrieve(inJobStream, jobRecordStream, errc)

	// run the IDsearcher
	runIdsearch(inJobStream, jobRecordStream, errc)
}
