package internal

import (
	"fmt"
	"time"

	psnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

type PortInfo struct {
	Port        int
	PID         int32
	ProcessName string
	CWD         string
	Cmdline     string
	CreateTime  time.Time
	IsDocker    bool
	DockerName  string
}

func (p *PortInfo) Uptime() string {
	return humanizeDuration(time.Since(p.CreateTime))
}

func FindProcessByPort(port int) (*PortInfo, error) {
	conns, err := psnet.Connections("tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to list tcp connections: %w", err)
	}

	var pid int32 = -1
	for _, c := range conns {
		if c.Status != "LISTEN" {
			continue
		}
		if int(c.Laddr.Port) == port {
			pid = c.Pid
			break
		}
	}

	if pid <= 0 {
		return nil, nil
	}

	proc, err := process.NewProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect process %d: %w", pid, err)
	}

	info := &PortInfo{Port: port, PID: pid}

	if name, err := proc.Name(); err == nil {
		info.ProcessName = name
	}
	if cwd, err := proc.Cwd(); err == nil {
		info.CWD = cwd
	}
	if cmdline, err := proc.Cmdline(); err == nil {
		info.Cmdline = cmdline
	}
	if createMs, err := proc.CreateTime(); err == nil {
		info.CreateTime = time.UnixMilli(createMs)
	}

	if name, ok := IsDockerPort(port); ok {
		info.IsDocker = true
		info.DockerName = name
	}

	return info, nil
}

func humanizeDuration(d time.Duration) string {
	if d < time.Second {
		return "just now"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
