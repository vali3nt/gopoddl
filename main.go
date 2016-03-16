package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/iamthemuffinman/logsip"
	"github.com/robfig/config"
)

var (
	progName = "gopoddl"
	store    *PodcastStore
	cfg      *config.Config
	log      *logsip.Logger
)

func checkConfigFiles(cfgPath, storePath string) bool {
	if !fileExists(cfgPath) || !fileExists(storePath) {
		log.Debug("Cfg path:", cfgPath)
		log.Debug("Store path:", storePath)
		log.Warn("Config file was not found, please run 'init' to create it")
		return false
	}
	return true
}

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
		storeFile := expandPath(c.GlobalString("store"))

		// was 'init' run
		if !checkConfigFiles(cfgFile, storeFile) {
			os.Exit(0)
		}

		// Load files
		if cfg, err = LoadConf(cfgFile); err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		// Podcast store
		if store, err = LoadStore(storeFile); err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		return nil
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			EnvVar: "PODDL_STORE",
			Name:   "store, s",
			Value:  "~/.gopoddl_db.json",
			Usage:  "path to podcast store file",
		},
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
		cli.BoolFlag{
			Name:  "nocolor",
			Usage: "disable colors",
		},
	}

	app.Commands = []cli.Command{
		cmdInit(), cmdList(), cmdAdd(), cmdRemove(),
		cmdReset(), cmdCheck(), cmdSync(),
	}

	app.Run(os.Args)
}
