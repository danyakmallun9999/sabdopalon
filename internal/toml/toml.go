// Package toml is a minimal, dependency-free TOML decoder sufficient for
// Sabdopalon's engine.toml (tables, key/value, strings, integers, booleans).
//
// It intentionally supports only the subset we need. If requirements grow
// beyond simple tables and scalars, swap this for a vetted library.
package toml

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Value is a parsed scalar value, typed as string/int/bool.
type Value any

// Table is a flat map of section -> key -> value.
// Nested tables beyond one level are represented with dotted keys flattened
// into the top-level section name (e.g. [a.b] becomes section "a.b").
type Table map[string]map[string]Value

// Decode parses TOML input from r into a Table.
func Decode(r io.Reader) (Table, error) {
	t := Table{}
	section := ""
	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := stripComment(raw)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// table header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if section == "" {
				return nil, fmt.Errorf("toml: line %d: empty table name", lineNo)
			}
			if _, ok := t[section]; !ok {
				t[section] = map[string]Value{}
			}
			continue
		}
		// key = value
		idx := strings.Index(line, "=")
		if idx < 0 {
			return nil, fmt.Errorf("toml: line %d: missing '=' in %q", lineNo, raw)
		}
		key := strings.TrimSpace(line[:idx])
		valStr := strings.TrimSpace(line[idx+1:])
		val, err := parseValue(valStr)
		if err != nil {
			return nil, fmt.Errorf("toml: line %d: %w", lineNo, err)
		}
		if section == "" {
			return nil, fmt.Errorf("toml: line %d: key %q before any table", lineNo, key)
		}
		t[section][key] = val
	}
	return t, scanner.Err()
}

// DecodeString is a convenience wrapper for string input.
func DecodeString(s string) (Table, error) {
	return Decode(strings.NewReader(s))
}

// GetString reads a value as a string (raw fallback).
func (t Table) GetString(section, key, fallback string) string {
	if sec, ok := t[section]; ok {
		if v, ok := sec[key]; ok {
			switch s := v.(type) {
			case string:
				return s
			case int:
				return strconv.Itoa(s)
			case bool:
				return strconv.FormatBool(s)
			}
		}
	}
	return fallback
}

// GetInt reads a value as an int, or fallback.
func (t Table) GetInt(section, key string, fallback int) int {
	if sec, ok := t[section]; ok {
		if v, ok := sec[key]; ok {
			if i, ok := v.(int); ok {
				return i
			}
		}
	}
	return fallback
}

// GetBool reads a value as a bool, or fallback.
func (t Table) GetBool(section, key string, fallback bool) bool {
	if sec, ok := t[section]; ok {
		if v, ok := sec[key]; ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
	}
	return fallback
}

// --- helpers ---

func stripComment(line string) string {
	// Strip # comments, but not inside quotes.
	inSingle := false
	inDouble := false
	for i, ch := range line {
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
}

func parseValue(s string) (Value, error) {
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}
	// string (double-quoted)
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) && len(s) >= 2 {
		return strings.ReplaceAll(s[1:len(s)-1], `\"`, `"`), nil
	}
	// string (single-quoted / literal)
	if strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`) && len(s) >= 2 {
		return s[1 : len(s)-1], nil
	}
	// bool
	if s == "true" {
		return true, nil
	}
	if s == "false" {
		return false, nil
	}
	// int
	if i, err := strconv.Atoi(s); err == nil {
		return i, nil
	}
	// fallback: treat as bare string
	return s, nil
}
