package money

import (
	"fmt"

	"github.com/AlexTransit/vender/currency"
)

//go:generate stringer -type=PollItemStatus -trimprefix=Status
type PollItemStatus byte

const (
	statusZero PollItemStatus = iota
	StatusInfo
	StatusError
	StatusFatal
	StatusDisabled
	StatusBusy
	StatusWasReset
	StatusCredit
	StatusRejected
	StatusEscrow
	StatusReturnRequest
	StatusDispensed
)

type PollItem struct {
	// TODO avoid time.Time for easy GC (contains pointer)
	// Time        time.Time
	Error        error
	DataNominal  currency.Nominal
	Status       PollItemStatus
	DataCount    uint8
	DataCashbox  bool
	HardwareCode byte
}

func (pi *PollItem) String() string {
	return fmt.Sprintf("status=%s cashbox=%v nominal=%s count=%d hwcode=%02x err=%v",
		pi.Status.String(),
		pi.DataCashbox,
		currency.Amount(pi.DataNominal).Format100I(),
		pi.DataCount,
		pi.HardwareCode,
		pi.Error,
	)
}

func (pi *PollItem) Amount() currency.Amount {
	if pi.DataCount == 0 {
		panic("code error PollItem.DataCount=0")
	}
	return currency.Amount(pi.DataNominal) * currency.Amount(pi.DataCount)
}
