package server

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// procMountsPath is the kernel's mount table. A fnOS storage space that is not
// listed there is not mounted, and a path below it then falls back to the
// system disk: os.MkdirAll would happily create /vol2/1000/影视 on the root
// filesystem, the download would fill that up, and once the space comes back
// everything under it would be shadowed and look like it vanished.
const procMountsPath = "/proc/mounts"

// readMountPoints is indirected so tests can describe a mount table instead of
// depending on the machine they run on.
var readMountPoints = func() ([]string, error) {
	data, err := os.ReadFile(procMountsPath)
	if err != nil {
		return nil, err
	}
	return parseMountPoints(data), nil
}

// parseMountPoints returns the mount points listed in /proc/mounts content.
// The kernel escapes whitespace inside paths, so \040 is a space.
func parseMountPoints(data []byte) []string {
	var points []string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		points = append(points, unescapeMountPoint(fields[1]))
	}
	return points
}

func unescapeMountPoint(raw string) string {
	if !strings.Contains(raw, `\`) {
		return raw
	}
	return strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	).Replace(raw)
}

// slashPath normalises a configured download path for the prefix comparisons
// below. fnOS paths always use forward slashes; the separator rewrite only
// matters for tests that run on Windows.
func slashPath(value string) string {
	cleaned := path.Clean(strings.ReplaceAll(strings.TrimSpace(value), `\`, "/"))
	if cleaned == "." {
		return ""
	}
	return cleaned
}

// storageSpaceOf returns the leading /volN segment of an fnOS storage path, or
// "" when the path does not live under a storage space.
func storageSpaceOf(value string) string {
	cleaned := slashPath(value)
	trimmed := strings.TrimPrefix(cleaned, "/")
	if trimmed == cleaned || trimmed == "" {
		return ""
	}
	segment, _, _ := strings.Cut(trimmed, "/")
	digits, ok := strings.CutPrefix(segment, "vol")
	if !ok || digits == "" || strings.Trim(digits, "0123456789") != "" {
		return ""
	}
	return "/" + segment
}

// mountFor returns the longest mount point that covers target. The root
// filesystem is ignored on purpose: a path covered only by "/" is not inside a
// mounted storage space, it just happens to exist on the system disk.
func mountFor(target string, mounts []string) (string, bool) {
	target = slashPath(target)
	if target == "" {
		return "", false
	}
	best := ""
	for _, mount := range mounts {
		mount = slashPath(mount)
		if mount == "" || mount == "/" {
			continue
		}
		if target != mount && !strings.HasPrefix(target, mount+"/") {
			continue
		}
		if len(mount) > len(best) {
			best = mount
		}
	}
	return best, best != ""
}

// checkStorageMounted reports why a download root under an fnOS storage space
// cannot be used right now. It returns "" when the root is fine, and also when
// the path is not under /volN at all (the root validator already rejects
// those) or when the mount table cannot be read.
func checkStorageMounted(root string) string {
	space := storageSpaceOf(root)
	if space == "" {
		return ""
	}
	mounts, err := readMountPoints()
	if err != nil {
		// No usable mount table (not Linux, or /proc not mounted): leave the
		// decision to the writability probe instead of blocking every download.
		return ""
	}
	if _, ok := mountFor(root, mounts); ok {
		return ""
	}
	return fmt.Sprintf("存储空间 %s 未挂载：%s 现在会建到系统盘上，请先在飞牛里挂载该存储空间再运行",
		space, slashPath(root))
}
