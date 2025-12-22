package diff

import (
	"strconv"
	"strings"
)

// LineType represents the type of a line in a diff.
type LineType int

const (
	// LineContext represents an unchanged context line (starts with ' ').
	LineContext LineType = iota
	// LineAddition represents an added line (starts with '+').
	LineAddition
	// LineDeletion represents a deleted line (starts with '-').
	LineDeletion
)

// Line represents a single line in a diff hunk.
type Line struct {
	Type     LineType // The type of change
	Content  string   // The line content (without the prefix)
	NewLine  *int     // Line number in new file (nil for deletions)
	Position int      // Position in diff (1-indexed from first @@)
}

// Hunk represents a single @@ hunk in a unified diff.
type Hunk struct {
	OldStart int    // Starting line in old file
	OldLines int    // Number of lines from old file
	NewStart int    // Starting line in new file
	NewLines int    // Number of lines in new file
	Lines    []Line // The lines in this hunk
}

// ParsedDiff represents a parsed unified diff for a single file.
type ParsedDiff struct {
	Hunks []Hunk
}

// Parse parses a unified diff string into a ParsedDiff.
// It handles standard git diff output including file headers.
func Parse(patch string) (ParsedDiff, error) {
	if patch == "" {
		return ParsedDiff{}, nil
	}

	lines := strings.Split(patch, "\n")
	result := ParsedDiff{}

	var currentHunk *Hunk
	position := 0
	currentNewLine := 0

	for _, line := range lines {
		// Skip empty lines at end
		if line == "" {
			continue
		}

		// Skip file headers (diff --git, index, ---, +++)
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			continue
		}

		// Skip "\ No newline at end of file" markers
		if strings.HasPrefix(line, "\\ ") {
			continue
		}

		// Parse hunk header
		if strings.HasPrefix(line, "@@") {
			// Save previous hunk if exists
			if currentHunk != nil {
				result.Hunks = append(result.Hunks, *currentHunk)
			}

			hunk, err := parseHunkHeader(line)
			if err != nil {
				// Skip malformed headers
				continue
			}

			currentHunk = &hunk
			currentNewLine = hunk.NewStart
			continue
		}

		// Skip if not in a hunk yet
		if currentHunk == nil {
			continue
		}

		// Parse diff line
		position++
		diffLine := Line{
			Position: position,
		}

		if len(line) > 0 {
			switch line[0] {
			case '+':
				diffLine.Type = LineAddition
				diffLine.Content = line[1:]
				diffLine.NewLine = IntPtr(currentNewLine)
				currentNewLine++
			case '-':
				diffLine.Type = LineDeletion
				diffLine.Content = line[1:]
				// Deletions don't have new-side line numbers
				diffLine.NewLine = nil
			case ' ':
				diffLine.Type = LineContext
				diffLine.Content = line[1:]
				diffLine.NewLine = IntPtr(currentNewLine)
				currentNewLine++
			default:
				// Treat unknown as context (handles edge cases)
				diffLine.Type = LineContext
				diffLine.Content = line
				diffLine.NewLine = IntPtr(currentNewLine)
				currentNewLine++
			}
		}

		currentHunk.Lines = append(currentHunk.Lines, diffLine)
	}

	// Don't forget the last hunk
	if currentHunk != nil {
		result.Hunks = append(result.Hunks, *currentHunk)
	}

	return result, nil
}

// FindPosition returns the diff position for a given new-side line number.
// Returns nil if the line is not in the diff (context-only file regions,
// deleted lines, or lines outside the diff).
// Position is 1-indexed from the first @@ hunk header.
func (pd ParsedDiff) FindPosition(newLineNumber int) *int {
	if newLineNumber <= 0 {
		return nil
	}

	for _, hunk := range pd.Hunks {
		for _, line := range hunk.Lines {
			if line.NewLine != nil && *line.NewLine == newLineNumber {
				return IntPtr(line.Position)
			}
		}
	}

	return nil
}

// parseHunkHeader parses a hunk header line like "@@ -10,7 +10,8 @@ optional context".
func parseHunkHeader(line string) (Hunk, error) {
	hunk := Hunk{}

	// Find the @@ markers
	parts := strings.Split(line, "@@")
	if len(parts) < 2 {
		return hunk, nil
	}

	// Parse the range info between @@ markers
	rangeInfo := strings.TrimSpace(parts[1])
	rangeParts := strings.Fields(rangeInfo)

	for _, part := range rangeParts {
		if strings.HasPrefix(part, "-") {
			// Old file range: -start,count or -start
			old := strings.TrimPrefix(part, "-")
			oldStart, oldLines := parseRange(old)
			hunk.OldStart = oldStart
			hunk.OldLines = oldLines
		} else if strings.HasPrefix(part, "+") {
			// New file range: +start,count or +start
			new := strings.TrimPrefix(part, "+")
			newStart, newLines := parseRange(new)
			hunk.NewStart = newStart
			hunk.NewLines = newLines
		}
	}

	return hunk, nil
}

// parseRange parses "start,count" or "start" format.
func parseRange(s string) (start, count int) {
	if idx := strings.Index(s, ","); idx >= 0 {
		start, _ = strconv.Atoi(s[:idx])
		count, _ = strconv.Atoi(s[idx+1:])
	} else {
		start, _ = strconv.Atoi(s)
		count = 1
	}
	return
}

// IntPtr returns a pointer to the given int value.
// Exported for use in tests across packages.
func IntPtr(n int) *int {
	return &n
}
