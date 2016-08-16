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

func syncPodcasts(startDate time.Time, nameOrID string, count int, chekMode bool) error {
	allReqs := [][]*grab.Request{}
	podcasts := []*Podcast{}

	if nameOrID == "" {
		podcasts = cfg.GetAllPodcasts()
	} else {
		p, err := cfg.GetPodcastByNameOrID(nameOrID)
		if err != nil {
			return err
		}
		podcasts = append(podcasts, p)
	}

	for n, podcast := range podcasts {

		var podcastList []*DownloadItem

		filter := MakeFilter(podcast)
		filter.Count = count
		filter.StartDate = startDate

		// download rss
		feed, err := getRss(podcast)
		if err != nil {
			printPodcastInfo(podcast, podcastList, n+1, err)
			continue
		}

		if len(feed.Channels) == 0 {
			log.Warnf(fmt.Sprintf("No channels in %s", podcast.Name))
			continue
		}

		// filter
		podcastList, err = filter.FilterItems(feed.Channels[0])
		if err != nil {
			printPodcastInfo(podcast, podcastList, n+1, err)
			continue
		}

		if chekMode {
			printPodcastInfo(podcast, podcastList, n+1, err)
			continue
		}

		// check for emptiness
		if len(podcastList) == 0 {
			log.Printf("%s : %s, %d files", color.CyanString("EMPTY"), podcast.Name, len(podcastList))
			continue
		}

		// create download requests
		allReqs = append(allReqs, createRequests(podcast, podcastList))

	}

	if !chekMode {
		startDownload(allReqs)

		for _, podcast := range podcasts {
			// FIXME: put right date according to rss or Item PubDate
			podcast.LastSynced = time.Now()
			if err := cfg.UpdatePodcast(podcast); err != nil {
				return err
			}
		}
	}

	return nil
}

func printPodcastInfo(podcast *Podcast, podcastList []*DownloadItem, index int, err error) {

	status := ""
	num := color.MagentaString("[" + strconv.Itoa(index) + "] ")
	if err != nil {
		status = color.RedString("FAIL")
	} else {
		color.GreenString("OK")
	}

	log.Printf("%s %s", num, podcast.Name)
	log.Printf("\t* Url             : %s %s", podcast.Url, status)
	if err != nil {
		log.Warnf("Error: %s", err)
	} else {
		log.Printf("\t* Awaiting files  : %d", len(podcastList))
		for k, podcast := range podcastList {
			log.Printf("\t\t* [%d] : %s", k, podcast.ItemTitle)
		}
	}
}

func createRequests(podcast *Podcast, podcastList []*DownloadItem) []*grab.Request {
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
	return reqs
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
		// close channels
		close(statusQueue)
		close(doneQueue)
	}()

	totalFiles := 0
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

				// ensure files downloaded one by one, so wait complition
				for !resp.IsComplete() {
					time.Sleep(500 * time.Microsecond)
				}
			}

		}(podcastReq)
	}
	checkDownloadProgress(statusQueue, totalFiles)
	log.Infof("%d files downloaded.\n", totalFiles)
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

func bytesToMb(bytesCount uint64) float64 {
	return float64(bytesCount) / float64(1024*1024)
}

func showProgressError(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui.Bypass(), "Error downloading %s: %v\n",
		status.Response.Request.URL(),
		status.Response.Error)
}

func showProgressDone(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui.Bypass(),
		"Finished %s [%d/%d] %0.2f / %0.2f Mb (%d%%)\n",
		status.Response.Filename,
		status.Current, status.Total,
		bytesToMb(status.Response.BytesTransferred()),
		bytesToMb(status.Response.Size),
		int(100*status.Response.Progress()))
}

func showProgressProc(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui, "Downloading %s [%d/%d] %0.2f / %0.2f Mb (%d%%)\n",
		status.Response.Filename,
		status.Current, status.Total,
		bytesToMb(status.Response.BytesTransferred()),
		bytesToMb(status.Response.Size),
		int(100*status.Response.Progress()))

}
