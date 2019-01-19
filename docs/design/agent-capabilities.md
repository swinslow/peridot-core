`SPDX-License-Identifier: CC-BY-4.0`

# Agent capabilities

An Agent can respond to an `AgentDescribe` request with various capabilities
as strings. The following are recognized by peridot:

- **codewriter**: The Agent can write files to disk, for analysis by a
codereader or for other purposes. Examples of a codewriter could include:
  - obtaining files by cloning a Git repo
  - unpacking a .zip file or tarball
  - retrieving dependencies over the network
  - recursively doing any of the above

- **codereader**: The Agent can read files that have been previously written
by a codewriter. Examples of a codereader could include:
  - license scanner or license ID scanner
  - copyright notice scanner
  - file type analysis tool
  - binary analysis tool

- **spdxwriter**: The Agent can create and write SPDX files to disk. Examples
of an spdxwriter could include:
  - any of the codereader examples listed above
  - obtaining content from a remote source about code or a dependency

- **spdxreader**: The Agent can consume SPDX files that have been previously
written to disk. Examples of an spdxreader could include:
  - reuse of previous license clearing decisions
  - policy enforcement (e.g., stop a build based on SPDX content)

- **artifactwriter**: The Agent can create and write other types of artifacts
to disk, *other than* SPDX files. Examples of an artifactwriter could include:
  - license notices text file generator
  - packaging up corresponding source code artifacts
