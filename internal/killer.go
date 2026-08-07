package internal

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func KillPID(pid int32) error {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	if err := proc.Terminate(); err == nil && !stillRunning(proc) {
		return nil
	}

	if err := proc.Kill(); err != nil && stillRunning(proc) {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	return nil
}

func stillRunning(proc *process.Process) bool {
	for range 10 {
		running, err := proc.IsRunning()
		if err != nil || !running {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
	return true
}
