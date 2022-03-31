package ui

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"

	// "net"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/engine/inventory"
	"github.com/AlexTransit/vender/internal/money"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	tele_api "github.com/AlexTransit/vender/tele"
	"github.com/juju/errors"
	"github.com/temoto/alive/v2"
)

const (
	serviceMenuInventory = "inventory"
	serviceMenuTest      = "test"
	serviceMenuReboot    = "reboot"
	serviceMenuNetwork   = "network"
	serviceMenuMoneyLoad = "money-load"
	serviceMenuReport    = "report"
)

var /*const*/ serviceMenu = []string{
	serviceMenuInventory,
	serviceMenuTest,
	serviceMenuReboot,
	serviceMenuNetwork,
	serviceMenuMoneyLoad,
	serviceMenuReport,
}
var /*const*/ serviceMenuMax = uint8(len(serviceMenu) - 1)

type uiService struct { //nolint:maligned
	// config
	resetTimeout time.Duration
	SecretSalt   []byte

	// state
	askReport bool
	menuIdx   uint8
	invIdx    uint8
	invList   []*inventory.Stock
	testIdx   uint8
	testList  []engine.Doer
}

func (ui *uiService) Init(ctx context.Context) {
	g := state.GetGlobal(ctx)
	config := g.Config.UI.Service
	ui.SecretSalt = []byte{0} // FIXME read from config
	ui.resetTimeout = helpers.IntSecondDefault(config.ResetTimeoutSec, 3*time.Second)
	errs := make([]error, 0, len(config.Tests))
	for _, t := range config.Tests {
		if d, err := g.Engine.ParseText(t.Name, t.Scenario); err != nil {
			errs = append(errs, err)
		} else {
			ui.testList = append(ui.testList, d)
		}
	}
	if err := helpers.FoldErrors(errs); err != nil {
		g.Log.Fatal(err)
	}
}

func (ui *UI) onServiceBegin(ctx context.Context) State {
	ui.g.Hardware.Input.Enable(true)
	ui.inputBuf = ui.inputBuf[:0]
	ui.Service.askReport = false
	ui.Service.menuIdx = 0
	ui.Service.invIdx = 0
	ui.Service.invList = make([]*inventory.Stock, 0, 16)
	ui.Service.testIdx = 0
	ui.g.Inventory.Iter(func(s *inventory.Stock) {
		ui.g.Log.Debugf("ui service inventory: - %s", s.String())
		ui.Service.invList = append(ui.Service.invList, s)
	})
	sort.Slice(ui.Service.invList, func(a, b int) bool {
		xa := ui.Service.invList[a]
		xb := ui.Service.invList[b]
		if xa.Code != xb.Code {
			return xa.Code < xb.Code
		}
		return xa.Name < xb.Name
	})
	// ui.g.Log.Debugf("invlist=%v, invidx=%d", ui.Service.invList, ui.Service.invIdx)

	if errs := ui.g.Engine.ExecList(ctx, "on_service_begin", ui.g.Config.Engine.OnServiceBegin); len(errs) != 0 {
		ui.g.Error(errors.Annotate(helpers.FoldErrors(errs), "on_service_begin"))
		return StateBroken
	}

	ui.g.Log.Debugf("ui service begin")
	ui.g.Tele.RoboSendState(tele_api.CurrentState_ServiceState)
	return StateServiceAuth
}

func (ui *UI) onServiceAuth() State {
	serviceConfig := &ui.g.Config.UI.Service
	if !serviceConfig.Auth.Enable {
		return StateServiceMenu
	}

	passVisualHash := VisualHash(ui.inputBuf, ui.Service.SecretSalt)
	ui.display.SetLines(
		serviceConfig.MsgAuth,
		fmt.Sprintf(msgServiceInputAuth, passVisualHash),
	)

	next, e := ui.serviceWaitInput()
	if next != StateDefault {
		return next
	}

	switch {
	case e.IsDigit():
		ui.inputBuf = append(ui.inputBuf, byte(e.Key))
		if len(ui.inputBuf) > 16 {
			ui.display.SetLines(MsgError, "len") // FIXME extract message string
			ui.serviceWaitInput()
			return StateServiceEnd
		}
		return ui.State()

	case e.IsZero() || input.IsReject(&e):
		return StateServiceEnd

	case input.IsAccept(&e):
		if len(ui.inputBuf) == 0 {
			ui.display.SetLines(MsgError, "empty") // FIXME extract message string
			ui.serviceWaitInput()
			return StateServiceEnd
		}

		// FIXME fnv->secure hash for actual password comparison
		inputHash := VisualHash(ui.inputBuf, ui.Service.SecretSalt)
		for i, p := range ui.g.Config.UI.Service.Auth.Passwords {
			if inputHash == p {
				ui.g.Log.Infof("service auth ok i=%d hash=%s", i, inputHash)
				return StateServiceMenu
			}
		}

		ui.display.SetLines(MsgError, "sorry") // FIXME extract message string
		ui.serviceWaitInput()
		return StateServiceEnd
	}
	ui.g.Log.Errorf("ui onServiceAuth unhandled branch")
	ui.display.SetLines(MsgError, "code error") // FIXME extract message string
	ui.serviceWaitInput()
	return StateServiceEnd
}

func (ui *UI) onServiceMenu() State {
	menuName := serviceMenu[ui.Service.menuIdx]
	ui.display.SetLines(
		msgServiceMenu,
		fmt.Sprintf("%d %s", ui.Service.menuIdx+1, menuName),
	)

	next, e := ui.serviceWaitInput()
	if next != StateDefault {
		return next
	}

	switch {
	case e.Key == input.EvendKeyCreamLess:
		ui.Service.menuIdx = addWrap(ui.Service.menuIdx, serviceMenuMax+1, -1)
	case e.Key == input.EvendKeyCreamMore:
		ui.Service.menuIdx = addWrap(ui.Service.menuIdx, serviceMenuMax+1, +1)

	case input.IsAccept(&e):
		if int(ui.Service.menuIdx) >= len(serviceMenu) {
			ui.g.Fatal(errors.Errorf("code error service menuIdx out of range"))
			return StateBroken
		}
		switch serviceMenu[ui.Service.menuIdx] {
		case serviceMenuInventory:
			return StateServiceInventory
		case serviceMenuTest:
			return StateServiceTest
		case serviceMenuReboot:
			return StateServiceReboot
		case serviceMenuNetwork:
			return StateServiceNetwork
		case serviceMenuMoneyLoad:
			return StateServiceMoneyLoad
		case serviceMenuReport:
			return StateServiceReport
		default:
			panic("code error")
		}

	case input.IsReject(&e):
		return StateServiceEnd

	case e.IsDigit():
		x := byte(e.Key) - byte('0')
		if x > 0 && x <= serviceMenuMax {
			ui.Service.menuIdx = x - 1
		}
	}
	return StateServiceMenu
}

func (ui *UI) onServiceInventory() State {
	if len(ui.Service.invList) == 0 {
		ui.display.SetLines(MsgError, "inv empty") // FIXME extract message string
		ui.serviceWaitInput()
		return StateServiceMenu
	}
	invCurrent := ui.Service.invList[ui.Service.invIdx]
	ui.display.SetLines(
		fmt.Sprintf("I%d %s", invCurrent.Code, invCurrent.Name),
		fmt.Sprintf("%.1f %s\x00", invCurrent.Value(), string(ui.inputBuf)), // TODO configurable decimal point
	)

	next, e := ui.serviceWaitInput()
	if next != StateDefault {
		return next
	}

	invIdxMax := uint8(len(ui.Service.invList))
	switch {
	case e.Key == input.EvendKeyCreamLess || e.Key == input.EvendKeyCreamMore:
		if len(ui.inputBuf) != 0 {
			ui.display.SetLines(MsgError, "set or clear?") // FIXME extract message string
			ui.serviceWaitInput()
			return StateServiceInventory
		}
		if e.Key == input.EvendKeyCreamLess {
			ui.Service.invIdx = addWrap(ui.Service.invIdx, invIdxMax, -1)
		} else {
			ui.Service.invIdx = addWrap(ui.Service.invIdx, invIdxMax, +1)
		}
	case e.Key == input.EvendKeyDot || e.IsDigit():
		ui.inputBuf = append(ui.inputBuf, byte(e.Key))

	case input.IsAccept(&e):
		if len(ui.inputBuf) == 0 {
			ui.g.Log.Errorf("ui onServiceInventory input=accept inputBuf=empty")
			ui.display.SetLines(MsgError, "empty") // FIXME extract message string
			ui.serviceWaitInput()
			return StateServiceInventory
		}

		x, err := strconv.ParseFloat(string(ui.inputBuf), 32)
		ui.inputBuf = ui.inputBuf[:0]
		if err != nil {
			ui.g.Log.Errorf("ui onServiceInventory input=accept inputBuf='%s'", string(ui.inputBuf))
			ui.display.SetLines(MsgError, "number-invalid") // FIXME extract message string
			ui.serviceWaitInput()
			return StateServiceInventory
		}

		invCurrent := ui.Service.invList[ui.Service.invIdx]
		invCurrent.Set(float32(x))
		ui.Service.askReport = true

	case input.IsReject(&e):
		// backspace semantic
		if len(ui.inputBuf) > 0 {
			ui.inputBuf = ui.inputBuf[:len(ui.inputBuf)-1]
			return StateServiceInventory
		}
		return StateServiceMenu
	}
	return StateServiceInventory
}

func (ui *UI) onServiceTest(ctx context.Context) State {
	ui.inputBuf = ui.inputBuf[:0]
	if len(ui.Service.testList) == 0 {
		ui.display.SetLines(MsgError, "no tests") // FIXME extract message string
		ui.serviceWaitInput()
		return StateServiceMenu
	}
	testCurrent := ui.Service.testList[ui.Service.testIdx]
	line1 := fmt.Sprintf("T%d %s", ui.Service.testIdx+1, testCurrent.String())
	ui.display.SetLines(line1, "")

wait:
	next, e := ui.serviceWaitInput()
	if next != StateDefault {
		return next
	}

	testIdxMax := uint8(len(ui.Service.testList))
	switch {
	case e.Key == input.EvendKeyCreamLess:
		ui.Service.testIdx = addWrap(ui.Service.testIdx, testIdxMax, -1)
	case e.Key == input.EvendKeyCreamMore:
		ui.Service.testIdx = addWrap(ui.Service.testIdx, testIdxMax, +1)

	case input.IsAccept(&e):
		ui.display.SetLines(line1, "in progress")
		if err := ui.g.Engine.ValidateExec(ctx, testCurrent); err == nil {
			ui.display.SetLines(line1, "OK")
		} else {
			ui.g.Error(err)
			ui.display.SetLines(line1, "error")
		}
		goto wait

	case input.IsReject(&e):
		return StateServiceMenu
	}
	return StateServiceTest
}

func (ui *UI) onServiceReboot(ctx context.Context) State {
	ui.display.SetLines("for reboot", "press 1") // FIXME extract message string

	next, e := ui.serviceWaitInput()
	if next != StateDefault {
		return next
	}

	switch {
	case e.Key == '1':
		ui.display.SetLines("reboot", "in progress") // FIXME extract message string
		ui.g.VmcStop(ctx)
		return StateStop
	}
	return StateServiceMenu
}

func (ui *UI) onServiceNetwork() State {

	ui.display.SetLines("for select net 0", "press 1") // FIXME extract message string

	next, e := ui.serviceWaitInput()
	if next != StateDefault {
		return next
	}

	switch {
	case e.Key == '1':
		ui.display.SetLines("wifi restart", "in progress") // FIXME extract message string

		// lsCmd := exec.Command("bash", "-c", "wpa_cli select_network 0 && wpa_cli enable_network 1")
		lsCmd := exec.Command("bash", "-c", "wpa_cli select_network 0")
		bashOut, err := lsCmd.Output()
		ui.g.Log.Infof("restart wlan (%v)", bashOut)
		if err != nil {
			ui.g.Log.Infof("%v", err)
			// panic(err)
		}

		lsCmd = exec.Command("bash", "-c", "wpa_cli enable_network 1")
		bashOut, err = lsCmd.Output()
		ui.g.Log.Infof("restart wlan (%v)", bashOut)
		if err != nil {
			ui.g.Log.Infof("%v", err)
			// panic(err)
		}

		return StateServiceEnd
	}
	return StateServiceMenu
	// 	allAddrs, _ := net.InterfaceAddrs()
	// 	addrs := make([]string, 0, len(allAddrs))
	// 	// TODO parse ignored networks from config
	// addrLoop:
	// 	for _, addr := range allAddrs {
	// 		ip, _, err := net.ParseCIDR(addr.String())
	// 		if err != nil {
	// 			ui.g.Log.Errorf("invalid local addr=%v", addr)
	// 			continue addrLoop
	// 		}
	// 		if ip.IsLoopback() {
	// 			continue addrLoop
	// 		}
	// 		addrs = append(addrs, ip.String())
	// 	}
	// 	listString := strings.Join(addrs, " ")
	// 	ui.display.SetLines("network", listString)

	// 	for {
	// 		next, e := ui.serviceWaitInput()
	// 		if next != StateDefault {
	// 			return next
	// 		}
	// 		if input.IsReject(&e) {
	// 			return StateServiceMenu
	// 		}
	// 	}
}

func (ui *UI) onServiceMoneyLoad(ctx context.Context) State {
	moneysys := money.GetGlobal(ctx)

	ui.display.SetLines("money-load", "0")
	alive := alive.NewAlive()
	defer func() {
		alive.Stop() // stop pending AcceptCredit
		alive.Wait()
	}()

	ui.Service.askReport = true
	accept := true
	loaded := currency.Amount(0)
	for {
		credit := moneysys.Credit(ctx)
		if credit > 0 {
			loaded += credit
			ui.display.SetLines("money-load", loaded.FormatCtx(ctx))
			// reset loaded credit
			_ = moneysys.WithdrawCommit(ctx, credit)
		}

		if accept {
			accept = false
			go moneysys.AcceptCredit(ctx, currency.MaxAmount, alive.StopChan(), ui.eventch)
		}
		switch e := ui.wait(ui.Service.resetTimeout); e.Kind {
		case types.EventInput:
			if input.IsReject(&e.Input) {
				return StateServiceMenu
			}

		case types.EventMoneyCredit:
			accept = true

		case types.EventLock:
			return StateLocked

		case types.EventStop:
			ui.g.Log.Debugf("onServiceMoneyLoad global stop")
			return StateServiceEnd

		case types.EventTime:

		default:
			panic(fmt.Sprintf("code error onServiceMoneyLoad unhandled event=%s", e.String()))
		}
	}
}

func (ui *UI) onServiceReport(ctx context.Context) State {
	_ = ui.g.Tele.Report(ctx, true)
	if errs := ui.g.Engine.ExecList(ctx, "service-report", []string{"money.cashbox_zero"}); len(errs) != 0 {
		ui.g.Error(errors.Annotate(helpers.FoldErrors(errs), "service-report"))
	}
	return StateServiceMenu
}

func (ui *UI) onServiceEnd(ctx context.Context) State {
	_ = ui.g.Inventory.Persist.Store()
	ui.inputBuf = ui.inputBuf[:0]

	if ui.Service.askReport {
		ui.display.SetLines("for tele report", "press 1") // FIXME extract message string
		if e := ui.wait(ui.Service.resetTimeout); e.Kind == types.EventInput && e.Input.Key == '1' {
			ui.Service.askReport = false
			ui.onServiceReport(ctx)
		}
	}

	if errs := ui.g.Engine.ExecList(ctx, "on_service_end", ui.g.Config.Engine.OnServiceEnd); len(errs) != 0 {
		ui.g.Error(errors.Annotate(helpers.FoldErrors(errs), "on_service_end"))
		return StateBroken
	}
	return StateDefault
}

func (ui *UI) serviceWaitInput() (State, types.InputEvent) {
	e := ui.wait(ui.Service.resetTimeout)
	switch e.Kind {
	case types.EventInput:
		return StateDefault, e.Input

	case types.EventMoneyCredit:
		ui.g.Log.Debugf("serviceWaitInput event=%s", e.String())
		return StateDefault, types.InputEvent{}

	case types.EventTime:
		// ui.g.Log.Infof("inactive=%v", inactive)
		ui.g.Log.Debugf("serviceWaitInput resetTimeout")
		return StateServiceEnd, types.InputEvent{}

	case types.EventLock:
		return StateLocked, types.InputEvent{}

	case types.EventService:
		ui.g.Log.Debugf("service exit")
		return StateServiceEnd, types.InputEvent{}

	case types.EventStop:
		ui.g.Log.Debugf("serviceWaitInput global stop")
		return StateServiceEnd, types.InputEvent{}

	default:
		panic(fmt.Sprintf("code error serviceWaitInput unhandled event=%#v", e))
	}
}

func VisualHash(input, salt []byte) string {
	h := fnv.New32()
	_, _ = h.Write(salt)
	_, _ = h.Write(input)
	_, _ = h.Write(salt)
	var buf [4]byte
	binary := h.Sum(buf[:0])
	b64 := base64.RawStdEncoding.EncodeToString(binary)
	return strings.ToLower(b64)
}

func addWrap(current, max uint8, delta int8) uint8 {
	return uint8((int32(current) + int32(max) + int32(delta)) % int32(max))
}
