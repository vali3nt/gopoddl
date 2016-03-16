/*
 * Simple logger
 */
package main

import (
	"fmt"
	"github.com/shiena/ansicolor"
	"io"
	"os"
	"path"
	"strings"
)

type Logger interface {
	Print(any ...interface{})
	Printf(format string, any ...interface{})
	Info(any ...interface{})
	Infof(format string, any ...interface{})
	Debug(any ...interface{})
	Debugf(format string, any ...interface{})
	Warn(any ...interface{})
	Warnf(format string, any ...interface{})
	Error(any ...interface{})
	Errorf(format string, any ...interface{})
	Fatal(any ...interface{})
	Fatalf(format string, any ...interface{})
	Color(color, str string) string
}

type loggerInstance struct {
	isDebug   bool
	isColored bool
	writer    io.Writer
}

// create logger with options file output, colors, debug
// if fileName is not empty colors are disabled
func MakeLogger(fileName string, isDebug bool, useColors bool) (Logger, error) {
	var (
		logger Logger
		err    error
	)
	if fileName != "" {
		logger, err = MakeFileLogger(isDebug, fileName)
		if err != nil {
			return nil, err
		}
	} else {
		logger = MakeStdoutLogger(isDebug, useColors)
	}
	return logger, err
}

func MakeStdoutLogger(debug bool, colored bool) Logger {
	var writer io.Writer
	if colored {
		writer = ansicolor.NewAnsiColorWriter(os.Stdout)
	} else {
		writer = os.Stdout
	}

	return &loggerInstance{
		isDebug:   debug,
		isColored: colored,
		writer:    writer,
	}
}

func MakeFileLogger(debug bool, filename string) (Logger, error) {
	writer, err := newFileHandler(filename)
	if err != nil {
		return nil, err
	}
	return &loggerInstance{
		isDebug:   debug,
		isColored: false,
		writer:    writer,
	}, nil
}

func newFileHandler(filename string) (f *os.File, err error) {
	dir := path.Dir(filename)
	err = os.MkdirAll(dir, 0777)
	if err != nil {
		return nil, err
	}

	f, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (d *loggerInstance) Print(any ...interface{}) {
	fmt.Fprint(d.writer, fmt.Sprintln(any...))
}

func (d *loggerInstance) Printf(format string, any ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	fmt.Fprintf(d.writer, format, any...)
}
func (d *loggerInstance) Info(any ...interface{}) {
	d.Print(any...)
}

func (d *loggerInstance) Infof(format string, any ...interface{}) {
	d.Printf(format, any...)
}

func (d *loggerInstance) Debug(any ...interface{}) {
	if d.isDebug {
		lvlMsg := d.Color("magenta", "DEBUG : ")
		d.Printf("%s%s", lvlMsg, fmt.Sprint(any...))
	}
}

func (d *loggerInstance) Debugf(format string, any ...interface{}) {
	if d.isDebug {
		format = d.Color("magenta", "DEBUG : ") + format
		d.Printf(format, any...)
	}
}

func (d *loggerInstance) Warn(any ...interface{}) {
	wrnMsg := d.Color("yellow", "WARN : "+fmt.Sprint(any...))
	d.Print(wrnMsg)
}

func (d *loggerInstance) Warnf(format string, any ...interface{}) {
	format = d.Color("yellow", "WARN : "+format)
	d.Printf(format, any...)
}

func (d *loggerInstance) Error(any ...interface{}) {
	wrnMsg := d.Color("red", "ERROR: "+fmt.Sprint(any...))
	d.Print(wrnMsg)
}

func (d *loggerInstance) Errorf(format string, any ...interface{}) {
	format = d.Color("red", "ERROR : "+format)
	d.Printf(format, any...)
}

func (d *loggerInstance) Fatal(any ...interface{}) {
	wrnMsg := d.Color("red", "FATAL: "+fmt.Sprint(any...))
	d.Print(wrnMsg)
	os.Exit(1)
}

func (d *loggerInstance) Fatalf(format string, any ...interface{}) {
	format = d.Color("red", "FATAL : "+format)
	d.Printf(format, any...)
	os.Exit(1)
}

func (d *loggerInstance) Color(color, str string) string {
	if !d.isColored {
		return str
	}
	colMap := map[string]int{
		"black":   0,
		"red":     1,
		"green":   2,
		"yellow":  3,
		"blue":    4,
		"magenta": 5,
		"cyan":    6,
		"white":   7,
	}
	if iColor, ok := colMap[strings.ToLower(color)]; ok {
		str = fmt.Sprintf("\x1b[%dm%s\x1b[0m", 30+iColor, str)
	}
	return str
}
