# wheeltimer

```
Linux
*    *    *    *    *    *
-    -    -    -    -    -
|    |    |    |    |    |
|    |    |    |    |    + year [optional]
|    |    |    |    +----- day of week (0 - 7) (Sunday=0 or 7)
|    |    |    +---------- month (1 - 12)
|    |    +--------------- day of month (1 - 31)
|    +-------------------- hour (0 - 23)
+------------------------- min (0 - 59)

Java(Spring) AND GO(wheeltimer)
*    *    *    *    *    *    *
-    -    -    -    -    -    -
|    |    |    |    |    |    |
|    |    |    |    |    |    + year [optional]
|    |    |    |    |    +----- day of week (0 - 7) (Sunday=0 or 7)
|    |    |    |    +---------- month (1 - 12)
|    |    |    +--------------- day of month (1 - 31)
|    |    +-------------------- hour (0 - 23)
|    +------------------------- min (0 - 59)
+------------------------------ second (0 - 59)

```

## crontab规则表达式验证网址

> 很实用的工具，推广一下： [crontab表达式验证网址](https://tool.lu/crontab/)


## wheeltimer: Use github.com/farmerx/wheeltimer

```
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

```


