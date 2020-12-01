package main

import (
	"math"
	"sync"
	"time"

	"github.com/cs3238-tsuzu/flappygopher-online/internal/message"
	"github.com/hajimehoshi/ebiten/v2"
)

type Gopher struct {
	// The gopher's position
	x16     int
	y16     int
	vy16    int
	running bool

	gopherImage *ebiten.Image

	volume    float64
	id, name  string
	updatedAt time.Time

	lock sync.RWMutex

	jumpPlayerPool      *AudioPool
	allocatedJumpPlayer *Audio
	hitPlayerPool       *AudioPool
	allocatedHitPlayer  *Audio

	playerLock sync.Mutex
}

func NewGopher(gopherImage *ebiten.Image, jumpPlayerPool, hitPlayerPool *AudioPool) *Gopher {
	g := &Gopher{}

	g.x16 = 0
	g.y16 = 100 * 16
	g.gopherImage = gopherImage
	g.jumpPlayerPool = jumpPlayerPool
	g.hitPlayerPool = hitPlayerPool
	g.volume = math.NaN()
	g.running = true

	return g
}

func (g *Gopher) reset() {
	g.x16 = 0
	g.y16 = 100 * 16
	g.running = true
}

func (g *Gopher) releasePlayer(force bool) {
	g.playerLock.Lock()
	defer g.playerLock.Unlock()

	if g.allocatedJumpPlayer != nil && (force || !g.allocatedJumpPlayer.IsPlaying()) {
		g.allocatedJumpPlayer.Close()
		g.allocatedJumpPlayer = nil
	}

	if g.allocatedHitPlayer != nil && (force || !g.allocatedHitPlayer.IsPlaying()) {
		g.allocatedHitPlayer.Close()
		g.allocatedHitPlayer = nil
	}
}

func (g *Gopher) play(player *Audio) {
	volume := g.volume
	if math.IsNaN(g.volume) {
		volume = 1
	}
	player.SetVolume(volume)

	player.Rewind()
	player.Play()
}

func (g *Gopher) playJumpSound() {
	g.playerLock.Lock()
	defer g.playerLock.Unlock()

	player := g.allocatedJumpPlayer

	if player == nil {
		player = g.jumpPlayerPool.Get()
		g.allocatedJumpPlayer = player
	}

	g.play(player)
}

func (g *Gopher) playHitSound() {
	g.playerLock.Lock()
	defer g.playerLock.Unlock()

	player := g.allocatedHitPlayer

	if player == nil {
		player = g.hitPlayerPool.Get()
		g.allocatedHitPlayer = player
	}

	g.play(player)
}

type PipeAtFn func(tileX int) (tileY int, ok bool)

func (g *Gopher) hit(pipeAtFn PipeAtFn) bool {
	if pipeAtFn == nil {
		return false
	}

	// if g.mode != ModeGame {
	// 	return false
	// }
	const (
		gopherWidth  = 30
		gopherHeight = 60
	)
	w, h := gopherImage.Size()
	x0 := floorDiv(g.x16, 16) + (w-gopherWidth)/2
	y0 := floorDiv(g.y16, 16) + (h-gopherHeight)/2
	x1 := x0 + gopherWidth
	y1 := y0 + gopherHeight
	if y0 < -tileSize*4 {
		return true
	}
	if y1 >= screenHeight-tileSize {
		return true
	}
	xMin := floorDiv(x0-pipeWidth, tileSize)
	xMax := floorDiv(x0+gopherWidth, tileSize)
	for x := xMin; x <= xMax; x++ {
		y, ok := pipeAtFn(x)
		if !ok {
			continue
		}
		if x0 >= x*tileSize+pipeWidth {
			continue
		}
		if x1 < x*tileSize {
			continue
		}
		if y0 < y*tileSize {
			return true
		}
		if y1 >= (y+pipeGapY)*tileSize {
			return true
		}
	}
	return false
}

func (g *Gopher) Update(jump bool, pipeAtFn PipeAtFn) bool {
	g.releasePlayer(false)

	g.lock.Lock()

	if g.running {
		g.x16 += 32
		if jump {
			g.vy16 = -96

			g.playJumpSound()
		}
		g.y16 += g.vy16

		// Gravity
		g.vy16 += 4
		if g.vy16 > 96 {
			g.vy16 = 96
		}

		hit := g.hit(pipeAtFn)

		if hit {
			g.running = false
			g.lock.Unlock()

			g.playHitSound()

			return true
		}
	}

	g.lock.Unlock()

	return false
}

func (g *Gopher) UpdateByMessage(msg *message.User) {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.x16 = msg.X16
	g.y16 = msg.Y16
	g.vy16 = msg.VY16
	g.id = msg.ID
	g.name = msg.Name
	g.running = msg.Running
	g.updatedAt = time.Now()
}

func (g *Gopher) ComposeMessage() (msg *message.Message) {
	g.lock.RLock()
	defer g.lock.RUnlock()

	return &message.Message{
		Kind: message.KindUpdate,
		User: message.User{
			Name:    g.name,
			X16:     g.x16,
			Y16:     g.y16,
			VY16:    g.vy16,
			Running: g.running,
			Score:   g.score(),
		},
	}
}

func (g *Gopher) Pos() (int, int) {
	g.lock.RLock()
	defer g.lock.RUnlock()

	return g.x16, g.y16
}

func (g *Gopher) Draw(screen *ebiten.Image, cameraX, cameraY int) {
	g.lock.RLock()
	defer g.lock.RUnlock()

	op := &ebiten.DrawImageOptions{}
	w, h := g.gopherImage.Size()
	op.GeoM.Translate(-float64(w)/2.0, -float64(h)/2.0)
	op.GeoM.Rotate(float64(g.vy16) / 96.0 * math.Pi / 6)
	op.GeoM.Translate(float64(w)/2.0, float64(h)/2.0)
	op.GeoM.Translate(float64(g.x16/16.0)-float64(cameraX), float64(g.y16/16.0)-float64(cameraY))
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(g.gopherImage, op)
}

func (g *Gopher) score() int {
	x := floorDiv(g.x16, 16) / tileSize
	if (x - pipeStartOffsetX) <= 0 {
		return 0
	}
	return floorDiv(x-pipeStartOffsetX, pipeIntervalX)
}

func (g *Gopher) Score() int {
	g.lock.RLock()
	defer g.lock.RUnlock()

	return g.score()
}

func (g *Gopher) Close() error {
	g.releasePlayer(true)

	return nil
}
