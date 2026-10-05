package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gomono"
)

const (
	tile     = 12
	baseW    = 20 * tile          // the smallest screen, in the art's pixels: 20 by 15 tiles of floor,
	baseH    = (15+1)*tile + msgH // a HUD row, and two lines of messages
	msgH     = 2*lineH + 2        // room for the font's descenders
	lineH    = 9                  // the Oryx font is drawn at 8px
	scale    = 4                  // the window's enlargement of the art, at first
	animTime = 12                 // ticks a walk/attack animation plays after a turn
)

// The screen in the art's pixels, and the part of it showing the floor: at least baseW by baseH, grown to
// fill the window by fit.
var (
	screenW, screenH = baseW, baseH
	viewW, viewH     = baseW, baseH - tile - msgH
)

// fit sizes the screen to fill a w×h window of real pixels: the art is enlarged by the largest whole
// number that still shows the smallest screen, and the floor's view grows to fill the rest, up to the
// whole floor. It returns that enlargement.
func fit(w, h int) int {
	s := max(1, min(w/baseW, h/baseH))
	viewW = min(max(w/s, baseW), mapW*tile)
	viewH = min(max(h/s, baseH)-tile-msgH, mapH*tile)
	screenW, screenH = viewW, viewH+tile+msgH
	return s
}

// Copied from the Oryx bundle: its Sliced/ folder and oryx-simplex.ttf. Gitignored: the license forbids
// redistributing them.
//
//go:embed assets
var assets embed.FS

// Tile sets, one per floor and repeating: the wall and stairs set, and the floor drawn with it.
type theme struct{ wall, floor string }

var themes = []theme{
	{"grey", "grey"}, {"dirt", "dirt"}, {"cave", "dark"},
	{"hedge", "moss"}, {"stone", "grey"}, {"red", "red"},
	{"frost", "frost"}, {"ice", "cold"}, {"turret", "mud"},
}

var (
	shot    = flag.String("shot", "", "save one rendered frame to this PNG and exit, to check rendering without a screen capture")
	ascii   = flag.Bool("ascii", false, "draw everything as coloured letters, as the game does without the Oryx art")
	iconset = flag.String("iconset", "", "write the app icon in every size into this .iconset folder and exit, for mac/app.sh")
	font    *text.GoTextFace // text at the base scale, for measuring; see faceAt for drawing
	fontSrc *text.GoTextFaceSource
	faces   = map[int]*text.GoTextFace{}

	white  = color.RGBA{0xff, 0xff, 0xff, 0xff}
	yellow = color.RGBA{0xff, 0xe0, 0x60, 0xff}
	grey   = color.RGBA{0xa0, 0xa0, 0xa0, 0xff}
)

type Game struct {
	sprites  map[string]*ebiten.Image
	world    *World
	floor    *ebiten.Image // the whole floor, drawn before the camera picks the part on screen
	low      *ebiten.Image // the screen at the art's own size, enlarged by scale with hard pixel edges
	labels   []label       // text for this frame, drawn after the enlargement at full resolution
	mode     string        // "" while playing; otherwise the screen or panel showing, e.g. "title", "use"
	back     string        // the mode to return to from the keys or scores panel
	spell    int           // the spell being aimed
	recorded bool          // the dead hero's score is on the list
	place    int           // its place there, from 0, or -1
	notice   string        // a problem to show on the title screen
	tick     int
	turnTick int                    // tick of the last turn, to time its animations
	lastTurn int                    // the world's turn count when the screen last looked
	floats   []floater              // HP changes rising off creatures
	drawn    map[*Entity][2]float64 // where each creature was drawn since the last turn, see at
	pending  [2]int                 // a direction pressed mid-slide, taken when it ends, see pace
	stuck    bool                   // the last step got nowhere, so a held key stops repeating, see pace
	sound    int                    // which of soundLevels is on, from 0
	from     map[*Entity][2]float64 // where each was drawn when this turn began, to slide on from there
	heard    heard                  // the hero's state when sounds last played
	music    *audio.Player          // the music playing, see playMusic
	tune     string                 // its name
	pixel    int                    // how many real pixels across an art pixel is drawn, see fit
	lookX    int                    // the tile being looked at, in "look" mode
	lookY    int
}

// loadSprites keys each sprite by its path under assets/ without extension, e.g. "Character/rat_idle_d_1".
// The folder stays in the key because Bosses/ and Character/ share names (cyclops, demon).
func loadSprites() map[string]*ebiten.Image {
	sprites := map[string]*ebiten.Image{}
	if !oryx() {
		return sprites // every sprite is drawn as its letter, see glyph
	}
	err := fs.WalkDir(assets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".png") {
			return err
		}
		f, err := assets.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		sprites[strings.TrimSuffix(strings.TrimPrefix(path, "assets/"), ".png")] = ebiten.NewImageFromImage(img)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return sprites
}

// oryx reports whether to draw with the Oryx art: it is in assets/, and -ascii doesn't turn it off.
func oryx() bool {
	_, err := assets.Open("assets/Character")
	return err == nil && !*ascii
}

func loadFont() *text.GoTextFace {
	data, err := assets.ReadFile("assets/oryx-simplex.ttf")
	if err != nil || !oryx() {
		data = gomono.TTF
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}
	fontSrc = src
	return faceAt(scale)
}

// faceAt is the font for art enlarged s times. Text is drawn at full resolution: drawn small and enlarged,
// the glyphs' anti-aliased edges would blur.
func faceAt(s int) *text.GoTextFace {
	if faces[s] == nil {
		faces[s] = &text.GoTextFace{Source: fontSrc, Size: float64(8 * s)}
	}
	return faces[s]
}

func justPressed(keys ...ebiten.Key) bool {
	return slices.ContainsFunc(keys, inpututil.IsKeyJustPressed)
}

// repeatDelay is how long, in ticks, a direction key is held before it repeats.
const repeatDelay = 18

// direction reads the arrow keys and WASD: a key just pressed, or else the latest pressed of those held
// for repeatDelay or more, so pressing a second key while holding one turns. held is how many ticks the
// key has been down, 1 for a fresh press, 0 for none; see steps.
func direction() (dx, dy, held int) {
	for _, d := range []struct {
		keys   []ebiten.Key
		dx, dy int
	}{
		{[]ebiten.Key{ebiten.KeyArrowLeft, ebiten.KeyA}, -1, 0},
		{[]ebiten.Key{ebiten.KeyArrowRight, ebiten.KeyD}, 1, 0},
		{[]ebiten.Key{ebiten.KeyArrowUp, ebiten.KeyW}, 0, -1},
		{[]ebiten.Key{ebiten.KeyArrowDown, ebiten.KeyS}, 0, 1},
	} {
		for _, k := range d.keys {
			switch t := inpututil.KeyPressDuration(k); {
			case t == 1:
				return d.dx, d.dy, 1
			case t >= repeatDelay && (held == 0 || t < held):
				dx, dy, held = d.dx, d.dy, t
			}
		}
	}
	return dx, dy, held
}

// pace decides whether a direction read now acts, see steps. While playing, steps never overlap: a press
// made mid-slide is kept and taken when the slide ends, so two keys pressed together step one after the
// other rather than as one 2-tile glide. And a held key stops at what it can't get past, rather than
// passing turns against a wall or walking on past a lava warning; pressing again goes ahead.
func (g *Game) pace(dx, dy, held int) (int, int) {
	since := g.tick - g.turnTick
	if !steps(held, since, g.mode == "") || held > 1 && g.stuck && g.mode == "" {
		dx, dy = 0, 0
	}
	switch {
	case g.mode != "":
		g.pending = [2]int{}
	case since < animTime && held == 1:
		g.pending, dx, dy = [2]int{dx, dy}, 0, 0
	case since >= animTime:
		if dx == 0 && dy == 0 {
			dx, dy = g.pending[0], g.pending[1]
		}
		g.pending = [2]int{}
	}
	return dx, dy
}

// steps reports whether a direction key down for held ticks acts now: when pressed, then once held for
// repeatDelay, again each time the last turn's slide ends (sinceTurn ticks ago) while playing, so a held
// walk is seamless and two held keys can't step twice in a blink, or every animTime elsewhere, like the
// look cursor.
func steps(held, sinceTurn int, playing bool) bool {
	switch {
	case held == 1:
		return true
	case held < repeatDelay:
		return false
	case playing:
		return sinceTurn >= animTime
	}
	return (held-repeatDelay)%animTime == 0
}

// letter returns the index of a letter typed this frame, a being 0, if it is below n.
func letter(n int) (int, bool) {
	for _, r := range ebiten.AppendInputChars(nil) {
		if i := int(r - 'a'); i >= 0 && i < n {
			return i, true
		}
	}
	return 0, false
}

// saveRun saves the run in progress, if there is one.
func (g *Game) saveRun() {
	if g.world != nil {
		if err := g.world.Save(); err != nil {
			log.Println("saving:", err)
		}
	}
}

func (g *Game) Update() error {
	g.tick++
	choosing := slices.Contains([]string{"use", "drop", "cast", "shop", "class"}, g.mode) // where F is a letter
	if justPressed(ebiten.KeyF11) || justPressed(ebiten.KeyF) && !choosing ||
		justPressed(ebiten.KeyEnter) && ebiten.IsKeyPressed(ebiten.KeyAlt) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	// The game has no use for the mouse, so in full screen its cursor is hidden. Checked every frame, since
	// the system can leave full screen too.
	cursor := ebiten.CursorModeVisible
	if ebiten.IsFullscreen() {
		cursor = ebiten.CursorModeHidden
	}
	if ebiten.CursorMode() != cursor {
		ebiten.SetCursorMode(cursor)
	}
	if justPressed(ebiten.KeyV) { // anywhere, so the music can be stopped from the title screen
		g.sound = (g.sound + 1) % len(soundLevels)
		if g.notice = soundLevels[g.sound]; g.world != nil {
			g.world.Log = []string{g.notice}
		}
	}
	g.playMusic()
	if ebiten.IsWindowBeingClosed() { // closing the window saves, like quitting from the menu
		g.saveRun()
		return ebiten.Termination
	}
	switch g.mode {
	case "title":
		switch {
		case justPressed(ebiten.KeyN):
			g.mode = "class"
		case justPressed(ebiten.KeyC) && HasSave():
			w, err := Continue()
			if err != nil {
				g.notice = "Couldn't continue: " + err.Error()
				return nil
			}
			g.world, g.mode, g.recorded, g.lastTurn = w, "", false, w.Turn
			if inBrowser { // continuing deleted the save, and a closed tab gives no warning
				g.saveRun()
			}
		case justPressed(ebiten.KeyH):
			g.back, g.mode = "title", "scores"
		case justPressed(ebiten.KeyK):
			g.back, g.mode = "title", "keys"
		case justPressed(ebiten.KeyQ, ebiten.KeyEscape) && !inBrowser: // a page has nothing to quit to
			return ebiten.Termination
		}
		return nil
	case "keys", "scores", "log":
		if justPressed(ebiten.KeyEscape, ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyM) {
			g.mode = g.back
		}
		return nil
	case "menu":
		switch {
		case justPressed(ebiten.KeyEscape):
			g.mode = ""
		case justPressed(ebiten.KeyK):
			g.back, g.mode = "menu", "keys"
		case justPressed(ebiten.KeyH):
			g.back, g.mode = "menu", "scores"
		case justPressed(ebiten.KeyQ):
			g.saveRun()
			if inBrowser {
				g.mode = "title"
				return nil
			}
			return ebiten.Termination
		}
		return nil
	case "class":
		if justPressed(ebiten.KeyEscape) {
			g.mode = "title"
		} else if i, ok := letter(len(classes)); ok {
			DeleteSave() // a new run gives up the saved one
			g.world, g.mode, g.recorded, g.lastTurn = NewGame(rand.Uint64(), classes[i]), "", false, 0
		}
		return nil
	}
	w := g.world
	if w.Over {
		if !g.recorded {
			g.place, g.recorded = w.RecordScore(), true
			DeleteSave()
		}
		switch {
		case justPressed(ebiten.KeyR):
			g.mode = "class"
		case justPressed(ebiten.KeyEscape):
			g.mode = "title"
		}
		return nil
	}
	dx, dy := g.pace(direction())

	switch g.mode { // choosing: a letter picks, Escape gives up
	case "use", "drop", "cast":
		if justPressed(ebiten.KeyEscape) {
			g.mode = ""
			return nil
		}
		n := len(w.Inventory)
		if g.mode == "cast" {
			n = len(w.Spells)
		}
		i, ok := letter(n)
		if !ok {
			return nil
		}
		switch {
		case g.mode == "drop":
			w.Drop(i)
		case g.mode == "use":
			w.Use(i)
		case w.Spells[i].Aimed:
			g.mode, g.spell = "aim", i
			w.Log = []string{"Cast it which way? (Esc to cancel)"}
			return nil
		default:
			w.Cast(i, 0, 0)
		}
		g.mode = ""
		g.afterAction()
		return nil
	case "shop": // a letter buys: the wares, then healing, then identifying
		if justPressed(ebiten.KeyEscape) {
			g.mode = ""
		} else if i, ok := letter(len(w.Wares) + 2); ok {
			switch i - len(w.Wares) {
			case 0:
				i = buyHeal
			case 1:
				i = buyIdentify
			}
			w.Buy(i)
		}
		return nil
	case "look":
		if justPressed(ebiten.KeyEscape, ebiten.KeyL) {
			g.mode = ""
		}
		g.lookX, g.lookY = min(max(g.lookX+dx, 0), mapW-1), min(max(g.lookY+dy, 0), mapH-1)
		return nil
	case "aim":
		if justPressed(ebiten.KeyEscape) {
			g.mode, w.Log = "", nil
		} else if dx != 0 || dy != 0 {
			w.Cast(g.spell, dx, dy)
			g.mode = ""
			g.afterAction()
		}
		return nil
	}

	switch {
	case (dx != 0 || dy != 0) && w.Level[w.Player.Y+dy][w.Player.X+dx] == '$':
		g.mode, w.Log = "shop", nil
		return nil
	case dx != 0 || dy != 0, justPressed(ebiten.KeySpace, ebiten.KeyPeriod):
		p := w.Player
		x, y, ahead := p.X, p.Y, w.Level[p.Y+dy][p.X+dx]
		w.Step(dx, dy)
		// Neither moved, struck nor changed what was ahead: a wall, a locked door, a lava warning.
		g.stuck = (dx != 0 || dy != 0) && p.X == x && p.Y == y && p.Anim != "atk" && w.Level[y+dy][x+dx] == ahead
	case justPressed(ebiten.KeyG):
		w.PickUp()
	case justPressed(ebiten.KeyI):
		g.mode = "use"
	case justPressed(ebiten.KeyX):
		g.mode = "drop"
	case justPressed(ebiten.KeyEscape):
		g.mode = "menu"
		return nil
	case justPressed(ebiten.KeyL):
		g.mode, g.lookX, g.lookY = "look", w.Player.X, w.Player.Y
		return nil
	case justPressed(ebiten.KeyM):
		g.back, g.mode = "", "log"
		return nil
	case justPressed(ebiten.KeyC):
		if len(w.Spells) == 0 {
			w.Log = []string{"You know no spells."}
		} else {
			g.mode = "cast"
		}
	default:
		return nil
	}
	g.afterAction()
	return nil
}

// afterAction starts the turn's animations and sounds, if the action took a turn, and floats its HP changes.
func (g *Game) afterAction() {
	w := g.world
	hs := w.TakeHits()
	if w.Turn != g.lastTurn {
		g.lastTurn, g.turnTick = w.Turn, g.tick
		if inBrowser { // a closed tab gives no warning, so the run is kept every turn
			g.saveRun()
		}
		if len(g.drawn) > 0 { // with no frame drawn since the last turn, the creatures are still where it began
			g.from = g.drawn
		}
		g.drawn = map[*Entity][2]float64{}
		g.playTurn(hs)
	}
	stack := map[*Entity]int{}
	for _, h := range hs {
		s, c := fmt.Sprint(h.Amount), color.NRGBA{0xff, 0xff, 0xff, 0xff}
		switch {
		case h.Kind == 'm':
			s, c = "miss", color.NRGBA{0xa0, 0xa0, 0xa0, 0xff}
		case h.Kind == 'h':
			s, c = "+"+s, color.NRGBA{0x60, 0xe0, 0x60, 0xff}
		case h.Kind == 'p':
			c = color.NRGBA{0xc0, 0x70, 0xff, 0xff}
		case h.To == w.Player:
			c = color.NRGBA{0xff, 0x50, 0x50, 0xff}
		}
		g.floats = append(g.floats, floater{s, h.To.X, h.To.Y, stack[h.To], c, g.tick})
		stack[h.To]++
	}
}

// floater is a number rising from a creature whose HP changed, fading as it goes.
type floater struct {
	s     string
	x, y  int // the tile it rises from
	stack int // how many rose from the same creature this turn before it, to keep them apart
	c     color.NRGBA
	born  int
}

const floatTime = 45 // ticks a number takes to rise and fade

func (g *Game) Draw(out *ebiten.Image) {
	if g.low == nil || g.low.Bounds() != image.Rect(0, 0, screenW, screenH) { // the window changed size
		g.low = ebiten.NewImage(screenW, screenH)
	}
	screen := g.low
	screen.Fill(color.Black)
	g.labels = g.labels[:0]
	if g.world != nil && g.mode != "title" {
		g.drawWorld(screen)
	}
	if full := map[string]func(*ebiten.Image){
		"title": g.drawTitle, "class": g.drawClasses, "menu": g.drawMenu, "keys": g.drawKeys, "scores": g.drawScores,
		"log": g.drawLog,
	}[g.mode]; full != nil {
		g.labels = g.labels[:0] // the panel covers everything
		full(screen)
	}

	// The art, enlarged with hard pixel edges by the whole number fit chose and centred, then the text over
	// it at full resolution.
	b := out.Bounds()
	s := max(1, g.pixel)
	ox, oy := float64((b.Dx()-screenW*s)/2), float64((b.Dy()-screenH*s)/2)
	out.Fill(color.Black) // the bars around it
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(s), float64(s))
	op.GeoM.Translate(ox, oy)
	out.DrawImage(screen, op)
	for _, l := range g.labels {
		op := &text.DrawOptions{}
		op.GeoM.Translate(l.x*float64(s)+ox, l.y*float64(s)+oy)
		op.ColorScale.ScaleWithColor(l.c)
		text.Draw(out, l.s, faceAt(s), op)
	}
	if *shot != "" && g.tick > 30 { // let a few idle frames pass first
		saveShot(out, *shot)
	}
}

// drawWorld draws the floor around the hero, the HUD, the messages and any open panel.
func (g *Game) drawWorld(screen *ebiten.Image) {
	w := g.world
	if g.floor == nil {
		g.floor = ebiten.NewImage(mapW*tile, mapH*tile)
	}
	dst := g.floor
	dst.Clear()

	// Tiles never seen stay black; seen ones out of sight are drawn dim, as remembered.
	theme := themes[(w.Depth-1)%len(themes)]
	for y, row := range w.Level {
		for x, c := range row {
			if !w.Seen[y][x] {
				continue
			}
			alpha := dim(w, x, y)
			if c != '#' { // doors, liquids, chests and the merchant sit on floor
				g.draw(dst, "World/floor_"+theme.floor, x, y, alpha)
			}
			if c == '$' {
				g.draw(dst, fmt.Sprintf("Character/dwarf_idle_d_%d", g.tick/20%2+1), x, y, alpha)
				continue
			}
			g.draw(dst, "World/"+tileSprite(byte(c), theme, g.tick/30%2+1), x, y, alpha)
		}
	}
	// Items stay where the hero last saw them; nothing else moves them.
	for _, it := range w.Floor {
		if w.Seen[it.Y][it.X] {
			g.draw(dst, "World/"+w.ItemSprite(it), it.X, it.Y, dim(w, it.X, it.Y))
		}
	}
	// Only monsters in sight are drawn. A faint light outline keeps dark ones visible on dark floors.
	var shown []*Entity
	for _, m := range w.Monsters {
		if w.Visible[m.Y][m.X] {
			shown = append(shown, m)
		}
	}
	faint := color.RGBA{0x90, 0x90, 0x90, 0x60}
	for _, m := range shown {
		if m.Lunge == [2]int{} {
			g.drawEntity(dst, m, faint)
		}
	}
	// The hero gets a blue outline, so it stands out from monsters that look like heroes.
	g.drawEntity(dst, w.Player, color.RGBA{0x40, 0x99, 0xff, 0xff})
	for _, m := range shown { // a monster lunging at the hero is drawn over it
		if m.Lunge != [2]int{} {
			g.drawEntity(dst, m, faint)
		}
	}

	// Over the sprites: HP bars on hurt monsters, and an alert on those that just spotted the player.
	for _, m := range shown {
		mx, my := g.at(m)
		x, y, width := float32(mx+1), float32(my), float32(tile-2)
		if m.Boss { // its sprite reaches half a tile beyond its own
			x, y, width = float32(mx-tile/2), float32(my-tile/2-2), float32(2*tile)
		}
		if m.HP < m.MaxHP {
			hpBar(dst, x, y, width, 1, m.HP, m.MaxHP)
		}
		icon := statusIcon(m)
		if m.Spotted {
			icon = "status_alert"
		}
		if icon != "" {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x+width/2-4), float64(y-9)) // FX sprites are 8px
			dst.DrawImage(g.sprite(fmt.Sprintf("FX/%s_%d", icon, g.tick/10%2+1)), op)
		}
	}
	if icon := statusIcon(w.Player); icon != "" {
		op := &ebiten.DrawImageOptions{}
		px, py := g.at(w.Player)
		op.GeoM.Translate(px+2, py-9)
		dst.DrawImage(g.sprite(fmt.Sprintf("FX/%s_%d", icon, g.tick/10%2+1)), op)
	}

	// Missiles fly from shooter to target while the turn's animation plays.
	if since := g.tick - g.turnTick; since < animTime {
		t := float64(since) / animTime
		for _, s := range w.Shots {
			if !w.Visible[s.FromY][s.FromX] && !w.Visible[s.To.Y][s.To.X] {
				continue
			}
			name := "FX/" + s.Missile
			if s.Missile == "arrow" {
				name = "FX/arrow_x"
				if s.FromX == s.To.X {
					name = "FX/arrow_y"
				}
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate( // FX sprites are 8px, centred in the 12px tile
				float64(s.FromX*tile)+float64((s.To.X-s.FromX)*tile)*t+2,
				float64(s.FromY*tile)+float64((s.To.Y-s.FromY)*tile)*t+2)
			dst.DrawImage(g.sprite(name), op)
		}
	}

	// The camera keeps the hero centred as it moves, or the tile being looked at, stopping at the
	// floor's edges.
	hx, hy := g.at(w.Player)
	if g.mode == "look" {
		hx, hy = float64(g.lookX*tile), float64(g.lookY*tile)
		vector.StrokeRect(dst, float32(hx)+0.5, float32(hy)+0.5, tile-1, tile-1, 1, yellow, false)
	}
	cx := min(max(int(hx)+tile/2-viewW/2, 0), mapW*tile-viewW)
	cy := min(max(int(hy)+tile/2-viewH/2, 0), mapH*tile-viewH)
	screen.DrawImage(dst.SubImage(image.Rect(cx, cy, cx+viewW, cy+viewH)).(*ebiten.Image), nil)

	// HP changes rise off the creatures and fade, where the hero can see.
	g.floats = slices.DeleteFunc(g.floats, func(f floater) bool { return g.tick-f.born >= floatTime })
	for _, f := range g.floats {
		if !w.Visible[f.y][f.x] {
			continue
		}
		age := float64(g.tick-f.born) / floatTime
		c := f.c
		c.A = uint8(255 * min(1, 2*(1-age))) // solid for the first half, then fading
		x := float64(f.x*tile+tile/2-cx) - text.Advance(f.s, font)/scale/2
		y := float64(f.y*tile-cy) - 6 - 8*age - float64(f.stack)*7
		g.label(f.s, x, y, c)
	}

	// HUD: the hero's HP, mana and level, a visible boss's HP, gold and the depth.
	hud := float64(viewH + 2)
	hpBar(screen, 2, float32(hud+1), 44, 4, max(w.Player.HP, 0), w.Player.MaxHP)
	stats := fmt.Sprintf("%d/%d", max(w.Player.HP, 0), w.Player.MaxHP)
	if w.MaxMana > 0 {
		vector.FillRect(screen, 2, float32(hud+6), 44, 2, color.RGBA{0x10, 0x20, 0x50, 0xff}, false)
		vector.FillRect(screen, 2, float32(hud+6), 44*float32(w.Mana)/float32(w.MaxMana), 2, color.RGBA{0x40, 0x80, 0xff, 0xff}, false)
		stats += fmt.Sprintf(" %dmp", w.Mana)
	}
	g.label(fmt.Sprintf("%s L%d", stats, w.ExpLevel), 50, hud, white)
	for _, m := range w.Monsters {
		if m.Boss && w.Visible[m.Y][m.X] {
			hpBar(screen, 116, float32(hud+2), 38, 4, m.HP, m.MaxHP)
		}
	}
	g.labelRight(fmt.Sprintf("$%d  Deep %d", w.Gold, w.Depth), hud, yellow)

	// Messages: this turn's events over two lines, dropping the oldest whole messages that don't fit.
	lines := wrap(w.Log, screenW-4)
	lines = lines[max(0, len(lines)-2):]
	if g.mode == "look" { // word by word, so a long description wraps, keeping its start
		lines = wrap(strings.Fields(w.Describe(g.lookX, g.lookY)), screenW-4)
		lines = lines[:min(len(lines), 2)]
	}
	for i, l := range lines {
		g.label(l, 2, float64(viewH+tile+i*lineH), white)
	}

	switch {
	case g.mode == "use", g.mode == "drop":
		g.labels = g.labels[:0] // the panel covers the HUD and messages
		g.drawInventory(screen)
	case g.mode == "shop":
		g.labels = g.labels[:0]
		g.drawShop(screen)
	case g.mode == "cast":
		g.labels = g.labels[:0]
		g.drawSpells(screen)
	case w.Over && g.mode == "":
		g.labels = g.labels[:0] // the recap covers everything
		g.drawRecap(screen)
	}
}

// drawRecap sums up the dead hero's run: how it ended, how far it got, what it had, and its last messages.
func (g *Game) drawRecap(screen *ebiten.Image) {
	w, p := g.world, g.world.Player
	panel(screen, 2, 2, screenW-4, screenH-4)
	y := 4.0
	para := func(parts []string, c color.Color) { // wrapped between parts, never inside one
		for _, l := range wrap(parts, screenW-12) {
			g.label(l, 6, y, c)
			y += lineH
		}
	}
	place := ""
	if g.recorded && g.place >= 0 {
		place = fmt.Sprintf(" (number %d)", g.place+1)
	}
	var gear []string
	for _, it := range w.Inventory {
		if it.Worn {
			gear = append(gear, w.ItemName(it)+",")
		}
	}
	if len(gear) == 0 {
		gear = []string{"nothing."}
	}
	gear[len(gear)-1] = strings.TrimSuffix(gear[len(gear)-1], ",") + "."

	para([]string{fmt.Sprintf("Killed by %s on depth %d.", w.Cause, w.Depth)}, yellow)
	para([]string{fmt.Sprintf("A level %d %s,", w.ExpLevel, w.Class.Name), fmt.Sprintf("after %d turns.", w.Turn)}, white)
	para([]string{fmt.Sprintf("Score %d%s,", w.Score(), place), fmt.Sprintf("%d kills,", w.Kills), fmt.Sprintf("%d gold.", w.Gold)}, white)
	para([]string{fmt.Sprintf("Max HP %d,", p.MaxHP), fmt.Sprintf("accuracy %d,", p.Atk), fmt.Sprintf("damage 1-%d,", p.Dmg), fmt.Sprintf("armour %d.", p.Def)}, white)
	y += 4
	para(append([]string{"Using"}, gear...), grey)
	y += 4

	g.label("Last messages", 6, y, yellow)
	y += lineH
	var lines []string
	fit := int((float64(screenH-6-lineH-2)-y)/lineH) - 1 // above the keys, with a line between
	h := w.History
	for _, m := range h[max(0, len(h)-fit):] {
		lines = append(lines, wrap(strings.Fields(m), screenW-12)...)
	}
	for _, l := range lines[max(0, len(lines)-fit):] {
		g.label(l, 6, y, white)
		y += lineH
	}
	g.label("R: a new hero.  Esc: title screen.", 6, float64(screenH-4-lineH-2), grey)
}

// drawTitle is the first screen: the name, and where to go from here.
func (g *Game) drawTitle(screen *ebiten.Image) {
	panel(screen, 2, 2, screenW-4, screenH-4)
	// Laid out for the smallest screen, and centred in a bigger one.
	ox, oy := float64(screenW-baseW)/2, float64(screenH-baseH)/2
	g.label("UNDERWICK", ox+92, oy+16, yellow)
	g.label("The dark below the village never ends.", ox+14, oy+16+lineH+2, grey)
	for i, c := range classes { // the heroes, waiting
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(math.Round(ox)+float64(78+i*18), math.Round(oy)+44)
		screen.DrawImage(g.sprite(fmt.Sprintf("Character/%s_idle_d_%d", c.Name, (g.tick/20+i)%2+1)), op)
	}
	options := [][2]string{{"N", "New game"}, {"H", "High scores"}, {"K", "Keys"}, {"Q", "Quit"}}
	if inBrowser {
		options = options[:3]
	}
	if HasSave() {
		options = append([][2]string{{"C", "Continue"}, {"N", "New game (ends the saved run)"}}, options[1:]...)
	}
	for i, o := range options {
		y := oy + 70 + float64(i)*(lineH+3)
		g.label(o[0], ox+40, y, yellow)
		g.label(o[1], ox+56, y, white)
	}
	if g.notice != "" {
		g.label(g.notice, ox+8, oy+136, yellow)
	}
	if oryx() {
		g.label("Art by Oryx Design Lab, oryxdesignlab.com", ox+6, float64(screenH-4-lineH-2), grey)
	}
}

// drawMenu is the pause menu during a run.
func (g *Game) drawMenu(screen *ebiten.Image) {
	panel(screen, 50, 50, screenW-100, 6*lineH+10)
	g.label("Paused", 58, 54, yellow)
	quit := "Save and quit"
	if inBrowser {
		quit = "Save and leave"
	}
	for i, o := range [][2]string{{"Esc", "Resume"}, {"K", "Keys"}, {"H", "High scores"}, {"Q", quit}} {
		y := 54 + float64(i+1)*lineH + 4
		g.label(o[0], 58, y, yellow)
		g.label(o[1], 84, y, white)
	}
}

// drawKeys explains the controls.
func (g *Game) drawKeys(screen *ebiten.Image) {
	panel(screen, 2, 2, screenW-4, screenH-4)
	g.label("Keys", 6, 5, yellow)
	for i, l := range [][2]string{
		{"Arrows, WASD", "move, attack; hold to walk"},
		{"Space, .", "wait a turn"},
		{"G", "pick up"},
		{"I", "pack: a letter uses an item,"},
		{"", "or puts it on or off"},
		{"X", "drop an item"},
		{"C", "cast a spell, then aim it"},
		{"L", "look around with a cursor"},
		{"M", "messages so far"},
		{"Esc", "menu, or close a panel"},
		{"V", "sound: all, effects, none"},
		{"F", "full screen, on or off"},
	} {
		y := 5 + float64(i+1)*lineH + 4
		g.label(l[0], 6, y, yellow)
		g.label(l[1], 84, y, white)
	}
	g.label("Walk into a door to open it. Locked doors", 6, 5+14*lineH+4, grey)
	g.label("need their key.", 6, 5+15*lineH+4, grey)
	g.label("With a bow, moving at a monster shoots.", 6, 5+16*lineH+4, grey)
	g.label("Walk into a merchant to shop.", 6, 5+17*lineH+4, grey)
	g.label("Esc to go back.", 6, float64(screenH-4-lineH-2), grey)
}

// drawLog shows the latest messages of the run, newest at the bottom.
func (g *Game) drawLog(screen *ebiten.Image) {
	panel(screen, 2, 2, screenW-4, screenH-4)
	g.label("Messages (Esc to close)", 6, 4, yellow)
	h := g.world.History
	fit := (screenH - 8 - lineH - 2) / lineH
	var lines []string
	for _, m := range h[max(0, len(h)-fit):] { // each message takes a line at least
		lines = append(lines, wrap(strings.Fields(m), screenW-12)...)
	}
	for i, l := range lines[max(0, len(lines)-fit):] {
		g.label(l, 6, 4+float64(i+1)*lineH+2, white)
	}
}

// drawScores lists the best runs.
func (g *Game) drawScores(screen *ebiten.Image) {
	panel(screen, 2, 2, screenW-4, screenH-4)
	g.label("High scores", 6, 5, yellow)
	ss := LoadScores()
	if len(ss) == 0 {
		g.label("No runs yet.", 6, 5+lineH+4, grey)
	}
	for i, s := range ss {
		y := 5 + float64(i+1)*(lineH+5)
		g.label(fmt.Sprintf("%2d. %5d  %s, depth %d", i+1, s.Points, s.Class, s.Depth), 6, y, white)
		g.label(fmt.Sprintf("killed by %s, %s", s.Cause, s.Date), 34, y+lineH-2, grey)
	}
	g.label("Esc to go back.", 6, float64(screenH-4-lineH-2), grey)
}

// drawClasses lets the player pick a hero for a new run.
func (g *Game) drawClasses(screen *ebiten.Image) {
	panel(screen, 2, 2, screenW-4, screenH-4)
	g.label("UNDERWICK", 6, 5, yellow)
	g.label("Choose your hero:", 6, 5+lineH+3, white)
	for i, c := range classes {
		y := 5 + float64(i)*3*lineH + 3*lineH
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(8, y)
		screen.DrawImage(g.sprite(fmt.Sprintf("Character/%s_idle_d_%d", c.Name, g.tick/20%2+1)), op)
		g.label(fmt.Sprintf("%c) %s", 'a'+i, strings.ToUpper(c.Name[:1])+c.Name[1:]), 24, y, white)
		g.label(c.About, 24, y+lineH, grey)
	}
	g.label("Esc to go back.", 6, float64(screenH-4-lineH-2), grey)
}

// drawSpells lists the spells the hero knows, lettered, for casting.
func (g *Game) drawSpells(screen *ebiten.Image) {
	w := g.world
	panel(screen, 2, 2, screenW-4, screenH-4)
	g.label(fmt.Sprintf("Cast which? You have %d mana. (Esc to close)", w.Mana), 6, 4, yellow)
	for i, s := range w.Spells {
		c := white
		if s.Cost > w.Mana {
			c = grey
		}
		g.label(fmt.Sprintf("%c) %s, %d mana", 'a'+i, s.Name, s.Cost), 6, 4+float64(i+1)*lineH+2, c)
	}
}

// tileSprite names the sprite for a map tile, under World/; frame animates liquids.
func tileSprite(c byte, th theme, frame int) string {
	switch c {
	case '#':
		return "wall_block_" + th.wall
	case '>':
		return "stair_down_" + th.wall
	case '+':
		return "door_wood_closed"
	case '/':
		return "door_wood_open"
	case '1':
		return "door_iron_closed"
	case '4':
		return "door_iron_open"
	case '2':
		return "door_magic_closed"
	case '5':
		return "door_magic_open"
	case '~':
		return fmt.Sprintf("liquid_water_%d", frame)
	case '=':
		return fmt.Sprintf("liquid_lava_%d", frame)
	case '%':
		return fmt.Sprintf("liquid_acid_%d", frame)
	case '^':
		return "floor_pit"
	case '&':
		return "object_chest_closed"
	case '0':
		return "object_chest_empty"
	}
	return "floor_" + th.floor
}

// drawInventory lists what the hero carries, lettered, for using or dropping.
func (g *Game) drawInventory(screen *ebiten.Image) {
	w := g.world
	panel(screen, 2, 2, screenW-4, screenH-4)
	title := "Use or wear which? Yellow: in use."
	if g.mode == "drop" {
		title = "Drop which? Yellow: in use."
	}
	g.label(title, 6, 4, yellow)
	if len(w.Inventory) == 0 {
		g.label("You carry nothing.", 6, 4+lineH+2, grey)
	}
	for i, it := range w.Inventory {
		y := 4 + float64(i+1)*lineH + 2
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(0.75, 0.75) // the 12px sprite in a 9px line
		op.GeoM.Translate(6, y-1)
		screen.DrawImage(g.sprite("World/"+w.ItemSprite(it)), op)
		c := white
		if it.Worn {
			c = yellow
		}
		g.label(fmt.Sprintf("%c) %s", 'a'+i, w.ItemName(it)), 18, y, c)
		g.labelRight(it.Bonuses(), y, grey)
	}
	p := w.Player
	g.label(fmt.Sprintf("You: %d max hp, 1-%d dmg, %d arm, %d acc", p.MaxHP, p.Dmg, p.Def, p.Atk), 6, float64(screenH-4-lineH-2), yellow)
}

// labelRight queues text flush with a full-width panel's right edge.
func (g *Game) labelRight(s string, y float64, c color.Color) {
	g.label(s, float64(screenW-6)-text.Advance(s, font)/scale, y, c)
}

// drawShop lists the merchant's wares and services, lettered, with their prices.
func (g *Game) drawShop(screen *ebiten.Image) {
	w := g.world
	panel(screen, 2, 2, screenW-4, screenH-4)
	g.label(fmt.Sprintf("The merchant's wares. You have %d gold.", w.Gold), 6, 4, yellow)
	y := 4.0
	row := func(i int, sprite, name string, price int) {
		y += lineH + 2
		if sprite != "" {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(0.75, 0.75) // the 12px sprite in a 9px line
			op.GeoM.Translate(6, y-1)
			screen.DrawImage(g.sprite("World/"+sprite), op)
		}
		c := white
		if price > w.Gold {
			c = grey
		}
		g.label(fmt.Sprintf("%c) %d", 'a'+i, price), 18, y, c)
		g.label(name, 56, y, c)
	}
	for i, it := range w.Wares {
		row(i, w.ItemSprite(it), w.ItemName(it), it.Price())
		g.labelRight(it.Bonuses(), y, grey)
	}
	if len(w.Wares) == 0 {
		y += lineH + 2
		g.label("Sold out.", 18, y, grey)
	}
	y += 4
	row(len(w.Wares), "", "heal your wounds", w.HealPrice())
	row(len(w.Wares)+1, "", "name your potions and scrolls", w.IdentifyPrice())
	if len(w.Log) > 0 {
		g.label(w.Log[len(w.Log)-1], 6, float64(screenH-4-2*lineH-4), yellow)
	}
	g.label("Esc to leave.", 6, float64(screenH-4-lineH-2), grey)
}

// wrap packs messages into lines no wider than width, never splitting one message across lines.
func wrap(msgs []string, width int) []string {
	var lines []string
	for _, m := range msgs {
		if n := len(lines); n > 0 && text.Advance(lines[n-1]+" "+m, font)/scale <= float64(width) {
			lines[n-1] += " " + m
		} else {
			lines = append(lines, m)
		}
	}
	return lines
}

type label struct {
	s    string
	x, y float64 // in the art's pixels, like everything else drawn on low
	c    color.Color
}

// label queues text for this frame, positioned in the art's pixels.
func (g *Game) label(s string, x, y float64, c color.Color) {
	g.labels = append(g.labels, label{s, x, y, c})
}

func panel(dst *ebiten.Image, x, y, w, h int) {
	vector.FillRect(dst, float32(x), float32(y), float32(w), float32(h), color.RGBA{0x10, 0x10, 0x18, 0xf0}, false)
	vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), 1, grey, false)
}

// statusIcon names the FX icon for a creature's worst condition, or "" if it has none.
func statusIcon(e *Entity) string {
	switch {
	case e.Sleep != 0:
		return "status_sleep"
	case e.Confused > 0:
		return "status_confuse"
	case e.Poison > 0:
		return "status_poisoned"
	}
	return ""
}

// dim is how bright to draw a seen tile: full in sight, dim when only remembered.
func dim(w *World, x, y int) float32 {
	if w.Visible[y][x] {
		return 1
	}
	return 0.35
}

func hpBar(dst *ebiten.Image, x, y, width, height float32, hp, maxHP int) {
	vector.FillRect(dst, x, y, width, height, color.RGBA{0x40, 0x10, 0x10, 0xff}, false)
	vector.FillRect(dst, x, y, width*float32(hp)/float32(maxHP), height, color.RGBA{0xe0, 0x30, 0x30, 0xff}, false)
}

func saveShot(screen *ebiten.Image, path string) {
	img := image.NewRGBA(screen.Bounds())
	screen.ReadPixels(img.Pix)
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	f.Close()
	os.Exit(0)
}

// at is where a creature is drawn this frame, in floor pixels: sliding over from where it was drawn as the
// turn began, so a move made before the last one finished carries on from mid-slide, and lunging halfway
// at whatever it struck in melee. A creature that jumped further than 2 tiles, by teleport or onto a
// new floor, appears at once.
func (g *Game) at(e *Entity) (x, y float64) {
	x, y = float64(e.X*tile), float64(e.Y*tile)
	defer func() {
		x, y = math.Round(x), math.Round(y) // whole art pixels, so the camera and the sprite never disagree by one
		if g.drawn == nil {
			g.drawn = map[*Entity][2]float64{}
		}
		g.drawn[e] = [2]float64{x, y}
	}()
	t := float64(g.tick-g.turnTick) / animTime
	if t >= 1 {
		return x, y
	}
	f, ok := g.from[e]
	if !ok {
		f = [2]float64{float64(e.FromX * tile), float64(e.FromY * tile)}
	}
	if math.Abs(x-f[0])+math.Abs(y-f[1]) <= 2*tile {
		x, y = f[0]+(x-f[0])*t, f[1]+(y-f[1])*t
	}
	l := math.Sin(math.Pi*t) * tile / 2
	return x + float64(e.Lunge[0])*l, y + float64(e.Lunge[1])*l
}

// walkFrames are where each direction's two walking frames really are: the Oryx pack files them under
// the wrong directions, the same way for every creature.
var walkFrames = map[string][2]string{"d": {"r_2", "u_2"}, "r": {"r_1", "u_1"}, "l": {"d_2", "l_2"}, "u": {"d_1", "l_1"}}

// drawEntity draws a creature in its current animation frame, over a 1px outline of the given colour.

func (g *Game) drawEntity(dst *ebiten.Image, e *Entity, outline color.RGBA) {
	since := g.tick - g.turnTick
	// Steps alternate on the clock rather than the turn, so quick moves still stride.
	pose := fmt.Sprintf("%s_%s_%d", e.Anim, e.Dir, since/(animTime/2)%2+1)
	switch {
	case since >= animTime:
		pose = fmt.Sprintf("idle_%s_%d", e.Dir, g.tick/20%2+1)
	case e.Anim == "walk":
		pose = "walk_" + walkFrames[e.Dir][g.tick/(animTime/2)%2]
	}
	px, py := g.at(e)
	if e.Boss { // 24px sprite centred on its one tile, spilling over the neighbours
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(px-tile/2, py-tile/2)
		dst.DrawImage(g.sprite("Bosses/"+e.Name+"_"+pose), op)
		return
	}
	name := "Character/" + e.Name + "_" + pose
	// A silhouette in the outline colour, shifted 1px each way, then the sprite over it.
	var cm colorm.ColorM
	cm.Scale(0, 0, 0, float64(outline.A)/0xff)
	cm.Translate(float64(outline.R)/0xff, float64(outline.G)/0xff, float64(outline.B)/0xff, 0)
	for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		op := &colorm.DrawImageOptions{}
		op.GeoM.Translate(px+float64(d[0]), py+float64(d[1]))
		colorm.DrawImage(dst, g.sprite(name), cm, op)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(px, py)
	dst.DrawImage(g.sprite(name), op)
}

// icon is the warrior (or its letter, without the Oryx files), cropped to its outline and enlarged with hard pixel edges to fill most of a
// size×size square, for the window and app icons.
func icon(size int) image.Image {
	src := glyph("Character/warrior_idle_d_1")
	if f, err := assets.Open("assets/Character/warrior_idle_d_1.png"); err == nil && oryx() {
		defer f.Close()
		if src, err = png.Decode(f); err != nil {
			log.Fatal(err)
		}
	}
	var b image.Rectangle // the opaque pixels
	for y := range tile {
		for x := range tile {
			if _, _, _, a := src.At(x, y).RGBA(); a > 0 {
				b = b.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	k := max(1, size*7/8/max(b.Dx(), b.Dy()))
	ox, oy := (size-b.Dx()*k)/2, (size-b.Dy()*k)/2
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range b.Dy() * k {
		for x := range b.Dx() * k {
			img.Set(ox+x, oy+y, src.At(b.Min.X+x/k, b.Min.Y+y/k))
		}
	}
	return img
}

// writeIconset writes the icon in the sizes macOS's iconutil expects in an .iconset folder.
func writeIconset(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, n := range []int{16, 32, 128, 256, 512} {
		for _, x := range []int{1, 2} {
			name := fmt.Sprintf("icon_%dx%d.png", n, n)
			if x == 2 {
				name = fmt.Sprintf("icon_%dx%d@2x.png", n, n)
			}
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				log.Fatal(err)
			}
			if err := png.Encode(f, icon(n*x)); err != nil {
				log.Fatal(err)
			}
			f.Close()
		}
	}
}

func (g *Game) sprite(name string) *ebiten.Image {
	img, ok := g.sprites[name]
	if !ok { // no Oryx file for it: its letter, made once
		img = ebiten.NewImageFromImage(glyph(name))
		g.sprites[name] = img
	}
	return img
}

func (g *Game) draw(dst *ebiten.Image, name string, x, y int, alpha float32) {
	img := g.sprite(name)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x*tile), float64(y*tile))
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(img, op)
}

func (g *Game) Layout(int, int) (int, int) { panic("LayoutF is used") }

// LayoutF makes the screen as big as the window in real pixels, so Draw can enlarge the art by whole
// numbers to fill a full screen or a Retina display without blurring.
func (g *Game) LayoutF(w, h float64) (float64, float64) {
	d := ebiten.Monitor().DeviceScaleFactor()
	w, h = math.Ceil(w*d), math.Ceil(h*d)
	g.pixel = fit(int(w), int(h))
	return w, h
}

func main() {
	flag.Parse()
	if *iconset != "" {
		writeIconset(*iconset)
		return
	}
	font = loadFont()
	audioCtx = audio.NewContext(sampleRate)
	loadSounds()
	ebiten.SetWindowSize(screenW*scale, screenH*scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// Full screen from the start; a browser only allows it after a key press, and a screenshot wants the window.
	ebiten.SetFullscreen(!inBrowser && *shot == "")
	ebiten.SetWindowIcon([]image.Image{icon(16), icon(32), icon(48), icon(64), icon(128)}) // not on macOS: see mac/app.sh
	ebiten.SetWindowTitle("Underwick")
	ebiten.SetWindowClosingHandled(true)
	if err := ebiten.RunGame(&Game{sprites: loadSprites(), mode: "title"}); err != nil {
		log.Fatal(err)
	}
}
