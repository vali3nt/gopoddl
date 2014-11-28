package main

import (
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/robfig/config"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	progName = "gopoddl"
	store    *PodcastStore
	cfg      *config.Config
	log      *Logger
)

/////////////////////////////////////////////////////////////////////
/// Config
/////////////////////////////////////////////////////////////////////

func InitConf(filePath string) error {
	homePath := expandPath("~/")
	header := `gopoddl - settings
each setting can be overridden per podcast section ( header : Podcast name)

Available tokens:    
    {{Title}}           Podcast title
    {{Name}}            Podcast name in the gopoddl
    {{PubDate}}         Podcast publish date
    {{ItemTitle}}       Podcast item title
    {{ItemUrl}}         Podcast Item url
    {{ItemDescription}} Podcast Item description
    {{ItemPubDate}}     Podcast item publish date
    {{ItemFileName}}    Podcast item filename from url
    {{CurrentDate}}     now date

Available settings:
    download-path       path to store where downloaded files
    						path sep is '/' , on win path will be adjusted
    						[required]
    separate-dir        save podcast items in seprate dir , following tokens can be used:
                            {{Title}}, {{Name}}, {{ItemPubDate}}, {{ItemTitle}}, {{CurrentDate}}
                            path sep is '/' , on win path will be adjusted
    disable             disable podcast
    date-format         tokens date format
    						Format : 20060102, 2006 - year, 01 - month, 02 - day
    						Details in 'const' https://golang.org/src/pkg/time/format.go
    mtype               mediatypes to download audio,video,...
    filter              filter for podcasts
    					if condition matched, podcast item will be downloaded
                        following tokens can be used:
                            {{ItemTitle}}, {{ItemUrl}}, {{ItemDescription}}
                        Format:
                        	<string> [not] in [suffix|prefix] <VAR> [and|or] .... 
                        Example:
                            "'Day' not in {{ItemDescription}} or 'Day' not in {{ItemTitle}}"
                            all podcast with 'Day' in title or in descripion will be ignored
                        Keywords:
                            not, in, prefix, suffix or, and , (), ', "
                            in     - search like '%string%'
                            prefix - search like '%string'
                        	suffix - search like 'string%'

`
	c := config.NewDefault()
	c.AddOption("", "download-path", homePath)
	c.AddOption("", "separate-dir", "")
	c.AddOption("", "disable", "false")
	c.AddOption("", "date-format", "20060102")
	c.AddOption("", "mtype", "audio")
	c.AddOption("", "filter", "")
	return c.WriteFile(filePath, 0644, header)
}

func LoadConf(filePath string) (*config.Config, error) {
	return config.ReadDefault(filePath)
}

/////////////////////////////////////////////////////////////////////
/// Helpers
/////////////////////////////////////////////////////////////////////

func writeToFile(filePath, content string) error {
	fs, err := os.Create(filePath)
	defer fs.Close()
	if err != nil {
		return err
	}
	fs.WriteString(content)
	return nil
}

type downloadStatus struct {
	podcastIdx  int
	fileCounter int
}

type downloadSet struct {
	podcastList  []PodcastItem
	downloadPath string
	overwrite    bool
	idx          int
}

func downloadPodcastList(d downloadSet, pChan chan downloadStatus) {
	fileCounter := 0
	for k := range d.podcastList {
		pItem := d.podcastList[k]
		targetDir := filepath.Join(d.downloadPath, pItem.Dir)
		targetPath := filepath.Join(targetDir, pItem.Filename)
		// target dir
		if !fileExists(targetDir) {
			if err := os.MkdirAll(targetDir, 0777); err != nil {
				log.Error("Failed to create dir %s : %s", targetDir, err)
				continue
			}
		} else if !d.overwrite && fileExists(targetPath) {
			log.Printf("%-15s %s -> %s", log.Color("cyan", "EXISTS"),
				pItem.Title,
				targetPath)
			continue
		}
		log.Printf("Start download %s -> %s", pItem.Title, pItem.Url)

		t0 := time.Now() // download start time
		if err := downloadFile(d.podcastList[k], targetPath); err != nil {

			log.Printf("%-15s %s -> %s",
				log.Color("red", "FAILED"),
				pItem.Title,
				targetPath)
			log.Error(err)

			if fileExists(targetPath) {
				if err := os.Remove(targetPath); err != nil {
					log.Warnf("Failed to remove file %s after failure", targetPath)
				}
			}
			continue
		}

		size := float64(d.podcastList[k].Size) / 1024 / 1024
		speed := size / time.Now().Sub(t0).Seconds()

		log.Printf("%-15s %s -> %s [%.2fMb, %.2f Mb/s]",
			log.Color("green", "OK"),
			pItem.Title,
			targetPath,
			size, speed)

		fileCounter++
	}
	pChan <- downloadStatus{d.idx, fileCounter} // send finished podcasts id
}

func downloadFile(pItem PodcastItem, targetPath string) error {
	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()
	resp, err := http.Get(pItem.Url)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Response code : %d", resp.StatusCode))
	}
	defer resp.Body.Close()
	size, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	if size != pItem.Size {
		return errors.New(fmt.Sprintf("%s size is differ %d bytes <> %d bytes", pItem.Url, pItem.Size, size))
	}
	return nil
}

// expand ~ to user home directory (platform independent)
func expandPath(p string) string {
	expandedPath := os.ExpandEnv(p)
	if strings.HasPrefix(p, "~") {
		pos := strings.Index(expandedPath, "/")
		HomeDir := ""
		if runtime.GOOS == "windows" {
			HomeDir = os.Getenv("USERPROFILE")
		} else {
			HomeDir = os.Getenv("HOME")
		}
		p = filepath.Join(HomeDir, expandedPath[pos:len(expandedPath)])
	}
	p, _ = filepath.Abs(p)
	return p
}

func fileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}

func parseTime(formatted string) (time.Time, error) {
	var layouts = [...]string{
		"20060102",
		"2006/01/02",
		"2006/1/2",
		"2006-01-02",
		"2006-1-2",
	}
	var t time.Time
	var err error
	formatted = strings.TrimSpace(formatted)
	for _, layout := range layouts {
		t, err = time.Parse(layout, formatted)
		if !t.IsZero() {
			break
		}
	}
	return t, err
}

func getCfgStringNoErr(section string, option string) string {
	val, _ := cfg.String(section, option)
	return strings.Trim(val, "\"")
}

/////////////////////////////////////////////////////////////////////
/// Main
/////////////////////////////////////////////////////////////////////

func main() {

	app := cli.NewApp()
	app.Name = progName
	app.Version = "0.0.1"
	app.Usage = "Podcast downloader"
	app.Before = func(c *cli.Context) (err error) {

		// Logger
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
		// no need to laod conf/store for init
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
		{ /////////////////////////////////////////////////////////////////////
			Name:      "init",
			ShortName: "i",
			Usage:     "create default config files",
			Action: func(c *cli.Context) {
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

			},
		},
		{ /////////////////////////////////////////////////////////////////////
			Name:      "list",
			ShortName: "l",
			Usage:     "list all podcasts",
			Action: func(c *cli.Context) {
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
			},
		},
		{ /////////////////////////////////////////////////////////////////////
			Name:        "add",
			ShortName:   "a",
			Usage:       "add podcast to sync",
			Description: "Usage: add <URL> [NAME]",
			Action: func(c *cli.Context) {
				if len(c.Args()) < 1 {
					log.Warn("Invlid usage.")
					cli.ShowCommandHelp(c, "add")
					return
				}
				if err := store.Add(c.Args().Get(0), c.Args().Get(1)); err != nil {
					log.Error(err)
				}

			},
		},
		{ /////////////////////////////////////////////////////////////////////
			Name:        "remove",
			ShortName:   "r",
			Usage:       "remove podcast from sync",
			Description: "Usage: remove <NAME or ID>",
			Action: func(c *cli.Context) {
				if len(c.Args()) < 1 {
					log.Warn("Invlid usage.")
					cli.ShowCommandHelp(c, "remove")
					return
				}
				if err := store.Remove(c.Args().First()); err != nil {
					log.Error(err)
				}
			},
		},
		{ /////////////////////////////////////////////////////////////////////
			Name:  "reset",
			Usage: "reset time and count for podcasts",
			Action: func(c *cli.Context) {
				var emptyDate time.Time
				for k := range store.Podcasts {
					store.Podcasts[k].LastSynced = emptyDate
					store.Podcasts[k].DownloadedFiles = 0
				}
				store.Save()
			},
		},
		{ /////////////////////////////////////////////////////////////////////
			Name:        "check",
			ShortName:   "c",
			Usage:       "check podcasts for availability",
			Description: "Usage: check [NAME or ID]",
			Action: func(c *cli.Context) {

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
			},
		},
		{ /////////////////////////////////////////////////////////////////////
			Name:        "sync",
			ShortName:   "s",
			Usage:       "start downloading",
			Description: "Usage: sync [NAME or ID]",
			Flags: []cli.Flag{
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
			},
			Action: func(c *cli.Context) {
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

				podcastCount := 0
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
						Count:        c.Int("count"),
						MediaType:    mtype,
						Filter:       getCfgStringNoErr(p.Name, "filter"),
						DateFormat:   getCfgStringNoErr(p.Name, "date-format"),
						SeperatePath: getCfgStringNoErr(p.Name, "separate-dir"),
					}
					// Get list
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
					log.Printf("%s : %s, %d files", log.Color("green", "START"),
						store.Podcasts[n].Name, len(podcastList))
					if !fileExists(downloadPath) {
						if err := os.MkdirAll(downloadPath, 0777); err != nil {
							log.Error(err)
							continue
						}
					}
					podcastCount++ // podcast counter
					d := downloadSet{
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
							log.Printf("%s : %s",
								log.Color("green", "DONE"),
								store.Podcasts[status.podcastIdx].Name)
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
				log.Printf("Finished at %s", time.Now())
			},
		},
	}

	app.Run(os.Args)
}
