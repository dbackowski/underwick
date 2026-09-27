package main

import (
	"math/rand/v2"
	"slices"
)

// Tuning knobs shared by all bodies. Per-body ones live in kinds.
const (
	sparkTurns = 3 // turns a bare spark survives without a body
	sight      = 6 // how far (in steps) monsters notice the player
	crumble    = 6 // turns a broken monster lasts before it falls apart, if nobody possesses it
)

// Kind is a body type. Name doubles as the sprite prefix.
type Kind struct {
	Name  string
	MaxHP int
	Dmg   int
	Moves int // actions per turn
	Range int // attack reach along a straight line: 1 is melee, more shoots Missile
	Decay int // a possessed body loses 1 HP every this many turns

	Missile string // FX sprite of what a ranged attack fires; "arrow" picks arrow_x or arrow_y
	Boss    bool   // never broken or possessed; killing it opens the stairs
}

// kinds maps a level character to a body type.
var kinds = map[byte]Kind{
	'r': {Name: "rat", MaxHP: 3, Dmg: 1, Moves: 2, Range: 1, Decay: 8},                         // fast and fragile
	'a': {Name: "gobarcher", MaxHP: 5, Dmg: 2, Moves: 1, Range: 5, Decay: 8, Missile: "arrow"}, // shoots down a line
	'o': {Name: "orc", MaxHP: 10, Dmg: 3, Moves: 1, Range: 1, Decay: 4},                        // strong, rots twice as fast

	// Bosses, one per act. Their HP grows on deeper floors, see descend.
	'D': {Name: "dragon", MaxHP: 16, Dmg: 3, Moves: 1, Range: 4, Missile: "proj_red_ball", Boss: true},    // breathes fire
	'E': {Name: "beholder", MaxHP: 12, Dmg: 2, Moves: 1, Range: 6, Missile: "proj_blue_ball", Boss: true}, // long-range eye beam
	'L': {Name: "lord", MaxHP: 14, Dmg: 3, Moves: 1, Range: 5, Missile: "proj_green_ball", Boss: true},    // dark magic
	'C': {Name: "cyclops", MaxHP: 20, Dmg: 4, Moves: 1, Range: 1, Boss: true},                             // a wall of HP
	'X': {Name: "demon", MaxHP: 14, Dmg: 2, Moves: 2, Range: 1, Boss: true},                               // fast
	'R': {Name: "reaper", MaxHP: 10, Dmg: 5, Moves: 1, Range: 1, Boss: true},                              // fragile, hits hardest
}

const bossKinds = "DELCXR"

var spark = Kind{Name: "spark", MaxHP: 1, Moves: 2} // quick, to reach a body in time

type Entity struct {
	Kind
	X, Y int
	HP   int
	Dir  string // l, r, u, d
	Anim string // what it did this step: idle, walk, atk

	// Monsters only: where the player was last seen, while hunting it, and turns spent broken.
	hunting      bool
	goalX, goalY int
	brokenFor    int
}

func newEntity(k Kind, x, y int) *Entity {
	return &Entity{Kind: k, X: x, Y: y, HP: k.MaxHP, Dir: "d", Anim: "idle"}
}

// Broken bodies can be possessed and are too hurt to act. Bosses fight to the death instead.
func (e *Entity) Broken() bool { return !e.Boss && e.HP > 0 && e.HP*3 <= e.MaxHP }

// Shot is a missile fired this step, kept for the renderer.
// It points at the target itself, which may have moved or died by the time the missile is drawn.
type Shot struct {
	FromX, FromY int
	To           *Entity
	Missile      string
}

type World struct {
	Level     []string
	Player    *Entity // a spark, or the body it possesses
	Monsters  []*Entity
	Shots     []Shot
	SparkLeft int // turns left to find a body while the player is a bare spark
	Depth     int // current floor, 1 to floors
	Over      bool
	Won       bool // took the stairs down from the last floor
	rng       *rand.Rand
	bosses    string // this run's boss for floors 3, 6 and 9, as kind characters
	decay     int    // turns since the current body last decayed
	acted     int    // player actions taken so far this turn
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

// NewGame starts a run on a generated first floor. The same seed always builds the same floors.
func NewGame(seed uint64) *World {
	rng := rand.New(rand.NewPCG(seed, 0))
	w := NewWorld(generate(rng, 1, 0))
	w.rng, w.Depth = rng, 1
	for _, i := range rng.Perm(len(bossKinds))[:floors/3] {
		w.bosses += bossKinds[i : i+1]
	}
	return w
}

// descend moves the player, in whatever body it has, to the start of a freshly generated next floor.
func (w *World) descend() {
	if w.Depth == floors {
		w.Over, w.Won = true, true
		return
	}
	w.Depth++
	var boss byte
	if w.Depth%3 == 0 && len(w.bosses) >= w.Depth/3 {
		boss = w.bosses[w.Depth/3-1]
	}
	next := NewWorld(generate(w.rng, w.Depth, boss))
	w.Player.X, w.Player.Y = next.Player.X, next.Player.Y
	w.Level, w.Monsters = next.Level, next.Monsters
	w.acted = 0
	for _, m := range w.Monsters {
		if m.Boss { // 1x on floor 3, 1.5x on floor 6, 2x on floor 9
			m.MaxHP += m.MaxHP * (w.Depth/3 - 1) / 2
			m.HP = m.MaxHP
		}
	}
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
		if w.Level[w.Player.Y][w.Player.X] == '>' {
			w.descend()
			return
		}
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

	w.Monsters = slices.DeleteFunc(w.Monsters, func(m *Entity) bool {
		if m.Broken() {
			m.brokenFor++
		}
		return m.brokenFor > crumble
	})
	// A copy, since a monster can die mid-turn (a boss caught in the spark's burst).
	for _, m := range slices.Clone(w.Monsters) {
		for range m.Moves {
			if m.HP > 0 {
				w.act(m)
			}
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
	m := w.monsterAt(tx, ty)
	switch {
	case m != nil && m.Broken():
		w.possess(m)
	case m != nil && w.IsSpark() && !m.Boss:
		// A bare spark haunts: each touch drains a third of the body's HP (never killing it), so two
		// touches break anything and the spark can make its own body.
		m.HP = max(1, m.HP-(m.MaxHP+2)/3)
		p.Anim = "atk"
	case w.free(tx, ty):
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
	if dist <= sight && w.canSee(m.X, m.Y, p.X, p.Y) {
		m.hunting, m.goalX, m.goalY = true, p.X, p.Y
	}
	if !m.hunting {
		return
	}
	if m.X == m.goalX && m.Y == m.goalY {
		m.hunting = false // reached the last sighting and the player is gone
		return
	}
	if sx, sy, ok := w.stepToward(m.X, m.Y, m.goalX, m.goalY); ok {
		m.X, m.Y, m.Dir, m.Anim = m.X+sx, m.Y+sy, dirName(sx, sy), "walk"
	}
}

var dirs = [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// canSee reports whether no wall lies on the straight (Bresenham) line between two tiles.
// The line steps along one axis at a time, so it can't slip through a diagonal gap between two walls.
func (w *World) canSee(x0, y0, x1, y1 int) bool {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	e := dx + dy
	for x0 != x1 || y0 != y1 {
		if w.Level[y0][x0] == '#' {
			return false
		}
		if e2 := 2 * e; e2 >= dy {
			e, x0 = e+dy, x0+sx
		} else if e2 <= dx {
			e, y0 = e+dx, y0+sy
		}
	}
	return true
}

// stepToward returns a free first step on a shortest path from x, y to gx, gy. Paths go around walls
// but through creatures, so a monster stuck behind another one waits its turn rather than detouring.
func (w *World) stepToward(x, y, gx, gy int) (dx, dy int, ok bool) {
	// Breadth-first search outward from the goal until it reaches the start.
	dist := map[[2]int]int{{gx, gy}: 0}
	for queue := [][2]int{{gx, gy}}; len(queue) > 0 && queue[0] != [2]int{x, y}; queue = queue[1:] {
		c := queue[0]
		for _, d := range dirs {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if _, seen := dist[n]; !seen && w.Level[n[1]][n[0]] != '#' {
				dist[n] = dist[c] + 1
				queue = append(queue, n)
			}
		}
	}
	here, reachable := dist[[2]int{x, y}]
	if !reachable {
		return 0, 0, false
	}
	for _, d := range dirs {
		if n, ok := dist[[2]int{x + d[0], y + d[1]}]; ok && n == here-1 && w.free(x+d[0], y+d[1]) {
			return d[0], d[1], true
		}
	}
	return 0, 0, false
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
		w.Shots = append(w.Shots, Shot{attacker.X, attacker.Y, target, attacker.Missile})
	}
	w.damage(target, attacker.Dmg)
}

func (w *World) damage(e *Entity, n int) {
	e.HP -= n
	if e.HP > 0 {
		return
	}
	if e == w.Player {
		// The body dies and the spark tears free onto the same tile, breaking every monster next to it,
		// so a spark killed in melee always has a body in reach.
		// A boss can't be broken, so the burst scorches it for a quarter of its HP instead: bodies spent
		// next to a boss are how the spark wears it down.
		for _, m := range slices.Clone(w.Monsters) { // a copy, since the burst can kill the boss
			switch {
			case abs(m.X-e.X)+abs(m.Y-e.Y) != 1:
			case m.Boss:
				w.damage(m, m.MaxHP/4)
			default:
				m.HP = min(m.HP, max(1, m.MaxHP/3))
			}
		}
		w.Player = newEntity(spark, e.X, e.Y)
		w.Player.Dir = e.Dir
		w.SparkLeft = sparkTurns
		return
	}
	if e.Boss { // the sealed stairs open where it fell
		w.Level[e.Y] = w.Level[e.Y][:e.X] + ">" + w.Level[e.Y][e.X+1:]
	}
	w.Monsters = slices.DeleteFunc(w.Monsters, func(m *Entity) bool { return m == e })
}

// possess moves the spark into m, restoring the body to full HP. Any previous body is left behind to rot.
func (w *World) possess(m *Entity) {
	w.Monsters = slices.DeleteFunc(w.Monsters, func(e *Entity) bool { return e == m })
	m.HP, m.Dir, m.Anim, m.brokenFor = m.MaxHP, w.Player.Dir, "idle", 0
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
