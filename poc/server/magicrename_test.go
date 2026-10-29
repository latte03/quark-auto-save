package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyMagicMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	config := `{
		"magic_regex": {"$OLD": {"pattern": "old", "replace": "{TASKNAME}"}},
		"tasklist": [
			{"taskname":"uses","pattern":"$OLD"},
			{"taskname":"custom","pattern":"raw.*"}
		]
	}`
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	rules := store.Snapshot().Magic
	if len(rules) != 1 || rules[0].ID == "" || rules[0].Name != "$OLD" || rules[0].Pattern != "old" {
		t.Fatalf("rules = %+v", rules)
	}
	tasks := store.Tasks()
	if tasks[0].MagicID != rules[0].ID || tasks[0].Pattern != "" {
		t.Fatalf("migrated task = %+v", tasks[0])
	}
	if tasks[1].MagicID != "" || tasks[1].Pattern != "raw.*" {
		t.Fatalf("raw pattern task changed: %+v", tasks[1])
	}

	// The next save persists the list form with ids.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Magic []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Pattern string `json:"pattern"`
		} `json:"magic_regex"`
		Tasks []struct {
			MagicID string `json:"magic_id"`
			Pattern string `json:"pattern"`
		} `json:"tasklist"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("persisted config is not the list form: %v", err)
	}
	if len(doc.Magic) != 1 || doc.Magic[0].ID == "" {
		t.Fatalf("persisted rules = %+v", doc.Magic)
	}
	if doc.Tasks[0].MagicID != doc.Magic[0].ID {
		t.Fatalf("persisted task = %+v", doc.Tasks[0])
	}
}

func TestRenameMagicKeepsTaskReferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	config := `{
		"magic_regex": {"$OLD": {"pattern": "old"}},
		"tasklist": [{"taskname":"uses","pattern":"$OLD"}]
	}`
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	rule := store.Snapshot().Magic[0]
	if err := store.RenameMagic(rule.ID, "$NEW"); err != nil {
		t.Fatal(err)
	}
	if name := store.Snapshot().Magic[0].Name; name != "$NEW" {
		t.Fatalf("name = %q", name)
	}
	if id := store.Tasks()[0].MagicID; id != rule.ID {
		t.Fatalf("task reference changed: %q", id)
	}

	if err := store.RenameMagic("missing", "$X"); err == nil {
		t.Fatal("unknown id must be rejected")
	}
	if err := store.RenameMagic(rule.ID, ""); err == nil {
		t.Fatal("empty name must be rejected by the handler, not the store")
	}
}

func TestDeleteReferencedMagicIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	config := `{
		"magic_regex": {"$OLD": {"pattern": "old"}},
		"tasklist": [{"taskname":"uses","pattern":"$OLD"}]
	}`
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Update(map[string]json.RawMessage{"magic_regex": json.RawMessage(`[]`)})
	if err == nil {
		t.Fatal("deleting a rule still used by a task must be rejected")
	}
	if len(store.Snapshot().Magic) != 1 {
		t.Fatal("rules changed despite the rejection")
	}
}

func TestRenameMagicAPI(t *testing.T) {
	instance, ts := newTestServerWithConfig(t, `{
		"cookie": ["kps=placeholder; sign=placeholder; vcode=placeholder"],
		"webui": {"username": "admin", "password": "test-pass"},
		"magic_regex": {"$OLD": {"pattern": "old"}},
		"tasklist": [{"taskname":"t","shareurl":"https://pan.quark.cn/s/abc","savepath":"/tv","pattern":"$OLD"}]
	}`)
	client := loginClient(t, ts.URL)
	id := instance.store.Snapshot().Magic[0].ID

	call := func(payload map[string]string) int {
		body, _ := json.Marshal(payload)
		request, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/magic/rename", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}

	if code := call(map[string]string{"id": id, "name": "$BETTER"}); code != http.StatusOK {
		t.Fatalf("rename = %d", code)
	}
	if name := instance.store.Snapshot().Magic[0].Name; name != "$BETTER" {
		t.Fatalf("name = %q", name)
	}
	if pattern := instance.store.Tasks()[0].Pattern; pattern != "" {
		t.Fatalf("task still keeps a name pattern: %q", pattern)
	}
	if code := call(map[string]string{"id": id, "name": ""}); code != http.StatusBadRequest {
		t.Fatalf("empty name = %d", code)
	}
}
