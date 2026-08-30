package helpers

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

func HexSpecialBytes(input []byte) string {
	const hexAlpha = "0123456789abcdef"
	rb := make([]byte, 0, len(input)*4)
	for _, b := range input {
		if unicode.In(rune(b), unicode.Digit, unicode.Letter, unicode.Punct, unicode.Space) {
			rb = append(rb, b)
		} else {
			rb = append(rb, '{', hexAlpha[b>>4], hexAlpha[b&0xf], '}')
		}
	}
	return string(rb)
}

func HexSpecialString(input string) string {
	var result strings.Builder
	for _, r := range input {
		if unicode.In(r, unicode.Digit, unicode.Letter, unicode.Punct, unicode.Space) {
			result.WriteString(string(r))
		} else {
			result.WriteString(fmt.Sprintf("{%02x}", r))
		}
	}
	return result.String()
}

func MustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
