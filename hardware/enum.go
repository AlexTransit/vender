package hardware

import (
	"context"
	"sync"

	"github.com/AlexTransit/vender/hardware/mdb/bill"
	"github.com/AlexTransit/vender/hardware/mdb/coin"
	"github.com/AlexTransit/vender/hardware/mdb/evend"
	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/internal/state"
)

func Enum(ctx context.Context) error {
	const N = 3
	errch := make(chan error, N+1)
	wg := sync.WaitGroup{}
	wg.Add(N)

	go helpers.WrapErrChan(&wg, errch, func() error { return bill.Enum(ctx) })
	go helpers.WrapErrChan(&wg, errch, func() error { return coin.Enum(ctx) })
	go helpers.WrapErrChan(&wg, errch, func() error { return evend.Enum(ctx) })

	wg.Wait()
	errch <- state.GetGlobal(ctx).CheckDevices()
	close(errch)
	return helpers.FoldErrChan(errch)
}
