package ui

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"

	// "net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/AlexTransit/vender/hardware/input"
	"github.com/AlexTransit/vender/helpers"
	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/AlexTransit/vender/internal/watchdog"
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
	resetTimeout time.Duration
	askReport    bool
	menuIdx      uint8
	invIdx       uint8
	// invList   []*inventory.Stock
	testIdx  uint8
	testList []engine.Doer
}

func (ui *uiService) Init(ctx context.Context) {
	g := state.GetGlobal(ctx)
	config := g.Config.UI_config.Service
	ui.resetTimeout = time.Second * time.Duration(config.ResetTimeoutSec)
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

func (ui *UI) onServiceBegin(ctx context.Context) types.UiState {
	config_global.VMC.KeyboardReader(true)
	watchdog.Disable()
	ui.inputBuf = ui.inputBuf[:0]
	if errs := ui.g.Engine.ExecList(ctx, "on_service_begin", ui.g.Config.Engine.OnServiceBegin); len(errs) != 0 {
		ui.g.Error(errors.Annotate(helpers.FoldErrors(errs), "on_service_begin"))
		return types.StateBroken
	}

	ui.g.Log.Debugf("ui service begin")
	ui.g.Tele.RoboSendState(tele_api.State_Service)
	return types.StateServiceMenu
}

func (ui *UI) onServiceMenu() types.UiState {
	menuName := serviceMenu[ui.Service.menuIdx]
	ui.display.SetLines(
		msgServiceMenu,
		fmt.Sprintf("%d %s", ui.Service.menuIdx+1, menuName),
	)

	next, e := ui.serviceWaitInput()
	if next != types.StateDefault {
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
			return types.StateBroken
		}
		switch serviceMenu[ui.Service.menuIdx] {
		case serviceMenuInventory:
			return types.StateServiceInventory
		case serviceMenuTest:
			return types.StateServiceTest
		case serviceMenuReboot:
			return types.StateServiceReboot
		case serviceMenuNetwork:
			return types.StateServiceNetwork
		case serviceMenuMoneyLoad:
			return types.StateServiceMoneyLoad
		case serviceMenuReport:
			return types.StateServiceReport
		default:
			panic("code error")
		}

	case input.IsReject(&e):
		return types.StateServiceEnd

	case e.IsDigit():
		x := byte(e.Key) - byte('0')
		if x > 0 && x <= serviceMenuMax {
			ui.Service.menuIdx = x - 1
		}
	}
	return types.StateServiceMenu
}

var lv bool

func (ui *UI) onServiceInventory() types.UiState {
	if len(ui.g.Inventory.Stocks) == 0 {
		ui.display.SetLine(1, "inv empty") // FIXME extract message string)
		ui.serviceWaitInput()
		return types.StateServiceMenu
	}
	s := ui.g.Inventory.Stocks[ui.Service.invIdx]
	if lv {
		// l1 := fmt.Sprintf("%.0f %s\x00", s.Value(), iname)
		ui.display.SetLines(
			fmt.Sprintf("%.0f %s", s.Value(), s.Ingredient.Name),
			fmt.Sprintf("%d Lev:%s %s", s.Code, s.ShowLevel(), string(ui.inputBuf)), // TODO configurable decimal point
		)
	} else {
		// l2 := fmt.Sprintf("%s %s", s.ShowLevel(), iname)
		ui.display.SetLines(
			fmt.Sprintf("%s %s", s.ShowLevel(), s.Ingredient.Name),
			fmt.Sprintf("%d Val:%.0f %s", s.Code, s.Value(), string(ui.inputBuf)), // TODO configurable decimal point
		)
	}
	next, e := ui.serviceWaitInput()
	if next != types.StateDefault {
		return next
	}

	invIdxMax := uint8(len(ui.g.Inventory.Stocks))
	switch {
	case e.Key == input.EvendKeyCreamLess || e.Key == input.EvendKeyCreamMore:
		if len(ui.inputBuf) != 0 {
			ui.display.SetLine(2, "set or clear?") // FIXME extract message string
			ui.serviceWaitInput()
			return types.StateServiceInventory
		}
		if e.Key == input.EvendKeyCreamLess {
			ui.Service.invIdx = addWrap(ui.Service.invIdx, invIdxMax, -1)
		} else {
			ui.Service.invIdx = addWrap(ui.Service.invIdx, invIdxMax, +1)
		}
	case e.Key == input.EvendKeyDot || e.IsDigit():
		ui.inputBuf = append(ui.inputBuf, byte(e.Key))
	case e.Key == input.EvendKeySugarLess || e.Key == input.EvendKeySugarMore:
		if lv {
			lv = false
		} else {
			lv = true
		}
		return types.StateServiceInventory
	case input.IsAccept(&e):
		if len(ui.inputBuf) == 0 {
			ui.g.Log.WarningF("ui onServiceInventory input=accept inputBuf=empty")
			ui.display.SetLine(2, "empty") // FIXME extract message string
			ui.serviceWaitInput()
			return types.StateServiceInventory
		}

		xt, err := strconv.ParseFloat(string(ui.inputBuf), 64)
		x := int(xt * 100)
		if err != nil {
			ui.g.Log.WarningF("ui onServiceInventory input=accept inputBuf='%s'", string(ui.inputBuf))
			ui.display.SetLine(2, "number-invalid") // FIXME extract message string
			ui.serviceWaitInput()
			return types.StateServiceInventory
		}
		if v := strings.Index(string(ui.inputBuf), "."); v >= 0 {
			lv = true
		}
		ui.inputBuf = ui.inputBuf[:0]

		if lv {
			ui.g.Inventory.Stocks[ui.Service.invIdx].SetLevel(x)
		} else {
			ui.g.Inventory.Stocks[ui.Service.invIdx].Set(float32(x) / 100)
		}
		ui.Service.askReport = true
		// invCurrent.TeleLow = false

	case input.IsReject(&e):
		// backspace semantic
		if len(ui.inputBuf) > 0 {
			ui.inputBuf = ui.inputBuf[:len(ui.inputBuf)-1]
			return types.StateServiceInventory
		}
		return types.StateServiceMenu
	}
	return types.StateServiceInventory
}

func (ui *UI) onServiceTest(ctx context.Context) types.UiState {
	ui.inputBuf = ui.inputBuf[:0]
	if len(ui.Service.testList) == 0 {
		ui.g.MustTextDisplay().SetLine(1, "no tests") // FIXME extract message string
		ui.serviceWaitInput()
		return types.StateServiceMenu
	}
	testCurrent := ui.Service.testList[ui.Service.testIdx]
	line1 := fmt.Sprintf("T%d %s", ui.Service.testIdx+1, testCurrent.String())
	ui.display.SetLines(line1, "")

wait:
	next, e := ui.serviceWaitInput()
	if next != types.StateDefault {
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
		return types.StateServiceMenu
	}
	return types.StateServiceTest
}

func (ui *UI) onServiceReboot(ctx context.Context) types.UiState {
	ui.display.SetLines("for reboot", "press 1") // FIXME extract message string

	next, e := ui.serviceWaitInput()
	if next != types.StateDefault {
		return next
	}

	switch {
	case e.Key == '1':
		ui.display.SetLines("reboot", "in progress") // FIXME extract message string
		ui.g.GlobalError = "reboot from menu"
		ui.g.VmcStop(ctx)
		return types.StateStop
	}
	return types.StateServiceMenu
}

func (ui *UI) onServiceNetwork() types.UiState {
	ui.display.SetLines("for select net 0", "press 1") // FIXME extract message string

	next, e := ui.serviceWaitInput()
	if next != types.StateDefault {
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

		return types.StateServiceEnd
	}
	return types.StateServiceMenu
}

func (ui *UI) onServiceMoneyLoad(ctx context.Context) types.UiState {
	ui.ms.TestingDispense()
	alive := alive.NewAlive()
	defer func() {
		alive.Stop() // stop pending AcceptCredit
		alive.Wait()
		ui.ms.ResetMoney()
	}()
	alive.Add(2)
	ui.Service.askReport = true
	ui.display.SetLines("money-load", "0")
	go ui.ms.AcceptCredit(ctx, 500000, alive, ui.eventch)
	for {
		ui.display.SetLines("money-load", ui.ms.GetCredit().FormatCtx(ctx))
		switch e := ui.wait(ui.Service.resetTimeout); e.Kind {
		case types.EventInput:
			if e.Input.Source == "money" {
				ui.ms.BillEscrowReject()
				continue
			}
			if input.IsReject(&e.Input) {
				return types.StateServiceMenu
			}
		case types.EventMoneyCredit, types.EventMoneyPreCredit:
		case types.EventStop, types.EventTime, types.EventService:
			return types.StateServiceEnd
		default:
			ui.g.Tele.ErrorStr(fmt.Sprintf("vmid:%d imposiblya event:%d ", ui.g.Config.Tele.VmId, e.Kind))
			// panic(fmt.Sprintf("code error onServiceMoneyLoad unhandled event=%v", e))
		}
	}
}

func (ui *UI) onServiceReport(ctx context.Context) types.UiState {
	_ = ui.g.Tele.Report(ctx, true)
	if errs := ui.g.Engine.ExecList(ctx, "service-report", []string{"money.cashbox_zero"}); len(errs) != 0 {
		ui.g.Error(errors.Annotate(helpers.FoldErrors(errs), "service-report"))
	}
	return types.StateServiceMenu
}

func (ui *UI) onServiceEnd(ctx context.Context) types.UiState {
	_ = ui.g.Inventory.InventorySave()
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
		return types.StateBroken
	}
	watchdog.Enable()
	return types.StateDefault
}

func (ui *UI) serviceWaitInput() (types.UiState, types.InputEvent) {
	e := ui.wait(ui.Service.resetTimeout)
	switch e.Kind {
	case types.EventInput:
		return types.StateDefault, e.Input

	case types.EventMoneyCredit:
		ui.g.Log.Debugf("serviceWaitInput event=%v", e)
		return types.StateDefault, types.InputEvent{}

	case types.EventTime:
		// ui.g.Log.Infof("inactive=%v", inactive)
		ui.g.Log.Debugf("serviceWaitInput resetTimeout")
		return types.StateServiceEnd, types.InputEvent{}

	case types.EventLock:
		return types.StateLocked, types.InputEvent{}

	case types.EventService:
		ui.g.Log.Debugf("service exit")
		return types.StateServiceEnd, types.InputEvent{}

	case types.EventStop:
		ui.g.Log.Debugf("serviceWaitInput global stop")
		return types.StateServiceEnd, types.InputEvent{}

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
