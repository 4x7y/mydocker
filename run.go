package main

import (
	"../mydocker/cgroups"
	"../mydocker/container"
	"../mydocker/network"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func Run(config *container.ContainerConfig) {
	// Refer to source code container_process.go
	// `containerProcess` is a `Cmd` struct which contains exe path, args, etc.
	// Commands that going to be executed by the new child process
	// is now passed through a pipe.
	log.Infof("Prepare container process ...")
	containerProcess, writePipe := container.NewParentProcess(
		config.TTY, config.Name, config.Volume, config.ImageName, config.Env)
	if containerProcess == nil {
		log.Errorf("New containerProcess process error")
		return
	}
	log.Info("Done.")

	// Starts the specified command but does not wait for it to complete
	// Equivalent: "/fork/exec /proc/self/exe init /bin/sh"
	// That is, "/fork/exec mydocker init /bin/sh"
	if err := containerProcess.Start(); err != nil {
		log.Error(err)
	}
	log.Infof("$ fork %s %s (child process pid=%d)",
		containerProcess.Args[0], containerProcess.Args[1], containerProcess.Process.Pid)

	// Now container is running ...

	// Record container process info
	log.Infof("Record container info ...")
	containerPid := containerProcess.Process.Pid
	runtimeInfo := makeContainerInfo(containerPid, config)
	if err := recordContainerInfo(runtimeInfo); err != nil {
		log.Errorf("Record container info error %v", err)
		return
	}
	log.Info("Done.")

	// Setup cgroups for container process
	log.Infof("CGroup configuring ...")
	cgroupManager := cgroups.NewCgroupManager(config.ID)
	cgroupManager.Set(config.Resource)
	cgroupManager.Apply(containerPid)
	log.Info("Done.")

	// Config container network, try connecting to config.NetworkName
	if config.NetworkName != "" {
		network.LoadExistNetwork()
		if err := network.Connect(config.NetworkName, runtimeInfo); err != nil {
			log.Errorf("%v", err)
			return
		}
	}

	// Pass commands to container process via os.Pipe
	// "stress --vm-bytes 200m --vm-keep -m 1" -> pipe -> container
	sendInitCommand(config.CmdArray, writePipe)

	// Waite for container process exit or isolate container if detach mode specified
	if config.TTY {
		// Waits for the `containerProcess` command to exit and waits for any copying
		// from stdout or stderr to complete.
		// Also, `Wait` releases any resources assoicated with the `containerProcess`
		containerProcess.Wait()

		// Tear down
		deleteContainerInfo(config.Name)
		container.DeleteWorkSpace(config.Volume, config.Name)
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

func makeContainerInfo(pid int, config *container.ContainerConfig) *container.ContainerInfo {
	containerInfo := &container.ContainerInfo{
		Id:          config.ID,
		Name:        config.Name,
		Volume:      config.Volume,
		Pid:         strconv.Itoa(pid),
		Command:     strings.Join(config.CmdArray, ""),
		CreatedTime: time.Now().Format("2006-01-02 15:04:05"),
		Status:      container.RUNNING,
		PortMapping: config.PortMapping,
	}

	return containerInfo
}

func recordContainerInfo(containerInfo *container.ContainerInfo) error {
	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return err
	}
	jsonStr := string(jsonBytes)

	dirUrl := fmt.Sprintf(container.DefaultInfoLocation, containerInfo.Name)
	if err := os.MkdirAll(dirUrl, 0622); err != nil {
		log.Errorf("Mkdir error %s error %v", dirUrl, err)
		return err
	} else {
		log.Infof("$ mkdir -p %s -m 0622", dirUrl)
	}
	fileName := dirUrl + container.ConfigName
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		log.Errorf("Create file %s error %v", fileName, err)
		return err
	}
	if _, err := file.WriteString(jsonStr); err != nil {
		log.Errorf("File write string error %v", err)
		return err
	} else {
		log.Infof("$ echo {\"id\":\"%s\", ...} > %s", containerInfo.Id, fileName)
	}

	return nil
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
