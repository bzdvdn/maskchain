package preprocessor

import (
	"encoding/csv"
	"fmt"
	"strings"
)

type CSVProcessor struct {
	name  string
	rules []Rule
}

func (p *CSVProcessor) Name() string { return p.name }

// countCommasOutsideQuotes returns the number of commas that are not inside quoted fields.
func countCommasOutsideQuotes(s string) int {
	count := 0
	inQuotes := false
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			// Handle escaped quotes: "" inside a quoted field
			if inQuotes && i+1 < len(s) && s[i+1] == '"' {
				i++ // skip the escaped quote
				continue
			}
			inQuotes = !inQuotes
		} else if s[i] == ',' && !inQuotes {
			count++
		}
	}
	return count
}

// @sk-task 25-shield-preprocessors#T2.1: Implement CSVProcessor.Process (AC-001, AC-002, AC-006)
func (p *CSVProcessor) Process(data string, namespace string) *ProcessResult {
	colMask := make(map[string]string)
	for _, r := range p.rules {
		for _, col := range r.Columns {
			colMask[col] = string(r.Mask)
		}
	}
	if len(colMask) == 0 {
		return &ProcessResult{ModifiedText: data, Replacements: map[string]string{}}
	}

	lines := strings.Split(data, "\n")
	var phIdx int
	replacements := make(map[string]string)

	// Scan for CSV blocks (quote-aware comma counting)
	type blockRange struct{ start, end int }
	var blocks []blockRange
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		cc := countCommasOutsideQuotes(trimmed)
		if cc == 0 {
			i++
			continue
		}
		commaCount := cc
		start := i
		i++
		for i < len(lines) {
			t := strings.TrimSpace(lines[i])
			if t == "" {
				break
			}
			if countCommasOutsideQuotes(t) != commaCount {
				break
			}
			i++
		}
		if i-start >= 2 {
			blocks = append(blocks, blockRange{start, i})
		}
	}

	if len(blocks) == 0 {
		return &ProcessResult{ModifiedText: data, Replacements: map[string]string{}}
	}

	// Process blocks in reverse order to preserve line indices
	for bi := len(blocks) - 1; bi >= 0; bi-- {
		b := blocks[bi]
		blockText := strings.Join(lines[b.start:b.end], "\n")

		r := csv.NewReader(strings.NewReader(blockText))
		records, err := r.ReadAll()
		if err != nil || len(records) < 2 {
			continue
		}

		header := records[0]
		colIdx := make(map[string]int)
		for ci, name := range header {
			colIdx[name] = ci
		}

		type target struct {
			idx  int
			mode string
		}
		var targets []target
		for col, mode := range colMask {
			if idx, ok := colIdx[col]; ok {
				targets = append(targets, target{idx, mode})
			}
		}
		if len(targets) == 0 {
			continue
		}

		maskedRecords := make([][]string, len(records))
		maskedRecords[0] = header
		for rowIdx := 1; rowIdx < len(records); rowIdx++ {
			row := make([]string, len(records[rowIdx]))
			copy(row, records[rowIdx])
			for _, t := range targets {
				if t.idx >= len(row) {
					continue
				}
				orig := row[t.idx]
				switch t.mode {
				case "full":
					ph := fmt.Sprintf("{{csv.%s.%d}}", namespace, phIdx)
					replacements[orig] = ph
					row[t.idx] = ph
					phIdx++
				case "surname":
					parts := strings.Fields(orig)
					if len(parts) > 0 {
						row[t.idx] = parts[0]
					}
				}
			}
			maskedRecords[rowIdx] = row
		}

		var buf strings.Builder
		w := csv.NewWriter(&buf)
		if err := w.WriteAll(maskedRecords); err != nil {
			continue
		}
		maskedBlock := strings.TrimRight(buf.String(), "\n")

		// Replace block in original lines
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:b.start]...)
		newLines = append(newLines, strings.Split(maskedBlock, "\n")...)
		newLines = append(newLines, lines[b.end:]...)
		lines = newLines
	}

	return &ProcessResult{ModifiedText: strings.Join(lines, "\n"), Replacements: replacements}
}
