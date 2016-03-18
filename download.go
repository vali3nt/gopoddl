package main

// github.com/cheggaaa/pb

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/fatih/color"
	"github.com/gosuri/uilive"
	rss "github.com/jteeuwen/go-pkg-rss"
)

func getRss(podcast *Podcast) (*rss.Feed, error) {

	feed := rss.New(1, true, nil, nil)
	if err := feed.Fetch(podcast.Url, nil); err != nil {
		return nil, err
	}
	return feed, nil
}

func getRssName(url string) (string, error) {
	feed := rss.New(1, true, nil, nil)
	if err := feed.Fetch(url, nil); err != nil {
		return "", err
	}
	return feed.Channels[0].Title, nil
}

func checkPodcasts() {
	var date time.Time
	// parse input date
	for n, podcast := range cfg.GetAllPodcasts() {

		filter := MakeFilter(podcast)
		filter.Count = -1
		filter.StartDate = date

		status := color.GreenString("OK")
		num := color.MagentaString("[" + strconv.Itoa(n+1) + "] ")
		var podcastList []*DownloadItem

		// Download rss

		feed, err := getRss(podcast)
		if err == nil {
			if len(feed.Channels) == 0 {
				log.Warnf(fmt.Sprintf("No channels in %s", podcast.Name))
				continue
			}
			podcastList, err = filter.FilterItems(feed.Channels[0])
		}
		// colorize error message
		if err != nil {
			status = color.RedString("FAIL")
		}

		log.Printf("%s %s", num, podcast.Name)
		log.Printf("\t* Url             : %s %s", podcast.Url, status)
		if err != nil {
			log.Debugf("Error: %s", err)
		} else {
			log.Printf("\t* Awaiting files  : %d", len(podcastList))
			for k := range podcastList {
				log.Printf("\t\t* [%d] : %s", k, podcastList[k].ItemTitle)
			}
		}
	}
}

func syncPodcasts(startDate time.Time, count int) error {
	allReqs := [][]*grab.Request{}
	for _, podcast := range cfg.GetAllPodcasts() {

		filter := MakeFilter(podcast)
		filter.Count = count
		filter.StartDate = startDate

		// download rss
		feed, err := getRss(podcast)
		if err != nil {
			log.Fatalf("Error %s: %v", podcast.Name, err)
			continue
		}

		if len(feed.Channels) == 0 {
			log.Warnf(fmt.Sprintf("No channels in %s", podcast.Name))
			continue
		}

		// filter
		var podcastList []*DownloadItem
		podcastList, err = filter.FilterItems(feed.Channels[0])
		if err != nil {
			log.Fatalf("Error %s: %v", podcast.Name, err)
			continue
		}

		// check for emptiness
		if len(podcastList) == 0 {
			log.Printf("%s : %s, %d files", color.CyanString("EMPTY"), podcast.Name, len(podcastList))
			continue
		}

		// create download requests
		reqs := []*grab.Request{}
		for _, entry := range podcastList {
			// create dir for each entry, path is set in filter
			// according to rules in configuration

			entryDownloadPath := filepath.Join(podcast.DownloadPath, entry.Dir)
			if !fileExists(entryDownloadPath) {
				if err := os.MkdirAll(entryDownloadPath, 0777); err != nil {
					log.Fatal(err)
					continue
				}
			}

			req, _ := grab.NewRequest(entry.Url)
			req.Filename = filepath.Join(entryDownloadPath, entry.Filename)
			req.Size = uint64(entry.Size)
			req.RemoveOnError = true
			reqs = append(reqs, req)
		}

		allReqs = append(allReqs, reqs)

	}

	startDownload(allReqs)

	for _, podcast := range cfg.GetAllPodcasts() {
		podcast.LastSynced = time.Now()
		if err := cfg.UpdatePodcast(podcast); err != nil {
			return err
		}
	}

	return nil
}

func startDownload(downloadReqs [][]*grab.Request) {
	requestCount := len(downloadReqs)
	statusQueue := make(chan *downloadStatus, requestCount)
	doneQueue := make(chan bool, requestCount)

	client := grab.NewClient()

	go func() {
		// wait while all requests will be in queue
		for i := 0; i < requestCount; i++ {
			<-doneQueue
		}
		// clise channels
		close(statusQueue)
		close(doneQueue)
	}()

	totalFiles := 0
	// FIXME : hangs if files exists
	for _, podcastReq := range downloadReqs {
		totalFiles += len(podcastReq)

		go func(requests []*grab.Request) {
			curPosition := 0
			podcastTotal := len(requests)
			for _, req := range requests {

				// increas position, used for printing
				curPosition++

				// start downloading
				resp := <-client.DoAsync(req)

				// send results to monitoring channel
				statusQueue <- &downloadStatus{
					Total:    podcastTotal,
					Current:  curPosition,
					Response: resp,
				}

			}
			doneQueue <- true
		}(podcastReq)
	}
	checkDownloadProgress(statusQueue, totalFiles)
	log.Infof("%d files successfully downloaded.\n", totalFiles)
}

type downloadStatus struct {
	Total    int // total requests count
	Current  int // current position
	Response *grab.Response
}

func checkDownloadProgress(respch <-chan *downloadStatus, reqCount int) {
	timer := time.NewTicker(200 * time.Millisecond)
	ui := uilive.New()

	completed := 0
	responses := make([]*downloadStatus, 0)

	ui.Start()
	for completed < reqCount {
		select {
		case resp := <-respch:
			if resp != nil {
				responses = append(responses, resp)
			}

		case <-timer.C:
			// print completed requests
			for i, resp := range responses {
				if resp != nil && resp.Response.IsComplete() {

					if resp.Response.Error != nil {
						showProgressError(ui, resp)
					} else {
						showProgressDone(ui, resp)
					}

					responses[i] = nil
					completed++
				}
			}

			// print in progress requests
			for _, resp := range responses {
				if resp != nil {
					showProgressProc(ui, resp)
				}
			}
		}
	}

	timer.Stop()
	ui.Stop()
}

func showProgressError(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui.Bypass(), "Error downloading %s: %v\n",
		status.Response.Request.URL(),
		status.Response.Error)
}

func showProgressDone(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui.Bypass(),
		"Finished %s [%d/%d] %d / %d bytes (%d%%)\n",
		status.Response.Filename,
		status.Current, status.Total,
		status.Response.BytesTransferred(),
		status.Response.Size,
		int(100*status.Response.Progress()))
}

func showProgressProc(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui, "Downloading %s [%d/%d] %d / %d bytes (%d%%)\n",
		status.Response.Filename,
		status.Current, status.Total,
		status.Response.BytesTransferred(),
		status.Response.Size,
		int(100*status.Response.Progress()))

}
