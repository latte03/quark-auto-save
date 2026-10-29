package quark

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"
)

type varsFixture struct {
	TaskName string `json:"taskname"`
	Year     int    `json:"year"`
	Sub      []struct {
		Pattern  string `json:"pattern"`
		Replace  string `json:"replace"`
		Filename string `json:"filename"`
		Result   string `json:"result"`
	} `json:"sub"`
	Dir []struct {
		Replace  string      `json:"replace"`
		Dir      []ShareFile `json:"dir_files"`
		Incoming []struct {
			FileName   string `json:"file_name"`
			FileNameIn string `json:"file_name_re_in"`
			FileNameRe string `json:"file_name_re"`
			UpdatedAt  int64  `json:"updated_at"`
			Dir        bool   `json:"dir"`
		} `json:"incoming"`
		DirAfter map[string]string `json:"dir_after"`
	} `json:"dir"`
	Sort []struct {
		Names []string `json:"names"`
		Want  []string `json:"want"`
	} `json:"sort"`
	Exists []struct {
		FileName  string   `json:"filename"`
		FileNames []string `json:"filenames"`
		IgnoreExt bool     `json:"ignore_ext"`
		Result    string   `json:"result"`
	} `json:"exists"`
}

func loadVarsFixture(t *testing.T) varsFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/magic_vars.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var fixture varsFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}
	return fixture
}

func newTestMagic(t *testing.T, fixture varsFixture) *Magic {
	t.Helper()
	magic := NewMagic(nil)
	magic.SetTaskName(fixture.TaskName)
	magic.SetClock(func() time.Time {
		return time.Date(fixture.Year, 5, 1, 12, 0, 0, 0, time.UTC)
	})
	return magic
}

func TestMagicVariablesParity(t *testing.T) {
	fixture := loadVarsFixture(t)
	magic := newTestMagic(t, fixture)
	for _, c := range fixture.Sub {
		name := fmt.Sprintf("%s/%q", c.Replace, c.Filename)
		t.Run(name, func(t *testing.T) {
			got, err := magic.Sub(c.Pattern, c.Replace, c.Filename)
			if err != nil {
				t.Fatalf("sub: %v", err)
			}
			if got != c.Result {
				t.Errorf("sub = %q, want %q", got, c.Result)
			}
		})
	}
}

func TestDirNumberingParity(t *testing.T) {
	fixture := loadVarsFixture(t)
	for i, c := range fixture.Dir {
		t.Run(fmt.Sprintf("%02d/%s", i, c.Replace), func(t *testing.T) {
			magic := newTestMagic(t, fixture)
			if err := magic.SetDirFileList(c.Dir, c.Replace); err != nil {
				t.Fatalf("set dir: %v", err)
			}
			files := make([]*ShareFile, 0, len(c.Incoming))
			for _, item := range c.Incoming {
				files = append(files, &ShareFile{
					FileName:   item.FileName,
					FileNameRe: item.FileNameIn,
					UpdatedAt:  item.UpdatedAt,
					Dir:        item.Dir,
				})
			}
			magic.SortFileList(files)
			for idx, item := range c.Incoming {
				if got := files[idx].FileNameRe; got != item.FileNameRe {
					t.Errorf("%s renamed to %q, want %q", item.FileName, got, item.FileNameRe)
				}
			}
			gotDir := map[string]string{}
			for k, v := range magic.DirFilenameDict() {
				gotDir[strconv.Itoa(k)] = v
			}
			if len(gotDir) != len(c.DirAfter) {
				t.Fatalf("dir dict = %v, want %v", gotDir, c.DirAfter)
			}
			for k, want := range c.DirAfter {
				if gotDir[k] != want {
					t.Errorf("dir[%s] = %q, want %q", k, gotDir[k], want)
				}
			}
		})
	}
}

func TestNaturalSortParity(t *testing.T) {
	fixture := loadVarsFixture(t)
	for i, c := range fixture.Sort {
		got := append([]string(nil), c.Names...)
		sort.SliceStable(got, func(a, b int) bool {
			return naturalLess(sortKey(got[a]), sortKey(got[b]))
		})
		if len(got) != len(c.Want) {
			t.Fatalf("case %d: length mismatch", i)
		}
		for pos := range got {
			if got[pos] != c.Want[pos] {
				t.Errorf("case %d pos %d = %q, want %q", i, pos, got[pos], c.Want[pos])
			}
		}
	}
}

func TestExistsParity(t *testing.T) {
	fixture := loadVarsFixture(t)
	magic := newTestMagic(t, fixture)
	for _, c := range fixture.Exists {
		name := fmt.Sprintf("%s/%v/%v", c.FileName, c.FileNames, c.IgnoreExt)
		t.Run(name, func(t *testing.T) {
			if got := magic.Exists(c.FileName, c.FileNames, c.IgnoreExt); got != c.Result {
				t.Errorf("exists = %q, want %q", got, c.Result)
			}
		})
	}
}
