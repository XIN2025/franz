package main

import (
	"sync"
	"time"
)

type Partition struct {
	id         int
	store      Storer
	consumers  map[string]*ConsumerGroup
	mutex      sync.RWMutex
	lastOffset int64
	created    time.Time
}

func NewPartition(id int, store Storer) *Partition {
	return &Partition{
		id:        id,
		store:     store,
		consumers: make(map[string]*ConsumerGroup),
		created:   time.Now(),
	}
}

func (p *Partition) Push(data []byte) (int64, error) {
	offset, err := p.store.Push(data)
	if err != nil {
		return 0, err
	}

	p.mutex.Lock()
	p.lastOffset = offset
	p.mutex.Unlock()

	return offset, nil
}
