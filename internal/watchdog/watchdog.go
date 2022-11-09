package watchdog

import (
	"os"
	"strconv"

	"github.com/AlexTransit/vender/internal/types"
)

var heartBeatFile = "/run/hb"
var wdTics = "31"

func WatchDogEnable() {
	if wdTics == "0" {
		return
	}
	f, _ := os.Create(heartBeatFile)
	if _, err := f.WriteString(wdTics); err != nil {
		types.Log.Errorf("error create watchdog heartBeatFile (%v)", err)
		return
	}
	f.Close()
}

func WatchDogDisable() {
	types.Log.Notice("watchdog disabled.")
	err := os.Remove(heartBeatFile)
	if err != nil {
		types.Log.Errorf("error disable watchdog. can`t delete heartBeatFile (%v)", err)
	}
}

func WatchDogSetTics(tics int) {
	wdTics = strconv.Itoa(tics)
	types.Log.Infof("watchdog set count:%d", tics)
}
