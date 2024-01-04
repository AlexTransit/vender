package broken

import (
	"time"

	"github.com/AlexTransit/vender/internal/sound"
	"github.com/AlexTransit/vender/internal/state"
)

var g *state.Global

func BrokenInit(v *state.Global) {
	g = v
}

func Broken() {
	sound.Broken()

	display, _ := g.TextDisplay()

	g.Tele.RoboSendBroken()
	display.SetLines(g.Config.UI.Front.MsgBrokenL1, g.Config.UI.Front.MsgBrokenL2)
	for {
		time.Sleep(time.Minute)
	}
}
