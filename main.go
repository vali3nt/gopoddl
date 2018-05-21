package main

import (
	"os"

	"github.com/iamthemuffinman/logsip"
	"github.com/urfave/cli"
)

var (
	progName = "gopoddl"
	cfg      *Config
	log      *logsip.Logger
)

func initLogger(debug bool) {
	log = logsip.New()
	if debug {
		log.SetLevel(logsip.DebugLevel)
	} else {
		log.SetLevel(logsip.InfoLevel)
	}
}

// entry point
func main() {

	app := cli.NewApp()
	app.Name = progName
	app.Version = "0.0.1"
	app.Usage = "Podcast downloader"
	app.Before = func(c *cli.Context) (err error) {

		initLogger(c.Bool("debug"))
		// skip rest of function for init
		switch c.Args().First() {
		case "init", "i", "help", "h":
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
