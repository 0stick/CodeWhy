package render

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/0stick/CodeWhy/internal/model"
)

type Options struct {
	JSON  bool
	Color bool
}

func Result(w io.Writer, result model.Result, options Options) error {
	if options.JSON {
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	return terminal(w, result, options.Color)
}

func terminal(w io.Writer, result model.Result, color bool) error {
	label := func(value string) string {
		if !color {
			return value
		}
		return "\x1b[1;36m" + value + "\x1b[0m"
	}
	value := func(text string) string {
		if !color {
			return text
		}
		return "\x1b[1m" + text + "\x1b[0m"
	}

	if result.Target != nil {
		if _, err := fmt.Fprintf(w, "%s %s:%d\n", label("Code:"), filepath.ToSlash(result.Target.File), result.Target.Line); err != nil {
			return err
		}
		if result.Target.Code != "" {
			if _, err := fmt.Fprintf(w, "%s %s\n", label("Target:"), result.Target.Code); err != nil {
				return err
			}
		}
		for _, line := range result.Target.Context {
			marker := " "
			if line.Current {
				marker = ">"
			}
			if _, err := fmt.Fprintf(w, "  %s %5d | %s\n", marker, line.Line, line.Code); err != nil {
				return err
			}
		}
	}

	shortSHA := result.Commit.SHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	date := result.Commit.Date
	if len(date) >= 10 {
		date = date[:10]
	}
	if _, err := fmt.Fprintf(w, "%s %s on %s by %s\n", label("Introduced:"), value(shortSHA), date, result.Commit.Author); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s %s\n", label("Commit:"), indentMultiline(result.Commit.Message, "        ")); err != nil {
		return err
	}
	if result.Commit.URL != "" {
		if _, err := fmt.Fprintf(w, "%s %s\n", label("Commit URL:"), result.Commit.URL); err != nil {
			return err
		}

	}
	if result.PullRequest != nil {
		if _, err := fmt.Fprintf(w, "%s #%d %s\n%s %s\n", label("Pull Request:"), result.PullRequest.Number, result.PullRequest.Title, label("PR URL:"), result.PullRequest.URL); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(w, "%s None found\n", label("Pull Request:")); err != nil {
			return err
		}
	}
	if len(result.Issues) == 0 {
		if _, err := fmt.Fprintf(w, "%s None found\n", label("Related Issue:")); err != nil {
			return err
		}
	} else {
		for index, issue := range result.Issues {
			issueLabel := "              "
			if index == 0 {
				issueLabel = label("Related Issue:")
			}
			if _, err := fmt.Fprintf(w, "%s #%d %s\n%s %s\n", issueLabel, issue.Number, issue.Title, label("Issue URL:"), issue.URL); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, "%s %s\n%s %s\n", label("Reason:"), result.Reason, label("Confidence:"), result.Confidence); err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		if _, err := fmt.Fprintf(w, "%s %s\n", label("Warning:"), warning); err != nil {
			return err
		}
	}
	return nil
}

func indentMultiline(value, indent string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "\n", "\n"+indent)
}
