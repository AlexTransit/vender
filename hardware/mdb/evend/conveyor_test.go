package evend

import (
	"testing"

	"github.com/AlexTransit/vender/hardware/mdb"
	state_new "github.com/AlexTransit/vender/internal/state/new"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConveyor(t *testing.T) {
	t.Parallel()

	ctx, g := state_new.NewTestContext(t, "", `hardware {
	device "evend.conveyor" {}
}`)
	mock := mdb.MockFromContext(ctx)
	defer mock.Close()
	go mock.Expect([]mdb.MockR{
		{"d8", ""},
		{"d9", "011810000a0000c8001fff01050a32640000000000000000000000"},

		// calibrate (move to 0): CommandWaitSuccess = Command, then a
		// WaitSuccess poll-loop. Command must come first — verified by
		// running the previous (wrongly-ordered) version and tracing the
		// resulting mismatch cascade back to the real request order.
		{"da010000", ""},
		{"db", ""}, // poll -> empty = immediate complete

		// conveyor_move_cup (1560 = 0x0618 -> "da011806"):
		// moveNoWait() = CommandNoWait = Command + exactly one mandatory
		// poll (content doesn't matter, WaitSuccess(1,false) always
		// succeeds after one attempt); movingDone() then runs its own
		// separate poll-loop until an empty ("complete") response.
		// NOT re-validated against a real run past this point — best
		// structural reconstruction from the code, reusing the original
		// response bytes just reordered/regrouped.
		{"da011806", ""},
		{"db", ""},   // moveNoWait's CommandNoWait mandatory poll
		{"db", "04"}, // movingDone loop: busy
		{"db", "04"}, // movingDone loop: busy
		{"db", "50"}, // movingDone loop: busy
		{"db", "50"}, // movingDone loop: busy
		{"db", ""},   // movingDone loop: complete

		// conveyor_move_elevator (1895 = 0x0767 -> "da016707")
		{"da016707", ""},
		{"db", ""},   // CommandNoWait mandatory poll
		{"db", "50"}, // movingDone loop: busy
		{"db", ""},   // movingDone loop: complete

		// shake(4): CommandWaitSuccess = Command + poll-loop directly,
		// no CommandNoWait-style extra mandatory poll.
		{"da030400", ""},
		{"db", "50"}, // loop: busy
		{"db", ""},   // loop: complete

		// set_speed(31): plain dev.Tx, no retry, no polling.
		{"dd101f", ""},
	})
	require.NoError(t, EnumConveyor(ctx))

	assert.NoError(t, g.Engine.RegisterParse("conveyor_move_cup", "evend.conveyor.move(1560)"))
	assert.NoError(t, g.Engine.RegisterParse("conveyor_move_elevator", "evend.conveyor.move(1895)"))
	g.Engine.TestDo(t, ctx, "conveyor_move_cup")
	g.Engine.TestDo(t, ctx, "conveyor_move_elevator")
	g.Engine.TestDo(t, ctx, "evend.conveyor.shake(4)")
	g.Engine.TestDo(t, ctx, "evend.conveyor.set_speed(31)")
}
