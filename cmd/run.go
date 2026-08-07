package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mubashirtaj/ghostport/internal"
	"github.com/spf13/cobra"
)

const maxAttempts = 4

var noRetry bool

var runCmd = &cobra.Command{
	Use:   "run [flags] -- <command> [args...]",
	Short: "Run any command and auto-inspect the port when it dies on a conflict",
	Long: `Run wraps a command — a dev server, a container, anything — and passes its
output straight through to your terminal.

If the command exits because the port it wanted was taken, GhostPort reads the
port out of the error message and opens the usual inspect prompt for it. Kill
the squatter and your command is started again automatically.

Works with whatever prints the error: node, python, go, docker, java, rails,
.NET, and anything else that says the address is already in use.`,
	Example: `  ghostport run npm run dev
  ghostport run docker compose up
  ghostport run python manage.py runserver
  ghostport run -- go run . --port 8080`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRun,
}

func init() {
	runCmd.Flags().BoolVar(&noRetry, "no-retry", false, "don't re-run the command after freeing the port")

	runCmd.Flags().SetInterspersed(false)

	rootCmd.AddCommand(runCmd)
}

type watchResult struct {
	exitCode    int
	sawConflict bool
	port        int
}

func runRun(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	argPort := internal.PortFromArgs(args)

	for attempt := 1; ; attempt++ {
		res, err := executeWatched(args)
		if err != nil {
			return err
		}

		if res.exitCode == 0 {
			return nil
		}

		port := res.port
		if port == 0 {
			port = argPort
		}

		if !res.sawConflict || port == 0 {
			if res.sawConflict {
				fmt.Fprintln(os.Stderr, mutedStyle.Render(
					"👻 GhostPort saw a port conflict but couldn't tell which port. Try `ghostport <port>`."))
			}
			return exitWith(res.exitCode)
		}

		fmt.Println()
		fmt.Println(titleStyle.Render(fmt.Sprintf("👻 GhostPort %s", displayVersion())))
		fmt.Println(mutedStyle.Render(fmt.Sprintf("  detected port conflict on %d", port)))

		freed, err := inspectPort(port)
		if err != nil {
			return err
		}

		if !freed || noRetry {
			return exitWith(res.exitCode)
		}

		if attempt >= maxAttempts {
			fmt.Println(mutedStyle.Render(
				fmt.Sprintf("Port %d keeps getting taken — giving up after %d attempts.", port, maxAttempts)))
			return exitWith(res.exitCode)
		}

		fmt.Println(labelStyle.Render(fmt.Sprintf("↻ Retrying: %s", strings.Join(args, " "))))
		fmt.Println()
	}
}

func executeWatched(args []string) (watchResult, error) {
	child := exec.Command(args[0], args[1:]...)

	sniffer := internal.NewConflictSniffer()
	child.Stdin = os.Stdin
	child.Stdout = io.MultiWriter(os.Stdout, sniffer)
	child.Stderr = io.MultiWriter(os.Stderr, sniffer)

	err := child.Run()
	sniffer.Flush()

	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return watchResult{}, fmt.Errorf("cannot run %q: %w", args[0], execErr.Err)
	}

	var res watchResult
	res.port, res.sawConflict = sniffer.Conflict()

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		res.exitCode = 0
	case errors.As(err, &exitErr):
		res.exitCode = exitErr.ExitCode()

		if res.exitCode < 0 {
			res.exitCode = 1
		}
	default:
		return watchResult{}, fmt.Errorf("failed to run %q: %w", args[0], err)
	}

	return res, nil
}

func exitWith(code int) error {
	if code == 0 {
		return nil
	}
	os.Exit(code)
	return nil
}
