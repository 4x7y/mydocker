package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Init is executed inside a container process
// It will be the first process to be called in a new container
// `syscall.Mount` is used to mount the proc filesystem, so that
// the system command like `ps` can read the process status.

func RunContainerInitProcess() error {
	cmdArray := readUserCommand()
	if cmdArray == nil || len(cmdArray) == 0 {
		return fmt.Errorf("Run container get user command error, cmdArray is nil")
	}
	log.Infof("Get cmdArray: %v", cmdArray)

	log.Infof("Setup mount point")
	setUpMount()

	// Since syscall.execve require absolute path of command, here we
	// find command absolute path in system PATH env using exec.LookPath
	// Example: fish -> /usr/bin/fish
	// Example: ls   -> /bin/ls
	path, err := exec.LookPath(cmdArray[0])
	if err != nil {
		log.Errorf("Exec loop path error %v", err)
		return err
	}
	log.Infof("Find \"%s\" absoulte path \"%s\"", cmdArray[0], path)

	// `os.syscall.Exec` invokes Linux execve(2) system call
	//
	// execve(2) executes the program pointed to by filename.  This causes
	// the program that is currently being run by the calling process to be
	// replaced with a new program, with newly initialized stack, heap, and
	// (initialized and uninitialized) data segments.
	if err := syscall.Exec(path, cmdArray[0:], os.Environ()); err != nil {
		log.Errorf(err.Error())
	}
	return nil
}

func readUserCommand() []string {

	// There are three standard file descriptions, STDIN, STDOUT, and STDERR.
	// They are assigned to 0, 1, and 2 respectively.
	// File descriptor 3 means a file handle, typically the first available.
	// Here it is the readPipe assigned in cmd.ExtraFiles when cmd is created.
	//
	// $ll /proc/self/fd
	// total 0
	// lrwx------ 1 root root 64 JUL 11 15:11 0 -> /dev/pts/2
	// lrwx------ 1 root root 64 JUL 11 15:11 1 -> /dev/pts/2
	// lrwx------ 1 root root 64 JUL 11 15:11 2 -> /dev/pts/2
	// lr-x------ 1 root root 64 JUL 11 15:11 3 -> pipe:[137828]   <--- pipe
	// lr-x------ 1 root root 64 JUL 11 15:11 4 -> /proc/7426/fd/
	pipe := os.NewFile(uintptr(3), "pipe")
	msg, err := ioutil.ReadAll(pipe)
	if err != nil {
		log.Errorf("init read pipe error %v", err)
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}

// Initialize mount point
func setUpMount() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Errorf("Get current location error %v", err)
		return
	}
	log.Infof("Current location is %s", pwd)

	// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	// pivotRoot(pwd)

	// Remount "/proc" to get accurate "top" && "ps" output
	// Meaning of mount flags:
	// MS_NOEXEC: do not run other program under this filesystem
	// MS_NOSUID: when process is running, do not allow set-user-ID or set-group-ID
	// MS_NODEV:  default parameter since Linux 2.4
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
}

// pivot_root() moves the root file system of the calling process to the directory
// put_old and makes new_root the new root file system of the calling process.
//
// The typical use of pivot_root() is during system startup, when the system mounts
// a temporary root file system (e.g., an initrd), then mounts the real root file
// system, and eventually turns the latter into the current root of all relevant
// processes or threads.

// Try the following commands in a ubuntu-16.04-amd64 machine (not sure if other
// system also work correctly).
// 1. Make a mount point
//    $ mkdir /ramroot
// 2. Mount a memory filesystem (tmpfs) to /ramroot
//    $ mount -n -t tmpfs -o size=256M none /ramroot
// 3. Busybox is lite linux kernel. Copy all files under ~/busybox to /ramroot
//    $ find ~/busybox -depth -xdev -print | cpio -pd --quiet /ramroot
// 4. Create mount point in new filesystem for current root
//    $ cd /ramroot
//    $ mkdir oldrooot
// 5. Pivot root (be sure you are the root user, otherwise busybox may not know
//    who you are, `sudo su`)
//    $ mount --make-rprivate / 			# necessay for pivot_root to work
//    $ pivot_root . /oldroot
// 6. Now you are in new root (busybox)
// 7. Move system and temporary filesystem to the new root
//    $ mount --move /oldroot/dev /dev
//    $ mount --move /oldroot/run /run
//    $ mount --move /oldroot/sys /sys
//    $ mount --move /oldroot/proc /proc
// 8. Try some commands to explore the new filesystem
//    $ ls
// 9. Pivot back to the old filesystem
//    $ sh
//    $ pivot_root /oldroot /oldroot/ramroot
// 10. Mount back system and temp filesystem again
//    $ mount --move /oldroot/dev /dev
//    $ mount --move /oldroot/run /run
//    $ mount --move /oldroot/sys /sys
//    $ mount --move /oldroot/proc /proc

func pivotRoot(root string) error {
	/**
	  为了使当前root的老 root 和新 root 不在同一个文件系统下，我们把root重新mount了一次
	  bind mount是把相同的内容换了一个挂载点的挂载方法
	*/
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("Mount rootfs to itself error: %v", err)
	}
	// 创建 rootfs/.pivot_root 存储 old_root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		return err
	}

	// pivot_root 到新的rootfs, 现在老的 old_root 是挂载在rootfs/.pivot_root
	// 挂载点现在依然可以在mount命令中看到
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	}
	// 修改当前的工作目录到根目录
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %v", err)
	}

	pivotDir = filepath.Join("/", ".pivot_root")
	// umount rootfs/.pivot_root
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %v", err)
	}
	// 删除临时文件夹
	return os.Remove(pivotDir)
}
