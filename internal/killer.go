package internal

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/process"
)

func KillPID(pid int32) error {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	if err := proc.Terminate(); err == nil {
		if running, _ := proc.IsRunning(); !running {
			return nil
		}
	}

	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	return nil
}
