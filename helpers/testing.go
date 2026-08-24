package helpers

import (
	"math/rand"
	"time"
)

type FatalFunc func(...any)

type Fataler interface {
	Fatal(...any)
}

func RandUnix() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}
