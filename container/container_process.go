package container

import (
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"syscall"
)

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

func NewParentProcess(tty bool) (*exec.Cmd, *os.File) {
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
	}
	cmd.ExtraFiles = []*os.File{readPipe}

	// return `Cmd` struct
	return cmd, writePipe
}
