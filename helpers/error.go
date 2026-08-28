package helpers

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type AppError struct {
	ErrorCode int32
	Err       error
}

func (e *AppError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("errorcode=%d", e.ErrorCode)
	}
	return e.Err.Error()
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) Code() int32 {
	return e.ErrorCode
}

func FoldErrors(errs []error) (err error) {
	for _, e := range errs {
		if e != nil {
			err = errors.Join(err, e)
		}
	}
	return err
}

func FoldErrChan(ch <-chan error) error {
	var errs []error
	for e := range ch {
		if e != nil {
			errs = append(errs, e)
		}
	}
	return FoldErrors(errs)
}

func WrapErrChan(wg *sync.WaitGroup, ch chan<- error, fun func() error) {
	defer wg.Done()
	if err := fun(); err != nil {
		ch <- err
	}
}

type LogDoItems map[time.Time]string

func SaveAndShowDoError(li []string, err error, errorFolder string) {
	sf := errorFolder + li[0]
	var d string
	for _, val := range li {
		d = d + val + "\n"
	}
	d = d + fmt.Sprintf("err: %v ", err)
	fmt.Printf("------------------- begin \n%v\n------------------- end\n", d)

	f, err := os.Create(sf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create file: %v\n", err)
		return
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "Failed to close file: %v\n", cerr)
		}
	}()

	_, err = f.WriteString(d)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to file: %v\n", err)
	}
}
