package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	rss "github.com/jteeuwen/go-pkg-rss"
)

var (
	ErrAlreadyExistInStore = errors.New("Podcast exists in store already")
	ErrWasNotFoundInStore  = errors.New("Podcast does not exist in store")
)

type PodcastFilter struct {
	MediaType    []string
	Count        int
	StartDate    time.Time
	Filter       string
	DateFormat   string
	SeperatePath string
}

type PodcastItem struct {
	Title      string // will used for log output
	Dir        string // seperate dir, can be empty
	Filename   string // file name from url
	Url        string // url to downaload
	Size       int64
	ItemTitile string
}

type Podcast struct {
	Name            string    // Name of podacst
	Url             string    // Url to rss xml file
	LastSynced      time.Time // Last synced time
	DownloadedFiles int       // TODO: add statistics
}

// reset statistic
func (p *Podcast) Reset() {
	var emptyDate time.Time
	p.LastSynced = emptyDate
	p.DownloadedFiles = 0
}

type PodcastStore struct {
	filePath string `json:"-"`
	Podcasts []Podcast
}

// Load store from file
func LoadStore(filePath string) (*PodcastStore, error) {
	p := &PodcastStore{
		filePath: filePath,
		Podcasts: []Podcast{},
	}

	file, err := os.Open(p.filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	rd := bufio.NewReader(file)

	dec := json.NewDecoder(rd)
	if err := dec.Decode(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Create file and write inital values to it
func InitStore(filePath string) error {
	fs, err := os.Create(filePath)
	defer fs.Close()
	if err != nil {
		return err
	}
	fs.WriteString("{}")
	return nil
}

// Save data to config file
func (c *PodcastStore) Save() (err error) {
	var file *os.File
	if fileExists(c.filePath) {
		if err := os.Remove(c.filePath); err != nil {
			return err
		}
	}
	file, err = os.Create(c.filePath)
	if err != nil {
		return nil
	}

	defer func() {
		err = file.Close()
	}()

	wr := bufio.NewWriter(file)

	defer func() {
		err = wr.Flush()
	}()

	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	wr.Write(data)
	log.Debug("Podcast store saved")
	return nil
}

// Find podcast from user input, search by name ,
// if not found, will try to search by index (nameOrId-1)
// if not found returns -1
func (c *PodcastStore) FindByNameOrNum(nameOrId string) int {
	// find by name
	for n := range c.Podcasts {
		if nameOrId == c.Podcasts[n].Name {
			return n
		}
	}
	// find by id
	n, err := strconv.Atoi(nameOrId)
	if err == nil && n > 0 && len(c.Podcasts) >= n {
		return n - 1
	}

	return -1
}

// reset time and count on all podcasts
func (c *PodcastStore) ResetAll() error {
	for n := range c.Podcasts {
		c.Podcasts[n].Reset()
	}
	return c.Save()
}

// Add new podacst to store, validate on name uniq
func (c *PodcastStore) Add(Url, Name string) error {
	if Name == "" {
		// have to load title from podcast
		feed := rss.New(1, true, nil, nil)
		if err := feed.Fetch(Url, nil); err != nil {
			return err
		}
		Name = feed.Channels[0].Title
	}

	if n := c.FindByNameOrNum(Name); n != -1 {
		return ErrAlreadyExistInStore
	}

	c.Podcasts = append(c.Podcasts, Podcast{
		Name: Name,
		Url:  Url,
	})

	return c.Save()
}

// Remove podcast from store
func (c *PodcastStore) Remove(nameOrId string) error {
	// search for name
	if n := c.FindByNameOrNum(nameOrId); n != -1 {
		// remove
		c.Podcasts = append(c.Podcasts[:n], c.Podcasts[n+1:]...)
	} else {
		return ErrWasNotFoundInStore
	}
	return c.Save()
}

// clean up file name
func (p *PodcastStore) buildFileName(uri string) string {
	filename := uri[strings.LastIndex(uri, "/"):len(uri)]
	escaped, _ := url.QueryUnescape(filename)
	return escaped

}

// return list podacsts filtered by data in 'filter'
func (p *PodcastStore) Filter(podcast Podcast, filter *PodcastFilter) ([]PodcastItem, error) {
	var itemsToDownload []PodcastItem
	feed := rss.New(1, true, nil, nil)
	if err := feed.Fetch(podcast.Url, nil); err != nil {
		return itemsToDownload, err
	}
	if len(feed.Channels) == 0 {
		log.Debugf("No channels in %s", podcast.Name)
		return itemsToDownload, nil
	}

	// No need to filter
	if filter == nil {
		log.Debug("no filter,return all")
		for n := range feed.Channels[0].Items {
			for k := range feed.Channels[0].Items[n].Enclosures {
				// TODO: make separate function
				url := feed.Channels[0].Items[n].Enclosures[k].Url
				filename := url[strings.LastIndex(url, "/"):len(url)]
				itemsToDownload = append(itemsToDownload, PodcastItem{
					Filename:   filename,
					Url:        url,
					Title:      feed.Channels[0].Items[n].Title,
					Size:       feed.Channels[0].Items[n].Enclosures[k].Length,
					ItemTitile: feed.Channels[0].Items[n].Title,
				})
			}
		}
		return itemsToDownload, nil
	}

	// filter by date
	for n := range feed.Channels[0].Items {
		itemDate, _ := feed.Channels[0].Items[n].ParsedPubDate()
		if !filter.StartDate.IsZero() {
			if itemDate.Before(filter.StartDate) {
				log.Debug("filter:skipped by StartDate:", feed.Channels[0].Items[n].Title)
				continue
			}
		} else if itemDate.Before(podcast.LastSynced) {
			log.Debug("filter:skipped by LastSynced:", feed.Channels[0].Items[n].Title)
			continue
		}

	E:
		for k := range feed.Channels[0].Items[n].Enclosures {
			// filter by mediatype
			if len(filter.MediaType) > 0 {
				toSkip := true
				for i := range filter.MediaType {
					if strings.HasPrefix(feed.Channels[0].Items[n].Enclosures[k].Type, strings.TrimSpace(filter.MediaType[i])) {
						toSkip = false
					}
				}
				if toSkip {
					log.Debug("filter:skipped by MediaType:", feed.Channels[0].Items[n].Title)
					continue E
				}
			}

			// filter by condition
			// {{ItemTitle}}, {{ItemUrl}}, {{ItemDescription}}
			if filter.Filter != "" {
				data := map[string]string{
					"ItemTitle":       feed.Channels[0].Items[n].Title,
					"ItemDescription": feed.Channels[0].Items[n].Description,
					"ItemUrl":         feed.Channels[0].Items[n].Enclosures[k].Url,
				}
				if ok, err := EvalFilter(filter.Filter, data); err != nil {
					return itemsToDownload, err
				} else {
					if !ok {
						log.Debug("filter:skipped by filter condition:", feed.Channels[0].Items[n].Title)
						continue E
					}
				}
			}

			// add dir
			// {{Title}}, {{Name}}, {{ItemPubDate}}, {{ItemTitle}}, {{CurrentDate}}
			sepPath := ""
			if filter.SeperatePath != "" {
				d, _ := feed.Channels[0].Items[n].ParsedPubDate()
				data := map[string]string{
					"Title":       feed.Channels[0].Title,
					"Name":        podcast.Name,
					"ItemTitle":   feed.Channels[0].Items[n].Title,
					"CurrentDate": time.Now().Format(filter.DateFormat),
					"ItemPubDate": d.Format(filter.DateFormat),
				}
				sepPath = EvalFormat(filter.SeperatePath, data)
			}
			// add url to list
			log.Debug("filter:added to download list:", feed.Channels[0].Items[n].Title)
			itemsToDownload = append(itemsToDownload, PodcastItem{
				Filename:   p.buildFileName(feed.Channels[0].Items[n].Enclosures[k].Url),
				Dir:        sepPath,
				Url:        feed.Channels[0].Items[n].Enclosures[k].Url,
				Title:      feed.Channels[0].Title,
				Size:       feed.Channels[0].Items[n].Enclosures[k].Length,
				ItemTitile: feed.Channels[0].Items[n].Title,
			})
		}

	}

	// filter by count
	count := len(itemsToDownload)
	// by count
	if filter.Count != -1 && count > filter.Count {
		count = filter.Count
	}

	return itemsToDownload[0:count], nil
}
