package ui

// TODO move all of these to config

import "time"

const (
	DefaultCream uint8 = 4
	MaxCream     uint8 = 6
	DefaultSugar uint8 = 4
	MaxSugar     uint8 = 8
)

const modTuneTimeout = 3 * time.Second

var ScaleAlpha = []byte{
	0x94, // empty
	0x95,
	0x96,
	0x97, // full
	// '0', '1', '2', '3',
}

const (
	MsgError = "error"

	msgServiceInputAuth = "\x8d %s\x00"
	msgServiceMenu      = "Menu"
)
