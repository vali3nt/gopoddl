package main

// github.com/cheggaaa/pb

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/fatih/color"
	"github.com/gosuri/uilive"
)

func checkPodcasts() {
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
		status := color.GreenString("OK")
		if err != nil {
			status = color.RedString("FAIL")
		}
		num := color.MagentaString("[" + strconv.Itoa(n+1) + "] ")

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
}

func syncPodcasts(startDate time.Time, count int, isOverwrite bool) error {
	allReqs := [][]*grab.Request{}
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
		filter := PodcastFilter{
			StartDate:    startDate,
			Count:        count,
			MediaType:    mtype,
			Filter:       getCfgStringNoErr(p.Name, "filter"),
			DateFormat:   getCfgStringNoErr(p.Name, "date-format"),
			SeperatePath: getCfgStringNoErr(p.Name, "separate-dir"),
		}
		// get list
		downloadPath := getCfgStringNoErr(p.Name, "download-path")
		podcastList, err := store.Filter(p, &filter)

		if err != nil {
			log.Fatalf("Error %s: %v", p.Name, err)
			continue
		}

		// check for emptiness
		if len(podcastList) == 0 {
			log.Printf("%s : %s, %d files", color.CyanString("cyan", "EMPTY"),
				store.Podcasts[n].Name, len(podcastList))
			continue
		}

		// create dir if needed
		if !fileExists(downloadPath) {
			if err := os.MkdirAll(downloadPath, 0777); err != nil {
				log.Fatal(err)
				continue
			}
		}

		// create download requests
		reqs := []*grab.Request{}
		for _, entry := range podcastList {
			downloadFilePath := filepath.Join(downloadPath, entry.Filename)
			if !isOverwrite && fileExists(downloadFilePath) {
				continue
			}
			req, _ := grab.NewRequest(entry.Url)
			req.Filename = downloadFilePath
			req.Size = uint64(entry.Size)
			req.RemoveOnError = true
			reqs = append(reqs, req)
		}

		allReqs = append(allReqs, reqs)

	}

	startDownload(allReqs)

	for n := range store.Podcasts {
		store.Podcasts[n].LastSynced = time.Now()
	}
	store.Save()

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
		"Finished %s[%d/%d] %d / %d bytes (%d%%)\n",
		status.Response.Filename,
		status.Current, status.Total,
		status.Response.BytesTransferred(),
		status.Response.Size,
		int(100*status.Response.Progress()))
}

func showProgressProc(ui *uilive.Writer, status *downloadStatus) {
	fmt.Fprintf(ui, "Downloading %s[%d/%d] %d / %d bytes (%d%%)\n",
		status.Response.Filename,
		status.Current, status.Total,
		status.Response.BytesTransferred(),
		status.Response.Size,
		int(100*status.Response.Progress()))

}
