package money

import (
	"math/rand"
	"testing"
	"time"

	"github.com/AlexTransit/vender/hardware"
	"github.com/AlexTransit/vender/hardware/mdb"
	state_new "github.com/AlexTransit/vender/internal/state/new"
	"github.com/stretchr/testify/require"
)

func TestAbort(t *testing.T) {
	t.Parallel()

	ctx, g := state_new.NewTestContext(t, "", `hardware{device "coin" {}} money{scale=100}`)
	mock := mdb.MockFromContext(ctx)
	defer mock.Close()
	mock.ExpectMap(map[string]string{
		"08":           "",
		"09":           "021643640200170102050a0a1900000000000000000000",
		"0f00":         "434f47303030303030303030303030463030313230303120202020029000000003",
		"0f0100000002": "",
		"0f05":         "01000600",
		"0a":           "0000110008",
		"0b":           "",
		"":             "",
	})

	require.NoError(t, hardware.InitMDBDevices(ctx))
	ms := MoneySystem{}
	require.NoError(t, ms.Start(ctx))
	mock.ExpectMap(nil)

	ms.dirty += g.Config.ScaleU(11)
	go mock.Expect([]mdb.MockR{
		{"0f020b", ""},
		{"0b", "02"},
		{"0b", ""},
		{"0f03", "0b00"},
	})
	require.NoError(t, ms.Abort(ctx))

	mock.ExpectMap(map[string]string{
		"0c0000ffff": "",
	})
	require.NoError(t, ms.Stop(ctx))
}

func TestWithdrawGift(t *testing.T) {
	t.Parallel()

	ctx, g := state_new.NewTestContext(t, "", `money{scale=100}`)

	require.NoError(t, hardware.InitMDBDevices(ctx))
	ms := MoneySystem{}
	require.NoError(t, ms.Start(ctx))

	gift := g.Config.ScaleU((rand.Uint32() % 100) + 3)
	price := gift - g.Config.ScaleU(2)
	ms.SetGiftCredit(ctx, gift)
	require.NoError(t, ms.WithdrawPrepare(ctx, price))

	// FIXME wait for change payout end
	time.Sleep(200 * time.Millisecond)

	ms.lk.RLock()
	t.Logf("dirty=%s", ms.dirty.FormatCtx(ctx))
	t.Logf("gift=%s", ms.giftCredit.FormatCtx(ctx))
	ms.lk.RUnlock()
}
