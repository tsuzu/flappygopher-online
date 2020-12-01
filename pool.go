package main

import (
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

type Audio struct {
	pool *AudioPool
	*audio.Player
}

func (a *Audio) Close() error {
	a.pool.pool.Put(a.Player)

	return nil
}

type AudioPool struct {
	pool *sync.Pool
}

func NewAudioPool(init func() *audio.Player) *AudioPool {
	return &AudioPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return init()
			},
		},
	}
}

func (pool *AudioPool) Get() *Audio {
	audio := pool.pool.Get().(*audio.Player)

	return &Audio{
		Player: audio,
		pool:   pool,
	}
}
