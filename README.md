gopoddl
===

Simple command-line podcast downloader 

## Usage

```bash 
   gopoddl [global options] command [command options] [arguments...]
```


## Commands:
   * init   - create default config files
   * list   - list all podcasts
   * add    - add podcast to sync
   * remove - remove podcast from sync
   * reset  - reset time and count for podcasts
   * check  - check podcasts for availability
   * sync   - start downloading
   * help   - Shows a list of commands or help for one command

## Installation

```bash
$ go get github.com/vali3nt/gopoddl
```

## Start
Create inital config files under home directory:
```bash
$ gopoddl init
```
And edit ~/.gopoddl_conf.ini, setup at least download-path

Then add podcast:
```bash
$ gopoddl add http://example.com/rss.xml SOMENAME
```
if name is omitted , would be retrieved from podcast

Start download:
```bash
$ gopoddl sync
```

* Number of podcasts items to download
* Start date for sync 

can be set by command line options

* Download path
* Media type
* Filter (download podcast item with some text in title) 
    
can be set configuration per podcast

## Author

[vali3nt](https://github.com/vali3nt)
