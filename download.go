package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type downloadStatus struct {
	podcastIdx  int
	fileCounter int
}

type downloadSet struct {
	ui           *uiInfo
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
				d.ui.Error("Failed to create dir %s : %s", targetDir, err)
				continue
			}
		} else if !d.overwrite && fileExists(targetPath) {
			d.ui.Printf("%-15s %s -> %s", log.Color("cyan", "EXISTS"),
				pItem.Title,
				targetPath)
			continue
		}
		if !d.ui.showProgress {
			log.Printf("Start download %s -> %s", pItem.Title, pItem.Url)
		} else {
			d.ui.uiBar.Incr()
			//d.ui.BanIncr()
		}

		t0 := time.Now() // download start time
		if err := downloadFile(d.podcastList[k], targetPath); err != nil {

			d.ui.Printf("%s -> %s : %s", pItem.Title, targetPath, err)

			if fileExists(targetPath) {
				if err := os.Remove(targetPath); err != nil {
					d.ui.Warn("Failed to remove file %s after failure", targetPath)
				}
			}
			continue
		}

		size := float64(d.podcastList[k].Size) / 1024 / 1024
		speed := size / time.Now().Sub(t0).Seconds()

		d.ui.Printf("%-15s %s -> %s [%.2fMb, %.2f Mb/s]",
			log.Color("green", "OK"),
			pItem.Title,
			targetPath,
			size, speed)

		fileCounter++
	}
	pChan <- downloadStatus{d.idx, fileCounter} // send finished podcasts id
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
