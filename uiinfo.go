package main

import (
	"fmt"
	"github.com/gosuri/uiprogress"
)

type uiInfo struct {
	uiBar        *uiprogress.Bar
	showProgress bool
}

func (u *uiInfo) BanIncr() {
	u.uiBar.Incr()
}

func (u *uiInfo) Printf(format string, msgs ...interface{}) {
	if u.showProgress {
		errChan <- fmt.Sprintf(format, msgs...)
	} else {
		log.Printf(format, msgs...)
	}
}

func (u *uiInfo) Error(format string, msgs ...interface{}) {
	var msgWithLvl []interface{}
	msgWithLvl = append(msgWithLvl, log.Color("red", "ERROR"), msgs)
	u.Printf("%-15s "+format, msgWithLvl...)
}

func (u *uiInfo) Warn(format string, msgs ...interface{}) {
	var msgWithLvl []interface{}
	msgWithLvl = append(msgWithLvl, log.Color("yellow", "WARN"), msgs)
	u.Printf("%-15s "+format, msgWithLvl...)
}
