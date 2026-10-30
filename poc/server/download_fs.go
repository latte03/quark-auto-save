package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ensureFreeSpace(root string, minFreeGB int64) string {
	if minFreeGB <= 0 {
		return ""
	}
	required := uint64(minFreeGB) * 1024 * 1024 * 1024
	if free := freeBytes(root); free < required {
		return fmt.Sprintf("可用空间不足：剩余 %s，低于保留阈值 %d GB", formatBytes(int64(free)), minFreeGB)
	}
	return ""
}

func safeRelPath(rel string) (string, bool) {
	parts := strings.Split(strings.ReplaceAll(rel, "\\", "/"), "/")
	var clean []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		if part == ".." || strings.ContainsRune(part, 0) {
			return "", false
		}
		clean = append(clean, part)
	}
	if len(clean) == 0 {
		return "", false
	}
	return filepath.Join(clean...), true
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(bytes)
	index := 0
	for value >= 1024 && index < len(units)-1 {
		value /= 1024
		index++
	}
	if index == 0 {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	return fmt.Sprintf("%.1f %s", value, units[index])
}

// localFileExists mirrors the transfer step: an existing file is skipped
// unless overwrite is on.
func localFileExists(root, rel string) bool {
	clean, ok := safeRelPath(rel)
	if !ok {
		return false
	}
	info, err := os.Stat(filepath.Join(filepath.Clean(root), clean))
	return err == nil && !info.IsDir()
}
