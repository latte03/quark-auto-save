package quark

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type magicCase struct {
	Rule     string `json:"rule"`
	Pattern  string `json:"pattern"`
	Replace  string `json:"replace"`
	Filename string `json:"filename"`
	Matched  bool   `json:"matched"`
	Result   string `json:"result"`
}

func loadMagicCases(t *testing.T) []magicCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/magic_cases.json")
	if err != nil {
		t.Fatalf("read golden cases: %v", err)
	}
	var payload struct {
		Cases []magicCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("parse golden cases: %v", err)
	}
	if len(payload.Cases) == 0 {
		t.Fatal("golden cases are empty")
	}
	return payload.Cases
}

func TestMagicParityWithPython(t *testing.T) {
	cases := loadMagicCases(t)
	rules := map[string]Rule{}
	for _, c := range cases {
		rules[c.Rule] = Rule{Pattern: c.Pattern, Replace: c.Replace}
	}
	magic := NewMagic(rules)

	for _, c := range cases {
		name := fmt.Sprintf("%s/%q", c.Rule, c.Filename)
		t.Run(name, func(t *testing.T) {
			pattern, replace := magic.Resolve(c.Rule, "")
			if pattern != c.Pattern {
				t.Fatalf("resolved pattern = %q, want %q", pattern, c.Pattern)
			}
			if replace != c.Replace {
				t.Fatalf("resolved replace = %q, want %q", replace, c.Replace)
			}
			matched, err := magic.Match(pattern, c.Filename)
			if err != nil {
				t.Fatalf("match: %v", err)
			}
			if matched != c.Matched {
				t.Errorf("match = %v, want %v", matched, c.Matched)
			}
			result, err := magic.Sub(pattern, replace, c.Filename)
			if err != nil {
				t.Fatalf("sub: %v", err)
			}
			if result != c.Result {
				t.Errorf("result = %q, want %q", result, c.Result)
			}
		})
	}
}

func TestPyPatternConversion(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`[\_\.\-]`, "[_\\.\\-]"},
		{`(?P<head>[Ss]\d{1,2})`, `(?<head>[Ss]\d{1,2})`},
		{`\d+\.\d+`, `\d+\.\d+`},
		{`a\ b`, `a\ b`},
	}
	for _, c := range cases {
		if got := pyPatternToDotNet(c.in); got != c.want {
			t.Errorf("pyPatternToDotNet(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPyReplacementConversion(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`\1E\2.\3`, `$1E$2.$3`},
		{`{TASKNAME}.S{SXX}E{E}`, `{TASKNAME}.S{SXX}E{E}`},
		{`$1`, `$$1`},
		{`\\`, `\`},
		{`a\nb`, "a\nb"},
	}
	for _, c := range cases {
		if got := pyReplacementToDotNet(c.in); got != c.want {
			t.Errorf("pyReplacementToDotNet(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
