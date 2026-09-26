package main

import "slices"

// Tuning knobs.
const (
	decayEvery = 4 // a possessed body loses 1 HP every this many player turns
	sparkTurns = 3 // turns a bare spark survives without a body
	sight      = 6 // how far (in steps) monsters notice the player
)

// kinds maps a level character to a body type. The name doubles as the sprite prefix.
var kinds = map[byte]struct {
	name    string
	hp, dmg int
}{
	'r': {"rat", 3, 1},
	'g': {"gobwar", 6, 2},
	'o': {"orc", 10, 3},
}

type Entity struct {
	Kind      string
	X, Y      int
	HP, MaxHP int
	Dmg       int
	Dir       string // l, r, u, d
	Anim      string // what it did this turn: idle, walk, atk
}

// Broken bodies can be possessed and are too hurt to act.
func (e *Entity) Broken() bool { return e.HP > 0 && e.HP*3 <= e.MaxHP }

type World struct {
	Level     []string
	Player    *Entity // a spark, or the body it possesses
	Monsters  []*Entity
	SparkLeft int // turns left to find a body while the player is a bare spark
	Over      bool
	decay     int // turns since the current body last decayed
}

// NewWorld parses a level: '#' wall, '@' player (in a goblin warrior body), kind letters for monsters.
func NewWorld(level []string) *World {
	w := &World{Level: level}
	for y, row := range level {
		for x := range row {
			c := row[x]
			if c == '@' {
				c = 'g'
			}
			k, ok := kinds[c]
			if !ok {
				continue
			}
			e := &Entity{Kind: k.name, X: x, Y: y, HP: k.hp, MaxHP: k.hp, Dmg: k.dmg, Dir: "d", Anim: "idle"}
			if row[x] == '@' {
				w.Player = e
			} else {
				w.Monsters = append(w.Monsters, e)
			}
		}
	}
	return w
}

func (w *World) IsSpark() bool { return w.Player.Kind == "spark" }

// Step runs one full turn: the player acts (dx, dy of 0, 0 waits), then every monster.
func (w *World) Step(dx, dy int) {
	if w.Over {
		return
	}
	w.Player.Anim = "idle"
	for _, m := range w.Monsters {
		m.Anim = "idle"
	}

	body := w.Player
	if dx != 0 || dy != 0 {
		p := w.Player
		p.Dir = dirName(dx, dy)
		tx, ty := p.X+dx, p.Y+dy
		if m := w.monsterAt(tx, ty); m != nil {
			if m.Broken() {
				w.possess(m)
			} else if !w.IsSpark() {
				w.hit(p, m)
			}
		} else if w.free(tx, ty) {
			p.X, p.Y, p.Anim = tx, ty, "walk"
		}
	}

	if w.IsSpark() {
		w.SparkLeft--
		if w.SparkLeft <= 0 {
			w.Over = true
			return
		}
	} else if w.Player == body { // a body possessed this turn starts decaying next turn
		if w.decay++; w.decay >= decayEvery {
			w.decay = 0
			w.damage(w.Player, 1)
		}
	}

	for _, m := range w.Monsters {
		w.act(m)
	}
}

func (w *World) act(m *Entity) {
	if m.Broken() {
		return
	}
	p := w.Player
	dx, dy := p.X-m.X, p.Y-m.Y
	switch dist := abs(dx) + abs(dy); {
	case dist == 1 && !w.IsSpark():
		m.Dir = dirName(dx, dy)
		w.hit(m, p)
	case dist > 1 && dist <= sight:
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

func (w *World) hit(attacker, target *Entity) {
	attacker.Anim = "atk"
	w.damage(target, attacker.Dmg)
}

func (w *World) damage(e *Entity, n int) {
	e.HP -= n
	if e.HP > 0 {
		return
	}
	if e == w.Player {
		// The body dies and the spark is thrown out onto the same tile.
		w.Player = &Entity{Kind: "spark", X: e.X, Y: e.Y, HP: 1, MaxHP: 1, Dir: e.Dir, Anim: "idle"}
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
