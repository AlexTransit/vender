package ui

// TODO move all of these to config

import "time"

const modTuneTimeout = 3 * time.Second

var ScaleAlpha = []byte{
	0x94, // empty
	0x95,
	0x96,
	0x97, // full
	// '0', '1', '2', '3',
}

const (
	// msgServiceInputAuth = "\x8d %s\x00"
	msgServiceMenu = "Menu"
)
