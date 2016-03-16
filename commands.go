package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/codegangsta/cli"
	"github.com/fatih/color"
)

// if argment len is less then count
// show usage and return false, otherwise return true
func checkArgumentsCount(c *cli.Context, commandName string, count int) bool {
	if len(c.Args()) < count {
		log.Warn("Invlid usage.")
		cli.ShowCommandHelp(c, commandName)
		return false
	}
	return true
}

// 'init' - command
func cmdInit() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "init"
	cmd.ShortName = "i"
	cmd.Usage = "create default config files"
	cmd.Action = func(c *cli.Context) {
		cfgFile := expandPath(c.GlobalString("config"))
		storeFile := expandPath(c.GlobalString("store"))

		if !fileExists(cfgFile) {
			log.Debugf("Creating : %s\n", cfgFile)
			if err := InitConf(cfgFile); err != nil { // create config file
				log.Fatal(err)
			}

			log.Print(color.GreenString("Default config created."))
			log.Printf("* Do not forget to change download_path in %s", cfgFile)
			log.Print("* It's set to HOME directory")
		} else {
			log.Printf("* Config file %s exists", cfgFile)
		}

		if !fileExists(storeFile) {
			log.Debugf("Creating : %s", storeFile)
			if err := InitStore(storeFile); err != nil { // create store file
				log.Fatal(err)
			}
		} else {
			log.Printf("* Store file %s exists", storeFile)
		}
	}
	return cmd
}

// 'list' - command
func cmdList() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "list"
	cmd.ShortName = "l"
	cmd.Usage = "list all podcasts"
	cmd.Action = func(c *cli.Context) {
		if len(store.Podcasts) == 0 {
			log.Warn("No podcasts in store yet")
			return
		}
		for n := range store.Podcasts {
			var lastUpdated string
			isDisabled := ""
			if disable, err := cfg.Bool(store.Podcasts[n].Name, "disable"); err != nil {
				log.Fatalf("Failed to get 'disable' option: %s", err)
			} else if disable {
				isDisabled = color.YellowString("[disabled]")
			}

			if store.Podcasts[n].LastSynced.IsZero() {
				lastUpdated = color.CyanString("Never")
			} else {
				lastUpdated = fmt.Sprintf("%s [%d days ago]",
					store.Podcasts[n].LastSynced.Format("2006-01-02 15:4"),
					int(time.Now().Sub(store.Podcasts[n].LastSynced)/(24*time.Hour)))
			}
			num := color.MagentaString("[" + strconv.Itoa(n+1) + "] ")
			log.Printf("%s %s %s", num, store.Podcasts[n].Name, isDisabled)
			log.Printf("\t* Url             : %s", store.Podcasts[n].Url)
			log.Printf("\t* Last synced     : %s", lastUpdated)
			log.Printf("\t* Files downloaded: %d", store.Podcasts[n].DownloadedFiles)
		}
	}
	return cmd
}

// 'add' - command
func cmdAdd() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "add"
	cmd.ShortName = "a"
	cmd.Usage = "add podcast to sync"
	cmd.Action = func(c *cli.Context) {
		if !checkArgumentsCount(c, "add", 1) {
			return
		}

		url := c.Args().Get(0)
		podcastName := c.Args().Get(1)

		if err := store.Add(url, podcastName); err != nil {
			if err == ErrAlreadyExistInStore {
				log.Warnf("Podcast <%s> exists in store already", podcastName)
			} else {
				log.Fatal(err)
			}
			return
		}
		log.Printf("* Podcast [%s] added", podcastName)
	}

	return cmd
}

// 'remove' - command
func cmdRemove() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "remove"
	cmd.ShortName = "r"
	cmd.Usage = "remove podcast from sync"
	cmd.Action = func(c *cli.Context) {
		if !checkArgumentsCount(c, "remove", 1) {
			return
		}

		nameOrId := c.Args().First()
		if err := store.Remove(nameOrId); err != nil {
			if err == ErrWasNotFoundInStore {
				log.Warnf("Name or ID <%s> was not found in store. do nothing", nameOrId)
			} else {
				log.Fatal(err)
			}
			return
		}
		log.Printf("* [%s] removed", nameOrId)
	}

	return cmd
}

// 'reset' - command
func cmdReset() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "reset"
	cmd.Usage = "reset time and count for podcasts"
	cmd.Action = func(c *cli.Context) {
		if err := store.ResetAll(); err != nil {
			log.Fatal(err)
		}
	}

	return cmd
}

// 'check' - command
func cmdCheck() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "check"
	cmd.ShortName = "c"
	cmd.Usage = "check podcasts for availability"
	cmd.Action = func(c *cli.Context) {
		checkPodcasts()
	}

	return cmd
}

// 'sync' - command
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
		var err error

		if c.IsSet("date") {
			if date, err = parseTime(c.String("date")); err != nil {
				log.Fatal(err)
				return
			}
		}

		podcastCount := c.Int("count")
		isOverwrite := c.Bool("overwrite")

		log.Infof("Started at %s", time.Now())
		if err = syncPodcasts(date, podcastCount, isOverwrite); err != nil {
			log.Fatal(err)
		}
		log.Infof("Finished at %s", time.Now())
	}

	return cmd
}
