package memorysync

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Pattern is one secret kind + its compiled regex.
type Pattern struct {
	Kind string
	Re   *regexp.Regexp
}

// Patterns is the regex pack (Shannon-entropy deferred to M3-Go).
var Patterns = []Pattern{
	{"anthropic", regexp.MustCompile(`(?:sk-ant-|bsk-)[A-Za-z0-9_-]{20,}`)},
	{"openai", regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`)},
	{"aws", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"google", regexp.MustCompile(`AIza[0-9A-Za-z_\-]{35}`)},
	{"github_pat", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`)},
	{"private_key", regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA |)PRIVATE KEY-----`)},
	{"bearer", regexp.MustCompile(`Bearer\s+[A-Za-z0-9_\-\.=]{20,}`)},
	{"env_assign", regexp.MustCompile(`(?:password|passwd|secret|token|api_key|apikey|access_key)\s*[=:]\s*\S+`)},
}

var envValue = regexp.MustCompile(`[=:]\s*(\S+)`)

// Match is a scan hit.
type Match struct{ Kind, Text string }

// ScanText returns all regex hits in text.
func ScanText(text string) []Match {
	var out []Match
	for _, p := range Patterns {
		for _, m := range p.Re.FindAllString(text, -1) {
			out = append(out, Match{p.Kind, m})
		}
	}
	return out
}

type redactMatch struct {
	start, end int
	kind, val  string
}

// redactTextInternal is the shared logic with a counter (placeholders globally unique).
func redactTextInternal(text string, counter *int) (string, map[string]string) {
	var matches []redactMatch
	for _, p := range Patterns {
		for _, idx := range p.Re.FindAllStringIndex(text, -1) {
			s, e := idx[0], idx[1]
			kind, val := p.Kind, text[s:e]
			if p.Kind == "env_assign" {
				if vm := envValue.FindStringSubmatchIndex(val); vm != nil {
					s = idx[0] + vm[2]
					e = idx[0] + vm[3]
					val = text[s:e]
				}
			}
			matches = append(matches, redactMatch{s, e, kind, val})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].start < matches[j].start })
	var chosen []redactMatch
	lastEnd := -1
	for _, m := range matches {
		if m.start < lastEnd {
			continue
		}
		chosen = append(chosen, m)
		lastEnd = m.end
	}
	var b strings.Builder
	mapping := map[string]string{}
	pos := 0
	for _, m := range chosen {
		*counter++
		b.WriteString(text[pos:m.start])
		ph := fmt.Sprintf("<redacted:%s:%d>", m.kind, *counter)
		mapping[ph] = m.val
		b.WriteString(ph)
		pos = m.end
	}
	b.WriteString(text[pos:])
	return b.String(), mapping
}

// RedactText redacts secrets in a single string (counter starts at 1).
func RedactText(text string) (string, map[string]string) {
	c := 0
	return redactTextInternal(text, &c)
}

// RedactObj recursively redacts all strings in o with a SHARED counter
// (placeholders globally unique — no collision across sibling strings).
func RedactObj(o any) (any, map[string]string) {
	mapping := map[string]string{}
	counter := 0
	var walk func(x any) any
	walk = func(x any) any {
		switch v := x.(type) {
		case string:
			red, m := redactTextInternal(v, &counter)
			for k, val := range m {
				mapping[k] = val
			}
			return red
		case []any:
			for i, item := range v {
				v[i] = walk(item)
			}
			return v
		case map[string]any:
			for k, val := range v {
				v[k] = walk(val)
			}
			return v
		}
		return x
	}
	return walk(o), mapping
}
