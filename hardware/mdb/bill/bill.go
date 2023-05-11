// Package bill incapsulates work with bill validators.
package bill

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/internal/state"

	oerr "github.com/juju/errors"
	"github.com/temoto/alive/v2"
)

type BillValidator struct { //nolint:maligned
	reseted bool
	mdb.Device
	pollmu        sync.Mutex // isolate active/idle polling
	configScaling uint16

	// DoEscrowAccept engine.Func
	// DoEscrowReject engine.Func
	// DoStacker      engine.Func
	billstate BllStateType
	billCmd   chan BillCommand

	// parsed from SETUP
	featureLevel      uint8
	supportedFeatures uint32
	escrowSupported   bool
	nominals          [16]currency.Nominal // final values, includes all scaling factors
	manafacturer      string

	// dynamic state useful for external code
	EscrowBill   currency.Nominal // assume only one bill may be in escrow position
	stackerFull  bool
	stackerCount uint32
}

type BllStateType byte

const (
	noStatae BllStateType = iota
	Broken
	reseted
	WaitConfigure
	notReady
	ready
)

type BillCommand byte

const (
	noCommand BillCommand = iota
	Stop
	ExecuteStop
	Accept
	ExecuteAccept
	Reject
	ExecuteReject
)

func (bv *BillValidator) init(ctx context.Context) error {
	const tag = deviceName + ".init"
	g := state.GetGlobal(ctx)
	mdbus, err := g.Mdb()
	if err != nil {
		return oerr.Annotate(err, tag)
	}
	bv.Device.Init(mdbus, 0x30, "bill", binary.BigEndian)
	config := g.Config.Hardware.Mdb.Bill
	bv.configScaling = 100
	if config.ScalingFactor != 0 {
		bv.configScaling = uint16(config.ScalingFactor)
	}
	bv.billCmd = make(chan BillCommand, 2)
	g.Engine.RegisterNewFunc(
		"bill.reset",
		func(ctx context.Context) error {
			return bv.BillReset()
		},
	)
	if err = bv.BillReset(); err != nil {
		return err
	}
	return nil
}

func (bv *BillValidator) SendCommand(cmd BillCommand) {
	select {
	case bv.billCmd <- cmd:
	default:
	}
}

func (bv *BillValidator) BillReset() (err error) {
	bv.reseted = false
	bv.setState(Broken)
	bv.SendCommand(Stop) // stop pre-polling. if running
	bv.pollmu.Lock()
	defer bv.pollmu.Unlock()
	if err = bv.Device.Tx(bv.Device.PacketReset, nil); err != nil {
		return err
	}
	mbe := money.BillEvent{}
	if err := bv.pollF(nil); err != nil || mbe.Err != nil {
		err = errors.Join(err, mbe.Err)
		return err
	}
	if !bv.reseted {
		err := errors.New("bill. no complete reset response")
		// ICT validator will not reset until it returns data (ICT валиадатор не отработает сброс, пока не отдаст данные)
		if e := bv.pollF(nil); err != nil || mbe.Err != nil {
			err = errors.Join(e, mbe.Err)
		}
		return err
	}
	if err = bv.setup(); err != nil {
		return err
	}
	if err = bv.commandExpansionIdentificationOptions(); err != nil {
		if _, ok := err.(mdb.FeatureNotSupported); ok {
			if err = bv.commandExpansionIdentification(); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if bv.manafacturer == "ICT" {
		time.Sleep(15 * time.Second)
	}
	if err := bv.pollF(nil); err != nil || mbe.Err != nil {
		err = errors.Join(err, mbe.Err)
		return err
	}
	if bv.readStacker() == -1 {
		return errors.New("bill. read stacker count, after reset")
	}
	bv.Log.Info("bill reset complete")
	bv.setState(WaitConfigure)
	return nil
}

func (bv *BillValidator) setup() error {
	const expectLength = 27
	var billFactors [16]uint8
	if err := bv.Device.TxReadSetup(); err != nil {
		return oerr.Trace(err)
	}
	bs := bv.Device.SetupResponse.Bytes()
	if len(bs) < expectLength {
		return fmt.Errorf("bill validator SETUP response=%s expected %d bytes", bv.Device.SetupResponse.Format(), expectLength)
	}

	bv.featureLevel = bs[0]
	currencyCode := bs[1:3]
	scalingFactor := bv.Device.ByteOrder.Uint16(bs[3:5])
	decimalPlaces := bs[5]
	scalingFinal := currency.Nominal(scalingFactor) * currency.Nominal(bv.configScaling)
	for i := decimalPlaces; i > 0 && scalingFinal > 10; i-- {
		scalingFinal /= 10
	}
	stackerCap := bv.Device.ByteOrder.Uint16(bs[6:8])
	billSecurityLevels := bv.Device.ByteOrder.Uint16(bs[8:10])
	bv.escrowSupported = bs[10] == 0xff

	bv.Log.Debugf("Bill Type Scaling Factors: %3v", bs[11:])
	for i, sf := range bs[11:] {
		if i >= 16 {
			bv.Log.Errorf("CRITICAL bill SETUP type factors count=%d > expected=%d", len(bs[11:]), 16)
			break
		}
		billFactors[i] = sf
		bv.nominals[i] = currency.Nominal(sf) * scalingFinal
	}
	bv.Log.Debugf("Bill Type calc. nominals:  %3v", bv.nominals)
	bv.Log.Debugf("Bill Validator Feature Level: %d", bv.featureLevel)
	bv.Log.Debugf("Country / Currency Code: %x", currencyCode)
	bv.Log.Debugf("Bill Scaling Factor: %d Decimal Places: %d final scaling: %d", scalingFactor, decimalPlaces, scalingFinal)
	bv.Log.Debugf("Stacker Capacity: %d", stackerCap)
	bv.Log.Debugf("Bill Security Levels: %016b", billSecurityLevels)
	bv.Log.Debugf("Escrow/No Escrow: %t", bv.escrowSupported)
	bv.Log.Debugf("Bill Type Credit: %x %v", bs[11:], bv.nominals)
	return nil
}

func (bv *BillValidator) commandExpansionIdentification() error {
	const tag = deviceName + ".ExpId"
	const expectLength = 29
	request := mdb.MustPacketFromHex("3700", true)
	response := mdb.Packet{}
	if err := bv.Device.Tx(request, &response); err != nil {
		return oerr.Annotate(err, tag)
	}
	bs := response.Bytes()
	bv.Log.Debugf("%s response=%x", tag, bs)
	if len(bs) < expectLength {
		return fmt.Errorf("%s response=%x length=%d expected=%d", tag, bs, len(bs), expectLength)
	}
	bv.manafacturer = string(bs[0 : 0+3])
	bv.Log.Infof("%s Manufacturer Code: '%s'", tag, bs[0:0+3])
	bv.Log.Debugf("%s Serial Number: '%s'", tag, string(bs[3:3+12]))
	bv.Log.Debugf("%s Model #/Tuning Revision: '%s'", tag, string(bs[15:15+12]))
	bv.Log.Debugf("%s Software Version: %x", tag, bs[27:27+2])
	return nil
}

// func (bv *BillValidator) commandFeatureEnable(requested Features) error {
// 	f := requested & bv.supportedFeatures
// 	buf := [6]byte{0x37, 0x01}
// 	bv.Device.ByteOrder.PutUint32(buf[2:], uint32(f))
// 	request := mdb.MustPacketFromBytes(buf[:], true)
// 	err := bv.Device.TxMaybe(request, nil)
// 	return errors.Annotate(err, deviceName+".FeatureEnable")
// }

func (bv *BillValidator) commandExpansionIdentificationOptions() error {
	const tag = deviceName + ".ExpIdOptions"
	if bv.featureLevel < 2 {
		return mdb.FeatureNotSupported(tag + " is level 1")
	}
	const expectLength = 33
	request := mdb.MustPacketFromHex("3702", true)
	response := mdb.Packet{}
	err := bv.Device.Tx(request, &response)
	if err != nil {
		return oerr.Annotate(err, tag)
	}
	bv.Log.Debugf("%s response=(%d)%s", tag, response.Len(), response.Format())
	bs := response.Bytes()
	if len(bs) < expectLength {
		return fmt.Errorf("%s response=%s expected %d bytes", tag, response.Format(), expectLength)
	}
	bv.supportedFeatures = bv.Device.ByteOrder.Uint32(bs[29 : 29+4])
	bv.Log.Infof("%s Manufacturer Code: '%s'", tag, bs[0:0+3])
	bv.Log.Infof("%s Serial Number: '%s'", tag, string(bs[3:3+12]))
	bv.Log.Infof("%s Model #/Tuning Revision: '%s'", tag, string(bs[15:15+12]))
	bv.Log.Infof("%s Software Version: %x", tag, bs[27:27+2])
	bv.Log.Infof("%s Optional Features: %b", tag, bv.supportedFeatures)
	return nil
}

// reads the number of banknotes in the stacker.
// if error return -1
func (bv *BillValidator) readStacker() (returBillsInStacker int32) {
	request := mdb.MustPacketFromHex("36", true)
	response := mdb.Packet{}
	err := bv.Device.Tx(request, &response)
	if err != nil {
		bv.Log.Errorf("bill error send read stacker command:%v", err)
		return -1
	}
	rb := response.Bytes()
	if len(rb) != 2 {
		bv.Log.Errorf("bill error request stacker command. need 2 byte. request(%x) ", rb)
		return -1
	}
	x := bv.Device.ByteOrder.Uint16(rb)
	bv.stackerFull = (x & 0x8000) != 0
	bv.stackerCount = uint32(x & 0x7fff)
	return int32(bv.stackerCount)
}

func (bv *BillValidator) BillStacked() bool {
	oldv := int32(bv.stackerCount)
	if oldv+1 != bv.readStacker() {
		bv.Log.Errorf("bill count does not match. preview value:%v return value:%v", oldv, bv.stackerCount)
		return false
	}
	return true
}

func (bv *BillValidator) disableAccept() (mbe money.BillEvent) {
	buf := [5]byte{0x34, 00, 00, 00, 00}
	request := mdb.MustPacketFromBytes(buf[:], true)
	if mbe.Err = bv.Device.Tx(request, nil); mbe.Err != nil {
		return
	}
	return mbe
}

func (bv *BillValidator) escrowAccept() (mbe money.BillEvent) {
	request := mdb.MustPacketFromHex("3501", true)
	if err := bv.Device.Tx(request, nil); err != nil {
		mbe.Err = err
	}
	return mbe
}

func (bv *BillValidator) escrowReject() (mbe money.BillEvent) {
	request := mdb.MustPacketFromHex("3500", true)
	if err := bv.Device.Tx(request, nil); err != nil {
		mbe.Err = err
	}
	return mbe
}

func (bv *BillValidator) enableAccept() (err error) {
	allNominals := bv.acceptNominals(0, 0)
	buf := [5]byte{0x34}
	bv.Device.ByteOrder.PutUint16(buf[1:], allNominals) // allow to accept
	if bv.escrowSupported {
		bv.Device.ByteOrder.PutUint16(buf[3:], allNominals) // allow to escrow
	}
	request := mdb.MustPacketFromBytes(buf[:], true)
	if err = bv.Device.Tx(request, nil); err != nil {
		return fmt.Errorf("bill. send disable accept packet not complete. (%v)", err)
	}
	return bv.pollF(nil)
}

// poll function.
// возвращает события и объедененную ошибку
func (bv *BillValidator) pollF(returnEvent func(money.BillEvent)) (err error) {
	var response mdb.Packet
	if err := bv.Device.Tx(bv.Device.PacketPoll, &response); err != nil {
		bv.Log.Errorf("bill boll TX error:%v", err)
		return err
	}
	rb := response.Bytes()
	if len(rb) == 0 {
		bv.setState(ready)
		return
	}
	for _, b := range rb {
		e := bv.decodeByte(b)
		if returnEvent != nil && e.Event != money.NoEvent {
			returnEvent(e)
		}
		err = errors.Join(err, e.Err)
	}
	return err
}

func (bv *BillValidator) GetState() BllStateType {
	return bv.billstate
}

func (bv *BillValidator) setState(bc BllStateType) {
	if bv.billstate == bc {
		return
	}
	bv.billstate = bc
}

func (bv *BillValidator) setBroken(err error) money.BillEvent {
	bv.Log.Errorf("bill broken:%v", err)
	bv.setState(Broken)
	return money.BillEvent{Err: err}
}

func (bv *BillValidator) decodeByte(b byte) (e money.BillEvent) {
	switch b { // status
	case StatusDefectiveMotor:
		return bv.setBroken(fmt.Errorf("defective motor"))
	case StatusSensorProblem:
		return bv.setBroken(fmt.Errorf("sensor problem"))
	case StatusValidatorBusy:
		bv.setState(notReady)
		return
	case StatusROMChecksumError:
		return bv.setBroken(fmt.Errorf("rom checksum error"))
	case StatusValidatorJammed:
		return bv.setBroken(fmt.Errorf("bill jamed"))
	case StatusValidatorWasReset:
		bv.reseted = true
		return
	case StatusBillRemoved:
		return bv.escrowOutEvent(fmt.Errorf("bill removed"), bv.EscrowBill)
	case StatusCashboxOutOfPosition:
		// return bv.setBroken(fmt.Errorf("cashbox out"))
		return bv.setBroken(fmt.Errorf("cashbox out"))
	case StatusValidatorDisabled:
		bv.setState(notReady)
		// bv.setState(notReady)
		return
	case StatusInvalidEscrowRequest:
		return bv.escrowOutEvent(fmt.Errorf("bill invalid escrow request"), bv.EscrowBill)
	case StatusBillRejected:
		return bv.escrowOutEvent(nil, bv.EscrowBill)
	case StatusCreditedBillRemoval: // fishing attempt
		if bv.EscrowBill != 0 {
			err := errors.New("fishing!!! credited bill removed: " + bv.EscrowBill.Format100I())
			return bv.escrowOutEvent(err, bv.EscrowBill)
		}
		bv.Log.Error("StatusCreditedBillRemoval. ecsrow = 0")
		return
	}

	if b&0x80 != 0 { //route status
		status, billType := b&0xf0, b&0x0f
		nominal := bv.nominals[billType]
		bv.setEscrowBill(0)
		switch status {
		case StatusRoutingBillStacked: // complete state.
			// return nil, money.BillEvent{Event: money.Stacked, BillNominal: nominal}
			bv.Log.Infof("bill stacked (%v)", nominal.Format100I())
			return money.BillEvent{Event: money.Stacked, BillNominal: nominal}
		case StatusRoutingEscrowPosition:
			// bv.setState(process)
			bv.setEscrowBill(bv.nominals[billType])
			bv.Log.Infof("bill in escrow (%v)", nominal.Format100I())
			return money.BillEvent{Event: money.InEscrow, BillNominal: nominal}
			// return nil, money.BillEvent{Event: money.InEscrow, BillNominal: nominal}
		case StatusRoutingBillReturned:
			bv.Log.Infof("bill RoutingBillReturned (%v)", nominal.Format100I())
			return money.BillEvent{Event: money.OutEscrow, BillNominal: nominal}
			// return nil, money.BillEvent{Event: money.OutEscrow, BillNominal: nominal}
		case StatusRoutingDisabledBillRejected:
			bv.Log.Infof("reject disabled bill (%v)", nominal.Format100I())
			return money.BillEvent{Event: money.OutEscrow, BillNominal: nominal}
			// return nil, money.BillEvent{Event: money.OutEscrow, BillNominal: nominal}
		//recycler status
		case StatusRoutingBillToRecycler, StatusRoutingBillToRecyclerManualFill, StatusRoutingManualDispense, StatusRoutingTransferredFromRecyclerToCashbox:
			bv.Log.Infof("non implement bill recycler status ")
		default:
			fmt.Printf("\033[41m unknow bill poll status%v \033[0m\n", status)
		}
	}
	if b&0x5f == b { // Number of attempts to input a bill while validator is disabled.
		attempts := b & 0x1f
		bv.Log.Debugf("bil b=%b Number of attempts to input a bill while validator is disabled: %d", b, attempts)
		return
	}
	if b&0x2f == b { // Bill Recycler (Only)
		bv.Log.Errorf("bill recycler not implement byte:%v", b)
		// return money.PollItem{HardwareCode: b, Status: money.StatusError, Error: err}
		return
	}
	if b&0x1f == b { // File Transport Layer
		bv.Log.Errorf("bill file transport layer not implement. byte:%v", b)
		return
	}
	return
}

func (bv *BillValidator) escrowOutEvent(err error, nominal currency.Nominal) money.BillEvent {
	bv.setEscrowBill(0)
	return money.BillEvent{Err: err, Event: money.OutEscrow, BillNominal: nominal}
}

// прием банкнот ( если не принимал ранее).
// останавливаем по времени.
func (bv *BillValidator) BillRun(alive *alive.Alive, returnEvent func(money.BillEvent)) {
	bv.pollmu.Lock()
	defer bv.pollmu.Unlock()
	defer alive.Done()
	if bv.billstate != WaitConfigure {
		returnEvent(money.BillEvent{Err: errors.New("bill state not valid:" + fmt.Sprint(bv.billstate) + " need reset")})
		return
	}
	if err := bv.enableAccept(); err != nil { // enable accept all posible
		returnEvent(money.BillEvent{Err: errors.New("config enable accept")})
		return
	}
	for len(bv.billCmd) > 0 {
		<-bv.billCmd
	}
	refreshTime := time.Duration(200 * time.Millisecond)
	refreshTimer := time.NewTimer(refreshTime)
	bv.setState(ready)
	defer bv.disableAccept()
	cmd := noCommand
	again := true
	for again {
		select {
		case cmd = <-bv.billCmd:
			switch cmd {
			case Stop:
				bv.disableAccept()
				if bv.EscrowBill > 0 {
					bv.escrowReject()
					_ = bv.pollF(returnEvent)
				}
				bv.setState(WaitConfigure)
				again = false
			case Accept:
				returnEvent(bv.escrowAccept())
			case Reject:
				returnEvent(bv.escrowReject())
			default:
				bv.Log.WarningF("bill ignore command-%v", cmd)
			}
		case <-refreshTimer.C:
			if err := bv.pollF(returnEvent); err != nil { // return all events. and stop if error
				bv.setState(Broken)
				again = false
			}
			refreshTimer.Reset(refreshTime)
		}
	}
	refreshTimer.Stop()
}

func (bv *BillValidator) setEscrowBill(n currency.Nominal) {
	atomic.StoreUint32((*uint32)(&bv.EscrowBill), uint32(n))
}

func (bv *BillValidator) acceptNominals(minNominal currency.Amount, maxNominal currency.Amount) (bitSet uint16) {
	for i, n := range bv.nominals {
		if n == 0 {
			continue
		}
		if currency.Amount(n) >= minNominal && (maxNominal == 0 || currency.Amount(n) <= maxNominal) {
			// if currency.Amount(n) <= maxNominal {
			bitSet |= 1 << uint(i)
		}
	}
	return bitSet
}

// bill poll status
const (
	StatusDefectiveMotor       byte = 0x01
	StatusSensorProblem        byte = 0x02
	StatusValidatorBusy        byte = 0x03
	StatusROMChecksumError     byte = 0x04
	StatusValidatorJammed      byte = 0x05
	StatusValidatorWasReset    byte = 0x06
	StatusBillRemoved          byte = 0x07
	StatusCashboxOutOfPosition byte = 0x08
	StatusValidatorDisabled    byte = 0x09
	StatusInvalidEscrowRequest byte = 0x0a
	StatusBillRejected         byte = 0x0b
	StatusCreditedBillRemoval  byte = 0x0c
)

// bill poll routing status
const (
	StatusRoutingBillStacked byte = 0x80 | (iota << 4)
	StatusRoutingEscrowPosition
	StatusRoutingBillReturned
	StatusRoutingBillToRecycler
	StatusRoutingDisabledBillRejected
	StatusRoutingBillToRecyclerManualFill
	StatusRoutingManualDispense
	StatusRoutingTransferredFromRecyclerToCashbox
)

// ------------------------------------------------------------------------------------------------------------

// func (bv *BillValidator) AcceptMax(max currency.Amount) engine.Doer {
// 	// config := state.GetConfig(ctx)
// 	enableBitset := uint16(0)
// 	escrowBitset := uint16(0)
// 	if max != 0 {
// 		for i, n := range bv.nominals {
// 			if n == 0 {
// 				continue
// 			}
// 			if currency.Amount(n) <= max {
// 				// TODO consult config
// 				// _ = config
// 				enableBitset |= 1 << uint(i)
// 				// TODO consult config
// 				escrowBitset |= 1 << uint(i)
// 			}
// 		}
// 	}
// 	return bv.NewBillType(enableBitset, escrowBitset)
// }

func (bv *BillValidator) SupportedNominals() []currency.Nominal {
	ns := make([]currency.Nominal, 0, 16) // TypeCount = 16
	for _, n := range bv.nominals {
		if n != 0 {
			ns = append(ns, n)
		}
	}
	return ns
}

// func (bv *BillValidator) NewBillType(accept, escrow uint16) engine.Doer {
// 	buf := [5]byte{0x34}
// 	bv.Device.ByteOrder.PutUint16(buf[1:], accept)
// 	bv.Device.ByteOrder.PutUint16(buf[3:], escrow)
// 	request := mdb.MustPacketFromBytes(buf[:], true)
// 	return engine.Func0{Name: deviceName + ".BillType", F: func() error {
// 		return bv.Device.TxKnown(request, nil)
// 	}}
// }

// func (bv *BillValidator) setEscrowBill(n currency.Nominal) {
// 	atomic.StoreUint32((*uint32)(&bv.escrowBill), uint32(n))
// }

func (bv *BillValidator) EscrowAmount() currency.Amount {
	u32 := atomic.LoadUint32((*uint32)(&bv.EscrowBill))
	return currency.Amount(u32)
}

func (bv *BillValidator) EscrowNominal() currency.Nominal {
	u32 := atomic.LoadUint32((*uint32)(&bv.EscrowBill))
	return currency.Nominal(u32)
}
