package internal

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func IsDockerPort(port int) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.Ports}}|{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}

	portStr := strconv.Itoa(port)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		portsField, name := parts[0], parts[1]

		if portMatches(portsField, portStr) {
			return strings.TrimSpace(name), true
		}
	}

	return "", false
}
func StopContainer(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "stop", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stop %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}

	return nil
}

func portMatches(portsField, portStr string) bool {
	mappings := strings.Split(portsField, ",")
	for _, mapping := range mappings {
		mapping = strings.TrimSpace(mapping)
		hostPart := mapping
		if idx := strings.Index(mapping, "->"); idx != -1 {
			hostPart = mapping[:idx]
		}
		if idx := strings.LastIndex(hostPart, ":"); idx != -1 {
			hostPart = hostPart[idx+1:]
		}
		if hostPart == portStr {
			return true
		}
	}
	return false
}
