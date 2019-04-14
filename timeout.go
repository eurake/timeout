package timeout

// 定时清除超时的map里的Key

import (
	"errors"
	"sync"
	"time"
)

type TimeoutMap struct {
	mutex           sync.Mutex
	cleanupTickTime time.Duration
	container       map[interface{}]*element
	cleaner         *time.Ticker
	cleanerStopChan chan bool
}

type element struct {
	value   interface{}
	expires time.Time
	cbs     []func(value interface{})
}

func New(cleanupTickTime time.Duration) *TimeoutMap {
	tm := &TimeoutMap{
		container:       make(map[interface{}]*element),
		cleanerStopChan: make(chan bool),
	}
	tm.cleaner = time.NewTicker(cleanupTickTime)

	go func() {
		for {
			select {
			case <-tm.cleaner.C:
				tm.cleanUp()
			case <-tm.cleanerStopChan:
				break
			}
		}
	}()

	return tm
}

func (tm *TimeoutMap) expireElement(k interface{}, v *element) {
	for _, cb := range v.cbs {
		cb(v.value)
	}

	tm.mutex.Lock()
	delete(tm.container, k)
	tm.mutex.Unlock()
}

// 把数据清除掉
func (tm *TimeoutMap) cleanUp() {
	now := time.Now()
	for k, v := range tm.container {
		if now.After(v.expires) {
			tm.expireElement(k, v)
		}
	}
}

func (tm *TimeoutMap) get(key interface{}) *element {
	tm.mutex.Lock()
	v, ok := tm.container[key]
	tm.mutex.Unlock()

	if !ok {
		return nil
	}

	if time.Now().After(v.expires) {
		tm.expireElement(key, v)
		return nil
	}
	return v
}

func (tm *TimeoutMap) Set(key, value interface{}, expiresAfter time.Duration, cb ...func(value interface{})) {
	tm.mutex.Lock()
	tm.container[key] = &element{
		value:   value,
		expires: time.Now().Add(expiresAfter),
		cbs:     cb,
	}
	tm.mutex.Unlock()
}

func (tm *TimeoutMap) Get(key interface{}) interface{} {
	v := tm.get(key)
	if v == nil {
		return nil
	}
	return v.value
}

func (tm *TimeoutMap) GetExpires(key interface{}) (time.Time, error) {
	v := tm.get(key)
	if v == nil {
		return time.Time{}, errors.New("key not found")
	}
	return v.expires, nil
}

func (tm *TimeoutMap) Contains(key interface{}) bool {
	return tm.get(key) != nil
}

func (tm *TimeoutMap) Remove(key interface{}) {
	tm.mutex.Lock()
	delete(tm.container, key)
	tm.mutex.Unlock()
}

func (tm *TimeoutMap) Refresh(key interface{}, d time.Duration) error {
	v := tm.get(key)
	if v == nil {
		return errors.New("key not found")
	}

	v.expires = v.expires.Add(d)
	return nil
}

func (tm *TimeoutMap) Flush() {
	tm.container = make(map[interface{}]*element)
}

func (tm *TimeoutMap) Size() int {
	return len(tm.container)
}

func (tm *TimeoutMap) StopCleaner() {
	go func() {
		tm.cleanerStopChan <- true
	}()
	tm.cleaner.Stop()
}
