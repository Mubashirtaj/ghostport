package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mubashirtaj/ghostport/internal"
	"github.com/spf13/cobra"
)

var Version = "dev"

var (
	colorGhost  = lipgloss.Color("135") // --purple---
	colorBusy   = lipgloss.Color("203") // ---red---
	colorFree   = lipgloss.Color("42")  // ---green----
	colorMuted  = lipgloss.Color("245") // ----gray----
	colorAccent = lipgloss.Color("39")  // ----blue----

	titleStyle = lipgloss.NewStyle().
			Foreground(colorGhost).
			Bold(true)

	busyStyle = lipgloss.NewStyle().
			Foreground(colorBusy).
			Bold(true)

	freeStyle = lipgloss.NewStyle().
			Foreground(colorFree).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGhost).
			Padding(1, 3)
)

var rootCmd = &cobra.Command{
	Use:     "ghostport [port]",
	Short:   "👻 Explain and kill whatever is using a local port",
	Version: Version,
	Args:    cobra.ExactArgs(1),
	RunE:    runRoot,
}

func Execute() {
	rootCmd.SetVersionTemplate("ghostport version {{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	port, err := strconv.Atoi(args[0])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %q: must be a number between 1 and 65535", args[0])
	}

	fmt.Println(titleStyle.Render(fmt.Sprintf("👻 GhostPort v%s", Version)))

	info, err := internal.FindProcessByPort(port)
	if err != nil {
		return fmt.Errorf("failed to inspect port %d: %w", port, err)
	}

	if info == nil {
		body := fmt.Sprintf("Port %d is %s\nNothing is listening here — it's all yours.",
			port, freeStyle.Render("FREE"))
		fmt.Println(boxStyle.Render(body))
		return nil
	}

	renderPortInfo(info)

	return promptAction(info)
}

func renderPortInfo(info *internal.PortInfo) {
	var b strings.Builder

	fmt.Fprintf(&b, "Port %d is %s\n\n", info.Port, busyStyle.Render("BUSY"))
	fmt.Fprintf(&b, "%s %s (PID: %d)\n", labelStyle.Render("→ Process:"), info.ProcessName, info.PID)

	if info.CWD != "" {
		fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("→ Project:"), info.CWD)
	}
	if info.Cmdline != "" {
		fmt.Fprintf(&b, "%s %s\n", labelStyle.Render("→ Running:"), info.Cmdline)
	}
	fmt.Fprintf(&b, "%s %s ago\n", labelStyle.Render("→ Uptime:"), info.Uptime())

	if info.IsDocker {
		fmt.Fprintf(&b, "%s %s container\n", labelStyle.Render("→ Docker:"), info.DockerName)
	}

	fmt.Fprintln(&b)
	fmt.Fprint(&b, mutedStyle.Render("Prompt: [k] Kill, [o] Open folder, [q] Quit"))

	fmt.Println(boxStyle.Render(b.String()))
}

func promptAction(info *internal.PortInfo) error {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil
		}
		choice := strings.ToLower(strings.TrimSpace(line))

		switch choice {
		case "k", "kill":
			if info.IsDocker {
				if err := internal.StopContainer(info.DockerName); err != nil {
					return fmt.Errorf("failed to stop container: %w", err)
				}
				fmt.Println(freeStyle.Render(fmt.Sprintf("✓ Stopped container %s", info.DockerName)))
				return nil
			}
			if err := internal.KillPID(info.PID); err != nil {
				return fmt.Errorf("failed to kill process: %w", err)
			}
			fmt.Println(freeStyle.Render(fmt.Sprintf("✓ Killed %s (PID: %d)", info.ProcessName, info.PID)))
			return nil
		case "o", "open":
			if info.CWD == "" {
				fmt.Println(mutedStyle.Render("No project folder known for this process."))
				continue
			}
			if err := openFolder(info.CWD); err != nil {
				fmt.Println(busyStyle.Render(fmt.Sprintf("Failed to open folder: %v", err)))
			}
			return nil
		case "q", "quit", "":
			return nil
		default:
			fmt.Println(mutedStyle.Render("Please choose [k] Kill, [o] Open folder, or [q] Quit"))
		}
	}
}

func openFolder(path string) error {
	var name string
	var args []string

	switch runtime.GOOS {
	case "windows":
		name, args = "explorer", []string{path}
	case "darwin":
		name, args = "open", []string{path}
	default:
		name, args = "xdg-open", []string{path}
	}

	return exec.Command(name, args...).Start()
}
