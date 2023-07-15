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
	// cmd := exec.Command("/usr/bin/sudo", "/usr/bin/touch", heartBeatFile, "&", "/usr/bin/sudo", "/usr/bin/chown", "vmc:vmc", heartBeatFile)
	// if err := cmd.Run(); err != nil {
	// 	types.Log.Errorf("cant creat heartBeatFile (%v)", err)
	// }
	// cmd = exec.Command("/usr/bin/sudo", "/usr/bin/chown", "vmc:vmc", heartBeatFile)
	// if err := cmd.Run(); err != nil {
	// 	types.Log.Errorf("cant change heartBeatFile owner (%v)", err)
	// }

	f, err := os.Create(heartBeatFile)
	_ = err
	if _, err := f.WriteString(wdTics); err != nil {
		types.Log.Errorf("error create watchdog heartBeatFile (%v)", err)
		return
	}
	f.Close()
}

func WatchDogDisable() {
	types.Log.Notice("watchdog disabled.")
	_ = os.Remove(heartBeatFile)
}

func WatchDogSetTics(tics int) {
	wdTics = strconv.Itoa(tics)
	types.Log.Infof("watchdog set count:%d", tics)
}
