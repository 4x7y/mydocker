package main

import (
	"./cgroups/subsystems"
	"./container"
	"./network"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
)

// To start a container:
// $ sudo mydocker run -ti /bin/sh
var runCommand = cli.Command{
	Name: "run",
	Usage: `Create a container with namespace and cgroups limit,
	-d and -ti cannot be used together
	Format:
		mydocker run [image] [-ti/-d] [command]
		mydocker run [image] -d --name [container name] [command]
		mydocker run [image] --cpushare [250] --cpuset [1] -m [128m] [command]
		mydocker run [image] -v [parent_url:container_url] [command]
		mydocker run [image] -e [myenv:value] -ti [command]
	Example:
		mydocker run busybox --name demo -d --cpuset 1 -m 128m -e my_var=122 "sleep 2"`,
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
		cli.BoolFlag{
			Name:  "d",
			Usage: "detach container",
		},
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
		},
		cli.StringSliceFlag{
			Name:  "e",
			Usage: "set environment",
		},
		cli.StringFlag{
			Name:  "net",
			Usage: "container network",
		},
		cli.StringSliceFlag{
			Name:  "p",
			Usage: "port mapping",
		},
	},

	// 1. check if parameters include `command`
	// 2. get user specified command
	// 3. call `run` function to prepare for container setup
	Action: func(context *cli.Context) error {
		// Assert that command must have at least one arg
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container command")
		}

		// Setup user specified container configuration
		var cmdArray []string
		for _, arg := range context.Args() {
			cmdArray = append(cmdArray, arg)
		}
		config := &container.ContainerConfig{
			TTY:         context.Bool("ti") || !context.Bool("d"),
			Env:         context.StringSlice("e"),
			Name:        context.String("name"),
			ID:          randStringBytes(10),
			Volume:      context.String("v"),
			Pipe:        nil,
			ImageName:   cmdArray[0],
			CmdArray:    cmdArray[1:],
			NetworkName: context.String("net"),
			PortMapping: context.StringSlice("p"),
			Resource: &subsystems.ResourceConfig{
				MemoryLimit: context.String("m"),
				CpuSet:      context.String("cpuset"),
				CpuShare:    context.String("cpushare"),
			},
		}
		// Let default name to be container ID
		if config.Name == "" {
			config.Name = config.ID
		}

		// Refer to file: run.go
		// Wait here until `cmd` exit
		// The `NewParentProcess` invoked in `Run` promise new container
		// process execute `initCommand` after start
		Run(config)

		// Exit mydocker process
		log.Info("Exit.")
		return nil
	},
}

// This command is invoked by child process
var initCommand = cli.Command{
	Name: "init",
	Usage: `[Do not call it] Init container process run user's process in container.`,

	// 1. get passed command parameters (use pipe instead)
	// 2. initialize container
	Action: func(context *cli.Context) error {
		log.Infof("Init action come on")

		// Run containerInitProcess
		// Refer to file: conainer/init.go
		err := container.RunContainerInitProcess()
		return err
	},
}

var commitCommand = cli.Command{
	Name: "commit",
	Usage: `commit a container into image
		mydocker commit [image name]`,
	Action: func(context *cli.Context) error {

		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container name")
		}
		imageName := context.Args().Get(0)
		commitContainer(imageName)
		return nil
	},
}

var listCommand = cli.Command{
	Name: "ps",
	Usage: `list all the containers
		mydocker ps`,
	Action: func(context *cli.Context) error {
		ListContainers()
		return nil
	},
}

var logCommand = cli.Command{
	Name: "logs",
	Usage: `print logs of a container
		mydocker logs [container name]`,
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Please input your container name")
		}
		containerName := context.Args().Get(0)
		logContainer(containerName)
		return nil
	},
}

var execCommand = cli.Command{
	Name: "exec",
	Usage: `exec a command into container
		mydocker exec [container name] [command]`,
	Action: func(context *cli.Context) error {
		// This is for callback
		// For the second time exec, ENV_EXEC_PID is set already
		// just jump over the following code
		if os.Getenv(ENV_EXEC_PID) != "" {
			log.Infof("pid callback pid %s", os.Getpid())
			return nil
		}

		if len(context.Args()) < 2 {
			return fmt.Errorf("Missing container name or command")
		}
		containerName := context.Args().Get(0)
		var commandArray []string
		for _, arg := range context.Args().Tail() {
			commandArray = append(commandArray, arg)
		}
		ExecContainer(containerName, commandArray)
		return nil
	},
}

var stopCommand = cli.Command{
	Name: "stop",
	Usage: `stop a container
		mydocker stop [container name]`,
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container name")
		}
		containerName := context.Args().Get(0)
		stopContainer(containerName)
		return nil
	},
}

var removeCommand = cli.Command{
	Name: "rm",
	Usage: `remove unused container
		mydocker rm [container name]`,
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("Missing container name")
		}
		containerName := context.Args().Get(0)
		removeContainer(containerName)
		return nil
	},
}

var networkCommand = cli.Command{
	Name: "network",
	Usage: `container network commands
		mydocker network create [network name] --driver [driver] --subnet [subnet]
		mydocker network list
		mydocker network remove [network name]
	Example:
		mydocker network create testnet --subnet 192.168.0.0/24 --driver bridge`,
	Subcommands: []cli.Command{
		{
			Name:  "create",
			Usage: "create a container network",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "driver",
					Usage: "network driver",
				},
				cli.StringFlag{
					Name:  "subnet",
					Usage: "subnet cidr",
				},
			},

			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				network.LoadExistNetwork()
				err := network.CreateNetwork(context.String("driver"), context.String("subnet"), context.Args()[0])
				if err != nil {
					return err
				}
				return nil
			},
		},
		{
			Name:  "list",
			Usage: "list container network",
			Action: func(context *cli.Context) error {
				network.LoadExistNetwork()
				network.ListNetwork()
				return nil
			},
		},
		{
			Name:  "remove",
			Usage: "remove container network",
			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("Missing network name")
				}
				network.LoadExistNetwork()
				err := network.DeleteNetwork(context.Args()[0])
				if err != nil {
					return err
				}
				return nil
			},
		},
	},
}
