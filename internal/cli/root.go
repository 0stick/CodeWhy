package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/0stick/CodeWhy/internal/analyze"
	"github.com/0stick/CodeWhy/internal/render"
	"github.com/0stick/CodeWhy/internal/target"
	"github.com/spf13/cobra"
)

var version = "0.1.0-dev"

func Execute() error {
	return ExecuteArgs(os.Args[1:], os.Stdout, os.Stderr)
}

func ExecuteArgs(args []string, stdout, stderr io.Writer) error {
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	err := cmd.Execute()
	if err == nil || !requestsJSON(args) {
		return err
	}
	if renderErr := render.Error(stdout, errorCode(err), err.Error()); renderErr != nil {
		return renderErr
	}
	return renderedError{err: err}
}

func ErrorWasRendered(err error) bool {
	var rendered renderedError
	return errors.As(err, &rendered)
}

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "codewhy <file>:<line>",
		Short:         "Explain why a line of code exists",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExplain(cmd, args[0])
		},
	}

	flags := cmd.PersistentFlags()
	flags.Bool("json", false, "output machine-readable JSON")
	flags.Bool("no-color", false, "disable terminal colors")
	flags.Bool("offline", false, "only use local Git information")
	flags.String("remote", "origin", "Git remote to inspect")
	flags.Int("context", 0, "show this many lines around the target")
	flags.BoolP("verbose", "v", false, "show analysis progress")

	explain := &cobra.Command{
		Use:   "explain <file>:<line>",
		Short: "Explain why a line of code exists",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExplain(cmd, args[0])
		},
	}
	cmd.AddCommand(explain)

	commit := &cobra.Command{
		Use:   "commit <sha>",
		Short: "Explain a commit using local and GitHub metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommit(cmd, args[0])
		},
	}
	cmd.AddCommand(commit)

	return cmd
}

func runExplain(cmd *cobra.Command, value string) error {
	location, err := target.Parse(value)
	if err != nil {
		return err
	}
	options, renderOptions, err := commandOptions(cmd)
	if err != nil {
		return err
	}
	result, err := analyze.New(".").Explain(cmd.Context(), location, options)
	if err != nil {
		return err
	}
	return render.Result(cmd.OutOrStdout(), result, renderOptions)
}

func runCommit(cmd *cobra.Command, sha string) error {
	options, renderOptions, err := commandOptions(cmd)
	if err != nil {
		return err
	}
	result, err := analyze.New(".").ExplainCommit(cmd.Context(), sha, options)
	if err != nil {
		return err
	}
	return render.Result(cmd.OutOrStdout(), result, renderOptions)
}

func commandOptions(cmd *cobra.Command) (analyze.Options, render.Options, error) {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	noColor, _ := cmd.Flags().GetBool("no-color")
	offline, _ := cmd.Flags().GetBool("offline")
	remote, _ := cmd.Flags().GetString("remote")
	contextLines, _ := cmd.Flags().GetInt("context")
	verbose, _ := cmd.Flags().GetBool("verbose")
	if contextLines < 0 {
		return analyze.Options{}, render.Options{}, fmt.Errorf("--context must be zero or greater")
	}
	if remote == "" {
		return analyze.Options{}, render.Options{}, fmt.Errorf("--remote cannot be empty")
	}

	var logger func(string)
	if verbose {
		logger = func(message string) {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "codewhy:", message)
		}
	}
	color := !jsonOutput && !noColor && os.Getenv("NO_COLOR") == "" && isTerminal(cmd.OutOrStdout())
	return analyze.Options{
			Remote:  remote,
			Offline: offline,
			Context: contextLines,
			Verbose: logger,
		}, render.Options{
			JSON:  jsonOutput,
			Color: color,
		}, nil
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type renderedError struct {
	err error
}

func (e renderedError) Error() string {
	return e.err.Error()
}

func (e renderedError) Unwrap() error {
	return e.err
}

func requestsJSON(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
		if value, found := strings.CutPrefix(arg, "--json="); found {
			enabled, err := strconv.ParseBool(value)
			return err == nil && enabled
		}
	}
	return false
}

func errorCode(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "expected <file>:<line>"), strings.Contains(message, "line must be a positive integer"), strings.Contains(message, "file path is empty"):
		return "invalid_target"
	case strings.Contains(message, "accepts ") && strings.Contains(message, "arg"):
		return "invalid_arguments"
	case strings.Contains(message, "unknown flag"), strings.Contains(message, "--context must"), strings.Contains(message, "--remote cannot"):
		return "invalid_option"
	case strings.Contains(message, "not inside a git repository"):
		return "not_git_repository"
	case strings.Contains(message, "git is required"):
		return "git_not_found"
	case strings.Contains(message, "cannot blame"):
		return "blame_failed"
	case strings.Contains(message, "cannot read commit"):
		return "commit_failed"
	default:
		return "command_failed"
	}
}
