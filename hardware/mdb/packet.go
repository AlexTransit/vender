package mdb

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	PacketMaxLength = 40
)

var (
	ErrPacketOverflow = errors.New("mdb: operation larger than max packet size")
	ErrPacketReadonly = errors.New("mdb: packet is readonly")

	PacketEmpty = &Packet{readonly: true}
	PacketAck   = MustPacketFromHex("00", true)
	PacketNak   = MustPacketFromHex("ff", true)
	PacketRet   = MustPacketFromHex("aa", true)
)

type InvalidChecksum struct {
	Received byte
	Actual   byte
}

func (ic InvalidChecksum) Error() string {
	return fmt.Sprintf("Invalid checksum received=%02x actual=%02x", ic.Received, ic.Actual)
}

type Packet struct {
	b [PacketMaxLength]byte
	l int

	readonly bool
}

func PacketFromBytes(b []byte, readonly bool) (Packet, error) {
	p := Packet{}
	_, err := p.Write(b)
	if err != nil {
		return *PacketEmpty, err
	}
	p.readonly = readonly
	return p, nil
}
func MustPacketFromBytes(b []byte, readonly bool) Packet {
	p, err := PacketFromBytes(b, readonly)
	if err != nil {
		panic(err)
	}
	return p
}

func PacketFromHex(s string, readonly bool) (Packet, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return *PacketEmpty, err
	}
	return PacketFromBytes(b, readonly)
}
func MustPacketFromHex(s string, readonly bool) Packet {
	p, err := PacketFromHex(s, readonly)
	if err != nil {
		panic(err)
	}
	return p
}

func (p *Packet) Bytes() []byte {
	return p.b[:p.l]
}

func (p *Packet) Equal(p2 *Packet) bool {
	return p.l == p2.l && bytes.Equal(p.Bytes(), p2.Bytes())
}

func (pl *Packet) write(p []byte) {
	pl.l = copy(pl.b[:], p)
}

func (sp *Packet) Write(p []byte) (n int, err error) {
	if sp.readonly {
		return 0, ErrPacketReadonly
	}
	pl := len(p)
	switch {
	case pl == 0:
		return 0, nil
	case pl > PacketMaxLength:
		return 0, ErrPacketOverflow
	}
	sp.write(p)
	return sp.l, nil
}

func (p *Packet) Len() int { return p.l }

func (p *Packet) Format() string {
	b := p.Bytes()
	h := hex.EncodeToString(b)
	hlen := len(h)
	ss := make([]string, (hlen/8)+1)
	for i := range ss {
		hi := (i + 1) * 8
		if hi > hlen {
			hi = hlen
		}
		ss[i] = h[i*8 : hi]
	}
	line := strings.Join(ss, " ")
	return line
}

func (p *Packet) Wire(ffDance bool) []byte {
	chk := byte(0)
	j := 0
	w := make([]byte, (p.l+2)*2)
	for _, b := range p.b[:p.l] {
		if ffDance && b == 0xff {
			w[j] = 0xff
			j++
		}
		w[j] = b
		j++
		chk += b
	}
	if ffDance {
		w[j] = 0xff
		w[j+1] = 0x00
		j += 2
	}
	w[j] = chk
	w = w[:j+1]
	return w
}

// Without checksum
func (p *Packet) TestHex(t testing.TB, expect string) {
	if _, err := hex.DecodeString(expect); err != nil {
		t.Fatalf("invalid expect=%s err=%s", expect, err)
	}
	actual := hex.EncodeToString(p.Bytes())
	if actual != expect {
		t.Fatalf("Packet=%s expected=%s", actual, expect)
	}
}
