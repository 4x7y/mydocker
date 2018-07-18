package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

var (
	RUNNING             string = "running"
	STOP                string = "stopped"
	Exit                string = "exited"
	DefaultInfoLocation string = "/var/run/mydocker/%s/"
	ConfigName          string = "config.json"
	ContainerLogFile    string = "container.log"
)

type ContainerInfo struct {
	Pid         string `json:"pid"`        // Conainter init process PID on host sys
	Id          string `json:"id"`         // Container ID
	Name        string `json:"name"`       // Container name
	Command     string `json:"command"`    // Command to be executed by init action
	CreatedTime string `json:"createTime"` // Create time
	Status      string `json:"status"`     // Container status
}

// create namespace isolated process os.exec.Cmd struct
// type Cmd struct {
//   Path			string
//   Args			[]string
//   Env			[]string
//   Dir			[]string
//   Stdin			io.Reader
//   Stdout			io.Writter
//   Stderr			io.Writter
//   SysProcAttr	*syscal.SysProcAttr
//   Process        *os.Process {Pid int}
//   ProcessState   *os.ProcessState {}
///  ...            ...
// }

func NewParentProcess(tty bool, volume, rootURL, mntURL, containerName string) (*exec.Cmd, *os.File) {
	// NewParentProcess will fork a new process with argument `init`
	//
	// PID  COMMAND
	// 4649 mydocker run -ti /bin/sh
	// 5665    |-- /proc/self/exe init 		(/proc/self/exe->mydocker)
	//
	// Add "init" before other arguments
	// For example, `$ mydocker` becomes `$ mydocker init`

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		log.Errorf("New pipe error %v", err)
		return nil, nil
	}

	// args = ["init" "/bin/sh"] ?

	// exec.Command() returns the `Cmd` struct to execute the named program
	// with the given arguments
	// It only sets the `Path` and `Args` in the return `Cmd` struct
	// In the function,
	// cmd.Path =  "/proc/self/exe"
	// cmd.Args = ["/proc/self/exe", "init"]
	cmd := exec.Command("/proc/self/exe", "init")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	// If tty is enabled (command parameter `ti`), terminal stdio is redirected
	// to current process
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// DETACH MODE
		// Create log directory
		dirURL := fmt.Sprintf(DefaultInfoLocation, containerName)
		if err := os.MkdirAll(dirURL, 0622); err != nil {
			log.Warnf("$ %v", err)
		} else {
			log.Infof("$ mkdir -p %s -m 0622", dirURL)
		}

		// Create log file
		stdLogFilePath := dirURL + ContainerLogFile
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			log.Errorf("%v", err)

			return nil, nil
		} else {
			log.Infof("$ touch %s", stdLogFilePath)
		}

		cmd.Stdout = stdLogFile
		log.Infof("cmd.stdout > container.logfile")
	}

	cmd.ExtraFiles = []*os.File{readPipe}
	log.Infof("Cloneflag: UTS|PID|NS(MNT)|NET|IPC")

	NewWorkSpace(rootURL, mntURL, volume)
	cmd.Dir = mntURL

	// return `Cmd` struct
	return cmd, writePipe
}

//Create a AUFS filesystem as container root workspace
func NewWorkSpace(rootURL string, mntURL string, volume string) {
	CreateReadOnlyLayer(rootURL)
	CreateWriteLayer(rootURL)
	CreateMountPoint(rootURL, mntURL)
	if volume != "" {
		volumeURLs := volumeUrlExtract(volume)
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			MountVolume(rootURL, mntURL, volumeURLs)
			log.Infof("%q", volumeURLs)
		} else {
			log.Errorf("Volume parameter input is not correct.")
		}
	}
}

func CreateReadOnlyLayer(rootURL string) {
	busyboxURL := rootURL + "/busybox"
	busyboxTarURL := rootURL + "/busybox.tar"
	busybox_dir_exist, err := PathExists(busyboxURL)
	if err != nil {
		log.Errorf("Fail to judge whether dir %s exists. %v", busyboxURL, err)
	}
	if busybox_dir_exist == false {
		if err := os.Mkdir(busyboxURL, 0777); err != nil {
			log.Warnf("$ %v", err)
		} else {
			log.Infof("$ mkdir %s -m 0777", busyboxURL)
		}

		if _, err := exec.Command("tar", "-xvf", busyboxTarURL, "-C", busyboxURL).CombinedOutput(); err != nil {
			log.Errorf("Untar dir %s error %v", busyboxURL, err)
		} else {
			log.Infof("$ tar -xvf %s -C %s", busyboxTarURL, busyboxURL)
		}
	}
}

func CreateWriteLayer(rootURL string) {
	writeURL := rootURL + "/writeLayer"
	if err := os.Mkdir(writeURL, 0777); err != nil {
		log.Warnf("$ %v", err)
	} else {
		log.Infof("$ mkdir %s -m 0777", writeURL)
	}
}

// Volumes provide the best and most predictable performance for write-heavy workloads.
// This is because they bypass the storage driver and do not incur any of the potential
// overheads introduced by thin provisioning and copy-on-write. Volumes have other
// benefits, such as allowing you to share data among containers and persisting even
// when no running container is using them.

func MountVolume(rootURL string, mntURL string, volumeURLs []string) {
	parentUrl := volumeURLs[0]
	exist, err := PathExists(parentUrl)
	if err != nil {
		log.Errorf("%v", err)
	}
	if exist {
		log.Infof("$ mkdir %s -m 0777 (File exists)", parentUrl)
	} else {
		if err := os.Mkdir(parentUrl, 0777); err != nil {
			log.Errorf("%v", parentUrl, err)
		} else {
			log.Infof("$ mkdir %s -m 0777", parentUrl)
		}
	}

	// Make mount point
	containerUrl := volumeURLs[1]
	containerVolumeURL := mntURL + containerUrl
	if err := os.Mkdir(containerVolumeURL, 0777); err != nil {
		log.Infof("Mkdir container dir %s error. %v", containerVolumeURL, err)
	} else {
		log.Infof("$ mkdir %s -m 0777", containerVolumeURL)
	}

	// Mount aufs for container volume
	dirs := "dirs=" + parentUrl
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("Mount volume failed. %v", err)
	} else {
		log.Infof("$ mount -t aufs -o %s none %s", dirs, containerVolumeURL)
		log.Infof("AUFS: %s[rw] -> %s[aufs]", parentUrl, containerVolumeURL)
	}

}

func CreateMountPoint(rootURL string, mntURL string) error {
	if err := os.Mkdir(mntURL, 0777); err != nil {
		log.Infof("Mkdir mountpoint dir %s error. %v", mntURL, err)
	} else {
		log.Infof("$ mkdir %s -m 0777", mntURL)
	}

	dirs := "dirs=" + rootURL + "/writeLayer:" + rootURL + "/busybox"
	cmd := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("Mount mountpoint dir failed. %v", err)
	} else {
		log.Infof("$ mount -t aufs -o %s none %s", dirs, mntURL)
		log.Infof("AUFS: %s[rw], %s[ro] -> %s[aufs]", rootURL+"/writeLayer", rootURL+"/busybox", mntURL)
	}

	return nil
}

//Delete the AUFS filesystem while container exit
func DeleteWorkSpace(rootURL string, mntURL string, volume string) {
	if volume != "" {
		volumeURLs := volumeUrlExtract(volume)
		length := len(volumeURLs)
		if length == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			DeleteMountPointWithVolume(rootURL, mntURL, volumeURLs)
		} else {
			DeleteMountPoint(rootURL, mntURL)
		}
	} else {
		DeleteMountPoint(rootURL, mntURL)
	}
	DeleteWriteLayer(rootURL)
}

func DeleteMountPoint(rootURL string, mntURL string) {
	// Starts the `umount /root/mnt` command in another process and waits
	// for it to complete.
	// cmd := exec.Command("mount", "")
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// if err := cmd.Run(); err != nil {
	// 	log.Errorf("%v", err)
	// }

	if err := syscall.Unmount(mntURL, syscall.MNT_FORCE); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ umount %s", mntURL)
	}

	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ rm -rf %s", mntURL)
	}
}

func DeleteMountPointWithVolume(rootURL string, mntURL string, volumeURLs []string) {
	containerUrl := mntURL + volumeURLs[1]
	// cmd := exec.Command("umount", containerUrl)
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// if err := cmd.Run(); err != nil {
	// 	log.Errorf("Umount volume failed. %v", err)
	// } else {
	// 	log.Infof("umount %s", containerUrl)
	// }
	if err := syscall.Unmount(containerUrl, syscall.MNT_DETACH); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ umount %s", containerUrl)
	}

	// cmd = exec.Command("umount", mntURL)
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// if err := cmd.Run(); err != nil {
	// 	log.Errorf("Umount mountpoint failed. %v", err)
	// } else {
	// 	log.Infof("umount %s", mntURL)
	// }
	if err := syscall.Unmount(mntURL, syscall.MNT_DETACH); err != nil {
		log.Errorf("%v", err)
	} else {
		log.Infof("$ umount %s", mntURL)
	}

	if err := os.RemoveAll(mntURL); err != nil {
		log.Errorf("Remove mountpoint dir %s error %v", mntURL, err)
	} else {
		log.Infof("$ rm -rf %s", mntURL)
	}
}

func DeleteWriteLayer(rootURL string) {
	writeURL := rootURL + "/writeLayer"
	if err := os.RemoveAll(writeURL); err != nil {
		log.Errorf("Remove writeLayer dir %s error %v", writeURL, err)
	} else {
		log.Infof("$ rm -rf %s", writeURL)
	}
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func volumeUrlExtract(volume string) []string {
	var volumeURLs []string
	volumeURLs = strings.Split(volume, ":")
	return volumeURLs
}
