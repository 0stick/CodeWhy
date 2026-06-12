package target

import (
	"fmt"
	"strconv"
	"strings"
)

type Location struct {
	File string
	Line int
}

func Parse(value string) (Location, error) {
	colon := strings.LastIndex(value, ":")
	if colon <= 0 || colon == len(value)-1 {
		return Location{}, fmt.Errorf("invalid target %q: expected <file>:<line>, for example src/auth.go:42", value)
	}

	line, err := strconv.Atoi(value[colon+1:])
	if err != nil || line < 1 {
		return Location{}, fmt.Errorf("invalid target %q: line must be a positive integer", value)
	}

	file := value[:colon]
	if strings.TrimSpace(file) == "" {
		return Location{}, fmt.Errorf("invalid target %q: file path is empty", value)
	}

	return Location{File: file, Line: line}, nil
}
