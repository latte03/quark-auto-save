package server

import (
	"testing"
	"time"
)

func waitRunDone(t *testing.T, run *Run) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if run.View().Done {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("run did not finish")
}

func TestLatestReturnsActiveDryRun(t *testing.T) {
	manager := NewRunManager()
	release := make(chan struct{})
	run, err := manager.Start("试运行", false, func(*Run) error {
		<-release
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if latest := manager.Latest(); latest == nil || latest.ID != run.ID {
		t.Fatalf("active dry run should be watchable, got %+v", latest)
	}
	close(release)
	waitRunDone(t, run)
}

func TestLatestSkipsDryRuns(t *testing.T) {
	manager := NewRunManager()
	dry, err := manager.Start("试运行", false, func(*Run) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	waitRunDone(t, dry)
	if latest := manager.Latest(); latest != nil {
		t.Fatalf("dry run must not appear in history, got %s", latest.ID)
	}

	real, err := manager.Start("正式运行", true, func(*Run) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	waitRunDone(t, real)
	latest := manager.Latest()
	if latest == nil || latest.ID != real.ID {
		t.Fatalf("latest = %+v, want %s", latest, real.ID)
	}
	if latest.View().Dry {
		t.Fatal("real run must not be marked dry")
	}
}
