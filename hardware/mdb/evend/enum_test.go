package evend

import (
	"testing"

	"github.com/AlexTransit/vender/hardware/mdb"
	state_new "github.com/AlexTransit/vender/internal/state/new"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	ctx, g := state_new.NewTestContext(t, "", `hardware {
device "evend.hopper1" {}
device "evend.conveyor" {}
}`)
	mock := mdb.MockFromContext(ctx)
	defer mock.Close()
	mock.ExpectMap(map[string]string{
		// relevant
		"40": "", "41": "", "d8": "", "d9": "",
		// irrelevant, only to reduce test log noise
		"48": "", "49": "", "50": "", "51": "", "58": "", "59": "", "60": "", "61": "",
		"68": "", "69": "", "70": "", "71": "", "78": "", "79": "",
		"b8": "", "b9": "", "c0": "", "c1": "", "c8": "", "c9": "",
		"d0": "", "d1": "", "e0": "", "e1": "", "e8": "", "e9": "",
	})
	require.NoError(t, EnumHopper(ctx, 1))
	require.NoError(t, EnumConveyor(ctx))

	mock.ExpectMap(nil)
	go mock.Expect([]mdb.MockR{
		// conveyor calibrate (move to 0): CommandWaitSuccess = Command +
		// single-poll immediate success (see TestConveyor's calibrate fix).
		{"da010000", ""},
		{"db", ""},
		// conveyor move to hopper position: moveNoWait=CommandNoWait
		// (Command + 1 mandatory poll) then movingDone's own poll-loop.
		{"da01fa00", ""},
		{"db", ""},
		{"db", ""},
		// hopper run: CommandWaitSuccess = Command + single-poll success.
		{"420a", ""},
		{"43", ""},
	})

	assert.NoError(t, g.Engine.RegisterParse("hopper1(?)", "evend.conveyor.move(250) evend.hopper1.run(?)"))
	assert.NoError(t, g.Engine.RegisterParse("conveyor_move_cup", "evend.conveyor.move(1560)"))
	assert.NoError(t, g.Engine.RegisterParse("conveyor_move_elevator", "evend.conveyor.move(1895)"))

	g.Engine.TestDo(t, ctx, "hopper1(10)")
}
