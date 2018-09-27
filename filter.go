package main

import (
	"net/url"
	"strings"
	"time"

	rss "github.com/mmcdole/gofeed"
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
	URL       string // url to downaload
	Size      int64
	ItemTitle string
}

// FilterItems filters items from podcast RSS, returns all passed DownloadItems
func (f *Filter) FilterItems(rssChannel *rss.Feed) ([]*DownloadItem, error) {
	itemsToDownload := []*DownloadItem{}
	items := rssChannel.Items
	// filter by date
	for _, item := range items {
		if !f.StartDate.IsZero() {
			if item.PublishedParsed.Before(f.StartDate) {
				log.Debug("filter:skipped by StartDate: ", item.Title)
				continue
			}
		} else if item.PublishedParsed.Before(f.LastSynced) {
			log.Debug("filter:skipped by LastSynced: ", item.Title)
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
					log.Debug("filter:skipped by MediaType: ", item.Title)
					continue E
				}
			}

			// filter by condition
			// {{ItemTitle}}, {{ItemUrl}}, {{ItemDescription}}
			if f.Filter != "" {
				data := map[string]string{
					"ItemTitle":       item.Title,
					"ItemDescription": item.Description,
					"ItemUrl":         enclosure.URL,
				}
				if ok, err := EvalFilter(f.Filter, data); err != nil {
					log.Fatal("filter:error:", err)
					continue E
				} else {
					if !ok {
						log.Debug("filter:skipped by filter condition: ", item.Title)
						continue E
					}
				}
			}

			// add dir
			// {{Title}}, {{Name}}, {{ItemPubDate}}, {{ItemTitle}}, {{CurrentDate}}
			sepPath := ""
			if f.SeperatePath != "" {
				data := map[string]string{
					"Title":       rssChannel.Title,
					"Name":        f.PodcastName,
					"ItemTitle":   item.Title,
					"CurrentDate": time.Now().Format(f.DateFormat),
					"ItemPubDate": item.PublishedParsed.Format(f.DateFormat),
				}
				sepPath = EvalFormat(f.SeperatePath, data)
			}

			// add url to list
			log.Debug("filter:added to download list:", item.Title)
			itemsToDownload = append(itemsToDownload,
				&DownloadItem{
					Filename:  buildFileName(enclosure.URL),
					Dir:       sepPath,
					URL:       enclosure.URL,
					Title:     rssChannel.Title,
					Size:      parseStrToInt(enclosure.Length),
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

// MakeFilter creates new filter from podcast settings
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

// strip query params like : fname?podcast => fname
func stripQueryString(inputUrl string) string {
    u, err := url.Parse(inputUrl)
    if err != nil {
        panic(err)
    }
    u.RawQuery = ""
    return u.String()
}

// clean up file name
func buildFileName(uri string) string {
	fname := uri[strings.LastIndex(uri, "/"):len(uri)] // cut fname from iru
	escaped, _ := url.QueryUnescape(stripQueryString(fname)) // clean up uri
	return escaped
}
