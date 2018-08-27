package main

// github.com/cheggaaa/pb

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/fatih/color"
	"github.com/gosuri/uilive"
	rss "github.com/mmcdole/gofeed"
)

func getFeed(url string) (*rss.Feed, error) {
	return rss.NewParser().ParseURL(url)
}

func syncPodcasts(startDate time.Time, nameOrID string, count int, chekMode bool, noUpdate bool) error {
	allReqs := [][]*grab.Request{}
	podcasts := []*Podcast{}
	retry := 0

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

		// retry count
		if podcast.Retry > retry {
			retry = podcast.Retry
		}
	}

	if !chekMode {
		total := startDownload(allReqs, retry)
		log.Infof("%d files downloaded.\n", total)
		if !noUpdate {
			for _, podcast := range podcasts {
				podcast.LastSynced = time.Now()
				if err := cfg.UpdatePodcast(podcast); err != nil {
					return err
				}
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

func startDownload(downloadReqs [][]*grab.Request, retryNum int) int {
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
	if len(failedRequests) > 0 && retryNum > 0 {
		log.Warnf("%d requests failed, retrying (remaining attempts %d)\n", len(failedRequests), retryNum)
		successCount += startDownload([][]*grab.Request{failedRequests}, retryNum-1)
	}

	if retryNum == 0 {
		log.Infof("%d total file (%d success, %d failed).\n",
			totalFiles,
			successCount,
			len(failedRequests))
	}
	return successCount
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
	return completed - len(failedRequests), failedRequests
}

func toProgress(completed int64, size int64) int64 {
	if size <= 0 || completed <= 0 {
		return 0
	}
	return int64(float64(completed) / float64(size) * 100)
}

func bytesToMb(bytesCount int64) float64 {
	if bytesCount <= 0 {
		return 0
	}
	return math.Abs(float64(bytesCount)) / float64(1024*1024)
}

var (
	errPrefix        = color.RedString("ERROR     ")
	donePrefix       = color.GreenString("DONE      ")
	inprogressPrefix = color.MagentaString("DOWNLOAD  ")
)

func calcProgress(bytesComplete int64, size int64) int {
	if size == 0 {
		return 0
	}
	return int(100 * (float64(bytesComplete) / float64(size)))
}

func showProgressError(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui.Bypass(), "%s: %s: %v\n",
		errPrefix,
		status.Response.Request.URL(),
		status.Response.Err())
}

func showProgressDone(ui *uilive.Writer, status *downloadStatus) {
	var actualSize = status.Response.Size
	if actualSize <= 0 {
		actualSize = status.Request.Size
	}
	// spaces to owerwrite pogress line
	fmt.Fprintf(ui.Bypass(),
		"%s: %s [%d/%d] %0.2f / %0.2f Mb (%d%%)   \n",
		donePrefix,
		status.Response.Filename,
		status.Current, status.Total,
		bytesToMb(status.Response.BytesComplete()),
		bytesToMb(actualSize),
		calcProgress(status.Response.BytesComplete(), actualSize))
}

func showProgressProc(ui *uilive.Writer, status *downloadStatus) {
	var actualSize = status.Response.Size
	if actualSize <= 0 {
		actualSize = status.Request.Size
	}
	fmt.Fprintf(ui, "%s: %s [%d/%d] %0.2f / %0.2f Mb (%d%%)\n",
		inprogressPrefix,
		status.Response.Filename,
		status.Current, status.Total,
		bytesToMb(status.Response.BytesComplete()),
		bytesToMb(actualSize),
		calcProgress(status.Response.BytesComplete(), actualSize))
}
