package main

import (
	"net/url"
	"strings"
	"time"

	rss "github.com/jteeuwen/go-pkg-rss"
)

type Filter struct {
	PodcastName  string
	MediaType    string
	Count        int
	StartDate    time.Time
	Filter       string
	DateFormat   string
	SeperatePath string
	LastSynced   time.Time
}

type DownloadItem struct {
	Title     string // will used for log output
	Dir       string // seperate dir, can be empty
	Filename  string // file name from url
	Url       string // url to downaload
	Size      int64
	ItemTitle string
}

func (f *Filter) FilterItems(rssChannel *rss.Channel) ([]*DownloadItem, error) {
	itemsToDownload := []*DownloadItem{}
	items := rssChannel.Items
	// filter by date
	for _, item := range items {
		itemDate, _ := item.ParsedPubDate()
		if !f.StartDate.IsZero() {
			if itemDate.Before(f.StartDate) {
				log.Debug("filter:skipped by StartDate:", item.Title)
				continue
			}
		} else if itemDate.Before(f.LastSynced) {
			log.Debug("filter:skipped by LastSynced:", item.Title)
			continue
		}

	E:
		for _, enclosure := range item.Enclosures {
			// filter by mediatype
			if len(f.MediaType) > 0 {
				toSkip := true
				if strings.HasPrefix(enclosure.Type, strings.TrimSpace(f.MediaType)) {
					toSkip = false
				}
				if toSkip {
					log.Debug("filter:skipped by MediaType:", item.Title)
					continue E
				}
			}

			// filter by condition
			// {{ItemTitle}}, {{ItemUrl}}, {{ItemDescription}}
			if f.Filter != "" {
				data := map[string]string{
					"ItemTitle":       item.Title,
					"ItemDescription": item.Description,
					"ItemUrl":         enclosure.Url,
				}
				if ok, err := EvalFilter(f.Filter, data); err != nil {
					return itemsToDownload, err
				} else {
					if !ok {
						log.Debug("filter:skipped by filter condition:", item.Title)
						continue E
					}
				}
			}

			// add dir
			// {{Title}}, {{Name}}, {{ItemPubDate}}, {{ItemTitle}}, {{CurrentDate}}
			sepPath := ""
			if f.SeperatePath != "" {
				d, _ := item.ParsedPubDate()
				data := map[string]string{
					"Title":       rssChannel.Title,
					"Name":        f.PodcastName,
					"ItemTitle":   item.Title,
					"CurrentDate": time.Now().Format(f.DateFormat),
					"ItemPubDate": d.Format(f.DateFormat),
				}
				sepPath = EvalFormat(f.SeperatePath, data)
			}

			// add url to list
			log.Debug("filter:added to download list:", item.Title)
			itemsToDownload = append(itemsToDownload,
				&DownloadItem{
					Filename:  buildFileName(enclosure.Url),
					Dir:       sepPath,
					Url:       enclosure.Url,
					Title:     rssChannel.Title,
					Size:      enclosure.Length,
					ItemTitle: item.Title,
				})
		}

	}

	// filter by count
	count := len(itemsToDownload)
	// by count
	if f.Count != -1 && count > f.Count {
		count = f.Count
	}

	return itemsToDownload[0:count], nil
}

func MakeFilter(podcast *Podcast) *Filter {
	return &Filter{
		PodcastName:  podcast.Name,
		MediaType:    podcast.Mtype,
		Filter:       podcast.Filter,
		DateFormat:   podcast.DateFormat,
		SeperatePath: podcast.SeparateDir,
		LastSynced:   podcast.LastSynced,
	}
}

// clean up file name
func buildFileName(uri string) string {
	filename := uri[strings.LastIndex(uri, "/"):len(uri)]
	escaped, _ := url.QueryUnescape(filename)
	return escaped
}
