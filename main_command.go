package main

import (
	"../mydocker/cgroups/subsystems"
	"../mydocker/container"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// This file defines two basic mydocker commands, including
// runCommand and intiCommand,

// To start a container:
// $ sudo mydocker run -ti /bin/sh
var runCommand = cli.Command{
	Name: "run",
	Usage: `Create a container with namespace and cgroups limit
			mydocker run -ti [command]`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "ti",
			Usage: "enable tty",
		},
		cli.StringFlag{
			Name:  "m",
			Usage: "memory limit",
		},
		cli.StringFlag{
			Name:  "cpushare",
			Usage: "cpushare limit",
		},
		cli.StringFlag{
			Name:  "cpuset",
			Usage: "cpuset limit",
		},
		cli.StringFlag{
			Name:  "v",
			Usage: "volume",
		},
	},

	// 1. check if parameters include `command`
	// 2. get user specified command
	// 3. call `run` function to prepare for container setup
	Action: func(context *cli.Context) error {
		// Assert that command must have at least one args
		// $ mydocker run ...
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container command")
		}

		var cmdArray []string
		for _, arg := range context.Args() {
			cmdArray = append(cmdArray, arg)
		}

		// Check if argument `ti` is specified
		tty := context.Bool("ti")
		resConf := &subsystems.ResourceConfig{
			MemoryLimit: context.String("m"),
			CpuSet:      context.String("cpuset"),
			CpuShare:    context.String("cpushare"),
		}
		volume := context.String("v")
		log.Infof("Run a new process (tty=%v cmdArray=%v resConf=%v)",
			tty, cmdArray, resConf)
		// Refer to file: run.go
		// Wait here until `cmd` exit
		// The `NewParentProcess` invoked in `Run` promise
		// new container process execute `initCommand` after start
		Run(tty, cmdArray, volume, resConf)

		return nil
	},
}

// This command is invoked by child process
var initCommand = cli.Command{
	Name:  "init",
	Usage: "Init container process run user's process in container. Do not call it outside",

	// 1. get passed command parameters (use pipe instead)
	// 2. initialize container
	Action: func(context *cli.Context) error {
		log.Infof("Init action come on")

		// Print out the argument of the command, for example /bin/sh
		// cmd := context.Args().Get(0)
		// log.Infof("Init command: %s", cmd)

		// Run containerInitProcess
		// Refer to file: conainer/init.go
		err := container.RunContainerInitProcess()
		return err
	},
}
