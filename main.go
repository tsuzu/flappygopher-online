// Copyright 2018 The Ebiten Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math/rand"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"github.com/cs3238-tsuzu/flappygopher-online/internal/form"
	"github.com/cs3238-tsuzu/flappygopher-online/internal/message"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	raudio "github.com/hajimehoshi/ebiten/v2/examples/resources/audio"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	resources "github.com/hajimehoshi/ebiten/v2/examples/resources/images/flappy"
	"github.com/hajimehoshi/ebiten/v2/text"
)

func init() {
	// rand.Seed(time.Now().UnixNano())
}

func floorDiv(x, y int) int {
	d := x / y
	if d*y == x || x >= 0 {
		return d
	}
	return d - 1
}

func floorMod(x, y int) int {
	return x - floorDiv(x, y)*y
}

const (
	screenWidth      = 640
	screenHeight     = 480
	tileSize         = 32
	fontSize         = 32
	smallFontSize    = fontSize / 2
	pipeWidth        = tileSize * 2
	pipeStartOffsetX = 8
	pipeIntervalX    = 8
	pipeGapY         = 5
)

var (
	gopherImage     *ebiten.Image
	tilesImage      *ebiten.Image
	arcadeFont      font.Face
	smallArcadeFont font.Face
	nameFont        font.Face
)

func init() {
	img, _, err := image.Decode(bytes.NewReader(resources.Gopher_png))
	if err != nil {
		log.Fatal(err)
	}
	gopherImage = ebiten.NewImageFromImage(img)

	img, _, err = image.Decode(bytes.NewReader(resources.Tiles_png))
	if err != nil {
		log.Fatal(err)
	}
	tilesImage = ebiten.NewImageFromImage(img)
}

func init() {
	tt, err := opentype.Parse(fonts.PressStart2P_ttf)
	if err != nil {
		log.Fatal(err)
	}
	const dpi = 72
	arcadeFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	smallArcadeFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    smallFontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}

	nameFont, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    8,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
}

var (
	audioContext = audio.NewContext(44100)
	jumpD        *vorbis.Stream
	jabD         *wav.Stream
)

func init() {
	var err error

	jumpD, err = vorbis.Decode(audioContext, bytes.NewReader(raudio.Jump_ogg))
	if err != nil {
		log.Fatal(err)
	}

	jabD, err = wav.Decode(audioContext, bytes.NewReader(raudio.Jab_wav))
	if err != nil {
		log.Fatal(err)
	}
}

type Mode int

const (
	ModeForm Mode = iota
	ModeTitle
	ModeGame
	ModeGameOver
)

type Game struct {
	mode Mode

	me *Gopher

	// Camera
	cameraX int
	cameraY int

	// Pipes
	pipeTileYs []int

	gameoverCount int

	jumpPlayerPool *AudioPool
	hitPlayerPool  *AudioPool

	client       *Client
	otherPlayers []*Gopher
	standingText string
	standing     []message.Result
	form         *form.Form

	step int
}

func NewGame() *Game {
	g := &Game{}
	g.jumpPlayerPool = NewAudioPool(func() *audio.Player {
		jumpPlayer, err := audio.NewPlayer(audioContext, jumpD)
		if err != nil {
			log.Fatal(err)
		}

		return jumpPlayer
	})

	g.hitPlayerPool = NewAudioPool(func() *audio.Player {
		hitPlayer, err := audio.NewPlayer(audioContext, jabD)
		if err != nil {
			log.Fatal(err)
		}

		return hitPlayer
	})

	g.form = &form.Form{}
	g.me = NewGopher(gopherImage, g.jumpPlayerPool, g.hitPlayerPool)
	g.cameraX = -240
	g.pipeTileYs = make([]int, 256)
	for i := range g.pipeTileYs {
		g.pipeTileYs[i] = rand.Intn(6) + 2
	}

	var err error
	g.client, err = NewClient("http://localhost:7777/ws", func() *Gopher {
		return NewGopher(gopherImage, g.jumpPlayerPool, g.hitPlayerPool)
	})

	if err != nil {
		panic(err)
	}

	return g
}

func (g *Game) init() {
	g.me.reset()
	g.cameraX = -240
	g.cameraY = 0
	g.me.running = true

	go g.client.sendMessage(context.Background(), g.me.ComposeMessage())
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) Update() error {
	switch g.mode {
	case ModeForm:
		name := g.form.Update()

		if name != "" {
			g.me.name = name
			fmt.Println(g.me.name)
			g.mode = ModeTitle
		}
		return nil
	case ModeTitle:
		if jump() {
			g.mode = ModeGame
			g.init()
		}
	case ModeGame:
		// g.x16 += 32
		g.cameraX += 2

		j := jump()
		hit := g.me.Update(j, g.pipeAt)

		if hit {
			g.mode = ModeGameOver
			g.gameoverCount = 30
		}

		if j || hit {
			go g.client.sendMessage(context.Background(), g.me.ComposeMessage())
		}
	case ModeGameOver:
		if g.gameoverCount > 0 {
			g.gameoverCount--
		}
		if g.gameoverCount == 0 && jump() {
			// g.init()
			g.mode = ModeTitle
		}
	}

	g.otherPlayers = g.client.List()

	for i := range g.otherPlayers {
		g.otherPlayers[i].Update(false, nil)
	}

	standing := g.client.Standing()
	standingText := make([]string, len(standing))

	for i := range standing {
		standingText[i] = fmt.Sprintf("%s(%d)", standing[i].Name, standing[i].Score)
	}
	g.standingText = strings.Join(standingText, "  ")
	g.standing = standing

	g.step++

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.mode == ModeForm {
		g.form.Draw(screen)

		return
	}

	screen.Fill(color.RGBA{0x80, 0xa0, 0xc0, 0xff})
	g.drawTiles(screen)

	for i := range g.otherPlayers {
		g.otherPlayers[i].Draw(screen, g.cameraX, g.cameraY)
	}

	if g.mode != ModeTitle {
		g.me.Draw(screen, g.cameraX, g.cameraY)
	}
	var texts []string
	switch g.mode {
	case ModeTitle:
		texts = []string{"FLAPPY GOPHER ONLINE", "", "World Record", "", "", "PRESS SPACE KEY", "", "OR TOUCH SCREEN"}
	case ModeGameOver:
		texts = []string{"", "GAME OVER!"}
	}

	drawText := func(i int, l string) {
		x := (screenWidth - len(l)*fontSize) / 2
		text.Draw(screen, l, arcadeFont, x, (i+4)*fontSize, color.White)
	}
	for i, l := range texts {
		drawText(i, l)
	}

	if g.mode == ModeTitle {
		if len(g.standingText) != 0 {
			width := text.BoundString(arcadeFont, g.standingText).Dx()
			step := -g.step
			step %= width

			for step < screenWidth {
				text.Draw(screen, g.standingText, arcadeFont, step, 7*fontSize, color.White)

				step += width + 50
			}
		}

		msg := []string{
			"Go Gopher by Renee French is",
			"licenced under CC BY 3.0.",
		}
		for i, l := range msg {
			x := (screenWidth - len(l)*smallFontSize) / 2
			text.Draw(screen, l, smallArcadeFont, x, screenHeight-4+(i-1)*smallFontSize, color.White)
		}
	}

	score := g.me.Score()
	scoreStr := fmt.Sprintf("%04d", score)
	text.Draw(screen, scoreStr, arcadeFont, screenWidth-len(scoreStr)*fontSize, fontSize, color.White)

	tps := fmt.Sprintf("TPS: %0.2f", ebiten.CurrentTPS())
	if g.mode == ModeTitle {
		ebitenutil.DebugPrint(screen, tps)

	} else {
		message := make([]string, 0, len(g.standing)+1)

		message = append(message, tps+"\n")
		for i := range g.standing {
			message = append(message, fmt.Sprintf("%s(%d)", g.standing[i].Name, g.standing[i].Score))
		}

		ebitenutil.DebugPrint(screen, strings.Join(message, "\n"))
	}
}

func (g *Game) pipeAt(tileX int) (tileY int, ok bool) {
	if (tileX - pipeStartOffsetX) <= 0 {
		return 0, false
	}
	if floorMod(tileX-pipeStartOffsetX, pipeIntervalX) != 0 {
		return 0, false
	}
	idx := floorDiv(tileX-pipeStartOffsetX, pipeIntervalX)
	return g.pipeTileYs[idx%len(g.pipeTileYs)], true
}

func (g *Game) drawTiles(screen *ebiten.Image) {
	const (
		nx           = screenWidth / tileSize
		ny           = screenHeight / tileSize
		pipeTileSrcX = 128
		pipeTileSrcY = 192
	)

	op := &ebiten.DrawImageOptions{}
	for i := -2; i < nx+1; i++ {
		// ground
		op.GeoM.Reset()
		op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
			float64((ny-1)*tileSize-floorMod(g.cameraY, tileSize)))
		screen.DrawImage(tilesImage.SubImage(image.Rect(0, 0, tileSize, tileSize)).(*ebiten.Image), op)

		// pipe
		if tileY, ok := g.pipeAt(floorDiv(g.cameraX, tileSize) + i); ok {
			for j := 0; j < tileY; j++ {
				op.GeoM.Reset()
				op.GeoM.Scale(1, -1)
				op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
					float64(j*tileSize-floorMod(g.cameraY, tileSize)))
				op.GeoM.Translate(0, tileSize)
				var r image.Rectangle
				if j == tileY-1 {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY, pipeTileSrcX+tileSize*2, pipeTileSrcY+tileSize)
				} else {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY+tileSize, pipeTileSrcX+tileSize*2, pipeTileSrcY+tileSize*2)
				}
				screen.DrawImage(tilesImage.SubImage(r).(*ebiten.Image), op)
			}
			for j := tileY + pipeGapY; j < screenHeight/tileSize-1; j++ {
				op.GeoM.Reset()
				op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
					float64(j*tileSize-floorMod(g.cameraY, tileSize)))
				var r image.Rectangle
				if j == tileY+pipeGapY {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize)
				} else {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY+tileSize, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize+tileSize)
				}
				screen.DrawImage(tilesImage.SubImage(r).(*ebiten.Image), op)
			}
		}
	}
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Flappy Gopher Online")

	if err := ebiten.RunGame(NewGame()); err != nil {
		panic(err)
	}
}
