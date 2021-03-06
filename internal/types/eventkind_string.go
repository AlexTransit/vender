// Code generated by "stringer -type=EventKind -trimprefix=Event"; DO NOT EDIT.

package types

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[EventInvalid-0]
	_ = x[EventInput-1]
	_ = x[EventMoneyCredit-2]
	_ = x[EventTime-3]
	_ = x[EventLock-4]
	_ = x[EventService-5]
	_ = x[EventStop-6]
	_ = x[EventFrontLock-7]
}

const _EventKind_name = "InvalidInputMoneyCreditTimeLockServiceStopLock"

var _EventKind_index = [...]uint8{0, 7, 12, 23, 27, 31, 38, 42, 46}

func (i EventKind) String() string {
	if i >= EventKind(len(_EventKind_index)-1) {
		return "EventKind(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _EventKind_name[_EventKind_index[i]:_EventKind_index[i+1]]
}
