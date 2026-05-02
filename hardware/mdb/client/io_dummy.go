package mdb_client

import "time"

type dummyUart struct{}

func NewDummyUart() *dummyUart {
	return &dummyUart{}
}

func (du *dummyUart) Open(_ string) error {
	return nil
}

func (du *dummyUart) Close() error {
	return nil
}

func (du *dummyUart) Break(_, _ time.Duration) error {
	return nil
}

func (du *dummyUart) Tx(_ []byte, _ []byte) (int, error) {
	return 0, nil
}
