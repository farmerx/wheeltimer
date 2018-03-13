package wheeltimer

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"context"
)

func Test_NewTimer(t *testing.T) {
	wg := &sync.WaitGroup{}

	wheel := NewTimingWheel(context.TODO())
	timeout := NewOnTimeOut(func() {
		fmt.Println(`hahahah `)
	})
	timerID := wheel.AddTimer(`1 7/13 * * * *`, timeout)
	fmt.Printf("Add timer %d\n", timerID)
	c := time.After(9 * time.Minute)
	wg.Add(1)
	go func() {
		for {
			select {
			case timeout := <-wheel.TimeOutChannel():
				timeout.Callback()
			case <-c:
				wg.Done()
			}
		}
	}()
	wg.Wait()
	wheel.Stop()
}
