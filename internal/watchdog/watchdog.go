package watchdog

import (
	"os"
	"strconv"
	"syscall"
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
	file     string
	log      *log2.Log
	// hbf string
	wdt string
}

const file = "hb"

var WD wdStruct

func Init(conf *Config, log *log2.Log) {
	WD.file = helpers.ConfigDefaultStr(conf.Folder, "/run/user/1000/") + file
	WD.log = log
	WD.Disabled = conf.Disabled
}

func WatchDogEnable() {
	if WD.Disabled {
		return
	}
	if err := createWatchDogFile(); err != nil {
		WD.log.WarningF("create watchdog error(%v) retry", err)
		go func() {
			time.Sleep(1 * time.Second)
			if err := createWatchDogFile(); err != nil {
				WD.Disabled = true
				WD.log.Errorf("watchdog disabled. create file two times error(%v)", err)
			}
		}()
	}
	f, err := os.OpenFile(WD.file, syscall.O_RDONLY, 0o666)
	if err != nil {
		WD.log.WarningF("open watchdog file error(%v)", err)
		return
	}
	buf := make([]byte, len(WD.wdt))
	_, err = f.Read(buf)
	if err != nil {
		WD.log.WarningF("read watchdog file error(%v)", err)
		return
	}
	if WD.wdt != string(buf) {
		WD.log.WarningF("new watchdog file dismatch wd(%s) tiks(%s)", WD.wdt, buf)
	}
	f.Close()
}

func createWatchDogFile() error {
	var f *os.File
	var err error
	f, err = os.Create(WD.file + file)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write([]byte(WD.wdt + "\n"))
	return err
}

func WatchDogDisable() {
	WD.log.Notice("watchdog disabled.")
	if err := os.Remove(WD.file); err != nil {
		e, ok := err.(*os.PathError)
		if ok && e.Err != syscall.ENOENT {
			WD.log.Errorf("delete heartBeatFile error(%v)", e)
		}
	}
}

func WatchDogSetTics(tics int) {
	if WD.Disabled {
		return
	}
	WD.wdt = strconv.Itoa(tics)
	WD.log.Infof("watchdog set count:%d", tics)
}
