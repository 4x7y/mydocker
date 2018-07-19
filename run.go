package main

import (
	"../mydocker/cgroups"
	"../mydocker/cgroups/subsystems"
	"../mydocker/container"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// func Run(tty bool, comArray []string, res *subsystems.ResourceConfig) {

func Run(tty bool, comArray []string, res *subsystems.ResourceConfig, containerName, volume, imageName string, envSlice []string) {
	// Refer to source code container_process.go
	// `parent` is a `Cmd` struct which contains exe path, args, etc.
	// Commands that going to be executed by the new child process
	// is now passed through a pipe.

	log.Infof("Prepare container process ...")
	parent, writePipe := container.NewParentProcess(tty, containerName, volume, imageName, envSlice)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	log.Info("Done.")

	// Starts the specified command but does not wait for it to complete
	// Equivalent: "/fork/exec /proc/self/exe init /bin/sh"
	// That is, "/fork/exec mydocker init /bin/sh"
	if err := parent.Start(); err != nil {
		log.Error(err)
	}
	log.Infof("$ fork %s %s (child process pid=%d)", parent.Args[0], parent.Args[1], parent.Process.Pid)

	// Now container is running ...

	// Record container process info
	log.Infof("Record container info ...")
	containerName, err := recordContainerInfo(parent.Process.Pid, comArray, containerName)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return
	}
	log.Info("Done.")

	// Setup cgroups for container process
	log.Infof("CGroup configuring ...")
	cgroupManager := cgroups.NewCgroupManager("mydocker")
	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)
	log.Info("Done.")

	// Pass commands to container process via os.Pipe
	// "stress --vm-bytes 200m --vm-keep -m 1" -> pipe -> container
	sendInitCommand(comArray, writePipe)

	if tty {
		// Waits for the `parent` command to exit and waits for any copying
		// from stdout or stderr to complete.
		// Also, `Wait` releases any resources assoicated with the `parent`
		parent.Wait()

		// Tear down
		deleteContainerInfo(containerName)
		container.DeleteWorkSpace(volume, containerName)
		syscall.Mount("proc", "/proc", "proc",
			uintptr(syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV), "")
		log.Infof("$ mount proc proc /proc")

		log.Infof("CGroups destroy ...")
		cgroupManager.Destroy()
		log.Infof("Done.")
	} else {
		log.Infof("Enter detach mode ...")
	}
}

func recordContainerInfo(containerPID int, commandArray []string, containerName string) (string, error) {
	id := randStringBytes(10)
	createTime := time.Now().Format("2006-01-02 15:04:05")
	command := strings.Join(commandArray, "")
	if containerName == "" {
		containerName = id
	}
	containerInfo := &container.ContainerInfo{
		Id:          id,
		Pid:         strconv.Itoa(containerPID),
		Command:     command,
		CreatedTime: createTime,
		Status:      container.RUNNING,
		Name:        containerName,
	}

	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return "", err
	}
	jsonStr := string(jsonBytes)

	dirUrl := fmt.Sprintf(container.DefaultInfoLocation, containerName)
	if err := os.MkdirAll(dirUrl, 0622); err != nil {
		log.Errorf("Mkdir error %s error %v", dirUrl, err)
		return "", err
	} else {
		log.Infof("$ mkdir -p %s -m 0622", dirUrl)
	}
	fileName := dirUrl + container.ConfigName
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		log.Errorf("Create file %s error %v", fileName, err)
		return "", err
	}
	if _, err := file.WriteString(jsonStr); err != nil {
		log.Errorf("File write string error %v", err)
		return "", err
	} else {
		log.Infof("$ echo {\"id\":\"%s\", ...} > %s", containerInfo.Id, fileName)
	}

	return containerName, nil
}

func deleteContainerInfo(containerId string) {
	dirURL := fmt.Sprintf(container.DefaultInfoLocation, containerId)
	if err := os.RemoveAll(dirURL); err != nil {
		log.Errorf("Remove dir %s error %v", dirURL, err)
	}
}

func sendInitCommand(comArray []string, writePipe *os.File) {
	command := strings.Join(comArray, " ")
	log.Infof("Send: [%s] -> pipe", command)
	writePipe.WriteString(command)
	writePipe.Close()
}

func randStringBytes(n int) string {
	letterBytes := "1234567890"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
