// SPDX-License-Identifier: Apache-2.0 OR GPL-2.0-or-later

package controller

import (
	"fmt"
	"path/filepath"
	"strconv"
)

func getCodeOutputDir(volPrefix string, jobSetID uint64, jobID uint64, agentName string) string {
	return filepath.Join(volPrefix, "code", strconv.FormatUint(jobSetID, 10), agentName, strconv.FormatUint(jobID, 10))
}

func getSpdxOutputDir(volPrefix string, jobSetID uint64, jobID uint64, agentName string) string {
	return filepath.Join(volPrefix, "spdx", strconv.FormatUint(jobSetID, 10), agentName, strconv.FormatUint(jobID, 10))
}

func getCodeInput(volPrefix string, jobSetID uint64, jobID uint64, agentName string) (string, string) {
	source := fmt.Sprintf("%s.%d.%d", agentName, jobSetID, jobID)
	dirpath := filepath.Join(volPrefix, "code", strconv.FormatUint(jobSetID, 10), agentName, strconv.FormatUint(jobID, 10))
	return source, dirpath
}

func getSpdxInput(volPrefix string, jobSetID uint64, jobID uint64, agentName string) (string, string) {
	source := fmt.Sprintf("%s.%d.%d", agentName, jobSetID, jobID)
	dirpath := filepath.Join(volPrefix, "spdx", strconv.FormatUint(jobSetID, 10), agentName, strconv.FormatUint(jobID, 10))
	return source, dirpath
}
