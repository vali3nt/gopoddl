package main

import (
	"github.com/robfig/config"
	"strings"
)

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

func getCfgStringNoErr(section string, option string) string {
	val, _ := cfg.String(section, option)
	return strings.Trim(val, "\"")
}
