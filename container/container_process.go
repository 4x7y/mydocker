package container

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"syscall"
)

var (
	RUNNING             string = "running"
	STOP                string = "stopped"
	Exit                string = "exited"
	DefaultInfoLocation string = "/var/run/mydocker/%s/"
	ConfigName          string = "config.json"
	ContainerLogFile    string = "container.log"
	RootUrl             string = "/root"
	MntUrl              string = "/root/mnt/%s"
	WriteLayerUrl       string = "/root/writeLayer/%s"
)

type ContainerInfo struct {
	Pid         string `json:"pid"`        // Conainter init process PID on host sys
	Id          string `json:"id"`         // Container ID
	Name        string `json:"name"`       // Container name
	Command     string `json:"command"`    // Command to be executed by init action
	CreatedTime string `json:"createTime"` // Create time
	Status      string `json:"status"`     // Container status
	Volume      string `json:"volume"`     // Container volume
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

func NewParentProcess(tty bool, containerName, volume, imageName string, envSlice []string) (*exec.Cmd, *os.File) {
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
		cmd.Stderr = stdLogFile
		log.Infof("Container.Stdout > container.logfile")
	}

	log.Infof("Container.NSFlag: UTS|PID|NS(MNT)|NET|IPC")
	cmd.ExtraFiles = []*os.File{readPipe}
	log.Infof("Container.Files : %s", "readPipe")
	cmd.Env = append(os.Environ(), envSlice...)
	log.Infof("Container.Env   : %v", envSlice)
	cmd.Dir = fmt.Sprintf(MntUrl, containerName)
	log.Infof("Container.Dir   : %s", cmd.Dir)

	NewWorkSpace(volume, imageName, containerName)

	// return `Cmd` struct
	return cmd, writePipe
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
