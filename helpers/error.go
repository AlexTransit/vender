package helpers

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

func FoldErrors(errs []error) (err error) {
	// // common fast path
	// if len(errs) == 0 {
	// 	return nil
	// }

	// ss := make([]string, 0, 1+len(errs))
	// for _, e := range errs {
	// 	if e != nil {
	// 		// ss = append(ss, e.Error())
	// 		ss = append(ss, errors.ErrorStack(e))
	// 		// ss = append(ss, errors.Details(e))
	// 	}
	// }
	for _, e := range errs {
		errors.Join(err, e)
	}
	return err
}

func FoldErrChan(ch <-chan error) error {
	errs := make([]error, 0, cap(ch))
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

	f, _ := os.Create(sf)
	_, _ = f.WriteString(d)
	f.Close()
}
