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

// maxAttempts bounds the run → conflict → kill → rerun cycle. Without a cap, a
// process supervised by something that restarts it would loop forever.
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
	// Stop parsing flags at the first positional argument so the wrapped
	// command keeps its own flags: `ghostport run npm run dev --port 3000`.
	runCmd.Flags().SetInterspersed(false)

	rootCmd.AddCommand(runCmd)
}

// watchResult is the outcome of one supervised run of the user's command.
type watchResult struct {
	exitCode    int
	sawConflict bool
	port        int
}

func runRun(cmd *cobra.Command, args []string) error {
	// Cobra prints usage for any error we return, which is noise once the
	// child command is the thing that failed.
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

		// Not a port problem, or one we can't pin to a number — stay out of the
		// way and exit exactly as the command did.
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

// executeWatched runs the command with its streams wired to the terminal, while
// a sniffer reads the same bytes looking for a port collision.
func executeWatched(args []string) (watchResult, error) {
	child := exec.Command(args[0], args[1:]...)

	sniffer := internal.NewConflictSniffer()
	child.Stdin = os.Stdin
	child.Stdout = io.MultiWriter(os.Stdout, sniffer)
	child.Stderr = io.MultiWriter(os.Stderr, sniffer)

	err := child.Run()
	sniffer.Flush()

	// The command couldn't be started at all — a typo'd binary, say. That's our
	// error to report, not an exit code to pass along.
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
		// A signal-killed process reports -1; treat it as a generic failure so
		// the code we eventually exit with is a valid one.
		if res.exitCode < 0 {
			res.exitCode = 1
		}
	default:
		return watchResult{}, fmt.Errorf("failed to run %q: %w", args[0], err)
	}

	return res, nil
}

// exitWith mirrors the wrapped command's exit code, so `ghostport run` is
// transparent to scripts and CI.
func exitWith(code int) error {
	if code == 0 {
		return nil
	}
	os.Exit(code)
	return nil
}
