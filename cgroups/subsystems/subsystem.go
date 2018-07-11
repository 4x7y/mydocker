package subsystems

// struct that used to pass resource configuration
// including memory limit, cpu share and # of cpu cores
type ResourceConfig struct {
	MemoryLimit string
	CpuShare    string
	CpuSet      string
}

// Subsystem interface
// Each subsystem implement following four APIs
type Subsystem interface {
	// Return name of subsystem, for example CPU memory
	Name() string

	// Each path represents a cgroup since cgroup resides on
	// hierarchy, which is the path of virtual filesystem
	Set(path string, res *ResourceConfig) error
	Apply(path string, pid int) error
	Remove(path string) error
}

// A subsystem array, each entry contains three pointers,
// pointing to CpusetSubSystem, MemorySubSystem and
// CpuSubSystem, respectively.
var (
	SubsystemsIns = []Subsystem{
		&CpusetSubSystem{},
		&MemorySubSystem{},
		&CpuSubSystem{},
	}
)
