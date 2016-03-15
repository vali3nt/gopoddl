package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/robfig/config"
)

var (
	progName = "gopoddl"
	store    *PodcastStore
	cfg      *config.Config
	log      Logger
	errChan  = make(chan string, 10000)
)

// entry point
func main() {

	app := cli.NewApp()
	app.Name = progName
	app.Version = "0.0.1"
	app.Usage = "Podcast downloader"
	app.Before = func(c *cli.Context) (err error) {

		// if c.IsSet("logfile") {
		// 	log, err = MakeFileLogger(c.Bool("debug"), c.String("logfile"))
		// 	if err != nil {
		// 		fmt.Printf("Failed to init logger. Error: %s\n", err)
		// 	}
		// } else {
		// 	useColors := true
		// 	if c.Bool("nocolor") {
		// 		useColors = false
		// 	}
		// 	log = MakeLogger(c.Bool("debug"), useColors)
		// }

		log, err = MakeLogger(c.String("logfile"), c.Bool("debug"), c.Bool("nocolor"))
		if err != nil {
			fmt.Printf("Failed to init logger. Error: %s\n", err)
			return nil
		}

		if c.Args().First() == "init" {
			return nil
		}

		cfg_file := expandPath(c.GlobalString("config"))
		store_file := expandPath(c.GlobalString("store"))

		if !fileExists(cfg_file) || !fileExists(store_file) {
			log.Debug("Cfg path:", cfg_file)
			log.Debug("Store path:", store_file)
			log.Warn("Config file was not found, please run 'init' to create it")
			os.Exit(0)
		} else {
			// Do not load file while in 'init' action
			// Load files
			log.Debugf("Conf file %s loaded", cfg_file)
			cfg, err = LoadConf(cfg_file)
			if err != nil {
				log.Error(err)
			}
			// Podcast store
			store, err = LoadStore(store_file)
			if err != nil {
				log.Error(err)
			}
			log.Debugf("Store file %s loaded", store_file)

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
		cli.StringFlag{
			Name:  "logfile",
			Usage: "send output to log file",
		},
	}

	app.Commands = []cli.Command{
		cmdInit(), cmdList(), cmdAdd(), cmdRemove(),
		cmdReset(), cmdCheck(), cmdSync(),
	}

	app.Run(os.Args)
}
