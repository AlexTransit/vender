package watchdog

import (
	"errors"
	"os"
	"strconv"

	config_global "github.com/AlexTransit/vender/internal/config"
	watchdog_config "github.com/AlexTransit/vender/internal/watchdog/config"
	"github.com/AlexTransit/vender/log2"
	"github.com/coreos/go-systemd/daemon"
)

type wdStruct struct {
	config *watchdog_config.Config
	log    *log2.Log
	wdt    string // watchdog tics
}

var WD wdStruct

func Init(conf *config_global.Config, log *log2.Log, timeout int) {
	WD.config = &conf.Watchdog
	WD.wdt = strconv.Itoa(conf.UI_config.Front.ResetTimeoutSec * 3)
	WD.log = log
	if !WD.config.Disabled {
		setTimerSec(timeout * 3)
	}
}

func Enable() {
	if WD.config.Disabled || WD.wdt == "0" {
		return
	}
	setUsec(WD.wdt)
	sendNotify(daemon.SdNotifyReady)
}

func Disable() {
	// if WD.config.Disabled {
	// 	return
	// }
	WD.log.Info("disable watchdog")
	// send disable watchdog for systemd
	sendNotify(daemon.SdNotifyReloading)
	setUsec("0")
}

func setUsec(usec string) {
	ok, err := daemon.SdNotify(false, "WATCHDOG_USEC="+usec)
	if !ok || err != nil {
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
	if WD.config.Disabled {
		return
	}
	sendNotify(daemon.SdNotifyWatchdog)
}

func ReinitRequired() bool {
	if _, err := os.Stat(WD.config.Folder + "vmc"); os.IsNotExist(err) {
		return true
	}
	return false
}

func SetDeviceInited() {
	WD.log.Info("devices inited")
	if err := os.MkdirAll(WD.config.Folder+"vmc", os.ModePerm); err != nil {
		WD.log.Warning(errors.New("create vender folder"), err)
	}
}

func DevicesInitializationRequired() {
	WD.log.Info("devices initialization required")
	os.RemoveAll(WD.config.Folder + "vmc")
}

// create broken file
// if file exits then not started after reboot
func SetBroken() {
	f, err := os.Create(config_global.VMC.BrokenFile)
	if err != nil {
		WD.log.Error(errors.New("create broken file "), err)
		return
	}
	f.Sync()
	f.Close()
}

func UnsetBroken() { os.Remove(config_global.VMC.BrokenFile) }

func IsBroken() bool {
	_, err := os.Stat(config_global.VMC.BrokenFile)
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
