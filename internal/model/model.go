package model

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// GenerateHandle creates a short stable handle like "pat_a1b2c"
func GenerateHandle(prefix string) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		// fallback to time-based
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano()%100000)
	}
	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteByte('_')
	for _, v := range b {
		sb.WriteByte(chars[int(v)%len(chars)])
	}
	return sb.String()
}

// Pattern represents a saved regex pattern
type Pattern struct {
	Handle    string    `json:"handle"`
	Name      string    `json:"name"`
	Pattern   string    `json:"pattern"`
	Flags     string    `json:"flags"`
	Workspace string    `json:"workspace"`
	CreatedAt time.Time `json:"created_at"`
}

// MatchResult represents a single match
type MatchResult struct {
	Full   string            `json:"full"`
	Start  int               `json:"start"`
	End    int               `json:"end"`
	Groups map[string]string `json:"groups,omitempty"`
}

// TestResult represents the result of testing a pattern against input
type TestResult struct {
	Matched bool           `json:"matched"`
Count   int            `json:"count"`
	Matches []MatchResult `json:"matches,omitempty"`
}

// CompileRegex compiles a pattern with the given flags
func CompileRegex(pattern, flags string) (*regexp.Regexp, error) {
	prefix := ""
	if strings.Contains(flags, "i") {
		prefix += "i"
	}
	if strings.Contains(flags, "m") {
		prefix += "m"
	}
	if strings.Contains(flags, "s") {
		prefix += "s"
	}
	if len(prefix) > 0 {
		pattern = "(?" + prefix + ")" + pattern
	}
	return regexp.Compile(pattern)
}

// TestPattern tests a regex pattern against input text
func TestPattern(pattern, flags, input string, findAll bool) (*TestResult, error) {
	re, err := CompileRegex(pattern, flags)
	if err != nil {
		return nil, err
	}

	result := &TestResult{}

	if findAll {
		matches := re.FindAllStringSubmatchIndex(input, -1)
		if matches == nil {
			return result, nil
		}
		result.Matched = true
		result.Count = len(matches)
		names := re.SubexpNames()
		for _, m := range matches {
			mr := MatchResult{
				Full:  input[m[0]:m[1]],
				Start: m[0],
				End:   m[1],
			}
			if len(names) > 1 {
				mr.Groups = make(map[string]string)
				for i := 2; i < len(m); i += 2 {
					if m[i] >= 0 && m[i+1] >= 0 {
						name := names[i/2]
						if name == "" {
							name = fmt.Sprintf("%d", i/2)
						}
						mr.Groups[name] = input[m[i]:m[i+1]]
					}
				}
			}
			result.Matches = append(result.Matches, mr)
		}
	} else {
		match := re.FindStringSubmatchIndex(input)
		if match == nil {
			return result, nil
		}
		result.Matched = true
		result.Count = 1
		mr := MatchResult{
			Full:  input[match[0]:match[1]],
			Start: match[0],
			End:   match[1],
		}
		names := re.SubexpNames()
		if len(names) > 1 {
			mr.Groups = make(map[string]string)
			for i := 2; i < len(match); i += 2 {
				if match[i] >= 0 && match[i+1] >= 0 {
					name := names[i/2]
					if name == "" {
						name = fmt.Sprintf("%d", i/2)
					}
					mr.Groups[name] = input[match[i]:match[i+1]]
				}
			}
		}
		result.Matches = []MatchResult{mr}
	}

	return result, nil
}

// ReplaceAll replaces all matches in input with replacement
func ReplaceAll(pattern, flags, input, replacement string) (string, int, error) {
	re, err := CompileRegex(pattern, flags)
	if err != nil {
		return "", 0, err
	}
	matches := re.FindAllStringIndex(input, -1)
	count := 0
	if matches != nil {
		count = len(matches)
	}
	result := re.ReplaceAllString(input, replacement)
	return result, count, nil
}

// Split splits input on pattern matches
func Split(pattern, flags, input string, limit int) ([]string, error) {
	re, err := CompileRegex(pattern, flags)
	if err != nil {
		return nil, err
	}
	return re.Split(input, limit), nil
}

// ValidatePattern checks if a pattern compiles
func ValidatePattern(pattern, flags string) error {
	_, err := CompileRegex(pattern, flags)
	return err
}
