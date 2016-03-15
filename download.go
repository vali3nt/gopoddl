package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
)

type downloadStatus struct {
	podcastIdx  int
	fileCounter int
}

type downloadSet struct {
	podcastList  []PodcastItem
	downloadPath string
	overwrite    bool
	idx          int
}

func downloadPodcastList(d downloadSet, pChan chan downloadStatus) {
	fileCounter := 0
	for k := range d.podcastList {
		pItem := d.podcastList[k]
		targetDir := filepath.Join(d.downloadPath, pItem.Dir)
		targetPath := filepath.Join(targetDir, pItem.Filename)
		// target dir
		if !fileExists(targetDir) {
			if err := os.MkdirAll(targetDir, 0777); err != nil {
				log.Error("Failed to create dir %s : %s", targetDir, err)
				continue
			}
		} else if !d.overwrite && fileExists(targetPath) {
			log.Printf("%-15s %s -> %s", log.Color("cyan", "EXISTS"), pItem.Title, targetPath)
			continue
		}

		log.Printf("Start download %s -> %s", pItem.Title, pItem.Url)

		t0 := time.Now() // download start time
		if err := downloadFile(d.podcastList[k], targetPath); err != nil {

			log.Printf("%s -> %s : %s", pItem.Title, targetPath, err)

			if fileExists(targetPath) {
				if err := os.Remove(targetPath); err != nil {
					log.Warn("Failed to remove file %s after failure", targetPath)
				}
			}
			continue
		}

		size := float64(d.podcastList[k].Size) / 1024 / 1024
		speed := size / time.Now().Sub(t0).Seconds()

		log.Printf("%-15s %s -> %s [%.2fMb, %.2f Mb/s]", log.Color("green", "OK"), pItem.Title, targetPath, size, speed)
		fileCounter++
	}

	// send finished podcasts id
	pChan <- downloadStatus{d.idx, fileCounter}
}

func downloadFile(pItem PodcastItem, targetPath string) error {
	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()
	resp, err := http.Get(pItem.Url)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Response code : %d", resp.StatusCode))
	}
	defer resp.Body.Close()
	size, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	if size != pItem.Size {
		return errors.New(fmt.Sprintf("%s size is differ %d bytes <> %d bytes", pItem.Url, pItem.Size, size))
	}
	return nil
}

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
		status := log.Color("green", "OK")
		if err != nil {
			status = log.Color("red", "FAIL")
		}
		num := log.Color("magenta", "["+strconv.Itoa(n+1)+"] ")

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
	pChan := make(chan downloadStatus)
	errChan := make(chan string, 10000)

	podcastCount := 0
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
			StartDate:    startDate,
			Count:        count,
			MediaType:    mtype,
			Filter:       getCfgStringNoErr(p.Name, "filter"),
			DateFormat:   getCfgStringNoErr(p.Name, "date-format"),
			SeperatePath: getCfgStringNoErr(p.Name, "separate-dir"),
		}
		// get list
		downloadPath := getCfgStringNoErr(p.Name, "download-path")
		podcastList, err := store.Filter(p, &f)

		if err != nil {
			log.Errorf("Error %s: %v", p.Name, err)
			continue
		}

		if len(podcastList) == 0 {
			log.Printf("%s : %s, %d files", log.Color("cyan", "EMPTY"),
				store.Podcasts[n].Name, len(podcastList))
			continue
		}
		if !fileExists(downloadPath) {
			if err := os.MkdirAll(downloadPath, 0777); err != nil {
				log.Error(err)
				continue
			}
		}
		podcastCount++ // podcast counter
		d := downloadSet{
			podcastList:  podcastList,
			downloadPath: downloadPath,
			overwrite:    isOverwrite,
			idx:          n,
		}

		go downloadPodcastList(d, pChan)
	}
	donePodcasts := 0
	if podcastCount != 0 {

	M:
		for {
			select {
			case status := <-pChan:
				log.Printf("%s : %s", log.Color("green", "DONE"), store.Podcasts[status.podcastIdx].Name)
				store.Podcasts[status.podcastIdx].LastSynced = time.Now()
				store.Podcasts[status.podcastIdx].DownloadedFiles += status.fileCounter
				store.Save()
				donePodcasts++
				if donePodcasts == podcastCount {
					break M
				}
			}
		}
	}
	close(pChan)
	close(errChan)
	return nil
}
