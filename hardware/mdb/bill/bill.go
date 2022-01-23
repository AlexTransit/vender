// Package bill incapsulates work with bill validators.
package bill

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/hardware/mdb"
	"github.com/AlexTransit/vender/hardware/money"
	"github.com/AlexTransit/vender/internal/engine"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/internal/types"
	"github.com/juju/errors"
	"github.com/temoto/alive/v2"
)

const (
	TypeCount = 16
)

//go:generate stringer -type=Features -trimprefix=Feature
type Features uint32

const (
	FeatureFTL Features = 1 << iota
	FeatureRecycling
)

const DefaultEscrowTimeout = 30 * time.Second

type BillValidator struct { //nolint:maligned
	mdb.Device
	pollmu        sync.Mutex // isolate active/idle polling
	configScaling uint16

	DoEscrowAccept engine.Func
	DoEscrowReject engine.Func
	DoStacker      engine.Func

	// parsed from SETUP
	featureLevel      uint8
	supportedFeatures Features
	escrowSupported   bool
	nominals          [TypeCount]currency.Nominal // final values, includes all scaling factors

	// dynamic state useful for external code
	escrowBill   currency.Nominal // assume only one bill may be in escrow position
	stackerFull  bool
	stackerCount uint32
}

var (
	packetEscrowAccept    = mdb.MustPacketFromHex("3501", true)
	packetEscrowReject    = mdb.MustPacketFromHex("3500", true)
	packetStacker         = mdb.MustPacketFromHex("36", true)
	packetExpIdent        = mdb.MustPacketFromHex("3700", true)
	packetExpIdentOptions = mdb.MustPacketFromHex("3702", true)
)

var (
	ErrDefectiveMotor   = errors.New("Defective Motor")
	ErrBillRemoved      = errors.New("Bill Removed")
	ErrEscrowImpossible = errors.New("An ESCROW command was requested for a bill not in the escrow position.")
	ErrAttempts         = errors.New("Attempts")
	ErrEscrowTimeout    = errors.New("ESCROW timeout")
)

func (bv *BillValidator) init(ctx context.Context) error {
	const tag = deviceName + ".init"
	g := state.GetGlobal(ctx)
	mdbus, err := g.Mdb()
	if err != nil {
		return errors.Annotate(err, tag)
	}
	bv.Device.Init(mdbus, 0x30, "bill", binary.BigEndian)
	config := g.Config.Hardware.Mdb.Bill
	bv.configScaling = 100
	if config.ScalingFactor != 0 {
		bv.configScaling = uint16(config.ScalingFactor)
	}

	bv.DoEscrowAccept = bv.newEscrow(true)
	bv.DoEscrowReject = bv.newEscrow(false)
	bv.DoStacker = bv.newStacker()
	g.Engine.Register(bv.DoEscrowAccept.Name, bv.DoEscrowAccept)
	g.Engine.Register(bv.DoEscrowReject.Name, bv.DoEscrowReject)

	bv.Device.DoInit = bv.newIniter()

	// TODO remove IO from Init()
	if err = g.Engine.Exec(ctx, bv.Device.DoInit); err != nil {
		return errors.Annotate(err, tag)
	}
	return nil
}

func (bv *BillValidator) AcceptMax(max currency.Amount) engine.Doer {
	// config := state.GetConfig(ctx)
	enableBitset := uint16(0)
	escrowBitset := uint16(0)

	if max != 0 {
		for i, n := range bv.nominals {
			if n == 0 {
				continue
			}
			if currency.Amount(n) <= max {
				// TODO consult config
				// _ = config
				enableBitset |= 1 << uint(i)
				// TODO consult config
				escrowBitset |= 1 << uint(i)
			}
		}
	}

	return bv.NewBillType(enableBitset, escrowBitset)
}

func (bv *BillValidator) SupportedNominals() []currency.Nominal {
	ns := make([]currency.Nominal, 0, TypeCount)
	for _, n := range bv.nominals {
		if n != 0 {
			ns = append(ns, n)
		}
	}
	return ns
}

func (bv *BillValidator) Run(ctx context.Context, alive *alive.Alive, fun func(money.PollItem) bool) {
	var stopch <-chan struct{}
	types.VMC.MonSys.BillOn = true
	if alive != nil {
		defer alive.Done()
		stopch = alive.StopChan()
	}
	pd := mdb.PollDelay{}
	parse := bv.pollFun(fun)
	var active bool
	var err error

	again := true
	for again {
		response := mdb.Packet{}
		bv.pollmu.Lock()
		err = bv.Device.TxKnown(bv.Device.PacketPoll, &response)
		bv.pollmu.Unlock()
		if err == nil {
			active, err = parse(response)
			types.VMC.MonSys.BillRun = !active
		}
		again = (alive != nil) && (alive.IsRunning()) && pd.Delay(&bv.Device, active, err != nil, stopch)
	}
	bv.setEscrowBill(0)
	types.VMC.MonSys.BillOn = false
}
func (bv *BillValidator) pollFun(fun func(money.PollItem) bool) mdb.PollRequestFunc {
	const tag = deviceName + ".poll"

	return func(p mdb.Packet) (bool, error) {
		bs := p.Bytes()
		if len(bs) == 0 {
			return false, nil
		}
		for _, b := range bs {
			pi := bv.parsePollItem(b)

			switch pi.Status {
			case money.StatusInfo:
				bv.Log.Infof("%s/info: %s", tag, pi.String())
				// TODO telemetry
			case money.StatusError:
				bv.Device.TeleError(errors.Annotate(pi.Error, tag))
				return false, nil
			case money.StatusFatal:
				bv.Device.TeleError(errors.Annotate(pi.Error, tag))
				return false, nil
			case money.StatusBusy, money.StatusReturnRequest, money.StatusDispensed:
			case money.StatusDisabled:
			case money.StatusRejected:
				return false, nil
			case money.StatusWasReset:
				bv.Log.Infof("bill was reset ")
				return false, nil
			default:
				fun(pi)
				// if fun(pi) {
				// 	return true, nil
				// }
			}
		}
		return true, nil
	}
}

func (bv *BillValidator) newIniter() engine.Doer {
	const tag = deviceName + ".init"
	return engine.NewSeq(tag).
		Append(bv.Device.DoReset).
		Append(engine.Func{Name: tag + "/poll", F: func(ctx context.Context) error {
			bv.Run(ctx, nil, func(money.PollItem) bool { return false })
			// POLL until it settles on empty response
			return nil
		}}).
		Append(engine.Func{Name: tag + "/setup", F: bv.CommandSetup}).
		Append(engine.Func0{Name: tag + "/expid", F: func() error {
			if err := bv.CommandExpansionIdentificationOptions(); err != nil {
				if _, ok := err.(mdb.FeatureNotSupported); ok {
					if err = bv.CommandExpansionIdentification(); err != nil {
						return err
					}
				} else {
					return err
				}
			}
			return nil
		}}).
		Append(bv.DoStacker).
		Append(engine.Sleep{Duration: bv.Device.DelayNext})
}

func (bv *BillValidator) NewBillType(accept, escrow uint16) engine.Doer {
	buf := [5]byte{0x34}
	bv.Device.ByteOrder.PutUint16(buf[1:], accept)
	bv.Device.ByteOrder.PutUint16(buf[3:], escrow)
	request := mdb.MustPacketFromBytes(buf[:], true)
	return engine.Func0{Name: deviceName + ".BillType", F: func() error {
		return bv.Device.TxKnown(request, nil)
	}}
}

func (bv *BillValidator) setEscrowBill(n currency.Nominal) {
	atomic.StoreUint32((*uint32)(&bv.escrowBill), uint32(n))
}
func (bv *BillValidator) EscrowAmount() currency.Amount {
	u32 := atomic.LoadUint32((*uint32)(&bv.escrowBill))
	return currency.Amount(u32)
}

func (bv *BillValidator) CommandSetup(ctx context.Context) error {
	const expectLength = 27
	var billFactors [TypeCount]uint8

	if err := bv.Device.TxSetup(); err != nil {
		return errors.Trace(err)
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
		if i >= TypeCount {
			bv.Log.Errorf("CRITICAL bill SETUP type factors count=%d > expected=%d", len(bs[11:]), TypeCount)
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

func (bv *BillValidator) CommandExpansionIdentification() error {
	const tag = deviceName + ".ExpId"
	const expectLength = 29
	request := packetExpIdent
	response := mdb.Packet{}
	if err := bv.Device.TxMaybe(request, &response); err != nil {
		return errors.Annotate(err, tag)
	}
	bs := response.Bytes()
	bv.Log.Debugf("%s response=%x", tag, bs)
	if len(bs) < expectLength {
		return fmt.Errorf("%s response=%x length=%d expected=%d", tag, bs, len(bs), expectLength)
	}
	bv.Log.Infof("%s Manufacturer Code: '%s'", tag, bs[0:0+3])
	bv.Log.Debugf("%s Serial Number: '%s'", tag, string(bs[3:3+12]))
	bv.Log.Debugf("%s Model #/Tuning Revision: '%s'", tag, string(bs[15:15+12]))
	bv.Log.Debugf("%s Software Version: %x", tag, bs[27:27+2])
	return nil
}

func (bv *BillValidator) CommandFeatureEnable(requested Features) error {
	f := requested & bv.supportedFeatures
	buf := [6]byte{0x37, 0x01}
	bv.Device.ByteOrder.PutUint32(buf[2:], uint32(f))
	request := mdb.MustPacketFromBytes(buf[:], true)
	err := bv.Device.TxMaybe(request, nil)
	return errors.Annotate(err, deviceName+".FeatureEnable")
}

func (bv *BillValidator) CommandExpansionIdentificationOptions() error {
	const tag = deviceName + ".ExpIdOptions"
	if bv.featureLevel < 2 {
		return mdb.FeatureNotSupported(tag + " is level 2+")
	}
	const expectLength = 33
	request := packetExpIdentOptions
	response := mdb.Packet{}
	err := bv.Device.TxMaybe(request, &response)
	if err != nil {
		return errors.Annotate(err, tag)
	}
	bv.Log.Debugf("%s response=(%d)%s", tag, response.Len(), response.Format())
	bs := response.Bytes()
	if len(bs) < expectLength {
		return fmt.Errorf("%s response=%s expected %d bytes", tag, response.Format(), expectLength)
	}
	bv.supportedFeatures = Features(bv.Device.ByteOrder.Uint32(bs[29 : 29+4]))
	bv.Log.Infof("%s Manufacturer Code: '%s'", tag, bs[0:0+3])
	bv.Log.Infof("%s Serial Number: '%s'", tag, string(bs[3:3+12]))
	bv.Log.Infof("%s Model #/Tuning Revision: '%s'", tag, string(bs[15:15+12]))
	bv.Log.Infof("%s Software Version: %x", tag, bs[27:27+2])
	bv.Log.Infof("%s Optional Features: %b", tag, bv.supportedFeatures)
	return nil
}

func (bv *BillValidator) newEscrow(accept bool) engine.Func {
	var tag string
	var request mdb.Packet
	if accept {
		tag = deviceName + ".escrow-accept"
		request = packetEscrowAccept
	} else {
		tag = deviceName + ".escrow-reject"
		request = packetEscrowReject
	}

	// FIXME passive poll loop (`Run`) will wrongly consume response to this
	// TODO find a good way to isolate this code from `Run` loop
	// - silly `Mutex` will do the job
	// - serializing via channel on mdb.Device would be better

	return engine.Func{Name: tag, F: func(ctx context.Context) error {
		if bv.escrowBill == 0 {
			bv.Log.Errorf("escrow (%v) not possilbe. no bills.", accept)
			return nil
		}
		bv.pollmu.Lock()
		defer bv.pollmu.Unlock()

		if err := bv.Device.TxKnown(request, nil); err != nil {
			return err
		}

		// > After an ESCROW command the bill validator should respond to a POLL command
		// > with the BILL STACKED, BILL RETURNED, INVALID ESCROW or BILL TO RECYCLER
		// > message within 30 seconds. If a bill becomes jammed in a position where
		// > the customer may be able to retrieve it, the bill validator
		// > should send a BILL RETURNED message.
		var result error
		fun := bv.pollFun(func(pi money.PollItem) bool {
			code := pi.HardwareCode
			switch code {
			case StatusValidatorDisabled:
				bv.Log.Errorf("CRITICAL likely code error: escrow request while disabled")
				result = ErrEscrowImpossible
				return true
			case StatusInvalidEscrowRequest:
				bv.Log.Errorf("CRITICAL likely code error: escrow request invalid")
				result = ErrEscrowImpossible
				return true
			case StatusRoutingBillStacked:
				return true
			case StatusRoutingBillReturned:
				return false
			case StatusRoutingBillToRecycler:
				return true
			default:
				return false
			}
		})
		d := bv.Device.NewPollLoop(tag, bv.Device.PacketPoll, DefaultEscrowTimeout, fun)
		if err := engine.GetGlobal(ctx).Exec(ctx, d); err != nil {
			return err
		}
		return result
	}}
}

func (bv *BillValidator) newStacker() engine.Func {
	const tag = deviceName + ".stacker"

	return engine.Func{Name: tag, F: func(ctx context.Context) error {
		request := packetStacker
		response := mdb.Packet{}
		err := bv.Device.TxKnown(request, &response)
		if err != nil {
			return errors.Annotate(err, tag)
		}
		rb := response.Bytes()
		if len(rb) != 2 {
			return errors.Errorf("%s response length=%d expected=2", tag, len(rb))
		}
		x := bv.Device.ByteOrder.Uint16(rb)
		bv.stackerFull = (x & 0x8000) != 0
		bv.stackerCount = uint32(x & 0x7fff)
		bv.Log.Debugf("%s full=%t count=%d", tag, bv.stackerFull, bv.stackerCount)
		return nil
	}}
}

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

func (bv *BillValidator) parsePollItem(b byte) money.PollItem {
	const tag = deviceName + ".poll-parse"

	switch b {
	case StatusDefectiveMotor:
		return money.PollItem{HardwareCode: b, Status: money.StatusFatal, Error: ErrDefectiveMotor}
	case StatusSensorProblem:
		return money.PollItem{HardwareCode: b, Status: money.StatusFatal, Error: money.ErrSensor}
	case StatusValidatorBusy:
		return money.PollItem{HardwareCode: b, Status: money.StatusBusy}
	case StatusROMChecksumError:
		return money.PollItem{HardwareCode: b, Status: money.StatusFatal, Error: money.ErrROMChecksum}
	case StatusValidatorJammed:
		return money.PollItem{HardwareCode: b, Status: money.StatusFatal, Error: money.ErrJam}
	case StatusValidatorWasReset:
		return money.PollItem{HardwareCode: b, Status: money.StatusWasReset}
	case StatusBillRemoved:
		return money.PollItem{HardwareCode: b, Status: money.StatusError, Error: ErrBillRemoved}
	case StatusCashboxOutOfPosition:
		return money.PollItem{HardwareCode: b, Status: money.StatusFatal, Error: money.ErrNoStorage}
	case StatusValidatorDisabled:
		return money.PollItem{HardwareCode: b, Status: money.StatusDisabled}
	case StatusInvalidEscrowRequest:
		return money.PollItem{HardwareCode: b, Status: money.StatusError, Error: ErrEscrowImpossible}
	case StatusBillRejected:
		return money.PollItem{HardwareCode: b, Status: money.StatusRejected}
	case StatusCreditedBillRemoval: // fishing attempt
		if bv.escrowBill == 0 {
			return money.PollItem{HardwareCode: b, Status: money.StatusError, Error: money.ErrFishingFail}
		} else {
			return money.PollItem{HardwareCode: b, Status: money.StatusError, Error: money.ErrFishingOK}
		}
	}

	if b&0x80 != 0 { // Bill Routing
		status, billType := b&0xf0, b&0x0f
		result := money.PollItem{
			DataCount:    1,
			DataNominal:  bv.nominals[billType],
			HardwareCode: status,
		}
		switch status {
		case StatusRoutingBillStacked:
			bv.Log.Infof("stacked bill:%v", result.DataNominal/100)
			bv.setEscrowBill(0)
			result.DataCashbox = true
			result.Status = money.StatusCredit
			// result.Status = money.StatusStacked
		case StatusRoutingEscrowPosition:
			if bv.EscrowAmount() != 0 {
				bv.Log.Errorf("%s b=%b CRITICAL likely code error, ESCROW POSITION with EscrowAmount not empty", tag, b)
			}
			dn := result.DataNominal
			// global.Log.Infof("escrow bill:%v",dn)
			bv.Log.Infof("escrow bill:%v", dn/100)

			bv.setEscrowBill(dn)
			result.Status = money.StatusEscrow
			// result.DataCount = 1
		case StatusRoutingBillReturned:
			if bv.EscrowAmount() == 0 {
				// most likely code error, but also may be rare case of boot up
				bv.Log.Errorf("%s b=%b CRITICAL likely code error, BILL RETURNED with EscrowAmount empty", tag, b)
			}
			bv.setEscrowBill(0)
			// bv.Log.Debugf("bill routing BILL RETURNED")
			// TODO make something smarter than Status:Escrow,DataCount:0
			// maybe Status:Info is enough?

			result.DataCount = 0
			result.DataNominal = 0
			result.Status = money.StatusInfo
		case StatusRoutingBillToRecycler:
			bv.setEscrowBill(0)
			// bv.Log.Debugf("bill routing BILL TO RECYCLER")
			result.Status = money.StatusCredit
		case StatusRoutingDisabledBillRejected:
			fmt.Printf("\n\033[41m StatusRoutingDisabledBillRejectedddd \033[0m\n\n")
			// TODO maybe rejected?
			// result.Status = money.StatusRejected
			result.Status = money.StatusInfo
			result.Error = fmt.Errorf("bill routing DISABLED BILL REJECTED")
		case StatusRoutingBillToRecyclerManualFill:
			fmt.Printf("\n\033[41m StatusRoutingBillToRecyclerManualFilllll \033[0m\n\n")
			result.Status = money.StatusInfo
			result.Error = fmt.Errorf("bill routing BILL TO RECYCLER – MANUAL FILL")
		case StatusRoutingManualDispense:
			fmt.Printf("\n\033[41m StatusRoutingManualDispenseeee \033[0m\n\n")
			result.Status = money.StatusInfo
			result.Error = fmt.Errorf("bill routing MANUAL DISPENSE")
		case StatusRoutingTransferredFromRecyclerToCashbox:
			fmt.Printf("\n\033[41m StatusRoutingTransferredFromRecyclerToCashboxxxx \033[0m\n\n")
			result.Status = money.StatusInfo
			result.Error = fmt.Errorf("bill routing TRANSFERRED FROM RECYCLER TO CASHBOX")
		default:
			panic("code error")
		}
		return result
	}

	if b&0x5f == b { // Number of attempts to input a bill while validator is disabled.
		attempts := b & 0x1f
		bv.Log.Debugf("%s b=%b Number of attempts to input a bill while validator is disabled: %d", tag, b, attempts)
		return money.PollItem{HardwareCode: 0x40, Status: money.StatusInfo, Error: ErrAttempts, DataCount: attempts}
	}

	if b&0x2f == b { // Bill Recycler (Only)
		err := errors.NotImplementedf("%s b=%b bill recycler", tag, b)
		bv.Log.Errorf(err.Error())
		return money.PollItem{HardwareCode: b, Status: money.StatusError, Error: err}
	}

	err := errors.Errorf("%s CRITICAL bill unknown b=%b", tag, b)
	bv.Log.Errorf(err.Error())
	return money.PollItem{HardwareCode: b, Status: money.StatusFatal, Error: err}
}
