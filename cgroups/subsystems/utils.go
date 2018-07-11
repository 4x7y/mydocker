package subsystems

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
)

// Get the absolute path of cgroup subsystem mount point
func FindCgroupMountpoint(subsystem string) string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	defer f.Close()

	// cat /proc/self/mountinfo
	// ...
	// 39 32 0:34 / /sys/fs/cgroup/memory rw,nosuid,nodev,noexec,relatime shared:17 - cgroup cgroup rw,memory
	// 45 32 0:40 / /sys/fs/cgroup/freezer rw,nosuid,nodev,noexec,relatime shared:23 - cgroup cgroup rw,freezer
	// ...

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// scan line by line
		txt := scanner.Text()
		// split a line by " " into fields
		fields := strings.Split(txt, " ")
		// split the last field by ","
		// "rw,memory" -> ["rw", "memory"]
		for _, opt := range strings.Split(fields[len(fields)-1], ",") {
			if opt == subsystem {
				// find "memory" = target subsystem
				// return cgroup mount path (fields[4]): "/sys/fs/cgroup/memory"
				return fields[4]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ""
	}

	return ""
}

func GetCgroupPath(subsystem string, cgroupPath string, autoCreate bool) (string, error) {
	cgroupRoot := FindCgroupMountpoint(subsystem)
	if _, err := os.Stat(path.Join(cgroupRoot, cgroupPath)); err == nil || (autoCreate && os.IsNotExist(err)) {
		if os.IsNotExist(err) {
			if err := os.Mkdir(path.Join(cgroupRoot, cgroupPath), 0755); err == nil {
			} else {
				return "", fmt.Errorf("error create cgroup %v", err)
			}
		}
		return path.Join(cgroupRoot, cgroupPath), nil
	} else {
		return "", fmt.Errorf("cgroup path error %v", err)
	}
}
