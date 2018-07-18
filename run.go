package main

import (
	"../mydocker/cgroups"
	"../mydocker/cgroups/subsystems"
	"../mydocker/container"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
	"syscall"
)

// func Run(tty bool, comArray []string, res *subsystems.ResourceConfig) {

func Run(tty bool, comArray []string, volume string, res *subsystems.ResourceConfig) {
	// Refer to source code container_process.go
	// `parent` is a `Cmd` struct which contains exe path, args, etc.
	// Commands that going to be executed by the new child process
	// is now passed through a pipe.

	mntURL := "/root/mnt"
	rootURL := "/root"
	parent, writePipe := container.NewParentProcess(tty, volume, rootURL, mntURL)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}

	// Starts the specified command but does not wait for it to complete
	// Equivalent: "/fork/exec /proc/self/exe init /bin/sh"
	// That is, "/fork/exec mydocker init /bin/sh"
	if err := parent.Start(); err != nil {
		log.Error(err)
	}
	// Use mydocker-cgroup as cgroup name
	log.Infof("Configuring cgroup ...")
	cgroupManager := cgroups.NewCgroupManager("mydocker")
	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)

	// Pass commands to container process via os.Pipe
	// "stress --vm-bytes 200m --vm-keep -m 1" -> pipe -> container
	sendInitCommand(comArray, writePipe)

	// Waits for the `parent` command to exit and waits for any copying
	// from stdout or stderr to complete.
	// Also, `Wait` releases any resources assoicated with the `parent`
	parent.Wait()

	// Tear down
	container.DeleteWorkSpace(rootURL, mntURL, volume)
	syscall.Mount("proc", "/proc", "proc",
		uintptr(syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV), "")
	log.Infof("$ mount proc proc /proc")

	log.Infof("CGroups Destroy ...")
	cgroupManager.Destroy()

	os.Exit(0)
}

func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Infof("Send: [%s] -> pipe", command)
	writePipe.WriteString(command)
	writePipe.Close()
}
