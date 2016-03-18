package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/iamthemuffinman/logsip"
)

var (
	progName = "gopoddl"
	cfg      *Config
	log      *logsip.Logger
)

// entry point
func main() {

	app := cli.NewApp()
	app.Name = progName
	app.Version = "0.0.1"
	app.Usage = "Podcast downloader"
	app.Before = func(c *cli.Context) (err error) {

		log = logsip.Default()
		log.DebugMode = c.Bool("debug")

		// skip rest of function for init
		if c.Args().First() == "init" {
			return nil
		}

		cfgFile := expandPath(c.GlobalString("config"))

		// was 'init' run
		if !fileExists(cfgFile) {
			log.Debug("Cfg path:", cfgFile)
			log.Warn("Config file was not found, please run 'init' to create it")
			os.Exit(0)
		}

		// Load files
		if cfg, err = NewConfig(cfgFile); err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		return nil
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			EnvVar: "PODDL_CONFIG",
			Name:   "config, c",
			Value:  "~/.gopoddl_conf.ini",
			Usage:  "path to config file",
		},
		cli.BoolFlag{
			Name:   "debug, d",
			EnvVar: "PODDL_DEBUG",
			Usage:  "enable debug",
		},
	}

	app.Commands = []cli.Command{
		cmdInit(), cmdList(), cmdAdd(), cmdRemove(),
		cmdReset(), cmdCheck(), cmdSync(),
	}

	app.Run(os.Args)
}
