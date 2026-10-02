package main

import (
	"fmt"
	"math/rand/v2"
	"slices"
)

// Tuning knobs. Per-creature ones live in kinds.
const (
	sight      = 6 // how far (in steps) monsters notice the player
	viewRadius = 7 // how far the player sees, in tiles
	regenEvery = 8 // turns for the hero to regain 1 HP
	bossEvery  = 5 // floors between bosses
)

// Kind is a creature type. Name doubles as the sprite prefix.
type Kind struct {
	Name  string
	MaxHP int
	Atk   int // accuracy: each point is +5% to hit
	Def   int // armour: each point is -5% to be hit
	Dmg   int // a hit deals 1 to Dmg
	Moves int // actions per turn
	Range int // attack reach along a straight line: 1 is melee, more shoots Missile
	Depth int // the shallowest floor it appears on
	XP    int // experience the hero gains for killing it

	Missile string // FX sprite of what a ranged attack fires; "arrow" picks arrow_x or arrow_y
	Boss    bool   // guards a floor's stairs, which open when it dies
}

// hero is the warrior's base, before levels and gear; it starts with a sword, see newRun.
var hero = Kind{Name: "warrior", MaxHP: 20, Atk: 2, Def: 1, Dmg: 2, Moves: 1, Range: 1}

// kinds maps a level character to a monster type.
// kinds maps a level character to a monster type.
var kinds = map[byte]Kind{
	'r': {Name: "rat", Depth: 1, XP: 2, MaxHP: 4, Dmg: 2, Moves: 2, Range: 1},                 // fast and fragile
	'b': {Name: "bat", Depth: 1, XP: 2, MaxHP: 3, Atk: 1, Def: 2, Dmg: 2, Moves: 2, Range: 1}, // fast, hard to hit
	's': {Name: "snake", Depth: 1, XP: 3, MaxHP: 6, Atk: 1, Dmg: 3, Moves: 1, Range: 1},
	'j': {Name: "slime", Depth: 2, XP: 3, MaxHP: 10, Dmg: 2, Moves: 1, Range: 1},                              // soaks up hits
	'g': {Name: "gobwar", Depth: 2, XP: 5, MaxHP: 8, Atk: 1, Def: 1, Dmg: 4, Moves: 1, Range: 1},              // goblin warrior
	'a': {Name: "gobarcher", Depth: 2, XP: 5, MaxHP: 6, Atk: 1, Dmg: 3, Moves: 1, Range: 5, Missile: "arrow"}, // shoots down a line
	'w': {Name: "wolf", Depth: 3, XP: 7, MaxHP: 8, Atk: 2, Dmg: 4, Moves: 2, Range: 1},                        // fast
	'k': {Name: "skel", Depth: 4, XP: 8, MaxHP: 12, Atk: 2, Def: 2, Dmg: 5, Moves: 1, Range: 1},               // skeleton
	'p': {Name: "spider", Depth: 4, XP: 8, MaxHP: 10, Atk: 3, Def: 1, Dmg: 4, Moves: 1, Range: 1},
	'z': {Name: "zombie", Depth: 5, XP: 9, MaxHP: 20, Atk: 1, Dmg: 6, Moves: 1, Range: 1},       // slow to kill
	'o': {Name: "orc", Depth: 5, XP: 10, MaxHP: 14, Atk: 3, Def: 2, Dmg: 6, Moves: 1, Range: 1}, // strong
	'n': {Name: "gnoll", Depth: 6, XP: 12, MaxHP: 16, Atk: 3, Def: 1, Dmg: 7, Moves: 1, Range: 1},
	'h': {Name: "skelarcher", Depth: 6, XP: 12, MaxHP: 12, Atk: 3, Def: 1, Dmg: 5, Moves: 1, Range: 6, Missile: "arrow"}, // skeleton archer
	'i': {Name: "imp", Depth: 7, XP: 14, MaxHP: 12, Atk: 4, Def: 2, Dmg: 5, Moves: 2, Range: 1},                          // fast
	'y': {Name: "ghost", Depth: 8, XP: 16, MaxHP: 16, Atk: 4, Def: 4, Dmg: 6, Moves: 1, Range: 1},                        // hard to hit
	'v': {Name: "skelwar", Depth: 9, XP: 18, MaxHP: 24, Atk: 4, Def: 3, Dmg: 8, Moves: 1, Range: 1},                      // skeleton warrior
	'm': {Name: "skelmage", Depth: 10, XP: 20, MaxHP: 16, Atk: 4, Def: 2, Dmg: 8, Moves: 1, Range: 5, Missile: "proj_blue_ball"},
	'f': {Name: "flame", Depth: 11, XP: 22, MaxHP: 20, Atk: 5, Def: 2, Dmg: 9, Moves: 1, Range: 3, Missile: "proj_orange_ball"}, // spits fire
	'l': {Name: "flayer", Depth: 12, XP: 26, MaxHP: 30, Atk: 5, Def: 3, Dmg: 10, Moves: 1, Range: 1},
	'x': {Name: "demon", Depth: 13, XP: 30, MaxHP: 34, Atk: 6, Def: 3, Dmg: 12, Moves: 1, Range: 1}, // lesser demon

	// Bosses, one every bossEvery floors. They grow stronger each time round, see descend.
	'D': {Name: "dragon", MaxHP: 18, Atk: 3, Def: 2, Dmg: 6, Moves: 1, Range: 4, Missile: "proj_red_ball", Boss: true},    // breathes fire
	'E': {Name: "beholder", MaxHP: 13, Atk: 3, Def: 1, Dmg: 5, Moves: 1, Range: 6, Missile: "proj_blue_ball", Boss: true}, // long-range eye beam
	'L': {Name: "lord", MaxHP: 16, Atk: 3, Def: 2, Dmg: 6, Moves: 1, Range: 5, Missile: "proj_green_ball", Boss: true},    // dark magic
	'C': {Name: "cyclops", MaxHP: 24, Atk: 2, Def: 2, Dmg: 8, Moves: 1, Range: 1, Boss: true},                             // a wall of HP
	'X': {Name: "demon", MaxHP: 17, Atk: 3, Def: 1, Dmg: 5, Moves: 2, Range: 1, Boss: true},                               // fast
	'R': {Name: "reaper", MaxHP: 12, Atk: 4, Def: 1, Dmg: 10, Moves: 1, Range: 1, Boss: true},                             // fragile, hits hardest
}

const bossKinds = "DELCXR"

// spawnable lists the monsters that live on a floor: those that first appear there or up to 6 floors
// above. Below the deepest first appearance the window stops sinking, so the deep keeps a mix.
func spawnable(depth int) []byte {
	deepest := 0
	for _, k := range kinds {
		if !k.Boss {
			deepest = max(deepest, k.Depth)
		}
	}
	var cs []byte
	for c, k := range kinds {
		if !k.Boss && k.Depth <= depth && k.Depth >= min(depth, deepest)-6 {
			cs = append(cs, c)
		}
	}
	slices.Sort(cs) // map order is random; the same seed must build the same floor
	return cs
}

// at returns k as met on a floor: every 4 floors below where it first appears it gains a quarter of
// its HP, 1 accuracy and 1 damage, and is worth more experience.
func (k Kind) at(depth int) Kind {
	b := max(0, depth-k.Depth) / 4
	k.MaxHP += k.MaxHP * b / 4
	k.Atk += b
	k.Dmg += b
	k.XP += k.XP * b / 2
	return k
}

type Entity struct {
	Kind
	X, Y int
	HP   int
	Dir  string // l, r, u, d
	Anim string // what it did this step: idle, walk, atk

	Spotted bool // a monster that noticed the player this step, for the alert icon

	// Monsters only: where the player was last seen, while hunting it.
	hunting      bool
	goalX, goalY int
}

func newEntity(k Kind, x, y int) *Entity {
	return &Entity{Kind: k, X: x, Y: y, HP: k.MaxHP, Dir: "d", Anim: "idle"}
}

// Shot is a missile fired this step, kept for the renderer.
// It points at the target itself, which may have moved or died by the time the missile is drawn.
type Shot struct {
	FromX, FromY int
	To           *Entity
	Missile      string
}

type World struct {
	Level    []string
	Player   *Entity
	Monsters []*Entity
	Shots    []Shot
	Log      []string // what happened this step, in order, for the message line
	Visible  [][]bool // tiles the player sees right now, by [y][x]
	Seen     [][]bool // tiles the player has ever seen on this floor
	Depth    int      // current floor, from 1 down without end
	Kills    int
	ExpLevel int // the hero's experience level, from 1
	XP       int // experience toward the next level
	Gold     int
	Over     bool // the hero is dead

	Inventory []*Item
	Floor     []*Item // items lying on this floor

	rng    *rand.Rand
	base   Kind                 // the hero's stats before gear; levels raise these
	faces  map[*ItemKind]string // this run's potion colours and scroll labels
	known  map[*ItemKind]bool   // potion and scroll kinds the hero has identified
	bosses string               // the order this run meets the bosses in, as kind characters
	regen  int                  // turns since the hero last regained HP
}

// NewWorld starts a run on a given level: '#' wall, '>' stairs down, '@' the hero, '*' an item, kind
// letters for monsters. Tests use it with hand-drawn levels.
func NewWorld(level []string) *World {
	w := newRun(rand.New(rand.NewPCG(1, 0)))
	w.load(level)
	return w
}

// newRun makes a fresh hero, carrying and wielding a sword, for a run drawing on rng.
func newRun(rng *rand.Rand) *World {
	w := &World{Depth: 1, ExpLevel: 1, rng: rng, base: hero, known: map[*ItemKind]bool{}}
	w.Player = newEntity(hero, 0, 0)
	w.shuffleFaces()
	w.Inventory = []*Item{{ItemKind: kindNamed("sword"), Worn: true}}
	w.recalc()
	return w
}

// load puts the hero on a new floor: its monsters, and items rolled for the current depth on its spots.
func (w *World) load(level []string) {
	w.Level, w.Monsters, w.Floor = level, nil, nil
	var spots [][2]int
	for y, row := range level {
		for x := range row {
			switch c := row[x]; c {
			case '@':
				w.Player.X, w.Player.Y = x, y
			case '*':
				spots = append(spots, [2]int{x, y})
			default:
				if k, ok := kinds[c]; ok {
					w.Monsters = append(w.Monsters, newEntity(k, x, y))
				}
			}
		}
	}
	w.Seen = grid(level)
	w.stockItems(spots)
	w.updateFOV()
}

func grid(level []string) [][]bool {
	g := make([][]bool, len(level))
	for y := range g {
		g[y] = make([]bool, len(level[y]))
	}
	return g
}

// updateFOV marks the tiles within viewRadius that the player has a clear line to, walls included.
func (w *World) updateFOV() {
	p := w.Player
	w.Visible = grid(w.Level)
	for y := max(0, p.Y-viewRadius); y <= min(len(w.Level)-1, p.Y+viewRadius); y++ {
		for x := max(0, p.X-viewRadius); x <= min(len(w.Level[y])-1, p.X+viewRadius); x++ {
			if (x-p.X)*(x-p.X)+(y-p.Y)*(y-p.Y) <= viewRadius*viewRadius && w.canSee(p.X, p.Y, x, y) {
				w.Visible[y][x], w.Seen[y][x] = true, true
			}
		}
	}
	// A sight line to a wall off the straight axes stops at the wall beside it, so a room's walls
	// would show in patches. Show every wall touching a visible floor tile instead.
	for y := range w.Level {
		for x := range w.Level[y] {
			if w.Level[y][x] != '#' || w.Visible[y][x] {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if ny >= 0 && ny < len(w.Level) && nx >= 0 && nx < len(w.Level[ny]) &&
						w.Level[ny][nx] != '#' && w.Visible[ny][nx] {
						w.Visible[y][x], w.Seen[y][x] = true, true
					}
				}
			}
		}
	}
}

// NewGame starts a run on a generated first floor. The same seed always plays out the same way.
func NewGame(seed uint64) *World {
	rng := rand.New(rand.NewPCG(seed, 0))
	w := newRun(rng)
	w.load(generate(rng, 1, 0))
	for _, i := range rng.Perm(len(bossKinds)) {
		w.bosses += bossKinds[i : i+1]
	}
	return w
}

// Score counts how deep the hero got, how much it killed, the level it reached and its gold.
func (w *World) Score() int { return 100*w.Depth + 10*w.Kills + 50*(w.ExpLevel-1) + w.Gold }

// xpFor is the experience it takes to go from level l to l+1. It grows with the square of the level:
// with a gentler curve the hero outgrows the monsters and a good run never ends.
func xpFor(l int) int { return 5 * l * l }

// gainXP adds experience and levels the hero up: each level gives 5 max HP and 1 accuracy, every
// 2nd level 1 damage and every 3rd 1 armour.
func (w *World) gainXP(n int) {
	w.XP += n
	for w.XP >= xpFor(w.ExpLevel) {
		w.XP -= xpFor(w.ExpLevel)
		w.ExpLevel++
		w.base.MaxHP += 5
		w.base.Atk++
		if w.ExpLevel%2 == 0 {
			w.base.Dmg++
		}
		if w.ExpLevel%3 == 0 {
			w.base.Def++
		}
		w.recalc()
		w.Player.HP += 5
		w.say("You reach level %d!", w.ExpLevel)
	}
}

// descend moves the hero to the start of a freshly generated next floor.
func (w *World) descend() {
	w.Depth++
	var boss byte
	round := w.Depth / bossEvery // how many bosses this one is, counting from 1
	if w.Depth%bossEvery == 0 && len(w.bosses) > 0 {
		boss = w.bosses[(round-1)%len(w.bosses)]
	}
	w.load(generate(w.rng, w.Depth, boss))
	w.say("Deep %d.", w.Depth)
	for _, m := range w.Monsters {
		if m.Boss { // each boss after the first gets half its base HP and 1 damage more
			m.MaxHP += m.MaxHP * (round - 1) / 2
			m.Dmg += round - 1
			w.say("A %s guards the stairs.", m.Name)
		} else {
			m.Kind = m.Kind.at(w.Depth)
		}
		m.HP = m.MaxHP
	}
}

func (w *World) say(format string, args ...any) { w.Log = append(w.Log, fmt.Sprintf(format, args...)) }

// Step is one turn of moving or attacking in a direction, or waiting for 0, 0.
func (w *World) Step(dx, dy int) {
	w.turn(func() {
		if dx != 0 || dy != 0 {
			w.playerAct(dx, dy)
		}
	})
}

// turn runs one turn: the hero's action, then every monster.
func (w *World) turn(act func()) {
	if w.Over {
		return
	}
	defer w.updateFOV()
	w.Shots, w.Log = nil, nil
	w.Player.Anim = "idle"
	for _, m := range w.Monsters {
		m.Anim, m.Spotted = "idle", false
	}

	act()
	if w.Level[w.Player.Y][w.Player.X] == '>' {
		w.descend()
		return
	}
	if w.Player.HP < w.Player.MaxHP {
		if w.regen++; w.regen >= regenEvery {
			w.regen = 0
			w.Player.HP++
		}
	}
	// A copy, since monsters can die mid-turn.
	for _, m := range slices.Clone(w.Monsters) {
		for range m.Moves {
			if m.HP > 0 && !w.Over {
				w.act(m)
			}
		}
	}
}

func (w *World) playerAct(dx, dy int) {
	p := w.Player
	p.Dir = dirName(dx, dy)
	if t := w.firstInLine(p, dx, dy, p.Range); t != nil {
		w.attack(p, t)
		return
	}
	if tx, ty := p.X+dx, p.Y+dy; w.free(tx, ty) {
		p.X, p.Y, p.Anim = tx, ty, "walk"
		w.pickUpGold()
		switch its := w.ItemsAt(p.X, p.Y); len(its) {
		case 0:
		case 1:
			w.say("You see a %s here.", w.ItemName(its[0]))
		default:
			w.say("Several items lie here.")
		}
	}
}

func (w *World) act(m *Entity) {
	p := w.Player
	dx, dy := p.X-m.X, p.Y-m.Y
	dist := abs(dx) + abs(dy)
	if (dx == 0 || dy == 0) && w.firstInLine(m, sign(dx), sign(dy), m.Range) == p {
		m.Dir = dirName(dx, dy)
		w.attack(m, p)
		return
	}
	if dist <= sight && w.canSee(m.X, m.Y, p.X, p.Y) {
		m.Spotted = m.Spotted || !m.hunting
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

// hitChance is the percent chance that a's attack lands on t: 70%, plus 5% per point of accuracy
// over the target's armour, never certain either way.
func hitChance(a, t *Entity) int { return min(max(70+5*(a.Atk-t.Def), 5), 95) }

func (w *World) attack(a, t *Entity) {
	a.Anim = "atk"
	if abs(t.X-a.X)+abs(t.Y-a.Y) > 1 {
		w.Shots = append(w.Shots, Shot{a.X, a.Y, t, a.Missile})
	}
	if w.rng.IntN(100) >= hitChance(a, t) {
		if a == w.Player {
			w.say("You miss the %s.", t.Name)
		} else {
			w.say("The %s misses.", a.Name)
		}
		return
	}
	if a == w.Player {
		w.say("You hit the %s.", t.Name)
	} else {
		w.say("The %s hits.", a.Name)
	}
	w.damage(t, 1+w.rng.IntN(a.Dmg))
}

func (w *World) damage(e *Entity, n int) {
	e.HP -= n
	if e.HP > 0 {
		return
	}
	if e == w.Player {
		w.Over = true
		w.say("You die.")
		return
	}
	w.Kills++
	w.gainXP(e.XP)
	if e.Boss { // the sealed stairs open where it fell
		w.Level[e.Y] = w.Level[e.Y][:e.X] + ">" + w.Level[e.Y][e.X+1:]
		w.say("The %s falls. The stairs open!", e.Name)
	} else {
		w.say("The %s dies.", e.Name)
	}
	w.Monsters = slices.DeleteFunc(w.Monsters, func(m *Entity) bool { return m == e })
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
