package main

import (
	"image"
	"image/color"
	"image/draw"
	"log"
	"slices"
	"strings"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Without the Oryx files, the game draws each sprite as a coloured letter, the way the old roguelikes
// did: @ for the hero, a monster's own level character, # for walls, ! for potions. glyph works out the
// letter from the sprite's name, so the game asks for sprites the same way either way.

type look struct {
	ch     rune
	fg, bg color.RGBA // bg fills the whole tile for floor-like sprites; transparent otherwise
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 0xff} }

// shade darkens a colour to f tenths.
func shade(c color.RGBA, f int) color.RGBA {
	return rgb(uint8(int(c.R)*f/10), uint8(int(c.G)*f/10), uint8(int(c.B)*f/10))
}

// themeColours tint each floor's walls and ground, by the themes' names.
var themeColours = map[string]color.RGBA{
	"grey": rgb(150, 150, 160), "dirt": rgb(170, 120, 70), "cave": rgb(130, 105, 95), "hedge": rgb(80, 150, 70),
	"stone": rgb(150, 145, 120), "red": rgb(190, 80, 70), "frost": rgb(150, 200, 230), "ice": rgb(120, 170, 220),
	"turret": rgb(165, 150, 130), "dark": rgb(100, 90, 90), "moss": rgb(80, 130, 70), "cold": rgb(110, 140, 170),
	"mud": rgb(130, 105, 70),
}

// creatureColours are by sprite name, heroes and the merchant included.
var creatureColours = map[string]color.RGBA{
	"warrior": rgb(220, 220, 230), "archer": rgb(120, 210, 110), "mage": rgb(120, 160, 255),
	"thief": rgb(170, 170, 170), "cleric": rgb(255, 240, 200), "dwarf": rgb(255, 210, 80),
	"rat": rgb(170, 120, 80), "bat": rgb(160, 130, 180), "snake": rgb(110, 190, 80), "slime": rgb(170, 230, 60),
	"gobwar": rgb(90, 170, 60), "gobarcher": rgb(150, 170, 60), "wolf": rgb(180, 180, 190),
	"skel": rgb(230, 225, 200), "spider": rgb(200, 70, 60), "zombie": rgb(140, 170, 110), "orc": rgb(60, 140, 60),
	"gnoll": rgb(200, 160, 100), "skelarcher": rgb(210, 200, 150), "imp": rgb(240, 90, 60),
	"ghost": rgb(190, 220, 255), "skelwar": rgb(240, 240, 255), "skelmage": rgb(130, 170, 255),
	"flame": rgb(255, 150, 40), "flayer": rgb(220, 90, 200), "demon": rgb(230, 50, 50),
	"dragon": rgb(240, 70, 50), "beholder": rgb(240, 130, 170), "lord": rgb(170, 100, 240),
	"cyclops": rgb(210, 170, 120), "reaper": rgb(200, 200, 200),
}

// lookOf is the letter for a sprite name like "Character/rat_idle_d_1" or "World/object_potion_red", and
// whether there is one.
func lookOf(name string) (look, bool) {
	folder, base, _ := strings.Cut(name, "/")
	parts := strings.Split(base, "_")
	last := parts[len(parts)-1]
	white, grey, yellow := rgb(240, 240, 240), rgb(150, 150, 150), rgb(255, 220, 90)
	switch folder {
	case "Character", "Bosses": // creature_anim_dir_frame
		c, ok := creatureColours[parts[0]]
		if !ok {
			return look{}, false
		}
		if parts[0] == "dwarf" || slices.ContainsFunc(classes, func(c *Class) bool { return c.Name == parts[0] }) {
			return look{ch: '@', fg: c}, true
		}
		for ch, k := range kinds {
			if k.Name == parts[0] && k.Boss == (folder == "Bosses") {
				return look{ch: rune(ch), fg: c}, true
			}
		}
	case "FX":
		switch {
		case base == "arrow_x":
			return look{ch: '-', fg: white}, true
		case base == "arrow_y":
			return look{ch: '|', fg: white}, true
		case parts[0] == "proj": // proj_colour_ball
			if c, ok := map[string]color.RGBA{"red": rgb(255, 80, 50), "orange": rgb(255, 160, 40),
				"blue": rgb(100, 160, 255), "green": rgb(120, 230, 90)}[parts[1]]; ok {
				return look{ch: '*', fg: c}, true
			}
		case parts[0] == "status": // status_kind_frame
			switch parts[1] {
			case "sleep":
				return look{ch: 'z', fg: rgb(150, 190, 255)}, true
			case "confuse":
				return look{ch: '?', fg: rgb(230, 130, 255)}, true
			case "poisoned":
				return look{ch: '*', fg: rgb(120, 230, 90)}, true
			case "alert":
				return look{ch: '!', fg: rgb(255, 80, 60)}, true
			}
		}
	case "World":
		theme := themeColours[last]
		water := map[string]color.RGBA{"water": rgb(50, 110, 220), "lava": rgb(220, 70, 30), "acid": rgb(110, 190, 40)}
		switch {
		case base == "floor_pit":
			return look{ch: '^', fg: grey, bg: rgb(15, 15, 15)}, true
		case parts[0] == "floor" && theme.A > 0:
			return look{ch: '.', fg: shade(theme, 5), bg: shade(theme, 2)}, true
		case parts[0] == "wall" && theme.A > 0:
			return look{ch: '#', fg: theme, bg: shade(theme, 4)}, true
		case parts[0] == "stair" && theme.A > 0:
			return look{ch: '>', fg: white, bg: shade(theme, 2)}, true
		case parts[0] == "door":
			c := map[string]color.RGBA{"wood": rgb(190, 130, 70), "iron": rgb(170, 175, 190), "magic": rgb(110, 160, 255)}[parts[1]]
			ch := '+'
			if last == "open" {
				ch = '/'
			}
			return look{ch: ch, fg: c, bg: shade(c, 3)}, c.A > 0
		case parts[0] == "liquid" && water[parts[1]].A > 0:
			c := water[parts[1]]
			ch := '~'
			if last == "2" { // its second frame, so it ripples
				ch = '≈'
			}
			return look{ch: ch, fg: shade(c, 10), bg: shade(c, 5)}, true
		case strings.HasPrefix(base, "object_chest"):
			c := rgb(200, 150, 60)
			if last == "empty" {
				c = grey
			}
			return look{ch: '&', fg: c}, true
		case base == "object_gold":
			return look{ch: '$', fg: yellow}, true
		case strings.HasPrefix(base, "object_key"):
			c := map[string]color.RGBA{"gold": yellow, "blue": rgb(110, 160, 255)}[last]
			return look{ch: '-', fg: c}, c.A > 0
		case strings.HasPrefix(base, "object_potion"):
			c := map[string]color.RGBA{"red": rgb(240, 70, 70), "blue": rgb(90, 140, 255), "green": rgb(90, 220, 90)}[last]
			return look{ch: '!', fg: c}, c.A > 0
		case base == "object_scroll":
			return look{ch: '?', fg: rgb(240, 230, 200)}, true
		case strings.HasPrefix(base, "object_tome"):
			return look{ch: '+', fg: rgb(230, 150, 90)}, true
		case base == "object_ring":
			return look{ch: '=', fg: yellow}, true
		case base == "object_amulet":
			return look{ch: '"', fg: yellow}, true
		case base == "object_bow":
			return look{ch: '}', fg: rgb(190, 140, 80)}, true
		case strings.Contains(" object_dagger object_sword object_staff object_spear object_axe object_hammer ", " "+base+" "):
			return look{ch: ')', fg: rgb(200, 210, 230)}, true
		case strings.HasPrefix(base, "object_"): // the rest is armour
			return look{ch: '[', fg: rgb(170, 190, 210)}, true
		}
	}
	return look{}, false
}

// glyph draws a sprite as its letter, at the size the Oryx sprite would have: 24px for bosses, 8px for
// effects, a tile for the rest. The letter's edges are made hard, to match the pixel art around it.
func glyph(name string) image.Image {
	l, ok := lookOf(name)
	if !ok {
		log.Fatalf("missing sprite %s, and no letter for it", name)
	}
	size := tile
	switch {
	case strings.HasPrefix(name, "Bosses/"):
		size = 2 * tile
	case strings.HasPrefix(name, "FX/"):
		size = 8
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.NewUniform(l.bg), image.Point{}, draw.Src)

	face := glyphFace(size)
	ink, _ := xfont.BoundString(face, string(l.ch))
	w, h := (ink.Max.X - ink.Min.X).Ceil(), (ink.Max.Y - ink.Min.Y).Ceil()
	mask := image.NewAlpha(img.Bounds())
	d := xfont.Drawer{Dst: mask, Src: image.Opaque, Face: face, Dot: fixed.Point26_6{ // the ink centred
		X: fixed.I((size-w+1)/2 - ink.Min.X.Floor()),
		Y: fixed.I((size-h+1)/2 - ink.Min.Y.Floor()),
	}}
	d.DrawString(string(l.ch))
	for i, a := range mask.Pix {
		if a >= 0x50 { // a low cut keeps thin strokes, so small letters stay readable
			img.SetNRGBA(i%size, i/size, color.NRGBA(l.fg))
		}
	}
	return img
}

var glyphFaces = map[int]xfont.Face{}

func glyphFace(size int) xfont.Face {
	if glyphFaces[size] == nil {
		f, err := opentype.Parse(gomonobold.TTF) // a size a quarter over the tile fills it, as sprites do
		if err != nil {
			log.Fatal(err)
		}
		glyphFaces[size], err = opentype.NewFace(f, &opentype.FaceOptions{Size: float64(size) * 5 / 4, DPI: 72, Hinting: xfont.HintingFull})
		if err != nil {
			log.Fatal(err)
		}
	}
	return glyphFaces[size]
}
