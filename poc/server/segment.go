package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Quark throttles a single download connection to roughly 100KB/s and the
// account total scales with the number of connections, so large files are
// fetched as parallel byte ranges.
const (
	segmentThreshold   = 64 << 20
	segmentLargeSize   = 1 << 30
	segmentCountSmall  = 8
	segmentCountLarge  = 16
	segmentMinSize     = 4 << 20
	segmentStateSuffix = ".json"
)

// errRangeUnsupported lets the caller fall back to one whole-file stream.
var errRangeUnsupported = errors.New("下载地址不支持分段请求")

type segmentProgress struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
	Done  int64 `json:"done"`
}

func (s segmentProgress) length() int64 { return s.End - s.Start + 1 }

type segmentState struct {
	Size     int64             `json:"size"`
	Segments []segmentProgress `json:"segments"`
}

func segmentPlan(size int64) []segmentProgress {
	if size < segmentThreshold {
		return nil
	}
	count := segmentCountSmall
	if size >= segmentLargeSize {
		count = segmentCountLarge
	}
	for count > 1 && size/int64(count) < segmentMinSize {
		count--
	}
	segments := make([]segmentProgress, count)
	base := size / int64(count)
	var start int64
	for index := range segments {
		end := start + base - 1
		if index == count-1 {
			end = size - 1
		}
		segments[index] = segmentProgress{Start: start, End: end}
		start = end + 1
	}
	return segments
}

// seedSegments marks an existing sequential prefix as downloaded, so a
// partial file from a single-stream attempt is not fetched again.
func seedSegments(segments []segmentProgress, existing int64) {
	for index := range segments {
		segments[index].Done = 0
	}
	if existing <= 0 {
		return
	}
	for index := range segments {
		segment := &segments[index]
		segment.Done = min(existing, segment.length())
		existing -= segment.Done
		if existing <= 0 {
			return
		}
	}
}

func loadSegmentState(path string, size int64) (segmentState, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return segmentState{}, false
	}
	var state segmentState
	if err := json.Unmarshal(raw, &state); err != nil || state.Size != size || len(state.Segments) == 0 {
		return segmentState{}, false
	}
	var expect int64
	for _, segment := range state.Segments {
		if segment.Start != expect || segment.End < segment.Start || segment.Done < 0 || segment.Done > segment.length() {
			return segmentState{}, false
		}
		expect = segment.End + 1
	}
	if expect != size {
		return segmentState{}, false
	}
	return state, true
}

func saveSegmentState(path string, state segmentState) {
	encoded, err := json.Marshal(state)
	if err != nil {
		return
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, encoded, 0o644); err != nil {
		return
	}
	_ = os.Rename(temp, path)
}

func snapshotSegments(mu *sync.Mutex, state *segmentState) segmentState {
	mu.Lock()
	defer mu.Unlock()
	return segmentState{Size: state.Size, Segments: append([]segmentProgress(nil), state.Segments...)}
}

func removePartialFiles(target string) {
	part := target + ".part"
	_ = os.Remove(part)
	_ = os.Remove(part + segmentStateSuffix)
}

// fetchSegmented writes parallel byte ranges into <target>.part and renames it
// once every segment is complete. Progress lives in a sidecar file so a pause,
// restart or retry continues instead of starting over.
func (q *downloadQueue) fetchSegmented(ctx context.Context, url, cookie, ua, target string, size int64, onProgress func(int64)) error {
	part := target + ".part"
	statePath := part + segmentStateSuffix
	state, ok := loadSegmentState(statePath, size)
	if !ok {
		state = segmentState{Size: size, Segments: segmentPlan(size)}
		if len(state.Segments) == 0 {
			return errRangeUnsupported
		}
		// A single-stream .part written before segmentation (or by an older
		// version) holds a valid prefix; keep it instead of starting over.
		if info, statErr := os.Stat(target + ".part"); statErr == nil && info.Size() > 0 && info.Size() < size {
			seedSegments(state.Segments, info.Size())
		}
	}

	file, err := os.OpenFile(part, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}
	defer file.Close()
	if info, statErr := file.Stat(); statErr != nil || info.Size() != size {
		if err := file.Truncate(size); err != nil {
			return fmt.Errorf("预分配失败: %w", err)
		}
	}

	var mu sync.Mutex
	var failure error
	persistDone := make(chan struct{})
	var persist sync.WaitGroup
	persist.Add(1)
	go func() {
		defer persist.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-persistDone:
				return
			case <-ticker.C:
				saveSegmentState(statePath, snapshotSegments(&mu, &state))
			}
		}
	}()

	var wait sync.WaitGroup
	for index := range state.Segments {
		mu.Lock()
		segment := state.Segments[index]
		mu.Unlock()
		if segment.Done >= segment.length() {
			continue
		}
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			if err := q.fetchSegment(ctx, url, cookie, ua, file, &state.Segments[index], &mu, onProgress); err != nil {
				mu.Lock()
				if failure == nil {
					failure = err
				}
				mu.Unlock()
			}
		}(index)
	}
	wait.Wait()
	close(persistDone)
	persist.Wait()

	snapshot := snapshotSegments(&mu, &state)
	saveSegmentState(statePath, snapshot)
	if failure != nil {
		return failure
	}
	for _, segment := range snapshot.Segments {
		if segment.Done != segment.length() {
			return fmt.Errorf("分段未完成：%d/%d", segment.Done, segment.length())
		}
	}
	_ = os.Remove(statePath)
	if err := os.Rename(part, target); err != nil {
		return fmt.Errorf("重命名失败: %w", err)
	}
	return nil
}

func (q *downloadQueue) fetchSegment(ctx context.Context, url, cookie, ua string, file *os.File, segment *segmentProgress, mu *sync.Mutex, onProgress func(int64)) error {
	mu.Lock()
	offset := segment.Start + segment.Done
	end := segment.End
	mu.Unlock()
	if offset > end {
		return nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	if ua != "" {
		request.Header.Set("User-Agent", ua)
	}
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, end))
	response, err := q.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK:
		return errRangeUnsupported
	case http.StatusRequestedRangeNotSatisfiable:
		return fmt.Errorf("分段请求被拒绝")
	default:
		return fmt.Errorf("下载失败: HTTP %d", response.StatusCode)
	}

	buffer := make([]byte, 256*1024)
	for offset <= end {
		// 服务端可能忽略 Range 上界整块返回，超出本段的内容必须丢弃，
		// 否则会覆盖相邻分段并把 Done 计到本段长度之外。
		chunk := buffer
		if remaining := end - offset + 1; remaining < int64(len(chunk)) {
			chunk = chunk[:remaining]
		}
		n, readErr := response.Body.Read(chunk)
		if n > 0 {
			if _, err := file.WriteAt(buffer[:n], offset); err != nil {
				return fmt.Errorf("写入失败: %w", err)
			}
			offset += int64(n)
			mu.Lock()
			segment.Done += int64(n)
			mu.Unlock()
			if onProgress != nil {
				onProgress(int64(n))
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}
	if offset != end+1 {
		return fmt.Errorf("分段提前结束")
	}
	return nil
}
