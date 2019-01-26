#!/bin/bash

# SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

# Generates Golang protobuf code from .proto files.
# Should be run from the top-level peridot-core directory.

protoc -I ./ pkg/agent/agent.proto --go_out=plugins=grpc:.
protoc -I ./ pkg/controller/controller.proto --go_out=plugins=grpc:.
