package misc

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"sync"
)

// Hook is a hook designed for dealing with logs in test scenarios.
type Hook struct {
	// Entries is an array of all entries that have been received by this hook.
	// For safe access, use the AllEntries() method, rather than reading this
	// value directly.
	Entries []*logrus.Entry
	mu      sync.RWMutex

	Field     string
	levels    []logrus.Level
	Formatter func(pid int) string
}

func NewHook(levels ...logrus.Level) *Hook {

	hook := Hook{
		Field:  "pid",
		levels: levels,
		Formatter: func(pid int) string {

			// logrus will automatically add double quotes if returned string has
			// space, so here we use `_` to substitute space ` `.
			str := fmt.Sprintf("%d", pid)
			length := len(str)
			var buffer bytes.Buffer
			for i := 0; i < 5-length; i++ {
				buffer.WriteString("_")
			}

			return buffer.String() + str
		},
	}
	if len(hook.levels) == 0 {
		hook.levels = logrus.AllLevels
	}

	return &hook
}

func (hook *Hook) Fire(entry *logrus.Entry) error {
	entry.Data[hook.Field] = hook.Formatter(os.Getpid())
	return nil
}

func (hook *Hook) Levels() []logrus.Level {
	return hook.levels
}
