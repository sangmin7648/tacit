package modeldownloader

import (
	"os/exec"
	"strconv"
	"strings"
)

// SystemMemory returns the installed RAM in bytes, or 0 when it cannot be read.
func SystemMemory() int64 {
	return parseMemSize(exec.Command("sysctl", "-n", "hw.memsize"))
}

func parseMemSize(cmd *exec.Cmd) int64 {
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
