package cgroups

import (
	"../cgroups/subsystems"
	"github.com/sirupsen/logrus"
)

type CgroupManager struct {
	// cgroup在hierarchy中的路径 相当于创建的cgroup目录相对于root cgroup目录的路径
	Path string
	// Resource configuration
	Resource *subsystems.ResourceConfig
}

func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{
		Path: path,
	}
}

// 将进程pid加入到这个cgroup中
func (c *CgroupManager) Apply(pid int) error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		subSysIns.Apply(c.Path, pid)
	}
	return nil
}

// Set cgroup resource limitation
func (c *CgroupManager) Set(res *subsystems.ResourceConfig) error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		subSysIns.Set(c.Path, res)
	}
	return nil
}

// Release cgroup
func (c *CgroupManager) Destroy() error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		if err := subSysIns.Remove(c.Path); err != nil {
			logrus.Warnf("%v", err)
		}
	}
	return nil
}
