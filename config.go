package main

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-ini/ini"
)

const (
	defaultComment = `# gopoddl - settings each setting can be overridden per podcast section ( header : Podcast name)
#
# Available tokens:
#    {{Title}}           Podcast title
#    {{Name}}            Podcast name in the gopoddl
#    {{PubDate}}         Podcast publish date
#    {{ItemTitle}}       Podcast item title
#    {{ItemUrl}}         Podcast Item url
#    {{ItemDescription}} Podcast Item description
#    {{ItemPubDate}}     Podcast item publish date
#    {{ItemFileName}}    Podcast item filename from url
#    {{CurrentDate}}     now date
#
# Available settings:
#    download-path       path to store where downloaded files
#                            path sep is '/' , on win path will be adjusted
#                            [required]
#    separate-dir        save podcast items in seprate dir , following tokens can be used:
#                            {{Title}}, {{Name}}, {{ItemPubDate}}, {{ItemTitle}}, {{CurrentDate}}
#                            path sep is '/' , on win path will be adjusted
#    disable             disable podcast
#    date-format         tokens date format
#                            Format : 20060102, 2006 - year, 01 - month, 02 - day
#                            Details in 'const' https://golang.org/src/pkg/time/format.go
#    mtype               mediatypes to download audio,video,...
#    filter              filter for podcasts
#                        if condition matched, podcast item will be downloaded
#                        following tokens can be used:
#                            {{ItemTitle}}, {{ItemUrl}}, {{ItemDescription}}
#                        Format:
#                            <string> [not] in [suffix|prefix] <VAR> [and|or] ....
#                        Example:
#                            "'Day' not in {{ItemDescription}} or 'Day' not in {{ItemTitle}}"
#                            all podcast with 'Day' in title or in descripion will be ignored
#                        Keywords:
#                            not, in, prefix, suffix or, and , (), ', "
#                            in     - search like '%string%'
#                            prefix - search like '%string'
#                            suffix - search like 'string%'
#
`
)

var (
	ErrPodacastAlreadyExist = errors.New("Podcast exists in store already")
	ErrPodcastWasNotFound   = errors.New("Podcast does not exist in store")
	//errIntenalNowAllowd     = errors.New("not allowed to work with DEFAULT section")
)

// Podcast - mandatory settings, set per podcast
type Podcast struct {
	Name            string    `ini:"-"`
	URL             string    `ini:"url"`
	LastSynced      time.Time `ini:"last-synced"`
	PodcastSettings `ini:"Podcast"`
}

// PodcastSettings - global settings, can be customized per podcast
type PodcastSettings struct {
	DownloadPath string `ini:"download-path"`
	SeparateDir  string `ini:"separate-dir"`
	Disabled     bool   `ini:"disabled"`
	DateFormat   string `ini:"date-format"`
	Filter       string `ini:"filter"`
	Mtype        string `ini:"mtype"`
}

// CreateDefaultConfig creates inital configurtion and save it to file
func CreateDefaultConfig(filePath string) error {
	cfg := ini.Empty()
	defaultSection := cfg.Section("")
	defaultSection.Comment = defaultComment

	defaultSettings := new(PodcastSettings)
	defaultSettings.DownloadPath = expandPath("~/")
	defaultSettings.Disabled = false
	defaultSettings.SeparateDir = ""
	defaultSettings.DateFormat = "20060102"
	defaultSettings.Mtype = "audio"
	defaultSettings.Filter = ""
	if err := defaultSection.ReflectFrom(defaultSettings); err != nil {
		return err
	}
	return cfg.SaveTo(filePath)
}

type Config struct {
	configPath string
	cfg        *ini.File
}

// NewConfig creates config object from file
func NewConfig(configPath string) (*Config, error) {
	var err error
	c := new(Config)
	c.configPath = configPath

	c.cfg, err = ini.InsensitiveLoad(configPath)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// UpdatePodcast updates last-synced for podacast to config file and saves it disk
func (c *Config) UpdatePodcast(podcast *Podcast) error {
	c.cfg.Section(podcast.Name).Key("last-synced").SetValue(podcast.LastSynced.Format(time.RFC3339))
	return c.cfg.SaveTo(c.configPath)
}

// AddPodcast - adds new podcast to config and saves it disk
func (c *Config) AddPodcast(name, url string) error {
	var emptyDate time.Time
	_, err := c.cfg.GetSection(name)
	if err == nil {
		return ErrPodacastAlreadyExist
	}
	c.cfg.Section(name).Key("url").SetValue(url)
	c.cfg.Section(name).Key("last-synced").SetValue(emptyDate.Format(time.RFC3339))
	return c.cfg.SaveTo(c.configPath)
}

// RemovePodcast  removes podcast from config and saves it disk
func (c *Config) RemovePodcast(name string) error {
	_, err := c.cfg.GetSection(name)
	if err != nil {
		return ErrPodcastWasNotFound
	}

	c.cfg.DeleteSection(name)
	return c.cfg.SaveTo(c.configPath)
}

// ResetAll reset LastSynced to nil for all podcasts
func (c *Config) ResetAll() error {
	var emptyTime time.Time
	for _, podcast := range c.GetAllPodcasts() {
		podcast.LastSynced = emptyTime
		c.UpdatePodcast(podcast)
	}
	return c.cfg.SaveTo(c.configPath)
}

// PodcastLen returns podcasts count
func (c *Config) PodcastLen() int {
	// deduct defult section
	return len(c.cfg.SectionStrings()) - 1
}

// GetPodcastByName retuns podcast settings by name
func (c *Config) GetPodcastByName(name string) (*Podcast, error) {
	// load default section
	pDefault := new(PodcastSettings)
	if err := c.cfg.Section(ini.DEFAULT_SECTION).MapTo(pDefault); err != nil {
		return nil, err
	}

	section, err := c.cfg.GetSection(name)
	if err != nil {
		return nil, ErrPodcastWasNotFound
	}

	if err := section.MapTo(pDefault); err != nil {
		return nil, err
	}
	podcast := &Podcast{PodcastSettings: *pDefault, Name: name}
	if err := section.MapTo(podcast); err != nil {
		return nil, err
	}

	return podcast, nil
}

// GetPodcastByNameOrID retuns podcast settings by name or index
func (c *Config) GetPodcastByNameOrID(nameOrID string) (*Podcast, error) {
	p, err := c.GetPodcastByName(nameOrID)
	if err != nil {
		var n int
		n, err = strconv.Atoi(nameOrID)
		if err == nil {
			p, err = c.GetPodcastByIndex(n - 1)
		}
	}
	if err == nil {
		return p, nil
	}
	return nil, err
}

// GetPodcastByIndex retuns podcast settings by index
func (c *Config) GetPodcastByIndex(index int) (*Podcast, error) {
	if index > c.PodcastLen() || index < 0 {
		return nil, fmt.Errorf(fmt.Sprintf("cfg: Invalid input : %v", index))
	}
	sectionName := c.cfg.SectionStrings()[index+1]
	return c.GetPodcastByName(sectionName)
}

// GetAllPodcasts retuns all podcasts stored in config
func (c *Config) GetAllPodcasts() []*Podcast {
	podcasts := []*Podcast{}
	for _, sectionName := range c.cfg.SectionStrings() {
		if sectionName == ini.DEFAULT_SECTION || sectionName == "default" {
			continue
		}
		p, err := c.GetPodcastByName(sectionName)
		if err != nil {
			log.Fatalf("cfg: Filed to get podcast %s, Error: %v", sectionName, err)
			continue
		}
		podcasts = append(podcasts, p)
	}
	return podcasts
}
