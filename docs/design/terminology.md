`SPDX-License-Identifier: CC-BY-4.0`

# Peridot Terminology

### Agent

An **Agent** is a separate process, running as a gRPC server, that can
start new Jobs. See [agent-capabilities.md] for more details on the specific
capabilities of any given Agent.

Each Agent would typically be running in a separate container.

Multiple instances of a single Agent codebase can be running simultaneously as
separate Agents. Each instance must be configured with a different `name`
(unique across peridot) but instances of the same Agent codebase would have
the same Agent `type`.

### Job

A **Job** is a single run of the Agent's `runAgent` functionality on a
particular codebase, etc. Each Job is started by the peridot controller with
configuration details that tell it e.g. where to find the code to be scanned,
the input SPDX files (if any), the output directory for code / SPDX / other
artifacts, and so on.

### JobSet

A **JobSet** is a pipeline set of one or more Jobs. Output directories from
earlier Jobs in the JobSet will be fed to the subsequent Jobs' configuration
as input directories.

Each JobSet has a name that must be unique across the peridot controller.

A JobSet consists of a series of steps; each step can be any of:
- an Agent name;
- a JobSet name;
- `concurrent`, which triggers a subset of Agents / JobSets / concurrents that
  can run concurrently

A JobSet is triggered by reference to a JobSetTemplate.

### JobSetTemplate

A **JobSetTemplate** defines what runs when a new JobSet is requested. The
user can define one or more JobSetTemplates (typically via a YAML
configuration file) and can then start a new JobSet via the template's name,
together with the applicable configuration details.
