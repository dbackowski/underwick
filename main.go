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
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	tile         = 12
	viewW, viewH = 20, 15 // tiles of the floor on screen at once
	screenW      = viewW * tile
	screenH      = (viewH+1)*tile + msgH // the view, a HUD row, and two lines of messages
	msgH         = 2*lineH + 2           // room for the font's descenders
	lineH        = 9                     // the Oryx font is drawn at 8px
	scale        = 4
	animTime     = 12 // ticks a walk/attack animation plays after a turn
)

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
	shot = flag.String("shot", "", "save one rendered frame to this PNG and exit, to check rendering without a screen capture")
	font *text.GoTextFace

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
	turnTick int       // tick of the last turn, to time its animations
	lastTurn int       // the world's turn count when the screen last looked
	floats   []floater // HP changes rising off creatures
	lookX    int       // the tile being looked at, in "look" mode
	lookY    int
}

// loadSprites keys each sprite by its path under assets/ without extension, e.g. "Character/rat_idle_d_1".
// The folder stays in the key because Bosses/ and Character/ share names (cyclops, demon).
func loadSprites() map[string]*ebiten.Image {
	sprites := map[string]*ebiten.Image{}
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

func loadFont() *text.GoTextFace {
	data, err := assets.ReadFile("assets/oryx-simplex.ttf")
	if err != nil {
		log.Fatal(err)
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}
	// At full window resolution: drawn small and enlarged, the glyphs' anti-aliased edges blur.
	return &text.GoTextFace{Source: src, Size: 8 * scale}
}

func justPressed(keys ...ebiten.Key) bool {
	return slices.ContainsFunc(keys, inpututil.IsKeyJustPressed)
}

// direction reads a just-pressed arrow key or WASD.
func direction() (dx, dy int) {
	switch {
	case justPressed(ebiten.KeyArrowLeft, ebiten.KeyA):
		return -1, 0
	case justPressed(ebiten.KeyArrowRight, ebiten.KeyD):
		return 1, 0
	case justPressed(ebiten.KeyArrowUp, ebiten.KeyW):
		return 0, -1
	case justPressed(ebiten.KeyArrowDown, ebiten.KeyS):
		return 0, 1
	}
	return 0, 0
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
		case justPressed(ebiten.KeyH):
			g.back, g.mode = "title", "scores"
		case justPressed(ebiten.KeyK):
			g.back, g.mode = "title", "keys"
		case justPressed(ebiten.KeyQ, ebiten.KeyEscape):
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
	dx, dy := direction()

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
	case dx != 0 || dy != 0, justPressed(ebiten.KeySpace, ebiten.KeyPeriod):
		w.Step(dx, dy)
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

// afterAction starts the turn's animations, if the action took a turn, and floats its HP changes.
func (g *Game) afterAction() {
	w := g.world
	if w.Turn != g.lastTurn {
		g.lastTurn, g.turnTick = w.Turn, g.tick
	}
	stack := map[*Entity]int{}
	for _, h := range w.TakeHits() {
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
	if g.low == nil {
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

	// The art, enlarged with hard pixel edges, then the text over it at full resolution.
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	out.DrawImage(screen, op)
	for _, l := range g.labels {
		op := &text.DrawOptions{}
		op.GeoM.Translate(l.x*scale, l.y*scale)
		op.ColorScale.ScaleWithColor(l.c)
		text.Draw(out, l.s, font, op)
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
			if c != '#' { // doors, liquids and chests sit on floor
				g.draw(dst, "World/floor_"+theme.floor, x, y, alpha)
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
	cx := min(max(int(hx)-viewW/2*tile, 0), (mapW-viewW)*tile)
	cy := min(max(int(hy)-viewH/2*tile, 0), (mapH-viewH)*tile)
	screen.DrawImage(dst.SubImage(image.Rect(cx, cy, cx+viewW*tile, cy+viewH*tile)).(*ebiten.Image), nil)

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
	hud := float64(viewH*tile + 2)
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
	g.label(fmt.Sprintf("$%d  Deep %d", w.Gold, w.Depth), 160, hud, yellow)

	// Messages: this turn's events over two lines, dropping the oldest whole messages that don't fit.
	msgs := w.Log
	if g.mode == "look" { // word by word, so a long description wraps
		msgs = strings.Fields(w.Describe(g.lookX, g.lookY))
	}
	lines := wrap(msgs, screenW-4)
	for i, l := range lines[max(0, len(lines)-2):] {
		g.label(l, 2, float64((viewH+1)*tile+i*lineH), white)
	}

	switch {
	case g.mode == "use", g.mode == "drop":
		g.labels = g.labels[:0] // the panel covers the HUD and messages
		g.drawInventory(screen)
	case g.mode == "cast":
		g.labels = g.labels[:0]
		g.drawSpells(screen)
	case w.Over && g.mode == "":
		panel(screen, 8, 56, screenW-16, 4*lineH+8)
		g.label(fmt.Sprintf("Killed by %s on depth %d.", w.Cause, w.Depth), 14, 60, white)
		place := ""
		if g.recorded && g.place >= 0 {
			place = fmt.Sprintf(", number %d!", g.place+1)
		}
		g.label(fmt.Sprintf("Score %d%s", w.Score(), place), 14, 60+lineH, yellow)
		g.label("R: a new hero.  Esc: title screen.", 14, 60+2*lineH+4, grey)
	}
}

// drawTitle is the first screen: the name, and where to go from here.
func (g *Game) drawTitle(screen *ebiten.Image) {
	panel(screen, 2, 2, screenW-4, screenH-4)
	g.label("UNDERWICK", 92, 16, yellow)
	g.label("The dark below the village never ends.", 14, 16+lineH+2, grey)
	for i, c := range classes { // the heroes, waiting
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(78+i*18), 44)
		screen.DrawImage(g.sprite(fmt.Sprintf("Character/%s_idle_d_%d", c.Name, (g.tick/20+i)%2+1)), op)
	}
	options := [][2]string{{"N", "New game"}, {"H", "High scores"}, {"K", "Keys"}, {"Q", "Quit"}}
	if HasSave() {
		options = append([][2]string{{"C", "Continue"}, {"N", "New game (ends the saved run)"}}, options[1:]...)
	}
	for i, o := range options {
		y := 70 + float64(i)*(lineH+3)
		g.label(o[0], 40, y, yellow)
		g.label(o[1], 56, y, white)
	}
	if g.notice != "" {
		g.label(g.notice, 8, 136, yellow)
	}
	g.label("Art by Oryx Design Lab, oryxdesignlab.com", 6, screenH-4-lineH-2, grey)
}

// drawMenu is the pause menu during a run.
func (g *Game) drawMenu(screen *ebiten.Image) {
	panel(screen, 50, 50, screenW-100, 6*lineH+10)
	g.label("Paused", 58, 54, yellow)
	for i, o := range [][2]string{{"Esc", "Resume"}, {"K", "Keys"}, {"H", "High scores"}, {"Q", "Save and quit"}} {
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
		{"Arrows, WASD", "move, attack, open doors"},
		{"Space, .", "wait a turn"},
		{"G", "pick up"},
		{"I", "pack: a letter uses an item,"},
		{"", "or puts it on or off"},
		{"X", "drop an item"},
		{"C", "cast a spell, then aim it"},
		{"L", "look around: move the cursor"},
		{"M", "messages so far"},
		{"Esc", "menu, or close a panel"},
	} {
		y := 5 + float64(i+1)*lineH + 4
		g.label(l[0], 6, y, yellow)
		g.label(l[1], 84, y, white)
	}
	g.label("Locked doors open with their key.", 6, 5+12*lineH+4, grey)
	g.label("With a bow, moving at a monster shoots.", 6, 5+13*lineH+4, grey)
	g.label("Esc to go back.", 6, screenH-4-lineH-2, grey)
}

// drawLog shows the latest messages of the run, newest at the bottom.
func (g *Game) drawLog(screen *ebiten.Image) {
	panel(screen, 2, 2, screenW-4, screenH-4)
	g.label("Messages (Esc to close)", 6, 4, yellow)
	h := g.world.History
	fit := (screenH - 8 - lineH - 2) / lineH
	for i, m := range h[max(0, len(h)-fit):] {
		g.label(m, 6, 4+float64(i+1)*lineH+2, white)
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
	g.label("Esc to go back.", 6, screenH-4-lineH-2, grey)
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
	g.label("Esc to go back.", 6, screenH-4-lineH-2, grey)
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
	title := "Use or wear which? (Esc to close)"
	if g.mode == "drop" {
		title = "Drop which? (Esc to cancel)"
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
		label := fmt.Sprintf("%c) %s", 'a'+i, w.ItemName(it))
		if it.Worn {
			label += " (in use)"
		}
		g.label(label, 18, y, white)
	}
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

func panel(dst *ebiten.Image, x, y, w, h float32) {
	vector.FillRect(dst, x, y, w, h, color.RGBA{0x10, 0x10, 0x18, 0xf0}, false)
	vector.StrokeRect(dst, x, y, w, h, 1, grey, false)
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

// drawEntity draws a creature in its current animation frame, over a 1px outline of the given colour.
// at is where a creature is drawn this frame, in floor pixels: sliding over from where it began the
// turn (unless it jumped, like a teleport), and lunging halfway at whatever it struck in melee.
func (g *Game) at(e *Entity) (float64, float64) {
	x, y := float64(e.X*tile), float64(e.Y*tile)
	t := float64(g.tick-g.turnTick) / animTime
	if t >= 1 {
		return x, y
	}
	if d := abs(e.X-e.FromX) + abs(e.Y-e.FromY); d > 0 && d <= 2 {
		fx, fy := float64(e.FromX*tile), float64(e.FromY*tile)
		x, y = fx+(x-fx)*t, fy+(y-fy)*t
	}
	l := math.Sin(math.Pi*t) * tile / 2
	return x + float64(e.Lunge[0])*l, y + float64(e.Lunge[1])*l
}

func (g *Game) drawEntity(dst *ebiten.Image, e *Entity, outline color.RGBA) {
	since := g.tick - g.turnTick
	anim, frame := e.Anim, since/(animTime/2)%2+1
	if since >= animTime {
		anim, frame = "idle", g.tick/20%2+1
	}
	px, py := g.at(e)
	if e.Boss { // 24px sprite centred on its one tile, spilling over the neighbours
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(px-tile/2, py-tile/2)
		dst.DrawImage(g.sprite(fmt.Sprintf("Bosses/%s_%s_%s_%d", e.Name, anim, e.Dir, frame)), op)
		return
	}
	name := fmt.Sprintf("Character/%s_%s_%s_%d", e.Name, anim, e.Dir, frame)
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

func (g *Game) sprite(name string) *ebiten.Image {
	img, ok := g.sprites[name]
	if !ok {
		panic("missing sprite " + name)
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

func (g *Game) Layout(int, int) (int, int) { return screenW * scale, screenH * scale }

func main() {
	flag.Parse()
	font = loadFont()
	ebiten.SetWindowSize(screenW*scale, screenH*scale)
	ebiten.SetWindowTitle("Underwick")
	ebiten.SetWindowClosingHandled(true)
	if err := ebiten.RunGame(&Game{sprites: loadSprites(), mode: "title"}); err != nil {
		log.Fatal(err)
	}
}
