// *    *    *    *    *    *    *
// -    -    -    -    -    -    -
// |    |    |    |    |    |    |
// |    |    |    |    |    |    + year [optional]
// |    |    |    |    |    +----- day of week (0 - 7) (Sunday=0 or 7)
// |    |    |    |    +---------- month (1 - 12)
// |    |    |    +--------------- day of month (1 - 31)
// |    |    +-------------------- hour (0 - 23)
// |    +------------------------- min (0 - 59)
// +------------------------------ second (0 - 59)

package wheeltimer

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	tickPeriod time.Duration = 500 * time.Millisecond
	bufferSize               = 1024 // channel max idle
)

var timerIds *AtomicInt64

func init() {
	timerIds = NewAtomicInt64(0)
}

// timerHeap is a heap-based priority queue
type timerHeapType []*timerType

func (heap timerHeapType) getIndexByID(id int64) int {
	for _, t := range heap {
		if t.id == id {
			return t.index
		}
	}
	return -1
}

func (heap timerHeapType) Len() int {
	return len(heap)
}

func (heap timerHeapType) Less(i, j int) bool {
	return heap[i].expiration.UnixNano() < heap[j].expiration.UnixNano()
}

func (heap timerHeapType) Swap(i, j int) {
	heap[i], heap[j] = heap[j], heap[i]
	heap[i].index = i
	heap[j].index = j
}

func (heap *timerHeapType) Push(x interface{}) {
	n := len(*heap)
	timer := x.(*timerType)
	timer.index = n
	*heap = append(*heap, timer)
}

func (heap *timerHeapType) Pop() interface{} {
	old := *heap
	n := len(old)
	timer := old[n-1]
	timer.index = -1
	*heap = old[0 : n-1]
	return timer
}

// OnTimeOut represents a timed task.
type OnTimeOut struct {
	Callback func()
}

// NewOnTimeOut returns OnTimeOut.
func NewOnTimeOut(cb func()) *OnTimeOut {
	// job .... callback 回调函数
	return &OnTimeOut{
		Callback: cb,
	}
}

// 'expiration' is the time when timer time out, if 'interval' > 0
// the timer will time out periodically, 'timeout' contains the callback
// to be called when times out
type timerType struct {
	id         int64         // id 定时任务的唯一id，可以这个id查找在队列中的定时任务
	expiration time.Time     // expiration 定时任务的到期时间点，当到这个时间点后，触发定时任务的执行，在优先队列中也是通过这个字段来排序
	interval   time.Duration // interval 定时任务的触发频率，每隔interval时间段触发一次
	timeout    *OnTimeOut    // timeout 这个结构中保存定时超时任务，这个任务函数参数必须符合相应的接口类型
	index      int           // index 保存在队列中的任务所在的下标 for container/heap
	// The schedule on which this job should be run.
	Schedule Schedule
	// The next time the job will run. This is the zero time if Cron has not been
	// started or this entry's schedule is unsatisfiable
	Next time.Time

	// The last time this job was run. This is the zero time if the job has never
	// been run.
	Prev time.Time
}

func newTimer(when time.Time, interv time.Duration, to *OnTimeOut, schedule Schedule) *timerType {
	return &timerType{
		id:         timerIds.GetAndIncrement(),
		expiration: when,
		interval:   interv,
		timeout:    to,
		Schedule:   schedule,
		Next:       when,
	}
}

func (t *timerType) isRepeat() bool {
	return int64(t.interval) > 0
}

// TimingWheel manages all the timed task.
type TimingWheel struct {
	timeOutChan chan *OnTimeOut    // 定义一个带缓存的chan来保存，已经触发的定时任务
	timers      timerHeapType      // 是[]*timerType类型的slice，保存所有定时任务
	ticker      *time.Ticker       // 当每一个ticker到来时，时间轮都会检查队列中head元素是否到达超时时间
	wg          *sync.WaitGroup    // 用于并发控制
	addChan     chan *timerType    // add timer in loop 通过带缓存的chan来向队列中添加任务
	cancelChan  chan int64         // cancel timer in loop 定时器停止的chan
	sizeChan    chan int           // get size in loop 返回队列中任务的数量的chan
	ctx         context.Context    // 用户并发控制
	cancel      context.CancelFunc // 用户并发控制
}

// NewTimingWheel returns a *TimingWheel ready for use.
func NewTimingWheel(ctx context.Context) *TimingWheel {
	timingWheel := &TimingWheel{
		timeOutChan: make(chan *OnTimeOut, bufferSize),
		timers:      make(timerHeapType, 0),
		ticker:      time.NewTicker(tickPeriod),
		wg:          &sync.WaitGroup{},
		addChan:     make(chan *timerType, bufferSize),
		cancelChan:  make(chan int64, bufferSize),
		sizeChan:    make(chan int),
	}
	timingWheel.ctx, timingWheel.cancel = context.WithCancel(ctx)
	heap.Init(&timingWheel.timers)
	timingWheel.wg.Add(1)
	go func() {
		timingWheel.start()
		timingWheel.wg.Done()
	}()
	return timingWheel
}

// TimeOutChannel returns the timeout channel.
func (tw *TimingWheel) TimeOutChannel() chan *OnTimeOut {
	return tw.timeOutChan
}

// AddTimer adds new timed task.
func (tw *TimingWheel) AddTimer(spec string, to *OnTimeOut) int64 {
	if to == nil {
		return int64(-1)
	}

	schedule, err := Parse(spec)
	if err != nil {
		return int64(-1)
	}
	now := time.Now().Local()
	when := schedule.Next(now)
	intervl := when.Sub(now)
	fmt.Println(`Next time:`, when)
	timer := newTimer(when, intervl, to, schedule)
	tw.addChan <- timer
	return timer.id
}

// Size returns the number of timed tasks.
func (tw *TimingWheel) Size() int {
	return <-tw.sizeChan
}

// CancelTimer cancels a timed task with specified timer ID.
func (tw *TimingWheel) CancelTimer(timerID int64) {
	tw.cancelChan <- timerID
}

// Stop stops the TimingWheel.
func (tw *TimingWheel) Stop() {
	tw.cancel()
	tw.wg.Wait()
}

// getExpired TimingWheel的寻找超时任务函数
func (tw *TimingWheel) getExpired() []*timerType {
	var expired = make([]*timerType, 0)
	for tw.timers.Len() > 0 {
		timer := heap.Pop(&tw.timers).(*timerType)
		elapsed := time.Since(timer.expiration).Seconds()
		if elapsed > 1.0 {
			fmt.Printf("elapsed %f\n", elapsed)
			//holmes.Warnf("elapsed %f\n", elapsed)
		}
		if elapsed > 0.0 {
			expired = append(expired, timer)
			continue
		} else {
			heap.Push(&tw.timers, timer)
			break
		}
	}
	return expired
}

func (tw *TimingWheel) update(timers []*timerType) {
	if timers != nil {
		for _, t := range timers {
			if t.isRepeat() { // repeatable timer task
				t.expiration = t.Next
				t.interval = time.Duration(t.Prev.Unix() - 8*3600)
				t.interval = t.Next.Sub(t.Prev)
				fmt.Println(`update contab cond id:`, t.id, ` 下次执行时刻:`, t.expiration, ` 时间间隔`, t.interval)
				// if task time out for at least 10 seconds, the expiration time needs
				// to be updated in case this task executes every time timer wakes up.
				if time.Since(t.expiration).Seconds() >= 10.0 {
					t.expiration = time.Now()
				}
				heap.Push(&tw.timers, t)
			}
		}
	}
}

// start函数，当创建一个TimeingWheel时，通过一个goroutine来执行start,在start中for循环和select来监控不同的channel的状态s
// <-tw.cancelChan 返回要取消的定时任务的id，并且在队列中删除
// tw.sizeChan <- 将定时任务的个数放入这个无缓存的channel中
// <-tw.ctx.Done() 当父context执行cancel时，该channel 就会有数值，表示该TimeingWheel 要停止
// <-tw.addChan 通过带缓存的addChan来向队列中添加任务
// <-tw.ticker.C ticker定时，当每一个ticker到来时，time包就会向该channel中放入当前Time，
// 当每一个Ticker到来时，TimeingWheel都需要检查队列中到到期的任务（tw.getExpired()），通过range来放入TimeOutChannelchannel中， 最后在更新队列。
func (tw *TimingWheel) start() {
	for {
		select {
		case timerID := <-tw.cancelChan:
			index := tw.timers.getIndexByID(timerID)
			if index >= 0 {
				heap.Remove(&tw.timers, index)
			}

		case tw.sizeChan <- tw.timers.Len():

		case <-tw.ctx.Done():
			tw.ticker.Stop()
			return

		case timer := <-tw.addChan:
			heap.Push(&tw.timers, timer)

		case <-tw.ticker.C:
			timers := tw.getExpired()
			for _, t := range timers {
				tw.TimeOutChannel() <- t.timeout
				t.Prev = t.Next
				t.Next = t.Schedule.Next(t.Prev)
				// 每一个执行完的任务都要重新计算一次下一次的执行时间
			}
			tw.update(timers)
		}
	}
}
