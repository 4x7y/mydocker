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
	log.Infof("Setup filesystem mount point")
	setUpMount()

	cmdArray := readUserCommand()
	if cmdArray == nil || len(cmdArray) == 0 {
		return fmt.Errorf("Run container get user command error, cmdArray is nil")
	}
	log.Infof("Get: pipe -> %v", cmdArray)
	// Since syscall.execve require absolute path of command, here we
	// find command absolute path in system PATH env using exec.LookPath
	// Example: fish -> /usr/bin/fish
	// Example: ls   -> /bin/ls
	log.Infof("Looking for %s absoulte path under container env $PATH", cmdArray[0])
	path, err := exec.LookPath(cmdArray[0])
	if err != nil {
		log.Errorf("Exec loop path error %v", err)
		return err
	}
	log.Infof("\"%s\" -> \"%s\"", cmdArray[0], path)

	// `os.syscall.Exec` invokes Linux execve(2) system call
	//
	// execve(2) executes the program pointed to by filename.  This causes
	// the program that is currently being run by the calling process to be
	// replaced with a new program, with newly initialized stack, heap, and
	// (initialized and uninitialized) data segments.
	log.Infof("$ exec %s -> %s (%s %s ...)", os.Args[0], path, os.Environ()[0], os.Environ()[2])
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
		log.Errorf("Init read pipe error %v", err)
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
	log.Infof("$ pwd = %s", pwd)

	pivotRoot(pwd)

	// Remount "/proc" to get accurate "top" && "ps" output
	// Meaning of mount flags:
	// MS_NOEXEC: do not run other program under this filesystem
	// MS_NOSUID: when process is running, do not allow set-user-ID or set-group-ID
	// MS_NODEV:  default parameter since Linux 2.4
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	if err := syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), ""); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ mount proc proc /proc")
	}

	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755"); err != nil {
		log.Errorf("Mount tmpfs error: %v", err)
		return
	} else {
		log.Infof("$ mount tmpfs tmpfs /dev -o nosuid,strictatime mode=755")
	}
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
//    $ cd ~/busybox
//    $ find . -depth -xdev -print | cpio -pd --quiet /ramroot
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
//    $ mount --move /oldroot/dev  /dev
//    $ mount --move /oldroot/run  /run
//    $ mount --move /oldroot/sys  /sys
//    $ mount --move /oldroot/proc /proc

func pivotRoot(root string) error {

	// Under Linux, bind mounts are available as a kernel feature. You can create one
	// with the mount command, by passing either the `--bind` command line option or
	// the bind mount option. The following two commands are equivalent:
	//
	// mount  --bind /a/dir /target/dir
	// mount -o bind /a/dir /target/dir
	//
	// Here, the “device” /some/where is not a disk partition like in the case of an
	// on-disk filesystem, but an existing directory. The mount point /else/where must
	// be an existing directory as usual. Note that no filesystem type is specified
	// either way: making a bind mount doesn't involve a filesystem driver, it copies
	// the kernel data structures from the original mount.
	//
	// A Linux bind mount is mostly indistinguishable from the original. The command
	//
	// df -T /else/where shows the same device and the same filesystem type as
	// df -T /some/where. The files /some/where/foo and /else/where/foo are
	//
	// indistinguishable, as if they were hard links. It is possible to unmount
	// /some/where, in which case /else/where remains mounted.
	//
	// With older kernels (I don't know exactly when, I think until some 3.x), bind mounts
	// were truly indistinguishable from the original. Recent kernels do track bind mounts
	// and expose the information through PID/mountinfo, which allows findmnt to indicate
	// bind mount as such.
	//
	// If there are mount points under /some/where, their contents are not visible under
	// /else/where. Instead of bind, you can use rbind, also replicate mount points underneath
	// /some/where. For example, if /some/where/mnt is a mount point then
	//
	// mount --rbind /some/where /else/where
	//
	// is equivalent to
	//
	// mount --bind /some/where /else/where
	// mount --bind /some/where/mnt /else/where/mnt
	//
	// In addition, Linux allows mounts to be declared as shared, slave, private or unbindable.
	// This affects whether that mount operation is reflected under a bind mount that replicates
	// the mount point. For more details, see the kernel documentation.
	//
	// Linux also provides a way to move mounts: where --bind copies, --move moves a mount point.
	//
	// It is possible to have different mount options in two bind-mounted directories. There is a
	// quirk, however: making the bind mount and setting the mount options cannot be done atomically,
	// they have to be two successive operations. (Older kernels did not allow this.) For example,
	// the following commands create a read-only view, but there is a small window of time during
	// which /else/where is read-write:
	//
	// mount --bind /some/where /else/where
	// mount -o remount,ro,bind /else/where

	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		log.Errorf("Command Failed: mount --make-rprivate /")
		return err
	} else {
		log.Infof("$ mount --make-rprivate /")
	}

	/**
	  为了使当前root的老 root 和新 root 不在同一个文件系统下，我们把root重新 mount 了一次
	  bind mount是把相同的内容换了一个挂载点的挂载方法
	*/
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("Mount rootfs to itself error: %v", err)
	} else {
		log.Infof("$ mount --bind %s /", root)
	}

	// 创建 rootfs/.pivot_root 存储 old_root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		if os.IsExist(err) {
			log.Warnf(".pivot_root dir exist")
		} else {
			log.Error("Mkdir %s failed, with error %v", pivotDir, err)
			return err
		}
	} else {
		log.Infof("$ mkdir %s", pivotDir)
	}

	// pivot_root 到新的rootfs, 现在老的 old_root 是挂载在rootfs/.pivot_root
	// 挂载点现在依然可以在mount命令中看到
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	} else {
		log.Infof("$ pivot_root %s %s", root, pivotDir)
	}

	// Change current work dir to root
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("\"cd / \": %v", err)
	} else {
		log.Infof("$ cd /")
	}

	pivotDir = filepath.Join("/", ".pivot_root")
	// umount rootfs/.pivot_root
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %v", err)
	} else {
		log.Infof("$ umount %s", pivotDir)
	}

	// Delete temporary directory
	if err := os.Remove(pivotDir); err != nil {
		return fmt.Errorf("remove pivotDir %v", err)
	} else {
		log.Infof("$ rmdir %s", pivotDir)
	}

	return nil
}

// execve("./mydocker", ["./mydocker", "run", "-ti", "fish"], [/* 30 vars */]) = 0
// clone(child_stack=0xc420046000, flags=CLONE_VM|CLONE_FS|CLONE_FILES|CLONE_SIGHAND|CLONE_THREAD|CLONE_SYSVSEM) = 113
// clone(child_stack=0xc420048000, flags=CLONE_VM|CLONE_FS|CLONE_FILES|CLONE_SIGHAND|CLONE_THREAD|CLONE_SYSVSEM) = 114
// clone(child_stack=0xc420044000, flags=CLONE_VM|CLONE_FS|CLONE_FILES|CLONE_SIGHAND|CLONE_THREAD|CLONE_SYSVSEM) = 115
// readlinkat(AT_FDCWD, "/proc/self/exe", "/home/yuechuan/mydocker/mydocker", 128) = 32
// clone(child_stack=0, flags=CLONE_VM|CLONE_VFORK|CLONE_NEWNS|CLONE_NEWUTS|CLONE_NEWIPC|CLONE_NEWPID|CLONE_NEWNET|SIGCHLD) = 116
