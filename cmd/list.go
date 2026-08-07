package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mubashirtaj/ghostport/internal"
	"github.com/spf13/cobra"
)

var commonDevPorts = []int{3000, 3001, 5173, 8000, 8080, 5432}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show the status of common development ports",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	fmt.Println(titleStyle.Render(fmt.Sprintf("👻 GhostPort v%s", Version)))

	rows := make([][]string, 0, len(commonDevPorts))

	for _, port := range commonDevPorts {
		info, err := internal.FindProcessByPort(port)
		if err != nil || info == nil {
			rows = append(rows, []string{
				fmt.Sprintf("%d", port),
				freeStyle.Render("FREE"),
				"-",
				"-",
			})
			continue
		}

		owner := info.ProcessName
		if info.IsDocker {
			owner = fmt.Sprintf("%s (docker: %s)", owner, info.DockerName)
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", port),
			busyStyle.Render("BUSY"),
			owner,
			fmt.Sprintf("%d", info.PID),
		})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(colorGhost)).
		Headers("PORT", "STATUS", "PROCESS", "PID").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return labelStyle.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	fmt.Println(t)

	return nil
}
