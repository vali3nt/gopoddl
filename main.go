package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/gosuri/uiprogress"
	"github.com/robfig/config"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	progName = "gopoddl"
	store    *PodcastStore
	cfg      *config.Config
	log      *Logger
	errChan  = make(chan string, 10000)
)

func cmdInit() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "init"
	cmd.ShortName = "i"
	cmd.Usage = "create default config files"
	cmd.Action = func(c *cli.Context) {
		cfgFile := expandPath(c.GlobalString("config"))
		if !fileExists(cfgFile) {
			log.Debugf("Creating : %s", cfgFile)
			if err := InitConf(cfgFile); err != nil { // create config file
				log.Error(err)
			}
			log.Print(log.Color("green", "Default config created."))
			log.Printf("* Do not forget to change download_path under %s", cfgFile)
			log.Print("* Now it set to HOME directory")
		} else {
			log.Printf("* Config file %s exists", cfgFile)
		}
		storeFile := expandPath(c.GlobalString("store"))
		// create empty store file, json format

		if !fileExists(storeFile) {
			log.Debugf("Creating : %s", storeFile)
			if err := InitStore(storeFile); err != nil { // create store file
				log.Error(err)
			}
		} else {
			log.Printf("* Store file %s exists", storeFile)
		}
	}
	return cmd
}

func cmdList() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "list"
	cmd.ShortName = "l"
	cmd.Usage = "list all podcasts"
	cmd.Action = func(c *cli.Context) {
		if len(store.Podcasts) == 0 {
			log.Warn("No podcasts yet")
			return
		}
		for n := range store.Podcasts {
			var lastUpdated string
			isDisabled := ""
			if disable, err := cfg.Bool(store.Podcasts[n].Name, "disable"); err != nil {
				log.Fatalf("Failed to get 'disable' option: %s", err)
			} else if disable {
				isDisabled = log.Color("yellow", "[disabled]")
			}

			if store.Podcasts[n].LastSynced.IsZero() {
				lastUpdated = log.Color("cyan", "Never")
			} else {
				lastUpdated = fmt.Sprintf("%s [%d days ago]", store.Podcasts[n].LastSynced.Format("2006-01-02 15:4"),
					int(time.Now().Sub(store.Podcasts[n].LastSynced)/(24*time.Hour)))
			}
			num := log.Color("magenta", "["+strconv.Itoa(n+1)+"] ")
			log.Printf("%s %s %s", num, store.Podcasts[n].Name, isDisabled)
			log.Printf("\t* Url             : %s", store.Podcasts[n].Url)
			log.Printf("\t* Last synced     : %s", lastUpdated)
			log.Printf("\t* Files downloaded: %d", store.Podcasts[n].DownloadedFiles)
		}
	}
	return cmd
}

func cmdAdd() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "add"
	cmd.ShortName = "a"
	cmd.Usage = "add podcast to sync"
	cmd.Action = func(c *cli.Context) {
		if len(c.Args()) < 1 {
			log.Warn("Invlid usage.")
			cli.ShowCommandHelp(c, "add")
			return
		}
		if err := store.Add(c.Args().Get(0), c.Args().Get(1)); err != nil {
			log.Error(err)
		}
	}

	return cmd
}

func cmdRemove() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "remove"
	cmd.ShortName = "r"
	cmd.Usage = "remove podcast from sync"
	cmd.Action = func(c *cli.Context) {
		if len(c.Args()) < 1 {
			log.Warn("Invlid usage.")
			cli.ShowCommandHelp(c, "remove")
			return
		}
		if err := store.Remove(c.Args().First()); err != nil {
			log.Error(err)
		}
	}

	return cmd
}

func cmdReset() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "reset"
	cmd.Usage = "reset time and count for podcasts"
	cmd.Action = func(c *cli.Context) {
		var emptyDate time.Time
		for k := range store.Podcasts {
			store.Podcasts[k].LastSynced = emptyDate
			store.Podcasts[k].DownloadedFiles = 0
		}
		store.Save()
	}

	return cmd
}

func cmdCheck() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "check"
	cmd.ShortName = "c"
	cmd.Usage = "check podcasts for availability"
	cmd.Action = func(c *cli.Context) {
		var date time.Time
		// parse input date
		for n := range store.Podcasts {
			p := store.Podcasts[n]

			// exclude disabled
			if disable, err := cfg.Bool(p.Name, "disable"); err != nil {
				log.Fatalf("Failed to get 'disable' option: %s", err)
			} else if disable {
				continue
			}

			// build filter
			mtype := strings.Split(getCfgStringNoErr(p.Name, "mtype"), ",")
			f := PodcastFilter{
				StartDate:    date,
				Count:        -1,
				MediaType:    mtype,
				Filter:       getCfgStringNoErr(p.Name, "filter"),
				DateFormat:   getCfgStringNoErr(p.Name, "date-format"),
				SeperatePath: getCfgStringNoErr(p.Name, "separate-dir"),
			}
			// Get list
			podcastList, err := store.Filter(p, &f)
			status := log.Color("green", "OK")
			if err != nil {
				status = log.Color("red", "FAIL")
			}
			num := log.Color("magenta", "["+strconv.Itoa(n+1)+"] ")

			log.Printf("%s %s", num, p.Name)
			log.Printf("\t* Url             : %s %s", p.Url, status)
			if err != nil {
				log.Debugf("Error: %s", err)
			} else {
				log.Printf("\t* Awaiting files  : %d", len(podcastList))
				for k := range podcastList {
					log.Printf("\t\t* [%d] : %s", k, podcastList[k].ItemTitile)
				}
			}
		}
	}

	return cmd
}

func cmdSync() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "sync"
	cmd.ShortName = "s"
	cmd.Usage = "add podcast to sync"

	cmd.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "date, d",
			Usage: "Start date for sync [format: YYYYMMDD]",
		},
		cli.IntFlag{
			Name:  "count, c",
			Value: -1,
			Usage: "Number of podcasts to download ( -1 means all )",
		},
		cli.BoolFlag{
			Name:  "overwrite, o",
			Usage: "Overwrite files on download",
		},
		cli.BoolFlag{
			Name:  "progress",
			Usage: "Show progress with bar",
		},
	}
	cmd.Action = func(c *cli.Context) {

		var date time.Time
		// parse input date
		if c.IsSet("date") {
			var err error
			date, err = parseTime(c.String("date"))
			if err != nil {
				log.Error(err)
			}
		}
		log.Printf("Started at %s", time.Now())

		pChan := make(chan downloadStatus)

		if c.Bool("progress") {
			uiprogress.Start()
		}

		podcastCount := 0
		for n := range store.Podcasts {
			var uiBar *uiprogress.Bar
			p := store.Podcasts[n]

			// exclude disabled
			if disable, err := cfg.Bool(p.Name, "disable"); err != nil {
				log.Fatalf("Failed to get 'disable' option: %s", err)
			} else if disable {
				continue
			}

			// build filter
			mtype := strings.Split(getCfgStringNoErr(p.Name, "mtype"), ",")
			f := PodcastFilter{
				StartDate:    date,
				Count:        c.Int("count"),
				MediaType:    mtype,
				Filter:       getCfgStringNoErr(p.Name, "filter"),
				DateFormat:   getCfgStringNoErr(p.Name, "date-format"),
				SeperatePath: getCfgStringNoErr(p.Name, "separate-dir"),
			}
			// get list
			downloadPath := getCfgStringNoErr(p.Name, "download-path")
			podcastList, err := store.Filter(p, &f)

			if err != nil {
				log.Errorf("Error %s: %v", p.Name, err)
				continue
			}

			if len(podcastList) == 0 {
				log.Printf("%s : %s, %d files", log.Color("cyan", "EMPTY"),
					store.Podcasts[n].Name, len(podcastList))
				continue
			}
			if !c.Bool("progress") {
				log.Printf("%s : %s, %d files", log.Color("green", "START"),
					store.Podcasts[n].Name, len(podcastList))
			} else {
				// show progress per podcast
				uiBar = uiprogress.AddBar(len(podcastList)).AppendCompleted()
				uiBar.PrependFunc(func(b *uiprogress.Bar) string {
					displayName := ""
					if len(p.Name) >= 20 {
						displayName = p.Name[:17] + "..."
					} else {
						displayName = p.Name
					}
					return fmt.Sprintf("%-20s (%d/%d)", displayName, b.Current(), len(podcastList))
				})
			}
			if !fileExists(downloadPath) {
				if err := os.MkdirAll(downloadPath, 0777); err != nil {
					log.Error(err)
					continue
				}
			}
			podcastCount++ // podcast counter
			d := downloadSet{
				ui:           &uiInfo{uiBar: uiBar, showProgress: c.Bool("progress")},
				podcastList:  podcastList,
				downloadPath: downloadPath,
				overwrite:    c.Bool("overwrite"),
				idx:          n,
			}

			go downloadPodcastList(d, pChan)
		}
		donePodcasts := 0
		if podcastCount != 0 {

		M:
			for {
				select {
				case status := <-pChan:
					if !c.Bool("progress") {
						log.Printf("%s : %s",
							log.Color("green", "DONE"),
							store.Podcasts[status.podcastIdx].Name)
					}
					// Update statistic
					store.Podcasts[status.podcastIdx].LastSynced = time.Now()
					store.Podcasts[status.podcastIdx].DownloadedFiles += status.fileCounter
					store.Save()
					donePodcasts++
					if donePodcasts == podcastCount {
						break M
					}
				}
			}
		}
		close(pChan)

		if c.Bool("pregress") {
			for msg := range errChan {
				log.Printf(msg)
			}
			uiprogress.Stop()
		}
		close(errChan)
		log.Printf("Finished at %s", time.Now())

	}

	return cmd
}

func main() {

	app := cli.NewApp()
	app.Name = progName
	app.Version = "0.0.1"
	app.Usage = "Podcast downloader"
	app.Before = func(c *cli.Context) (err error) {

		if c.IsSet("logfile") {
			log, err = MakeFileLogger(c.Bool("debug"), c.String("logfile"))
			if err != nil {
				fmt.Printf("Failed to init logger. Error: %s\n", err)
			}
		} else {
			useColors := true
			if c.Bool("nocolor") {
				useColors = false
			}
			log = MakeLogger(c.Bool("debug"), useColors)
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
