package watchdog

import (
	"errors"
	"os"
	"strconv"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/log2"
	"github.com/coreos/go-systemd/daemon"
)

type Config struct {
	Disabled bool
	Folder   string
}
type wdStruct struct {
	disabled bool
	folder   string
	log      *log2.Log
	wdt      string // watchdog tics
}

const brokenFile = "/home/vmc/broken"

var WD wdStruct

func Init(conf *Config, log *log2.Log, timeout int) {
	WD.folder = helpers.ConfigDefaultStr(conf.Folder, "/run/user/1000/")
	WD.log = log
	WD.disabled = conf.Disabled
	if !WD.disabled {
		setTimerSec(timeout * 2)
	}
}

func Enable() {
	if WD.disabled || WD.wdt == "0" {
		return
	}
	setUsec(WD.wdt)
}

func Disable() {
	if WD.disabled {
		return
	}
	WD.log.Info("disable watchdog")
	setUsec("0")
}

func setUsec(usec string) {
	ok, err := daemon.SdNotify(false, "WATCHDOG_USEC="+usec)
	if !ok {
		WD.log.Errorf("watchdog not set. interval:%s microsecond error:%v", usec, err)
	}
}

func setTimerSec(sec int) {
	WD.wdt = "0"
	if sec != 0 {
		WD.wdt = strconv.Itoa(sec) + "000000"
	}
}

func Refresh() {
	if WD.disabled {
		return
	}
	sendNotify(daemon.SdNotifyWatchdog)
}

func ReinitRequired() bool {
	if _, err := os.Stat(WD.folder + "vmc"); os.IsNotExist(err) {
		return true
	}
	return false
}

func SetDeviceInited() {
	WD.log.Info("devices inited")
	if err := os.MkdirAll(WD.folder+"vmc", os.ModePerm); err != nil {
		WD.log.Warning(errors.New("create vender folder"), err)
	}
}

func DevicesInitializationRequired() {
	WD.log.Info("devices initialization required")
	os.RemoveAll(WD.folder + "vmc")
}

func SetBroken() {
	f, err := os.Create(brokenFile)
	if err != nil {
		WD.log.Error(errors.New("create broken file "), err)
		return
	}
	f.Sync()
	f.Close()
}

func UnsetBroken() { os.Remove(brokenFile) }

func IsBroken() bool {
	_, err := os.Stat(brokenFile)
	return !os.IsNotExist(err)
}

// send a ready signal to systemd
func ServiceStarted() { sendNotify(daemon.SdNotifyReady) }

func sendNotify(signal string) {
	ok, err := daemon.SdNotify(false, signal)
	if !ok {
		WD.log.Errorf("watchdog not updated error:%v", err)
	}
}
