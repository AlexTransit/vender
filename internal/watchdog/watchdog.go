package watchdog

import (
	"os"
	"strconv"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/log2"
)

type Config struct {
	Disabled bool
	Folder   string
}
type wdStruct struct {
	Disabled bool
	Folder   string
	log      *log2.Log
	// hbf string
	wdt string
}

const file = "hb"

var WD wdStruct

func Init(conf *Config, log *log2.Log) {
	WD.Folder = helpers.ConfigDefaultStr(conf.Folder, "/run/user/1000/")
	WD.log = log
	WD.Disabled = conf.Disabled
}

func WatchDogEnable() {
	if WD.wdt == "0" {
		return
	}
	wdf := WD.Folder + "hb"
	createWatchDogFile()
	b, err := os.ReadFile(wdf)
	hbfd := string(b)
	if err != nil || hbfd != WD.wdt {
		WD.log.Errorf("error check watchdog heartBeatFile read data(%v) error(%v)", hbfd, err)
		go func() {
			time.Sleep(1 * time.Second)
			if e := os.Remove(wdf); e != nil {
				WD.log.Errorf("error delete incorect heartBeat File. error(%v)", e)
			}
			createWatchDogFile()
		}()
	}
}

func createWatchDogFile() {
	f, err := os.Create(WD.Folder + file)
	_ = err
	if _, err := f.WriteString(WD.wdt); err != nil {
		WD.log.Errorf("error create watchdog heartBeatFile (%v)", err)
		return
	}
	f.Sync()
	f.Close()
}

func WatchDogDisable() {
	WD.log.Notice("watchdog disabled.")
	if err := os.Remove(WD.Folder + file); err != nil {
		WD.log.Errorf("delete heartBeatFile error(%v)", err)
	}
}

func WatchDogSetTics(tics int) {
	if WD.Disabled {
		return
	}
	WD.wdt = strconv.Itoa(tics)
	WD.log.Infof("watchdog set count:%d", tics)
}
