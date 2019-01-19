`SPDX-License-Identifier: CC-BY-4.0`

There are (at least) two ways that management of agents and jobs could be structured:

**Heavyweight agents**: Each agent contains a mini-controller which acts as a proxy and communication point with the master controller. The mini-controller is responsible for knowing about the 1+ separate agent functions that are actually performing the agent analysis processes. The mini-controller is also responsible for tracking each job's status, and communicating status and results back to the master controller.

*Benefits*: Gives the master controller a single point of contact for checking in. Enables the agent to scale its analysis functions without the master controller knowing which internal agent is actually processing each job request.

*Drawbacks*: Each agent becomes significantly heavier, as each one now needs to track state for job status. Even if this can typically reuse code provided by peridot, it cuts into the goal of having new agents be extremely lightweight to add in.

**Lightweight agents**: Each agent maintains a long-lived gRPC connection with the master controller (in this model, the only job controller). The agent is not responsible for knowing overall state of similar jobs running on agents of the same type. Only the master controller can see the status of all agents and jobs.

*Benefits*: Agents can have fewer responsibilities and can be more disconnected. Agents do not need to store any state beyond handling the job they are currently working on. Master controller has an ongoing connection to the particular agent instance running each job.

*Drawbacks*: gRPC connections now must be long-lived. A disrupted connection means losing contact with the specific agent instance that was running a particular job, and likely means the job should be considered dead. Requires gRPC bidirectional streaming which adds complexity.
