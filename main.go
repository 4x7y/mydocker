package main

import (
	"github.com/onrik/logrus/filename"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
)

const usage = `mydocker is a simple container runt me implementation 
				The purpose of this project is to learn how docker works 
				and how to write a docker by ourselves.
				Enjoy it, just for fun.`

func main() {
	app := cli.NewApp()
	app.Name = "myDocker"
	app.Usage = usage

	app.Commands = []cli.Command{
		initCommand,
		runCommand,
	}

	app.Before = func(context *cli.Context) error {
		log.AddHook(filename.NewHook())
		// Log as JSON instead of the default ASCII formatter.
		log.SetFormatter(&log.TextFormatter{})

		log.SetOutput(os.Stdout)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
