package server

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"qas-poc/quark"
)

// taskTimes returns the normalized daily times for a task. Tasks saved before
// per-task scheduling existed fall back to the times encoded in crontab.
func taskTimes(task *quark.Task, crontab string) []string {
	if len(task.RunTime) > 0 {
		return normalizeTimes(task.RunTime)
	}
	if times, ok := crontabTimes(crontab); ok {
		return times
	}
	return nil
}

func normalizeTimes(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if clock, ok := parseClock(value); ok && !seen[clock] {
			seen[clock] = true
			out = append(out, clock)
		}
	}
	sort.Strings(out)
	return out
}

// parseClock accepts H:MM or HH:MM and returns a zero-padded HH:MM.
func parseClock(value string) (string, bool) {
	hour, minute, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok {
		return "", false
	}
	h, err := strconv.Atoi(hour)
	if err != nil || h < 0 || h > 23 {
		return "", false
	}
	m, err := strconv.Atoi(minute)
	if err != nil || m < 0 || m > 59 {
		return "", false
	}
	return fmt.Sprintf("%02d:%02d", h, m), true
}

// crontabTimes extracts daily clock times from a crontab such as
// "0 8,18,20 * * *". Only explicit minute/hour lists are supported; anything
// with steps, ranges or wildcard hours returns ok=false so the caller can
// keep the legacy cron entry.
func crontabTimes(spec string) ([]string, bool) {
	fields := strings.Fields(spec)
	if len(fields) != 5 {
		return nil, false
	}
	for _, field := range fields[2:] {
		if field != "*" {
			return nil, false
		}
	}
	minutes, ok := intList(fields[0], 59)
	if !ok {
		return nil, false
	}
	hours, ok := intList(fields[1], 23)
	if !ok {
		return nil, false
	}
	if len(minutes)*len(hours) > 120 {
		return nil, false
	}
	seen := map[string]bool{}
	times := make([]string, 0, len(minutes)*len(hours))
	for _, hour := range hours {
		for _, minute := range minutes {
			clock := fmt.Sprintf("%02d:%02d", hour, minute)
			if !seen[clock] {
				seen[clock] = true
				times = append(times, clock)
			}
		}
	}
	sort.Strings(times)
	return times, true
}

func intList(field string, max int) ([]int, bool) {
	if field == "*" {
		return nil, false
	}
	var out []int
	for _, part := range strings.Split(field, ",") {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 0 || value > max {
			return nil, false
		}
		out = append(out, value)
	}
	return out, true
}

// dueTaskIndices selects tasks whose clock time matches at and whose weekday
// and end date allow a run.
func dueTaskIndices(tasks []*quark.Task, crontab string, at time.Time) []int {
	clock := at.Format("15:04")
	var indices []int
	for index, task := range tasks {
		if task.IsOnce() {
			continue
		}
		times := taskTimes(task, crontab)
		if len(times) == 0 || !containsTime(times, clock) {
			continue
		}
		if scheduled, err := task.IsScheduled(at); err != nil || !scheduled {
			continue
		}
		indices = append(indices, index)
	}
	return indices
}

func containsTime(times []string, clock string) bool {
	for _, value := range times {
		if value == clock {
			return true
		}
	}
	return false
}

// legacyTaskIndices selects tasks without any usable time so a complex legacy
// crontab can still drive them.
func legacyTaskIndices(tasks []*quark.Task, crontab string, at time.Time) []int {
	var indices []int
	for index, task := range tasks {
		if task.IsOnce() || len(taskTimes(task, crontab)) > 0 {
			continue
		}
		if scheduled, err := task.IsScheduled(at); err != nil || !scheduled {
			continue
		}
		indices = append(indices, index)
	}
	return indices
}

// nextTimesForTask lists the upcoming fire times for one task.
func nextTimesForTask(task *quark.Task, crontab string, now time.Time, limit int) []time.Time {
	if task.IsOnce() {
		return nil
	}
	times := taskTimes(task, crontab)
	if len(times) == 0 || limit <= 0 {
		return nil
	}
	out := make([]time.Time, 0, limit)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for offset := 0; offset < 370 && len(out) < limit; offset++ {
		date := day.AddDate(0, 0, offset)
		for _, clock := range times {
			parts := strings.SplitN(clock, ":", 2)
			h, _ := strconv.Atoi(parts[0])
			m, _ := strconv.Atoi(parts[1])
			candidate := time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, now.Location())
			if !candidate.After(now) {
				continue
			}
			if scheduled, err := task.IsScheduled(candidate); err != nil || !scheduled {
				continue
			}
			out = append(out, candidate)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func configNextRuns(cfg *quark.Config, now time.Time) []string {
	crontab := crontabValue(cfg)
	out := nextRunsForTasks(cfg.Tasks, crontab, now, 3)
	if len(out) > 0 {
		return out
	}
	// No task times yet: show the legacy crontab schedule so the settings
	// panel still answers "下一次什么时候跑".
	return crontabNextRuns(crontab, now, 3)
}

func crontabNextRuns(spec string, now time.Time, count int) []string {
	schedule, err := cron.ParseStandard(strings.TrimSpace(spec))
	if err != nil || count <= 0 {
		return []string{}
	}
	out := make([]string, 0, count)
	next := now
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)
	for len(out) < count {
		next = schedule.Next(next)
		if next.IsZero() {
			break
		}
		label := next.Format("15:04")
		switch {
		case next.Year() == today.Year() && next.YearDay() == today.YearDay():
			out = append(out, "今天 "+label)
		case next.Year() == tomorrow.Year() && next.YearDay() == tomorrow.YearDay():
			out = append(out, "明天 "+label)
		default:
			out = append(out, weekdayLabels[int(next.Weekday())]+" "+label)
		}
	}
	return out
}

// nextRunsForTasks merges upcoming times across tasks for the settings panel.
func nextRunsForTasks(tasks []*quark.Task, crontab string, now time.Time, count int) []string {
	var all []time.Time
	for _, task := range tasks {
		all = append(all, nextTimesForTask(task, crontab, now, count)...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Before(all[j]) })
	if len(all) > count {
		all = all[:count]
	}
	out := make([]string, 0, len(all))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, at := range all {
		label := at.Format("15:04")
		switch {
		case at.Year() == today.Year() && at.YearDay() == today.YearDay():
			label = "今天 " + label
		case at.Year() == today.AddDate(0, 0, 1).Year() && at.YearDay() == today.AddDate(0, 0, 1).YearDay():
			label = "明天 " + label
		default:
			label = weekdayLabels[int(at.Weekday())] + " " + label
		}
		out = append(out, label)
	}
	return out
}
