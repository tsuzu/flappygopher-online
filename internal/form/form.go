package form

import (
	"image"
	"image/color"
	"log"
	"strings"

	"github.com/cs3238-tsuzu/prasoba/text"
	textsoba "github.com/cs3238-tsuzu/prasoba/text"
	"github.com/cs3238-tsuzu/prasoba/transformer"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	screenWidth   = 640
	screenHeight  = 480
	fontSize      = 32
	smallFontSize = 13
)

var (
	chars = [][]byte{
		[]byte("1234567890<"),
		[]byte("qwertyuiop"),
		[]byte("asdfghjkl"),
		[]byte("zxcvbnm"),
	}

	keys = [][]ebiten.Key{
		{
			ebiten.Key1,
			ebiten.Key2,
			ebiten.Key3,
			ebiten.Key4,
			ebiten.Key5,
			ebiten.Key6,
			ebiten.Key7,
			ebiten.Key8,
			ebiten.Key9,
			ebiten.Key0,
			ebiten.KeyBackspace,
		},
		{

			ebiten.KeyQ,
			ebiten.KeyW,
			ebiten.KeyE,
			ebiten.KeyR,
			ebiten.KeyT,
			ebiten.KeyY,
			ebiten.KeyU,
			ebiten.KeyI,
			ebiten.KeyO,
			ebiten.KeyP,
		},
		{

			ebiten.KeyA,
			ebiten.KeyS,
			ebiten.KeyD,
			ebiten.KeyF,
			ebiten.KeyG,
			ebiten.KeyH,
			ebiten.KeyJ,
			ebiten.KeyK,
			ebiten.KeyL,
		},
		{
			ebiten.KeyZ,
			ebiten.KeyX,
			ebiten.KeyC,
			ebiten.KeyV,
			ebiten.KeyB,
			ebiten.KeyN,
			ebiten.KeyM,
		},
	}

	buttons [][]*textsoba.Text
	frames  [][]*transformer.Rect
	flags   [][]bool

	arcadeFont, smallArcadeFont font.Face

	capsText  *textsoba.Text
	capsFrame *transformer.Rect

	okText *textsoba.Text
)

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

	size := 60
	for i := range chars {
		btns := []*textsoba.Text{}
		frms := []*transformer.Rect{}

		y := screenHeight - (len(chars)-i)*size
		for j, r := range chars[i] {
			x := (screenWidth-len(chars[i])*size)/2 + j*size

			t := textsoba.NewText(string(r), arcadeFont).WithColor(color.White).Center(x+size/2, y+size/2)
			f := transformer.NewRect(image.Rectangle{Max: image.Pt(size, size)}).Center(x+size/2, y+size/2)

			btns = append(btns, t)
			frms = append(frms, f)
		}

		buttons = append(buttons, btns)
		frames = append(frames, frms)
		flags = append(flags, make([]bool, len(chars[i])))
	}

	{
		base := frames[len(chars)-1][0].Min()

		capsFrame = transformer.NewRect(image.Rectangle{
			Min: image.Pt(0, base.Y),
			Max: image.Pt(base.X, screenHeight),
		})

		center := capsFrame.Min().Add(capsFrame.Max()).Div(2)

		capsText = text.NewText("CapsLock", smallArcadeFont).
			WithColor(color.White).
			Center(center.X, center.Y)
	}

	okText = text.NewText("OK", arcadeFont).
		WithColor(color.White).
		Center(screenWidth/2, 200)
}

type Form struct {
	caps bool
	form string
}

func (f *Form) Update() string {
	if capsFrame.Clicked() {
		f.caps = !f.caps
	}

	for i := range buttons {
		for j := range buttons[i] {
			clicked := frames[i][j].Clicked() || inpututil.IsKeyJustPressed(keys[i][j])

			if clicked {
				if chars[i][j] == '<' {
					if len(f.form) > 0 {
						f.form = f.form[:len(f.form)-1]
					}
				} else if len(f.form) < 10 {
					key := string(chars[i][j])

					if f.caps || ebiten.IsKeyPressed(ebiten.KeyShift) {
						key = strings.ToUpper(key)
					}

					f.form += key
				}
			}

			flags[i][j] = clicked
		}
	}

	if len(f.form) != 0 && (okText.Clicked() || ebiten.IsKeyPressed(ebiten.KeyEnter)) {
		name := f.form + " Gopher"
		f.form = ""

		return name
	}

	return ""
}

func (f *Form) drawBox(screen *ebiten.Image, frame *transformer.Rect, text *textsoba.Text, flag bool) {
	min := frame.Min()
	size := frame.Size()

	col := color.RGBA{90, 90, 90, 255}
	if flag {
		col = color.RGBA{128, 128, 128, 255}
	}
	ebitenutil.DrawRect(screen, float64(min.X), float64(min.Y), float64(size.X), float64(size.Y), col)

	text.Draw(screen)
}

func (f *Form) Draw(screen *ebiten.Image) {
	textsoba.NewText(f.form+" Gopher", arcadeFont).
		WithColor(color.White).
		Center(screenWidth/2, 100).
		Draw(screen)

	okText.Draw(screen)

	for i := range buttons {
		for j := range buttons[i] {
			f.drawBox(screen, frames[i][j], buttons[i][j], flags[i][j])
		}
	}

	f.drawBox(screen, capsFrame, capsText, f.caps)
}
