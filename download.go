package main

// github.com/cheggaaa/pb

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/cavaliercoder/grab"
	"github.com/fatih/color"
	"github.com/gosuri/uilive"
	rss "github.com/mmcdole/gofeed"
)

func getFeed(url string) (*rss.Feed, error) {
	return rss.NewParser().ParseURL(url)
}

func syncPodcasts(startDate time.Time, nameOrID string, count int, retryAttempts uint, chekMode bool) error {
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
		feed, err := getFeed(podcast.URL)
		if err != nil {
			printPodcastInfo(podcast, podcastList, n+1, err)
			continue
		}

		// filter
		podcastList, err = filter.FilterItems(feed)
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
		failedReqs := startDownload(allReqs)
		downloadWithRetry(failedReqs, retryAttempts)

		for _, podcast := range podcasts {
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
	log.Printf("\t* Url             : %s %s", podcast.URL, status)
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
		entryDir := filepath.Join(podcast.DownloadPath, entry.Dir)
		entryPath := filepath.Join(entryDir, entry.Filename)
		req, err := grab.NewRequest(entryPath, entry.URL)
		req.Size = entry.Size
		if err != nil {
			log.Errorf("NewRequest failed with %s\n", err)
			continue
		}
		req.Size = entry.Size
		reqs = append(reqs, req)
	}
	return reqs
}

func startDownload(downloadReqs [][]*grab.Request) []*grab.Request {
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
				resp := client.Do(req)

				// send results to monitoring channel
				statusQueue <- &downloadStatus{
					Total:    podcastTotal,
					Current:  curPosition,
					Response: resp,
					Request:  req,
				}

				// ensure files downloaded one by one, so wait complition
				for !resp.IsComplete() {
					time.Sleep(500 * time.Microsecond)
				}
			}

		}(podcastReq)
	}
	successCount, failedRequests := checkDownloadProgress(statusQueue, totalFiles)
	log.Infof("%d total file (%d success, %d failed).\n",
		totalFiles,
		successCount,
		len(failedRequests))
	return failedRequests
}

type downloadStatus struct {
	Total    int // total requests count
	Current  int // current position
	Response *grab.Response
	Request  *grab.Request
}

func checkDownloadProgress(respch <-chan *downloadStatus, reqCount int) (int, []*grab.Request) {
	timer := time.NewTicker(200 * time.Millisecond)
	ui := uilive.New()

	completed := 0
	successCount := 0
	responses := make([]*downloadStatus, 0)
	failedRequests := make([]*grab.Request, 0)

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

					if resp.Response.Err() != nil {
						showProgressError(ui, resp)
						failedRequests = append(failedRequests, resp.Request)
					} else {
						showProgressDone(ui, resp)
						successCount++
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
	return successCount, failedRequests
}

func downloadWithRetry(reqs []*grab.Request, retryAttempts uint) {
	retryReqs := [][]*grab.Request{
		reqs,
	}
	retry.Do(
		func() error {
			resp := startDownload(retryReqs)
			if len(resp) > 0 {
				return errors.New("Some podcasts download finished with error")
			}
			return nil
		},
		retry.Attempts(retryAttempts),
		retry.OnRetry(func(n uint, err error) {
			fmt.Printf("Retry #%d: %s\n", n, err)
		}),
	)
}

func toProgress(completed int64, size int64) int64 {
	if size == 0 {
		return 0
	}
	return int64(float64(completed) / float64(size) * 100)
}

func bytesToMb(bytesCount int64) float64 {
	return math.Abs(float64(bytesCount)) / float64(1024*1024)
}

func showProgressError(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui.Bypass(), "Error downloading %s: %v\n",
		status.Response.Request.URL(),
		status.Response.Err())
}

func showProgressDone(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui.Bypass(),
		"Finished %s [%d/%d] %0.2f / %0.2f Mb (%d%%)\n",
		status.Response.Filename,
		status.Current, status.Total,
		bytesToMb(status.Response.BytesComplete()),
		bytesToMb(status.Request.Size),
		toProgress(status.Response.BytesComplete(), status.Request.Size),
	)
}

func showProgressProc(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui, "Downloading %s [%d/%d] %0.2f / %0.2f Mb (%d%%)\n",
		status.Response.Filename,
		status.Current, status.Total,
		bytesToMb(status.Response.BytesComplete()),
		bytesToMb(status.Request.Size),
		toProgress(status.Response.BytesComplete(), status.Request.Size),
	)
}
