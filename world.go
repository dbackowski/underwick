package main

import "slices"

// Tuning knobs shared by all bodies. Per-body ones live in kinds.
const (
	sparkTurns = 3 // turns a bare spark survives without a body
	sight      = 6 // how far (in steps) monsters notice the player
)

// Kind is a body type. Name doubles as the sprite prefix.
type Kind struct {
	Name  string
	MaxHP int
	Dmg   int
	Moves int // actions per turn
	Range int // attack reach along a straight line: 1 is melee, more shoots arrows
	Decay int // a possessed body loses 1 HP every this many turns
}

// kinds maps a level character to a body type.
var kinds = map[byte]Kind{
	'r': {Name: "rat", MaxHP: 3, Dmg: 1, Moves: 2, Range: 1, Decay: 4},       // fast and fragile
	'a': {Name: "gobarcher", MaxHP: 5, Dmg: 2, Moves: 1, Range: 5, Decay: 4}, // shoots down a line
	'o': {Name: "orc", MaxHP: 10, Dmg: 3, Moves: 1, Range: 1, Decay: 2},      // strong, rots twice as fast
}

var spark = Kind{Name: "spark", MaxHP: 1, Moves: 1}

type Entity struct {
	Kind
	X, Y int
	HP   int
	Dir  string // l, r, u, d
	Anim string // what it did this step: idle, walk, atk
}

func newEntity(k Kind, x, y int) *Entity {
	return &Entity{Kind: k, X: x, Y: y, HP: k.MaxHP, Dir: "d", Anim: "idle"}
}

// Broken bodies can be possessed and are too hurt to act.
func (e *Entity) Broken() bool { return e.HP > 0 && e.HP*3 <= e.MaxHP }

// Shot is an arrow fired this step, kept for the renderer.
// It points at the target itself, which may have moved or died by the time the arrow is drawn.
type Shot struct {
	FromX, FromY int
	To           *Entity
}

type World struct {
	Level     []string
	Player    *Entity // a spark, or the body it possesses
	Monsters  []*Entity
	Shots     []Shot
	SparkLeft int // turns left to find a body while the player is a bare spark
	Over      bool
	decay     int // turns since the current body last decayed
	acted     int // player actions taken so far this turn
}

// NewWorld parses a level: '#' wall, '@' player (in a goblin archer body), kind letters for monsters.
func NewWorld(level []string) *World {
	w := &World{Level: level}
	for y, row := range level {
		for x := range row {
			if row[x] == '@' {
				w.Player = newEntity(kinds['a'], x, y)
			} else if k, ok := kinds[row[x]]; ok {
				w.Monsters = append(w.Monsters, newEntity(k, x, y))
			}
		}
	}
	return w
}

func (w *World) IsSpark() bool { return w.Player.Name == "spark" }

// Step is one player action (dx, dy of 0, 0 waits out the rest of the turn).
// The turn ends, and monsters act, once the body has used all its moves.
func (w *World) Step(dx, dy int) {
	if w.Over {
		return
	}
	w.Shots = nil
	w.Player.Anim = "idle"
	for _, m := range w.Monsters {
		m.Anim = "idle"
	}

	body := w.Player
	waited := dx == 0 && dy == 0
	if !waited {
		w.playerAct(dx, dy)
	}
	if w.acted++; !waited && w.acted < w.Player.Moves {
		return
	}
	w.acted = 0

	if w.IsSpark() {
		w.SparkLeft--
		if w.SparkLeft <= 0 {
			w.Over = true
			return
		}
	} else if w.Player == body { // a body possessed this turn starts decaying next turn
		if w.decay++; w.decay >= w.Player.Decay {
			w.decay = 0
			w.damage(w.Player, 1)
		}
	}

	for _, m := range w.Monsters {
		for range m.Moves {
			w.act(m)
		}
	}
}

func (w *World) playerAct(dx, dy int) {
	p := w.Player
	p.Dir = dirName(dx, dy)
	if t := w.firstInLine(p, dx, dy, p.Range); t != nil && !t.Broken() {
		w.attack(p, t)
		return
	}
	tx, ty := p.X+dx, p.Y+dy
	if m := w.monsterAt(tx, ty); m != nil && m.Broken() {
		w.possess(m)
	} else if w.free(tx, ty) {
		p.X, p.Y, p.Anim = tx, ty, "walk"
	}
}

func (w *World) act(m *Entity) {
	if m.Broken() {
		return
	}
	p := w.Player
	dx, dy := p.X-m.X, p.Y-m.Y
	dist := abs(dx) + abs(dy)
	if !w.IsSpark() && (dx == 0 || dy == 0) && w.firstInLine(m, sign(dx), sign(dy), m.Range) == p {
		m.Dir = dirName(dx, dy)
		w.attack(m, p)
		return
	}
	if dist > 1 && dist <= sight {
		// ponytail: greedy chase with no line of sight or pathfinding, so monsters sense through walls and get stuck on them; add both when levels get twisty
		sx, sy := sign(dx), sign(dy)
		steps := [][2]int{{sx, 0}, {0, sy}}
		if abs(dy) > abs(dx) {
			steps[0], steps[1] = steps[1], steps[0]
		}
		for _, s := range steps {
			if (s[0] != 0 || s[1] != 0) && w.free(m.X+s[0], m.Y+s[1]) {
				m.X, m.Y, m.Dir, m.Anim = m.X+s[0], m.Y+s[1], dirName(s[0], s[1]), "walk"
				break
			}
		}
	}
}

// firstInLine returns the first creature within n tiles of from in direction dx, dy, stopping at walls.
func (w *World) firstInLine(from *Entity, dx, dy, n int) *Entity {
	for i := 1; i <= n; i++ {
		x, y := from.X+dx*i, from.Y+dy*i
		if w.Level[y][x] == '#' {
			return nil
		}
		if w.Player.X == x && w.Player.Y == y {
			return w.Player
		}
		if m := w.monsterAt(x, y); m != nil {
			return m
		}
	}
	return nil
}

func (w *World) attack(attacker, target *Entity) {
	attacker.Anim = "atk"
	if abs(target.X-attacker.X)+abs(target.Y-attacker.Y) > 1 {
		w.Shots = append(w.Shots, Shot{attacker.X, attacker.Y, target})
	}
	w.damage(target, attacker.Dmg)
}

func (w *World) damage(e *Entity, n int) {
	e.HP -= n
	if e.HP > 0 {
		return
	}
	if e == w.Player {
		// The body dies and the spark is thrown out onto the same tile.
		w.Player = newEntity(spark, e.X, e.Y)
		w.Player.Dir = e.Dir
		w.SparkLeft = sparkTurns
		return
	}
	w.Monsters = slices.DeleteFunc(w.Monsters, func(m *Entity) bool { return m == e })
}

// possess moves the spark into m, restoring the body to full HP. Any previous body is left behind to rot.
func (w *World) possess(m *Entity) {
	w.Monsters = slices.DeleteFunc(w.Monsters, func(e *Entity) bool { return e == m })
	m.HP, m.Dir, m.Anim = m.MaxHP, w.Player.Dir, "idle"
	w.Player = m
	w.decay = 0
}

func (w *World) monsterAt(x, y int) *Entity {
	for _, m := range w.Monsters {
		if m.X == x && m.Y == y {
			return m
		}
	}
	return nil
}

func (w *World) free(x, y int) bool {
	return w.Level[y][x] != '#' && w.monsterAt(x, y) == nil && (w.Player.X != x || w.Player.Y != y)
}

func dirName(dx, dy int) string {
	switch {
	case dx < 0:
		return "l"
	case dx > 0:
		return "r"
	case dy < 0:
		return "u"
	}
	return "d"
}

func abs(n int) int { return max(n, -n) }

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
