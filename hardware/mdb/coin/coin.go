package coin

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/temoto/alive/v2"
)

const (
	TypeCount              = 16
	defaultDispenseTimeout = 15 * time.Second
)

//go:generate stringer -type=CoinRouting -trimprefix=Routing
type CoinRouting uint8

const (
	RoutingCashBox CoinRouting = 0
	RoutingTubes   CoinRouting = 1
	RoutingNotUsed CoinRouting = 2
	RoutingReject  CoinRouting = 3
)

//go:generate stringer -type=Features -trimprefix=Feature
type Features uint32

const (
	FeatureAlternativePayout Features = 1 << iota
	FeatureExtendedDiagnostic
	FeatureControlledManualFillPayout
	FeatureFTL
)

type CoinAcceptor struct { //nolint:maligned
	mdb.Device
	giveSmart       bool
	dispenseTimeout time.Duration
	pollmu          sync.Mutex // isolate active/idle polling

	// parsed from SETUP
	featureLevel      uint8
	supportedFeatures Features
	nominals          [TypeCount]currency.Nominal // final values, including all scaling factors
	scalingFactor     uint8
	typeRouting       uint16

	// dynamic state useful for external code
	tubesmu sync.Mutex
	tub     []tube
	tubes   currency.NominalGroup
}
type tube struct {
	nominal  currency.Nominal
	count    uint
	tubeFull bool
}

type CoinCommand byte

const (
	noCommand CoinCommand = iota
	Stop
)

var (
	// packetTubeStatus = mdb.MustPacketFromHex("0a", true)
	// packetExpIdent   = mdb.MustPacketFromHex("0f00", true)
	// packetDiagStatus = mdb.MustPacketFromHex("0f05", true)
	// packetPayoutPoll   = mdb.MustPacketFromHex("0f04", true)
	packetPayoutStatus = mdb.MustPacketFromHex("0f03", true)
)

var (
	ErrNoCredit      = errors.New("no Credit")
	ErrDoubleArrival = errors.New("double Arrival")
	ErrCoinRouting   = errors.New("coin Routing")
	ErrCoinJam       = errors.New("coin Jam")
	ErrSlugs         = errors.New("slugs")
)

func (ca *CoinAcceptor) init(ctx context.Context) error {
	g := state.GetGlobal(ctx)
	mdbus, err := g.Mdb()
	if err != nil {
		return err
	}
	ca.Device.Init(mdbus, 0x08, "coin", binary.BigEndian)
	config := &g.Config.Hardware.Mdb.Coin
	ca.giveSmart = config.GiveSmart || config.XXX_Deprecated_DispenseSmart
	ca.dispenseTimeout = helpers.IntSecondDefault(config.DispenseTimeoutSec, defaultDispenseTimeout)
	ca.scalingFactor = 1
	g.Engine.RegisterNewFunc(
		"coin.reset",
		func(ctx context.Context) error {
			return ca.CoinReset()
		},
	)
	g.Engine.Register("coin.dispence(?)",
		engine.FuncArg{Name: "coin.dispence", F: func(ctx context.Context, arg engine.Arg) (err error) {
			err = ca.Dispence(currency.Amount(arg))
			return err
		}})

	err = ca.CoinReset()
	return err
}

func (ca *CoinAcceptor) TestingDispense() {
	for _, n := range ca.tub {
		// fmt.Printf("\033[41m %v %v \033[0m\n", i, n.nominal)
		_, _ = ca.DispenceCoin(n.nominal)
	}
}

func (ca *CoinAcceptor) Dispence(amount currency.Amount) (err error) {
	if amount == 0 {
		return nil
	}
	dispenceAmount := amount
	m := "dispense coin: "
	ca.Log.Infof("dispence (%s) tubes (%v)", amount.Format100I(), ca.tubes)
	for {
		dispenseNominal, e := ca.maximumAvailableNominal(dispenceAmount, true)
		// AlexM FIXME
		if dispenseNominal == 1000 && ca.tubes.InTube(500) > ca.tubes.InTube(1000) {
			dispenseNominal = 500
		}
		// END AlexM FIXME
		if e != nil {
			ca.Log.Error(e)
			err = errors.Join(err, e)
			dispenceAmount = currency.Amount(dispenseNominal)
		}
		_, e = ca.DispenceCoin(dispenseNominal)
		m = m + dispenseNominal.Format100I() + " "
		err = errors.Join(err, e)
		dispenceAmount -= currency.Amount(dispenseNominal)
		if dispenceAmount <= 0 {
			break
		}
	}
	ca.Log.Info(m)
	return err
}

func (ca *CoinAcceptor) maximumAvailableNominal(notMore currency.Amount, priorityFullTubes bool) (n currency.Nominal, err error) {
	if len(ca.tub) <= 1 {
		return 0, fmt.Errorf("no money to dispense (%s)", notMore.Format100I())
	}
	if priorityFullTubes {
		for _, v := range ca.tub {
			if notMore >= currency.Amount(v.nominal) && v.tubeFull {
				return v.nominal, nil
			}
		}
	}
	for _, v := range ca.tub {
		if notMore >= currency.Amount(v.nominal) {
			return v.nominal, nil
		}
	}
	n = ca.tub[len(ca.tub)-1].nominal // get maximum avalible
	return n, fmt.Errorf("return bigged need:%s returned:%s", notMore.Format100I(), n.Format100I())
}

func (ca *CoinAcceptor) DispenceCoin(nominal currency.Nominal) (complete bool, err error) {
	if err = ca.TubeStatus(); err != nil {
		return false, err
	}
	inTubeBefore := ca.tubes.InTube(nominal)
	if inTubeBefore == 0 {
		return false, fmt.Errorf("can`t dispense, tube value = 0")
	}
	if inTubeBefore != ca.tubes.InTube(nominal) {
		ca.Log.Errorf("nominal %v preview and now tubes dismash preview value (%v) now(%v)", nominal, inTubeBefore, ca.tubes)
	}
	coinType := ca.nominalCoinType(nominal)
	request := mdb.MustPacketFromBytes([]byte{0x0d, (1 << 4) + uint8(coinType)}, true)
	if e := ca.Device.Tx(request, nil); e != nil {
		return false, fmt.Errorf("coin tx command. error:%v", e)
	}
	// timeout poll dispense 1 coin
	var errp error
	for i := 0; i < 50; i++ {
		time.Sleep(500 * time.Millisecond)
		var emptyResponse bool
		emptyResponse, errp = ca.pollF(nil)
		err = errors.Join(err, errp)
		if emptyResponse {
			// stop poll
			if ert := ca.TubeStatus(); err != nil {
				err = errors.Join(err, ert)
				return false, err
			}
			InTubeNow := ca.tubes.InTube(nominal)
			if inTubeBefore-1 == ca.tubes.InTube(nominal) {
				return true, err
			}
			ee := fmt.Errorf("dispense one coin error. tube dismach nominal %v value before(%v) now(%v)", nominal.Format100I(), inTubeBefore, InTubeNow)
			ca.Log.Warning(ee)
			return false, ee
		}
	}
	return false, fmt.Errorf("coin dispense poll timeout t tube statuserror:%v", err)
}

func (ca *CoinAcceptor) CoinRun(alive *alive.Alive, returnEvent func(money.ValidatorEvent)) {
	ca.EnableAccept(1000)
	refreshTime := time.Duration(200 * time.Millisecond)
	refreshTimer := time.NewTimer(refreshTime)
	ca.pollmu.Lock()
	stopRun := alive.StopChan()
	defer func() {
		refreshTimer.Stop()
		ca.DisableAccept()
		ca.pollmu.Unlock()
		alive.Done()
	}()
	again := true
	for again {
		select {
		case <-stopRun:
			again = false
		case <-refreshTimer.C:
			_, _ = ca.pollF(returnEvent)
			refreshTimer.Reset(refreshTime)
		}
	}
}

func (ca *CoinAcceptor) CoinReset() (err error) {
	ca.pollmu.Lock()
	defer ca.pollmu.Unlock()
	if err = ca.Device.Tx(ca.Device.PacketReset, nil); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	e := money.ValidatorEvent{}
	_, e.Err = ca.pollF(nil)
	time.Sleep(200 * time.Millisecond)
	_, e.Err = ca.pollF(nil)
	// if recive "changer was Reset" then read setup and status
	if e.Err != nil {
		return e.Err
	}
	return ca.readSetupAndStatus()
}

func (ca *CoinAcceptor) readSetupAndStatus() (err error) {
	if err = ca.setup(); err != nil {
		return err
	}
	if err = ca.CommandExpansionIdentification(); err != nil {
		return err
	}
	if err = ca.CommandFeatureEnable(FeatureExtendedDiagnostic); err != nil {
		return err
	}
	diagResult := new(DiagResult)
	if err = ca.ExpansionDiagStatus(diagResult); err != nil {
		return err
	}
	if !diagResult.OK() {
		ca.TeleError(errors.New("coin reset error:" + diagResult.Error()))
	}
	if err = ca.TubeStatus(); err != nil {
		return err
	}
	return nil
}

func (ca *CoinAcceptor) pollF(returnEvent func(money.ValidatorEvent)) (empty bool, err error) {
	var response mdb.Packet
	if err := ca.Device.Tx(ca.Device.PacketPoll, &response); err != nil {
		ca.Log.Errorf("coin poll TX error:%v", err)
		return false, err
	}
	rb := response.Bytes()
	if len(rb) == 0 {
		return true, nil
	}
	for i := 0; i < len(rb); i++ {
		var e money.ValidatorEvent
		if rb[i]>>6 > 0 {
			e = ca.decodeByte(rb[i], rb[i+1])
			i++
		} else {
			e = ca.decodeByte(rb[i])
		}
		if returnEvent != nil && e.Event != money.NoEvent {
			returnEvent(e)
		}
		err = errors.Join(err, e.Err)
	}
	return false, err
}

func (ca *CoinAcceptor) decodeByte(b byte, b2 ...byte) (ve money.ValidatorEvent) {
	switch b {
	case 0x01: // Escrow request
		return money.ValidatorEvent{Event: money.CoinRejectKey}
	case 0x02: // Changer Payout Busy
	case 0x03: // No Credit
		return money.ValidatorEvent{Err: fmt.Errorf("coin. no credit. coin valid and not arrived")}
	case 0x04: // Defective Tube Sensor
		return money.ValidatorEvent{Err: fmt.Errorf("coin defective tube sensor")}
	case 0x05: // Double Arrival
		ca.Log.Warning("coin double arrival")
	// 	return money.PollItem{Status: money.StatusInfo, Error: ErrDoubleArrival}
	case 0x06: // Acceptor Unplugged
		return money.ValidatorEvent{Err: fmt.Errorf("coin acceptor unplugged")}
	case 0x07: // Tube Jam
		return money.ValidatorEvent{Err: fmt.Errorf("tube jam")}
	case 0x08: // ROM checksum error
		return money.ValidatorEvent{Err: fmt.Errorf("ROM checksum error")}
	case 0x09: // Coin Routing Error
		return money.ValidatorEvent{Err: fmt.Errorf("coin Routing Error")}
	case 0x0a: // Changer Busy
	case 0x0b: // Changer was Reset
		// if err := ca.readSetupAndStatus(); err != nil {
		// 	return money.ValidatorEvent{Err: fmt.Errorf("coin read setupConfig Error (%v)", err)}
		// }
		ca.Log.Info("coin reset complete")
	case 0x0c: // Coin Jam
		return money.ValidatorEvent{Err: fmt.Errorf("coin jam")}
	case 0x0d: // Possible Credited Coin Removal
		return money.ValidatorEvent{Err: fmt.Errorf("possible Credited Coin Removal")}
	}

	if b>>5 == 1 { // Slug count 001xxxxx
		slugs := b & 0x1f
		ca.Log.WarningF("slug count(%v)", slugs)
	}
	if b>>6 == 1 { // Coins Deposited
		// 	// b=01yyxxxx b2=number of coins in tube
		// 	// yy = coin routing
		// 	// xxxx = coin type
		coinType := b & 0xf
		routing := CoinRouting((b >> 4) & 3)
		ve.Nominal = ca.coinTypeNominal(coinType)
		ve.Event = money.Stacked
		// 	pi := money.PollItem{
		// 		DataNominal: ca.coinTypeNominal(coinType),
		// 		DataCount:   1,
		// 	}
		m := fmt.Sprintf("coin (%s) ", ve.Nominal.Format100I())
		switch routing {
		case RoutingCashBox:
			ve.Event = money.CoinCredit
			m = m + "income to cashbox"
		case RoutingTubes:
			ve.Event = money.CoinCredit
			ca.tubes.Add(ve.Nominal)
			m = m + "income to tube"
		case RoutingNotUsed:
			ve.Event = money.NoEvent
			m = m + " !!! routingNotUsed HZ what it"
		case RoutingReject:
			ve.Event = money.NoEvent
			m = m + " !!! reject"
		default:
			fmt.Printf("\033[41m panic \033[0m\n")
		}
		ca.Log.Info(m)
	}
	if b&0x80 != 0 { // Coins Dispensed Manually
		// b=1yyyxxxx b2=number of coins in tube
		// yyy = coins dispensed
		// xxxx = coin type
		count := (b >> 4) & 7
		nominal := ca.coinTypeNominal(b & 0xf)
		// return money.PollItem{Status: money.StatusDispensed, DataNominal: nominal, DataCount: count}
		fmt.Printf("\033[41m dispense count(%v) nominal(%v) tubevoint(%v) \033[0m\n", count, nominal, b2)
		return money.ValidatorEvent{}
	}

	// err := oerr.Errorf("parsePollItem unknown=%x", b)
	return ve
}

func (ca *CoinAcceptor) setup() error {
	const tag = deviceName + ".setup"
	const expectLengthMin = 7
	if err := ca.Device.TxReadSetup(); err != nil {
		return err
	}
	bs := ca.Device.SetupResponse.Bytes()
	if len(bs) < expectLengthMin {
		return errors.New("coin setup response small" + fmt.Sprintf("%v", bs))
	}
	ca.featureLevel = bs[0]
	ca.scalingFactor = bs[3]
	ca.typeRouting = ca.Device.ByteOrder.Uint16(bs[5 : 5+2])
	for i, sf := range bs[7 : 7+TypeCount] {
		if sf == 0 {
			continue
		}
		n := currency.Nominal(sf) * currency.Nominal(ca.scalingFactor)
		ca.Device.Log.Debugf("coin [%d] sf=%d nominal=(%d)%s",
			i, sf, n, currency.Amount(n).Format100I())
		ca.nominals[i] = n
	}
	ca.tubes.SetValid(ca.nominals[:])
	ca.Device.Log.Debugf("%s Changer Feature Level: %d", tag, ca.featureLevel)
	ca.Device.Log.Debugf("%s Country / Currency Code: %x", tag, bs[1:1+2])
	ca.Device.Log.Debugf("%s Coin Scaling Factor: %d", tag, ca.scalingFactor)
	ca.Device.Log.Debugf("%s Decimal Places: %d", tag, bs[4])
	ca.Device.Log.Debugf("%s Coin Type Routing: %b", tag, ca.typeRouting)
	ca.Device.Log.Debugf("%s Coin Type Credit: %x %v", tag, bs[7:], ca.nominals)
	return nil
}

func (ca *CoinAcceptor) CommandExpansionIdentification() error {
	const tag = deviceName + ".ExpId"
	const expectLength = 33
	request := mdb.MustPacketFromHex("0f00", true)
	response := mdb.Packet{}
	err := ca.Device.Tx(request, &response)
	if err != nil {
		return errors.New(tag + err.Error())
	}
	ca.Device.Log.Debugf("%s response=(%d)%s", tag, response.Len(), response.Format())
	bs := response.Bytes()
	if len(bs) < expectLength {
		return errors.New("coin command get ExpansionIdentification return: " + response.Format())
	}
	ca.supportedFeatures = Features(ca.Device.ByteOrder.Uint32(bs[29 : 29+4]))
	ca.Device.Log.Infof("%s Manufacturer Code: '%s'", tag, bs[0:0+3])
	ca.Device.Log.Debugf("%s Serial Number: '%s'", tag, string(bs[3:3+12]))
	ca.Device.Log.Debugf("%s Model #/Tuning Revision: '%s'", tag, string(bs[15:15+12]))
	ca.Device.Log.Debugf("%s Software Version: %x", tag, bs[27:27+2])
	ca.Device.Log.Debugf("%s Optional Features: %b", tag, ca.supportedFeatures)
	return nil
}

func (ca *CoinAcceptor) CommandFeatureEnable(requested Features) error {
	const tag = deviceName + ".FeatureEnable"
	f := requested & ca.supportedFeatures
	buf := [6]byte{0x0f, 0x01}
	ca.Device.ByteOrder.PutUint32(buf[2:], uint32(f))
	request := mdb.MustPacketFromBytes(buf[:], true)
	if err := ca.Device.Tx(request, nil); err != nil {
		return errors.New(tag + err.Error())
	}
	return nil
}

func (ca *CoinAcceptor) TubeStatus() error {
	const tag = deviceName + ".tubestatus"
	const expectLengthMin = 2
	request := mdb.MustPacketFromHex("0a", true)
	response := mdb.Packet{}
	if err := ca.Device.Tx(request, &response); err != nil {
		return err
	}
	bs := response.Bytes()
	if len(bs) < expectLengthMin {
		return errors.New("tube status response small. response: " + response.String())
	}
	fulls := ca.Device.ByteOrder.Uint16(bs[0:2])
	counts := bs[2:18]
	ca.tubesmu.Lock()
	defer ca.tubesmu.Unlock()
	ca.tubes.Clear()
	ca.tub = make([]tube, 0)
	ct := make(map[uint32]bool)
	for coinType := uint8(0); coinType < TypeCount; coinType++ {
		full := (fulls & (1 << coinType)) != 0
		nominal := ca.coinTypeNominal(coinType)
		if counts[coinType] != 0 {
			ct[uint32(nominal)] = full
		}
		if full && counts[coinType] == 0 {
		} else if counts[coinType] != 0 {
			if err := ca.tubes.AddMany(nominal, uint(counts[coinType])); err != nil {
				return err
			}
		}
	}
	for k, v := range ct {
		ca.tub = append(ca.tub, tube{currency.Nominal(k), ca.tubes.InTube(currency.Nominal(k)), v})
	}
	sort.Slice(ca.tub[:], func(i, j int) bool {
		return ca.tub[i].nominal > ca.tub[j].nominal
	})
	ca.Device.Log.Debugf("%s tubes=%s", tag, ca.tubes.String())
	return nil
}

func (ca *CoinAcceptor) coinTypeNominal(b byte) currency.Nominal {
	if b >= TypeCount {
		ca.Device.Log.Errorf("invalid coin type: %d", b)
		return 0
	}
	return ca.nominals[b]
}

func (ca *CoinAcceptor) ExpansionDiagStatus(result *DiagResult) error {
	const tag = deviceName + ".ExpansionSendDiagStatus"
	if ca.supportedFeatures&FeatureExtendedDiagnostic == 0 {
		ca.Device.Log.Debugf("%s feature is not supported", tag)
		return nil
	}
	request := mdb.MustPacketFromHex("0f05", true)
	response := mdb.Packet{}
	err := ca.Device.Tx(request, &response)
	if err != nil {
		return err
	}
	dr, err := parseDiagResult(response.Bytes(), ca.Device.ByteOrder)
	ca.Device.Log.Infof("%s result=%s", tag, dr.Error())
	if result != nil {
		*result = dr
	}
	return err
}

func (ca *CoinAcceptor) EnableAccept(maximumNominal currency.Amount) (err error) {
	enableBitset := uint16(0)
	if maximumNominal != 0 {
		for i, n := range ca.nominals {
			if n == 0 {
				continue
			}
			if currency.Amount(n) <= maximumNominal {
				enableBitset |= 1 << uint(i)
			}
		}
	}
	buf := [5]byte{0x0c}
	ca.Device.ByteOrder.PutUint16(buf[1:], enableBitset)
	ca.Device.ByteOrder.PutUint16(buf[3:], enableBitset)
	request := mdb.MustPacketFromBytes(buf[:], true)
	if err = ca.Device.Tx(request, nil); err != nil {
		return fmt.Errorf("bill. send enable accept packet not complete. (%v)", err)
	}
	return nil
}

func (ca *CoinAcceptor) DisableAccept() {
	buf := [5]byte{0x0c, 00, 00, 00, 00}
	request := mdb.MustPacketFromBytes(buf[:], true)
	if err := ca.Device.Tx(request, nil); err != nil {
		ca.Device.TeleError(fmt.Errorf("bill. send disable accept packet not complete. (%v)", err))
	}
}

// -----------------------------------------------------------------

func (ca *CoinAcceptor) AcceptMax(max currency.Amount) engine.Doer {
	// config := state.GetConfig(ctx)
	enableBitset := uint16(0)

	if max != 0 {
		for i, n := range ca.nominals {
			if n == 0 {
				continue
			}
			if currency.Amount(n) <= max {
				// TODO consult config
				// _ = config
				enableBitset |= 1 << uint(i)
			}
		}
	}

	return ca.NewCoinType(enableBitset, 0xffff)
}

func (ca *CoinAcceptor) SupportedNominals() []currency.Nominal {
	ns := make([]currency.Nominal, 0, TypeCount)
	for _, n := range ca.nominals {
		if n != 0 {
			ns = append(ns, n)
		}
	}
	return ns
}

// func (ca *CoinAcceptor) Run(ctx context.Context, alive *alive.Alive, fun func(money.PollItem) bool) {
// 	var stopch <-chan struct{}
// 	if alive != nil {
// 		defer alive.Done()
// 		stopch = alive.StopChan()
// 	}
// 	pd := mdb.PollDelay{}
// 	parse := ca.pollFun(fun)
// 	var active bool
// 	var err error
// 	again := true
// 	for again {
// 		response := mdb.Packet{}
// 		ca.pollmu.Lock()
// 		err = ca.Device.TxKnown(ca.Device.PacketPoll, &response)
// 		if err == nil {
// 			active, err = parse(response)
// 		}
// 		ca.pollmu.Unlock()
// 		again = (alive != nil) && (alive.IsRunning()) && pd.Delay(&ca.Device, active, err != nil, stopch)
// 		// TODO try pollmu.Unlock() here
// 	}
// }

// func (ca *CoinAcceptor) pollFun(fun func(money.PollItem) bool) mdb.PollRequestFunc {
// 	const tag = deviceName + ".poll"
// 	return func(p mdb.Packet) (bool, error) {
// 		bs := p.Bytes()
// 		if len(bs) == 0 {
// 			return false, nil
// 		}
// 		var pi money.PollItem
// 		skip := false
// 		for i, b := range bs {
// 			if skip {
// 				skip = false
// 				continue
// 			}
// 			b2 := byte(0)
// 			if i+1 < len(bs) {
// 				b2 = bs[i+1]
// 			}
// 			pi, skip = ca.parsePollItem(b, b2)
// 			switch pi.Status {
// 			case money.StatusInfo:
// 				ca.Device.Log.Infof("%s/info: %s", tag, pi.String())
// 				// TODO telemetry
// 			case money.StatusError:
// 				ca.Device.TeleError(oerr.Annotate(pi.Error, tag))
// 			case money.StatusFatal:
// 				ca.Device.TeleError(oerr.Annotate(pi.Error, tag))
// 			case money.StatusBusy:
// 			case money.StatusWasReset:
// 				ca.Device.Log.Infof("coin was reset")
// 				return false, nil
// 				// TODO telemetry
// 			default:
// 				fun(pi)
// 			}
// 		}
// 		return true, nil
// 	}
// }

// func (ca *CoinAcceptor) newIniter() engine.Doer {
// 	const tag = deviceName + ".init"
// 	return engine.NewSeq(tag).
// 		Append(ca.Device.DoReset).
// 		Append(engine.Func{Name: tag + "/poll", F: func(ctx context.Context) error {
// 			ca.Run(ctx, nil, func(money.PollItem) bool { return false })
// 			return nil
// 		}}).
// 		// Append(ca.newSetuper()).
// 		Append(engine.Func0{Name: tag + "/expid-diag", F: func() error {
// 			if err := ca.CommandExpansionIdentification(); err != nil {
// 				return err
// 			}
// 			if err := ca.CommandFeatureEnable(FeatureExtendedDiagnostic); err != nil {
// 				return err
// 			}
// 			diagResult := new(DiagResult)
// 			if err := ca.ExpansionDiagStatus(diagResult); err != nil {
// 				return err
// 			}
// 			return nil
// 		}}).
// 		Append(engine.Func0{Name: tag + "/tube-status", F: ca.TubeStatus})
// }

// func (ca *CoinAcceptor) newSetuper() engine.Doer {
// 	const tag = deviceName + ".setup"
// 	return engine.Func{Name: tag, F: func(ctx context.Context) error {
// 		const expectLengthMin = 7
// 		if err := ca.Device.TxSetup(); err != nil {
// 			return oerr.Annotate(err, tag)
// 		}
// 		bs := ca.Device.SetupResponse.Bytes()
// 		if len(bs) < expectLengthMin {
// 			return oerr.Errorf("%s response=%s expected >= %d bytes",
// 				tag, ca.Device.SetupResponse.Format(), expectLengthMin)
// 		}
// 		ca.featureLevel = bs[0]
// 		ca.scalingFactor = bs[3]
// 		ca.typeRouting = ca.Device.ByteOrder.Uint16(bs[5 : 5+2])
// 		for i, sf := range bs[7 : 7+TypeCount] {
// 			if sf == 0 {
// 				continue
// 			}
// 			n := currency.Nominal(sf) * currency.Nominal(ca.scalingFactor)
// 			ca.Device.Log.Debugf("%s [%d] sf=%d nominal=(%d)%s",
// 				tag, i, sf, n, currency.Amount(n).FormatCtx(ctx))
// 			ca.nominals[i] = n
// 		}
// 		ca.tubes.SetValid(ca.nominals[:])
// 		ca.Device.Log.Debugf("%s Changer Feature Level: %d", tag, ca.featureLevel)
// 		ca.Device.Log.Debugf("%s Country / Currency Code: %x", tag, bs[1:1+2])
// 		ca.Device.Log.Debugf("%s Coin Scaling Factor: %d", tag, ca.scalingFactor)
// 		// ca.Device.Log.Debugf("%s Decimal Places: %d", tag, bs[4])
// 		ca.Device.Log.Debugf("%s Coin Type Routing: %b", tag, ca.typeRouting)
// 		ca.Device.Log.Debugf("%s Coin Type Credit: %x %v", tag, bs[7:], ca.nominals)
// 		return nil
// 	}}
// }

// func (ca *CoinAcceptor) TubeStatus() error {
// 	const tag = deviceName + ".tubestatus"
// 	const expectLengthMin = 2
// 	response := mdb.Packet{}
// 	err := ca.Device.TxKnown(packetTubeStatus, &response)
// 	if err != nil {
// 		return oerr.Annotate(err, tag)
// 	}
// 	ca.Device.Log.Debugf("%s response=(%d)%s", tag, response.Len(), response.Format())
// 	bs := response.Bytes()
// 	if len(bs) < expectLengthMin {
// 		return oerr.Errorf("%s response=%s expected >= %d bytes",
// 			tag, response.Format(), expectLengthMin)
// 	}
// 	fulls := ca.Device.ByteOrder.Uint16(bs[0:2])
// 	counts := bs[2:18]
// 	ca.Device.Log.Debugf("%s fulls=%b counts=%v", tag, fulls, counts)
// 	ca.tubesmu.Lock()
// 	defer ca.tubesmu.Unlock()
//		ca.tubes.Clear()
//		for coinType := uint8(0); coinType < TypeCount; coinType++ {
//			full := (fulls & (1 << coinType)) != 0
//			nominal := ca.coinTypeNominal(coinType)
//			if full && counts[coinType] == 0 {
//				nominalString := currency.Amount(nominal).Format100I() // TODO use FormatCtx(ctx)
//				ca.Device.TeleError(fmt.Errorf("%s coinType=%d nominal=%s problem (jam/sensor/etc)", tag, coinType, nominalString))
//			} else if counts[coinType] != 0 {
//				if err := ca.tubes.AddMany(nominal, uint(counts[coinType])); err != nil {
//					return oerr.Annotatef(err, "%s tubes.Add coinType=%d", tag, coinType)
//				}
//			}
//		}
//		ca.Device.Log.Debugf("%s tubes=%s", tag, ca.tubes.String())
//		return nil
//	}

func (ca *CoinAcceptor) Tubes() *currency.NominalGroup {
	ca.tubesmu.Lock()
	result := ca.tubes.Copy()
	ca.tubesmu.Unlock()
	return result
}

func (ca *CoinAcceptor) NewCoinType(accept, dispense uint16) engine.Doer {
	buf := [5]byte{0x0c}
	ca.Device.ByteOrder.PutUint16(buf[1:], accept)
	ca.Device.ByteOrder.PutUint16(buf[3:], dispense)
	request := mdb.MustPacketFromBytes(buf[:], true)
	return engine.Func0{Name: deviceName + ".CoinType", F: func() error {
		return ca.Device.TxKnown(request, nil)
	}}
}

// func (ca *CoinAcceptor) CommandExpansionIdentification() error {
// 	const tag = deviceName + ".ExpId"
// 	const expectLength = 33
// 	request := packetExpIdent
// 	response := mdb.Packet{}
// 	err := ca.Device.TxMaybe(request, &response)
// 	if err != nil {
// 		if oerr.Cause(err) == mdb.ErrTimeoutMDB {
// 			ca.Device.Log.Infof("%s request=%x not supported (timeout)", tag, request.Bytes())
// 			return nil
// 		}
// 		return oerr.Annotate(err, tag)
// 	}
// 	ca.Device.Log.Debugf("%s response=(%d)%s", tag, response.Len(), response.Format())
// 	bs := response.Bytes()
// 	if len(bs) < expectLength {
// 		return oerr.Errorf("%s response=%s expected %d bytes", tag, response.Format(), expectLength)
// 	}
// 	ca.supportedFeatures = Features(ca.Device.ByteOrder.Uint32(bs[29 : 29+4]))
// 	ca.Device.Log.Infof("%s Manufacturer Code: '%s'", tag, bs[0:0+3])
// 	ca.Device.Log.Debugf("%s Serial Number: '%s'", tag, string(bs[3:3+12]))
// 	ca.Device.Log.Debugf("%s Model #/Tuning Revision: '%s'", tag, string(bs[15:15+12]))
// 	ca.Device.Log.Debugf("%s Software Version: %x", tag, bs[27:27+2])
// 	ca.Device.Log.Debugf("%s Optional Features: %b", tag, ca.supportedFeatures)
// 	return nil
// }

// CommandExpansionSendDiagStatus returns:
// - `nil` if command is not supported by device, result is not modified
// - otherwise returns nil or MDB/parse error, result set to valid DiagResult
// func (ca *CoinAcceptor) ExpansionDiagStatus(result *DiagResult) error {
// 	const tag = deviceName + ".ExpansionSendDiagStatus"

// 	if ca.supportedFeatures&FeatureExtendedDiagnostic == 0 {
// 		ca.Device.Log.Debugf("%s feature is not supported", tag)
// 		return nil
// 	}
// 	response := mdb.Packet{}
// 	err := ca.Device.TxMaybe(packetDiagStatus, &response)
// 	if err != nil {
// 		if oerr.Cause(err) == mdb.ErrTimeoutMDB {
// 			ca.Device.Log.Infof("%s request=%x not supported (timeout)", tag, packetDiagStatus.Bytes())
// 			return nil
// 		}
// 		return oerr.Annotate(err, tag)
// 	}
// 	dr, err := parseDiagResult(response.Bytes(), ca.Device.ByteOrder)
// 	ca.Device.Log.Debugf("%s result=%s", tag, dr.Error())
// 	if result != nil {
// 		*result = dr
// 	}
// 	return oerr.Annotate(err, tag)
// }

// func (ca *CoinAcceptor) CommandFeatureEnable(requested Features) error {
// 	const tag = deviceName + ".FeatureEnable"
// 	f := requested & ca.supportedFeatures
// 	buf := [6]byte{0x0f, 0x01}
// 	ca.Device.ByteOrder.PutUint32(buf[2:], uint32(f))
// 	request := mdb.MustPacketFromBytes(buf[:], true)
// 	err := ca.Device.TxMaybe(request, nil)
// 	if oerr.Cause(err) == mdb.ErrTimeoutMDB {
// 		ca.Device.Log.Infof("%s request=%x not supported (timeout)", tag, request.Bytes())
// 		return nil
// 	}
// 	return oerr.Annotate(err, tag)
// }

// func (ca *CoinAcceptor) coinTypeNominal(b byte) currency.Nominal {
// 	if b >= TypeCount {
// 		ca.Device.Log.Errorf("invalid coin type: %d", b)
// 		return 0
// 	}
// 	return ca.nominals[b]
// }

func (ca *CoinAcceptor) nominalCoinType(nominal currency.Nominal) int8 {
	for ct, n := range ca.nominals {
		if n == nominal && ((1<<uint(ct))&ca.typeRouting != 0) {
			return int8(ct)
		}
	}
	return -1
}

func (ca *CoinAcceptor) parsePollItem(b, b2 byte) (money.PollItem, bool) {
	switch b {
	case 0x01: // Escrow request
		return money.PollItem{Status: money.StatusReturnRequest}, false
	case 0x02: // Changer Payout Busy
		return money.PollItem{Status: money.StatusBusy}, false
	// high
	case 0x03: // No Credit
		return money.PollItem{Status: money.StatusError, Error: ErrNoCredit}, false
	// high
	case 0x04: // Defective Tube Sensor
		return money.PollItem{Status: money.StatusFatal, Error: money.ErrSensor}, false
	case 0x05: // Double Arrival
		return money.PollItem{Status: money.StatusInfo, Error: ErrDoubleArrival}, false
	// high
	case 0x06: // Acceptor Unplugged
		return money.PollItem{Status: money.StatusFatal, Error: money.ErrNoStorage}, false
	// high
	case 0x07: // Tube Jam
		return money.PollItem{Status: money.StatusFatal, Error: money.ErrJam}, false
	// high
	case 0x08: // ROM checksum error
		return money.PollItem{Status: money.StatusFatal, Error: money.ErrROMChecksum}, false
	// high
	case 0x09: // Coin Routing Error
		return money.PollItem{Status: money.StatusError, Error: ErrCoinRouting}, false
	case 0x0a: // Changer Busy
		return money.PollItem{Status: money.StatusBusy}, false
	case 0x0b: // Changer was Reset
		return money.PollItem{Status: money.StatusWasReset}, false
	// high
	case 0x0c: // Coin Jam
		return money.PollItem{Status: money.StatusFatal, Error: ErrCoinJam}, false
	case 0x0d: // Possible Credited Coin Removal
		return money.PollItem{Status: money.StatusError, Error: money.ErrFraud}, false
	}
	if b>>5 == 1 { // Slug count 001xxxxx
		slugs := b & 0x1f
		ca.Device.Log.Debugf("Number of slugs: %d", slugs)
		return money.PollItem{Status: money.StatusInfo, Error: ErrSlugs, DataCount: slugs}, false
	}
	if b>>6 == 1 { // Coins Deposited
		// b=01yyxxxx b2=number of coins in tube
		// yy = coin routing
		// xxxx = coin type
		coinType := b & 0xf
		routing := CoinRouting((b >> 4) & 3)
		pi := money.PollItem{
			DataNominal: ca.coinTypeNominal(coinType),
			DataCount:   1,
		}
		switch routing {
		case RoutingCashBox:
			pi.Status = money.StatusCredit
			pi.DataCashbox = true
		case RoutingTubes:
			pi.Status = money.StatusCredit
		case RoutingNotUsed:
			pi.Status = money.StatusError
			// pi.Error = oerr.Errorf("routing=notused b=%x pi=%s", b, pi.String())
			ers := fmt.Sprintf("routing=notused b=%x pi=%s", b, pi.String())
			pi.Error = errors.New(ers)
		case RoutingReject:
			pi.Status = money.StatusRejected
		default:
			// pi.Status = money.StatusFatal
			ers := fmt.Sprintf("code error b=%x routing=%b", b, routing)
			panic(errors.New(ers))
		}
		ca.Device.Log.Debugf("deposited coinType=%d routing=%v pi=%s", coinType, routing, pi.String())
		return pi, true
	}
	if b&0x80 != 0 { // Coins Dispensed Manually
		// b=1yyyxxxx b2=number of coins in tube
		// yyy = coins dispensed
		// xxxx = coin type
		count := (b >> 4) & 7
		nominal := ca.coinTypeNominal(b & 0xf)
		return money.PollItem{Status: money.StatusDispensed, DataNominal: nominal, DataCount: count}, true
	}
	ers := fmt.Sprintf("parsePollItem unknown=%x", b)
	err := errors.New(ers)
	return money.PollItem{Status: money.StatusFatal, Error: err}, false
}
