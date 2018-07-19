package main

import (
	"../mydocker/misc"
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
		commitCommand,
		listCommand,
		logCommand,
		execCommand,
		stopCommand,
	}

	app.Before = func(context *cli.Context) error {

		// Setup logger
		log.AddHook(filename.NewHook())
		log.AddHook(misc.NewHook())
		var textFormatter = new(log.TextFormatter)
		// textFormatter.TimestampFormat = "15:04:05"
		// textFormatter.FullTimestamp = true
		textFormatter.ForceColors = true
		textFormatter.DisableTimestamp = true
		log.SetFormatter(textFormatter)
		log.SetOutput(os.Stdout)

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
