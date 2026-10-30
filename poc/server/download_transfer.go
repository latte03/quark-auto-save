package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

type transferResult int

const (
	transferDone transferResult = iota
	transferSkipped
	transferFailed
	transferInterrupted
)

func (q *downloadQueue) transfer(job *downloadJob, ctx context.Context, item downloadItem) transferResult {
	rel, ok := safeRelPath(item.RelPath)
	if !ok {
		job.addFailed(fmt.Sprintf("非法路径 %q", item.RelPath))
		return transferFailed
	}
	root := filepath.Clean(job.Root)
	target := filepath.Join(root, rel)
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		job.addFailed(fmt.Sprintf("路径越界 %q", item.RelPath))
		return transferFailed
	}

	if info, err := os.Stat(target); err == nil && !info.IsDir() && !job.Overwrite {
		job.addSkipped()
		return transferSkipped
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		job.addFailed(fmt.Sprintf("创建目录失败: %v", err))
		return transferFailed
	}
	if message := ensureFreeSpace(job.Root, job.MinFreeGB); message != "" {
		job.fail(message)
		return transferFailed
	}
	if ctx.Err() != nil {
		return transferInterrupted
	}

	links, cookies, err := job.Client.Download([]string{item.FID})
	if err != nil {
		job.addFailed(fmt.Sprintf("获取下载地址失败: %v", err))
		q.logf("✗ %s：获取下载地址失败: %v", rel, err)
		return transferFailed
	}
	if len(links) == 0 || links[0].DownloadURL == "" {
		job.addFailed("未返回下载地址")
		return transferFailed
	}

	url := links[0].DownloadURL
	ua := job.Client.UserAgent()
	var fileBytes atomic.Int64
	onProgress := func(delta int64) {
		fileBytes.Add(delta)
		job.addProgress(delta)
	}

	// Quark throttles each connection to roughly 100KB/s, so large files are
	// split into parallel ranges; the total scales with the stream count.
	var size int64
	if item.Size >= segmentThreshold {
		err = q.fetchSegmented(ctx, url, cookies, ua, target, item.Size, onProgress)
		if errors.Is(err, errRangeUnsupported) {
			removePartialFiles(target)
			size, err = q.fetchFile(ctx, url, cookies, ua, target, onProgress)
		} else if err == nil {
			size = item.Size
		}
	} else {
		size, err = q.fetchFile(ctx, url, cookies, ua, target, onProgress)
	}
	if err != nil {
		job.dropProgress(fileBytes.Load())
		if ctx.Err() != nil || job.isPaused() || job.isCanceled() || job.aborted.Load() {
			return transferInterrupted
		}
		job.addFailed(fmt.Sprintf("%s: %v", rel, err))
		q.logf("✗ %s：%v", rel, err)
		return transferFailed
	}
	job.addDone(size, fileBytes.Load())
	q.logf("✓ %s（%s）", rel, formatBytes(size))
	return transferDone
}

// fetchFile downloads into <target>.part with Range resume and renames on
// success. Interrupted transfers keep the .part file for the next attempt.
// onProgress receives the bytes written by this attempt (resumed bytes are
// not counted).
func (q *downloadQueue) fetchFile(ctx context.Context, url, cookie, ua, target string, onProgress func(int64)) (int64, error) {
	part := target + ".part"
	var offset int64
	if info, err := os.Stat(part); err == nil && info.Size() > 0 {
		offset = info.Size()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("构造请求失败: %w", err)
	}
	if cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	if ua != "" {
		request.Header.Set("User-Agent", ua)
	}
	if offset > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	response, err := q.http.Do(request)
	if err != nil {
		return 0, fmt.Errorf("下载失败: %w", err)
	}
	defer response.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch response.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK:
		offset = 0
		flags |= os.O_TRUNC
	case http.StatusRequestedRangeNotSatisfiable:
		if info, statErr := os.Stat(part); statErr == nil && info.Size() > 0 {
			if err := os.Rename(part, target); err == nil {
				return info.Size(), nil
			}
		}
		return 0, fmt.Errorf("下载地址拒绝续传")
	default:
		return 0, fmt.Errorf("下载失败: HTTP %d", response.StatusCode)
	}

	file, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return 0, fmt.Errorf("写入失败: %w", err)
	}
	body := io.Reader(response.Body)
	if onProgress != nil {
		body = &progressReader{reader: response.Body, report: onProgress}
	}
	written, copyErr := io.CopyBuffer(file, body, make([]byte, 256*1024))
	closeErr := file.Close()
	if copyErr != nil {
		return offset + written, fmt.Errorf("传输中断，可续传: %w", copyErr)
	}
	if closeErr != nil {
		return offset + written, fmt.Errorf("写入失败: %w", closeErr)
	}
	if err := os.Rename(part, target); err != nil {
		return offset + written, fmt.Errorf("重命名失败: %w", err)
	}
	return offset + written, nil
}

func (q *downloadQueue) logf(format string, args ...any) {
	if q.logger != nil {
		q.logger.Printf(format, args...)
	}
}

// progressReader reports bytes as the copy loop consumes them.
type progressReader struct {
	reader io.Reader
	report func(int64)
}

func (p *progressReader) Read(buffer []byte) (int, error) {
	n, err := p.reader.Read(buffer)
	if n > 0 {
		p.report(int64(n))
	}
	return n, err
}
