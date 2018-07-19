package container

import (
	"../cgroups/subsystems"
	"os"
)

type ContainerInfo struct {
	Pid         string   `json:"pid"`         // Conainter init process PID on host sys
	Id          string   `json:"id"`          // Container ID
	Name        string   `json:"name"`        // Container name
	Command     string   `json:"command"`     // Command to be executed by init action
	CreatedTime string   `json:"createTime"`  // Create time
	Status      string   `json:"status"`      // Container status
	Volume      string   `json:"volume"`      // Container volume
	PortMapping []string `json:"portmapping"` // Port mapping
}

type ContainerConfig struct {
	TTY         bool
	Name        string
	ID          string
	Volume      string
	ImageName   string
	Env         []string
	Network     string
	PortMapping []string
	Pipe        *os.File
	CmdArray    []string
	Resource    *subsystems.ResourceConfig
}
