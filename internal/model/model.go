package model

const SchemaVersion = "1"

type Result struct {
	SchemaVersion string      `json:"schema_version"`
	Target        *Target     `json:"target,omitempty"`
	Commit        Commit      `json:"commit"`
	PullRequest   *Reference  `json:"pull_request,omitempty"`
	Issues        []Reference `json:"issues"`
	Reason        string      `json:"reason"`
	Confidence    string      `json:"confidence"`
	Warnings      []string    `json:"warnings"`
}

type Target struct {
	File       string        `json:"file"`
	Line       int           `json:"line"`
	SourceFile string        `json:"source_file,omitempty"`
	SourceLine int           `json:"source_line,omitempty"`
	Code       string        `json:"code,omitempty"`
	Context    []ContextLine `json:"context,omitempty"`
}

type ContextLine struct {
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Current bool   `json:"current"`
}

type Commit struct {
	SHA     string   `json:"sha"`
	Author  string   `json:"author"`
	Date    string   `json:"date"`
	Message string   `json:"message"`
	Diff    string   `json:"diff,omitempty"`
	Files   []string `json:"files"`
	URL     string   `json:"url,omitempty"`
}

type Reference struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body,omitempty"`
	URL    string `json:"url"`
	State  string `json:"state,omitempty"`
}
