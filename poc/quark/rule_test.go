package quark

import "testing"

func TestParseRulesMigratesLegacyMap(t *testing.T) {
	rules, err := ParseRules([]byte(`{"$A":{"pattern":"a"},"$B":{"pattern":"b","replace":"x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("rules = %+v", rules)
	}
	if rules[0].Name != "$A" || rules[1].Name != "$B" || rules[1].Replace != "x" {
		t.Fatalf("rules = %+v", rules)
	}
	if rules[0].ID == "" || rules[0].ID != DeriveRuleID("$A") {
		t.Fatalf("id = %q", rules[0].ID)
	}
	again, err := ParseRules([]byte(`{"$A":{"pattern":"a"},"$B":{"pattern":"b","replace":"x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if again[0].ID != rules[0].ID {
		t.Fatalf("ids are not stable: %q vs %q", rules[0].ID, again[0].ID)
	}
}

func TestParseRulesNormalizesList(t *testing.T) {
	rules, err := ParseRules([]byte(`[{"id":"m1","pattern":"p"},{"id":"m1","name":"dup","pattern":"q"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("rules = %+v", rules)
	}
	if rules[0].Name != "m1" {
		t.Fatalf("missing name not filled: %+v", rules[0])
	}
	if rules[0].ID == rules[1].ID {
		t.Fatalf("duplicate ids: %+v", rules)
	}
}

func TestResolveTaskPrefersIDAndFailsClosed(t *testing.T) {
	magic := NewMagic([]Rule{{ID: "m1", Name: "$A", Pattern: "A.*", Replace: "R"}})

	pattern, replace := magic.ResolveTask("m1", "$STALE", "")
	if pattern != "A.*" || replace != "R" {
		t.Fatalf("id resolve = %q/%q", pattern, replace)
	}
	if pattern, _ := magic.ResolveTask("gone", "$A", ""); pattern != `(?!x)x` {
		t.Fatalf("unknown id must fail closed, got %q", pattern)
	}
	if pattern, replace := magic.ResolveTask("", "$A", "custom"); pattern != "A.*" || replace != "custom" {
		t.Fatalf("name resolve = %q/%q", pattern, replace)
	}
	if pattern, _ := magic.ResolveTask("", "raw.*", ""); pattern != "raw.*" {
		t.Fatalf("raw resolve = %q", pattern)
	}
}
