package main

import (
	"testing"

	"qas-poc/quark"
)

func TestPickFilesPrefersFilesWithoutDuplicates(t *testing.T) {
	list := []quark.ShareFile{
		{FID: "f1", FileName: "a.mkv"},
		{FID: "d1", FileName: "folder", Dir: true},
		{FID: "f2", FileName: "b.mkv"},
	}
	got := pickFiles(list, 2)
	if len(got) != 2 || got[0].FID != "f1" || got[1].FID != "f2" {
		t.Fatalf("limit 2 = %#v, want f1,f2", got)
	}
	got = pickFiles(list, 3)
	if len(got) != 3 || got[2].FID != "d1" {
		t.Fatalf("limit 3 = %#v, want f1,f2,d1", got)
	}
}
