package main

import (
	"io/ioutil"
	"os"
	"testing"

	"gopkg.in/stretchr/testify.v1/assert"
)

func TestConfigOptions(t *testing.T) {
	// create config file
	content := []byte(`
download-path = /data/Podcasts
separate-dir  = {{CurrentDate}}
disabled      = false
date-format   = 2006Jan
filter        =
mtype         = audio

[Radio Record]
url         = http://localgost/rss.xml
filter      = 'Хрусталев' in {{ItemTitle}}
last-synced = 2016-08-11T14:21:57+03:00
mtype         = video
`)
	tmpfile, err := ioutil.TempFile("", "testconfig")
	if err != nil {
		t.Error("Failed to create tmp file", err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	if _, err = tmpfile.Write(content); err != nil {
		t.Error("Failed to write to tmp file", err)
	}
	if err = tmpfile.Close(); err != nil {
		t.Error("Failed to close tmp file", err)
	}

	if cfg, err = NewConfig(tmpfile.Name()); err != nil {
		t.Error("Failed to read config", err)
	}

	assert.Equal(t, 1, cfg.PodcastLen(), "Porcast Length")
	var p *Podcast
	p, err = cfg.GetPodcastByName("Radio Record")
	if err != nil {
		t.Error("Failed to find podcast by name", err)
	}

	assert.Equal(t, "http://localgost/rss.xml", p.Url, "Pocast.Url is incorrect")
	assert.Equal(t, "{{CurrentDate}}", p.SeparateDir, "Pocast.SeparateDir is incorrect")
	assert.Equal(t, false, p.Disabled, "Pocast.Disabled is incorrect")
	assert.Equal(t, "2006Jan", p.DateFormat, "Pocast.DateFormat  is incorrect")
	assert.Equal(t, "video", p.Mtype, "Pocast.Mtype  is incorrect")
	assert.Equal(t, "'Хрусталев' in {{ItemTitle}}", p.Filter, "Podcast.Filter is incorrect")

}
