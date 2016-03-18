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
	cmd.Usage = "create default config file"
	cmd.Action = func(c *cli.Context) {
		cfgFile := expandPath(c.GlobalString("config"))

		if !fileExists(cfgFile) {
			log.Debugf("Creating : %s\n", cfgFile)
			if err := CreateDefaultConfig(cfgFile); err != nil { // create config file
				log.Fatal(err)
			}

			log.Info(color.GreenString("Default config created."))
			log.Infof("* Do not forget to change download_path in %s", cfgFile)
			log.Info("* It's set to HOME directory by default")
		} else {
			log.Infof("* Config file %s exists", cfgFile)
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
		if cfg.PodcastLen() == 0 {
			log.Warn("No podcasts added yet")
			return
		}
		for n, podcast := range cfg.GetAllPodcasts() {
			var lastUpdated string
			isDisabledStr := ""
			if podcast.Disabled {
				isDisabledStr = color.YellowString("[disabled]")
			}

			if podcast.LastSynced.IsZero() {
				lastUpdated = color.CyanString("Never")
			} else {
				lastUpdated = fmt.Sprintf("%s [%d days ago]", podcast.LastSynced.Format("2006-01-02 15:4"),
					int(time.Now().Sub(podcast.LastSynced)/(24*time.Hour)))
			}
			num := color.MagentaString("[" + strconv.Itoa(n+1) + "] ")
			log.Printf("%s %s %s", num, podcast.Name, isDisabledStr)
			log.Printf("\t* Url             : %s", podcast.Url)
			log.Printf("\t* Last synced     : %s", lastUpdated)
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

		if podcastName == "" {
			var err error
			podcastName, err = getRssName(url)
			if err != nil {
				log.Fatalf("Failed to get podacast name from url: %s, Error: %v", url, err)
				return
			}
		}

		if err := cfg.AddPodcast(podcastName, url); err != nil {
			if err == ErrPodacastAlreadyExist {
				log.Warnf("Podcast <%s> exists already", podcastName)
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
		p, err := cfg.GetPodcastByName(nameOrId)
		if err != nil {
			var n int
			n, err = strconv.Atoi(nameOrId)
			if err == nil {
				p, err = cfg.GetPodcastByIndex(n - 1)
			}
		}

		if err == nil {
			cfg.RemovePodcast(p.Name)
			log.Printf("* [%s] removed", nameOrId)
		} else if err == ErrPodcastWasNotFound {
			log.Warnf("Name or ID <%s> was not found in store. do nothing", nameOrId)
		} else {
			log.Fatal(err)
		}
	}

	return cmd
}

// 'reset' - command
func cmdReset() cli.Command {
	cmd := cli.Command{}
	cmd.Name = "reset"
	cmd.Usage = "reset time and count for podcasts"
	cmd.Action = func(c *cli.Context) {
		if err := cfg.ResetAll(); err != nil {
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
// TODO: add to sync only one podcast
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

		log.Infof("Started at %s", time.Now())
		if err = syncPodcasts(date, podcastCount); err != nil {
			log.Fatal(err)
		}
		log.Infof("Finished at %s", time.Now())
	}

	return cmd
}
