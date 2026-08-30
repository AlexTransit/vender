package evend

import (
	"testing"

	"github.com/AlexTransit/vender/hardware/mdb"
	state_new "github.com/AlexTransit/vender/internal/state/new"
	"github.com/stretchr/testify/require"
)

func TestElevator(t *testing.T) {
	t.Parallel()

	ctx, g := state_new.NewTestContext(t, "", `hardware {
	device "evend.elevator" {}
}`)
	mock := mdb.MockFromContext(ctx)
	defer mock.Close()
	go mock.Expect([]mdb.MockR{
		// EnumElevator -> InitMiherElevator -> dev.Rst()
		{"d0", ""},
		{"d1", "04000b0100011805de07020000000a01"},

		// move(100): currentPos==-1 -> me.reset() -> ANOTHER dev.Rst()
		// (see the firmware-quirk comment in miher-elevator.go — this
		// second reset is what the current code actually does; not
		// touched here, just accounted for in the mock)
		{"d0", ""},
		{"d1", "04000b0100011805de07020000000a01"},

		// moveNoWait(100): Command(0x03, position=0x64, 0x64) -> "d2036464"
		// (previously hardcoded as "d2036400" in this test — wrong: the
		// trailing byte is a fixed 0x64 constant unrelated to position,
		// only coincidentally equal to position's own hex value here)
		{"d2036464", ""},

		// mvComplete: WaitSuccess poll (proto1) -> immediate success
		{"d3", "0d00"},
	})
	require.NoError(t, EnumElevator(ctx))

	g.Engine.TestDo(t, ctx, "evend.elevator.move(100)")
}
