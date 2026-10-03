package main

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
)

// Tuning knobs. Per-creature ones live in kinds.
const (
	sight      = 6 // how far (in steps) monsters notice the player
	viewRadius = 7 // how far the player sees, in tiles
	regenEvery = 8 // turns for the hero to regain 1 HP
	bossEvery  = 5 // floors between bosses
)

// wanderEvery is how many turns pass between monsters wandering onto a floor, so waiting has a price.
var wanderEvery = 150

const maxMonsters = 30 // wanderers stop coming once a floor holds this many

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

	Poisons, Confuses int // percent chance a hit also poisons or confuses
}

// hero is the warrior's base, before levels and gear: the class tests use.
var hero = classes[0].Kind

// kinds maps a level character to a monster type.
// kinds maps a level character to a monster type.
var kinds = map[byte]Kind{
	'r': {Name: "rat", Depth: 1, XP: 2, MaxHP: 4, Dmg: 2, Moves: 2, Range: 1},                 // fast and fragile
	'b': {Name: "bat", Depth: 1, XP: 2, MaxHP: 3, Atk: 1, Def: 2, Dmg: 2, Moves: 2, Range: 1}, // fast, hard to hit
	's': {Name: "snake", Depth: 1, XP: 3, MaxHP: 6, Atk: 1, Dmg: 3, Moves: 1, Range: 1, Poisons: 30},
	'j': {Name: "slime", Depth: 2, XP: 3, MaxHP: 10, Dmg: 2, Moves: 1, Range: 1},                              // soaks up hits
	'g': {Name: "gobwar", Depth: 2, XP: 5, MaxHP: 8, Atk: 1, Def: 1, Dmg: 4, Moves: 1, Range: 1},              // goblin warrior
	'a': {Name: "gobarcher", Depth: 2, XP: 5, MaxHP: 6, Atk: 1, Dmg: 3, Moves: 1, Range: 5, Missile: "arrow"}, // shoots down a line
	'w': {Name: "wolf", Depth: 3, XP: 7, MaxHP: 8, Atk: 2, Dmg: 4, Moves: 2, Range: 1},                        // fast
	'k': {Name: "skel", Depth: 4, XP: 8, MaxHP: 12, Atk: 2, Def: 2, Dmg: 5, Moves: 1, Range: 1},               // skeleton
	'p': {Name: "spider", Depth: 4, XP: 8, MaxHP: 10, Atk: 3, Def: 1, Dmg: 4, Moves: 1, Range: 1, Poisons: 40},
	'z': {Name: "zombie", Depth: 5, XP: 9, MaxHP: 20, Atk: 1, Dmg: 6, Moves: 1, Range: 1},       // slow to kill
	'o': {Name: "orc", Depth: 5, XP: 10, MaxHP: 14, Atk: 3, Def: 2, Dmg: 6, Moves: 1, Range: 1}, // strong
	'n': {Name: "gnoll", Depth: 6, XP: 12, MaxHP: 16, Atk: 3, Def: 1, Dmg: 7, Moves: 1, Range: 1},
	'h': {Name: "skelarcher", Depth: 6, XP: 12, MaxHP: 12, Atk: 3, Def: 1, Dmg: 5, Moves: 1, Range: 6, Missile: "arrow"}, // skeleton archer
	'i': {Name: "imp", Depth: 7, XP: 14, MaxHP: 12, Atk: 4, Def: 2, Dmg: 5, Moves: 2, Range: 1, Confuses: 30},            // fast
	'y': {Name: "ghost", Depth: 8, XP: 16, MaxHP: 16, Atk: 4, Def: 4, Dmg: 6, Moves: 1, Range: 1, Confuses: 20},          // hard to hit
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

// Tiles, besides '#' wall, '.' floor and '>' stairs down, as generate draws them. A locked door
// opens to its own open form: iron '1' to '4' with a gold key, magic '2' to '5' with a blue key.
// An opened chest '&' becomes '0'.
func blocksSight(c byte) bool { return strings.IndexByte("#+12", c) >= 0 }  // walls and closed doors
func solid(c byte) bool       { return strings.IndexByte("#+12&", c) >= 0 } // and chests: nothing stands there
func hazard(c byte) bool      { return strings.IndexByte("~=%^", c) >= 0 }  // water, lava, acid, pits

type Entity struct {
	Kind
	X, Y int
	HP   int
	Dir  string // l, r, u, d
	Anim string // what it did this step: idle, walk, atk

	Spotted bool // a monster that noticed the player this step, for the alert icon

	// For animating the turn: where it stood when the turn began, and the way it struck in melee.
	FromX, FromY int
	Lunge        [2]int

	Poison   int // turns left of losing 1 HP a turn
	Confused int // turns left of stumbling about
	Sleep    int // turns left asleep; asleep until woken, if negative

	// Monsters only: where the player was last seen, while hunting it.
	hunting      bool
	goalX, goalY int
}

func newEntity(k Kind, x, y int) *Entity {
	return &Entity{Kind: k, X: x, Y: y, HP: k.MaxHP, Dir: "d", Anim: "idle", FromX: x, FromY: y}
}

// Hit is a change to a creature's HP this turn, kept for the floating numbers.
type Hit struct {
	To     *Entity
	Amount int
	Kind   byte // 'd' damage, 'p' poison, 'h' healing, 'm' a miss
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
	Hits     []Hit    // HP changes this turn, until the screen takes them, see TakeHits
	Turn     int      // turns played, so the screen can tell an action from one that did nothing
	Log      []string // what happened this step, in order, for the message line
	History  []string // the last messages of the run, oldest first
	Visible  [][]bool // tiles the player sees right now, by [y][x]
	Seen     [][]bool // tiles the player has ever seen on this floor
	Depth    int      // current floor, from 1 down without end
	Kills    int
	ExpLevel int // the hero's experience level, from 1
	XP       int // experience toward the next level
	Gold     int
	Over     bool     // the hero is dead
	Cause    string   // what killed it, e.g. "an orc" or "lava"
	Actions  []Action // everything the hero did this run, for saving

	Inventory []*Item
	Floor     []*Item // items lying on this floor

	Class         *Class
	Spells        []*Spell
	Mana, MaxMana int

	rng    *rand.Rand
	base   Kind                 // the hero's stats before gear; levels raise these
	faces  map[*ItemKind]string // this run's potion colours and scroll labels
	known  map[*ItemKind]bool   // potion and scroll kinds the hero has identified
	bosses string               // the order this run meets the bosses in, as kind characters
	regen  int                  // turns since the hero last regained HP
	warned [2]int               // the lava tile the hero was last warned about
	manaIn int                  // turns since the hero last regained mana
	seed   uint64               // the run's seed, for saving
	hurtBy string               // what is hurting the hero this turn, for the cause of death
	idle   int                  // turns since a monster last wandered onto this floor
}

// NewWorld starts a warrior's run on a given level, in the format generate makes. Tests use it with
// hand-drawn levels.
func NewWorld(level []string) *World {
	w := newRun(rand.New(rand.NewPCG(1, 0)), classes[0])
	w.load(level)
	return w
}

// newRun makes a fresh hero of a class, with its starting gear in use, for a run drawing on rng.
func newRun(rng *rand.Rand, c *Class) *World {
	w := &World{Depth: 1, ExpLevel: 1, rng: rng, base: c.Kind, known: map[*ItemKind]bool{}, Class: c}
	w.Player = newEntity(c.Kind, 0, 0)
	w.shuffleFaces()
	for _, name := range c.Start {
		w.Inventory = append(w.Inventory, &Item{ItemKind: kindNamed(name), Worn: true})
	}
	for _, name := range c.Spells {
		w.Spells = append(w.Spells, spellNamed(name))
	}
	w.Mana, w.MaxMana = c.Mana, c.Mana
	w.recalc()
	return w
}

// load puts the hero on a new floor: its monsters, its keys, and items rolled for the current depth
// on its spots. What stands or lies on a tile is taken off the map, leaving plain floor.
func (w *World) load(level []string) {
	w.Monsters, w.Floor = nil, nil
	var spots [][2]int
	rows := make([][]byte, len(level))
	for y, row := range level {
		rows[y] = []byte(row)
		for x := range row {
			switch c := row[x]; c {
			case '@':
				w.Player.X, w.Player.Y = x, y
			case '*':
				spots = append(spots, [2]int{x, y})
			case '(', ')':
				k := goldKey
				if c == ')' {
					k = blueKey
				}
				w.Floor = append(w.Floor, &Item{ItemKind: k, X: x, Y: y})
			default:
				k, ok := kinds[c]
				if !ok {
					continue
				}
				w.Monsters = append(w.Monsters, newEntity(k, x, y))
			}
			rows[y][x] = '.'
		}
	}
	w.Level = make([]string, len(rows))
	for y := range rows {
		w.Level[y] = string(rows[y])
	}
	w.Seen = grid(level)
	w.stockItems(spots)
	w.idle = 0
	w.Player.FromX, w.Player.FromY = w.Player.X, w.Player.Y // arrive, rather than slide across the map
	w.updateFOV()
}

// wander brings a monster of the floor's kinds onto it, out of the hero's sight but where it can walk to
// the hero, already on the hero's trail.
func (w *World) wander() {
	p := w.Player
	reach := walkable(func(x, y int) byte { return w.Level[y][x] }, p.X, p.Y)
	var spots [][2]int
	for y, row := range w.Level { // in order, not over the map: a replayed save must pick the same tile
		for x := range row {
			if reach[[2]int{x, y}] && !w.Visible[y][x] && row[x] != '>' && w.free(x, y) && abs(x-p.X)+abs(y-p.Y) >= 8 {
				spots = append(spots, [2]int{x, y})
			}
		}
	}
	if len(spots) == 0 {
		return
	}
	s := spots[w.rng.IntN(len(spots))]
	pool := spawnable(w.Depth)
	m := newEntity(kinds[pool[w.rng.IntN(len(pool))]].at(w.Depth), s[0], s[1])
	m.hunting, m.goalX, m.goalY = true, p.X, p.Y
	w.Monsters = append(w.Monsters, m)
}

// settle sends a third of a new floor's monsters to sleep, until something wakes them.
func (w *World) settle() {
	for _, m := range w.Monsters {
		if !m.Boss && w.rng.IntN(3) == 0 {
			m.Sleep = -1
		}
	}
}

func (w *World) setTile(x, y int, c byte) { w.Level[y] = w.Level[y][:x] + string(c) + w.Level[y][x+1:] }

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
			if !blocksSight(w.Level[y][x]) || w.Visible[y][x] {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if ny >= 0 && ny < len(w.Level) && nx >= 0 && nx < len(w.Level[ny]) &&
						!blocksSight(w.Level[ny][nx]) && w.Visible[ny][nx] {
						w.Visible[y][x], w.Seen[y][x] = true, true
					}
				}
			}
		}
	}
}

// NewGame starts a run as a class on a generated first floor. The same seed and class always play out
// the same way.
func NewGame(seed uint64, c *Class) *World {
	rng := rand.New(rand.NewPCG(seed, 0))
	w := newRun(rng, c)
	w.seed = seed
	w.load(generate(rng, 1, 0))
	w.settle()
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
		w.heal(w.Player, 5)
		if w.MaxMana > 0 {
			w.MaxMana += 2
			w.Mana += 2
		}
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
	w.settle()
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

// say reports something that happened, on the message line and in the history.
func (w *World) say(format string, args ...any) {
	m := fmt.Sprintf(format, args...)
	w.Log = append(w.Log, m)
	w.History = append(w.History, m)
	if len(w.History) > maxHistory {
		w.History = w.History[1:]
	}
}

const maxHistory = 100

// tileNames says what each tile is, for Describe.
var tileNames = map[byte]string{
	'#': "a wall", '.': "the floor", '>': "stairs down", '+': "a closed door", '/': "an open door",
	'1': "an iron door, locked", '2': "a magic door, locked", '4': "an open iron door", '5': "an open magic door",
	'&': "a chest", '0': "an empty chest", '~': "water", '=': "lava", '%': "acid", '^': "a pit",
}

// Describe says what the hero knows of a tile: what stands there, if in sight, what lies there and
// the tile itself. A tile never seen is unknown.
func (w *World) Describe(x, y int) string {
	if !w.Seen[y][x] {
		return "You haven't seen that."
	}
	var parts []string
	if w.Visible[y][x] {
		if x == w.Player.X && y == w.Player.Y {
			parts = append(parts, "you")
		} else if m := w.monsterAt(x, y); m != nil {
			s := fmt.Sprintf("%s (%d/%d HP", m.Name, m.HP, m.MaxHP)
			for _, st := range []struct {
				on   bool
				name string
			}{{m.Sleep != 0, "asleep"}, {m.Confused > 0, "confused"}, {m.Poison > 0, "poisoned"}} {
				if st.on {
					s += ", " + st.name
				}
			}
			parts = append(parts, s+")")
		}
	}
	for _, it := range w.ItemsAt(x, y) {
		parts = append(parts, w.ItemName(it))
	}
	parts = append(parts, tileNames[w.Level[y][x]])
	return strings.Join(parts, ", ")
}

// record notes an action for the save, while the hero lives.
func (w *World) record(a Action) {
	if !w.Over {
		w.Actions = append(w.Actions, a)
	}
}

// article puts "a" or "an" before a name.
func article(name string) string {
	if strings.ContainsRune("aeiou", rune(name[0])) {
		return "an " + name
	}
	return "a " + name
}

// Step is one turn of moving or attacking in a direction, or waiting for 0, 0. Walking into lava
// asks first: the first try only warns.
func (w *World) Step(dx, dy int) {
	w.record(Action{Do: 'm', X: dx, Y: dy})
	tx, ty := w.Player.X+dx, w.Player.Y+dy
	if (dx != 0 || dy != 0) && w.Level[ty][tx] == '=' && w.Level[w.Player.Y][w.Player.X] != '=' &&
		w.warned != [2]int{tx, ty} && !w.Over {
		w.warned = [2]int{tx, ty}
		w.Log = []string{"That is lava! Move there again to step in."}
		return
	}
	w.warned = [2]int{}
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
	w.Turn++
	w.Shots, w.Log, w.Hits = nil, nil, nil
	for _, e := range append([]*Entity{w.Player}, w.Monsters...) {
		e.Anim, e.Spotted, e.Lunge, e.FromX, e.FromY = "idle", false, [2]int{}, e.X, e.Y
	}

	p, fromX, fromY := w.Player, w.Player.X, w.Player.Y
	act()
	switch w.Level[p.Y][p.X] {
	case '>':
		w.descend()
		return
	case '^':
		w.say("You fall through a pit!")
		w.hurtBy = "a fall"
		if w.damage(p, 1+w.rng.IntN(6)); !w.Over {
			w.descend()
		}
		return
	case '=':
		w.say("The lava burns you!")
		w.hurtBy = "lava"
		w.damage(p, 10+w.Depth)
	case '%':
		w.say("The acid burns!")
		w.hurtBy = "acid"
		w.damage(p, 3)
	}
	if w.Over {
		return
	}
	if w.Level[p.Y][p.X] == '~' && (p.X != fromX || p.Y != fromY) {
		w.monstersAct() // wading is slow: the monsters get an extra turn
	}
	if w.Player.HP < w.Player.MaxHP {
		if w.regen++; w.regen >= regenEvery {
			w.regen = 0
			w.Player.HP++
		}
	}
	w.monstersAct()

	if w.idle++; wanderEvery > 0 && w.idle >= wanderEvery && len(w.Monsters) < maxMonsters {
		w.idle = 0
		w.wander()
	}
	if w.Mana < w.MaxMana {
		if w.manaIn++; w.manaIn >= manaEvery {
			w.manaIn = 0
			w.Mana++
		}
	}
	if p.Confused > 0 {
		if p.Confused--; p.Confused == 0 {
			w.say("You feel steadier.")
		}
	}
	if p.Poison > 0 && !w.Over {
		p.Poison--
		w.hurtBy = "poison"
		w.hurt(p, 1, 'p')
	}
}

func (w *World) monstersAct() {
	// A copy, since monsters can die mid-turn.
	for _, m := range slices.Clone(w.Monsters) {
		for range m.Moves {
			if m.HP > 0 && !w.Over {
				w.act(m)
			}
		}
	}
}

// unlock opens a locked door with the matching key from the pack, which the lock keeps.
func (w *World) unlock(x, y int, c byte) {
	door, key, open := "iron door", goldKey, byte('4')
	if c == '2' {
		door, key, open = "magic door", blueKey, '5'
	}
	i := slices.IndexFunc(w.Inventory, func(it *Item) bool { return it.ItemKind == key })
	if i < 0 {
		w.say("The %s is locked. It needs a %s.", door, key.Name)
		return
	}
	w.Inventory = slices.Delete(w.Inventory, i, i+1)
	w.setTile(x, y, open)
	w.say("You unlock the %s with the %s.", door, key.Name)
}

// openChest spills three items onto the chest's tile, better than the floor's own.
func (w *World) openChest(x, y int) {
	w.setTile(x, y, '0')
	for range 3 {
		it := rollItem(w.rng, w.Depth+2)
		it.X, it.Y = x, y
		w.Floor = append(w.Floor, it)
	}
	w.say("You open the chest.")
}

func (w *World) playerAct(dx, dy int) {
	p := w.Player
	if p.Confused > 0 && w.rng.IntN(2) == 0 {
		d := dirs[w.rng.IntN(len(dirs))]
		dx, dy = d[0], d[1]
		w.say("You stumble.")
	}
	p.Dir = dirName(dx, dy)
	if t := w.firstInLine(p, dx, dy, p.Range); t != nil {
		w.attack(p, t)
		return
	}
	tx, ty := p.X+dx, p.Y+dy
	switch c := w.Level[ty][tx]; {
	case c == '+':
		w.setTile(tx, ty, '/')
		w.say("You open the door.")
	case c == '1' || c == '2':
		w.unlock(tx, ty, c)
	case c == '&':
		w.openChest(tx, ty)
	case w.free(tx, ty):
		p.X, p.Y, p.Anim = tx, ty, "walk"
		switch w.Level[ty][tx] {
		case '~':
			w.say("You wade into deep water.")
		case '%':
			w.say("You step into acid!")
		case '=':
			w.say("You step into lava!")
		}
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
	switch {
	case m.Sleep > 0: // put to sleep: it wakes when the time is up
		m.Sleep--
		return
	case m.Sleep < 0: // asleep on its own: each turn it could see the hero, it may wake
		odds := 4
		if w.Class != nil && w.Class.Stealth {
			odds = 8
		}
		if dist > sight || !w.canSee(m.X, m.Y, p.X, p.Y) || w.rng.IntN(odds) > 0 {
			return
		}
		m.Sleep = 0
		if w.Visible[m.Y][m.X] {
			w.say("The %s wakes up.", m.Name)
		}
	}
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
		if w.Level[m.Y+sy][m.X+sx] == '+' {
			w.setTile(m.X+sx, m.Y+sy, '/')
			return
		}
		m.X, m.Y, m.Dir, m.Anim = m.X+sx, m.Y+sy, dirName(sx, sy), "walk"
	}
}

var dirs = [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// canSee reports whether nothing that blocks sight lies on the straight (Bresenham) line between two tiles.
// The line steps along one axis at a time, so it can't slip through a diagonal gap between two walls.
func (w *World) canSee(x0, y0, x1, y1 int) bool {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	e := dx + dy
	for x0 != x1 || y0 != y1 {
		if blocksSight(w.Level[y0][x0]) {
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

// stepToward returns a first step on a shortest path from x, y to gx, gy: onto a free tile, or into
// a closed door to open it. Paths go around walls, locked doors, chests and hazards, but through
// creatures, so a monster stuck behind another one waits its turn rather than detouring.
func (w *World) stepToward(x, y, gx, gy int) (dx, dy int, ok bool) {
	// Breadth-first search outward from the goal until it reaches the start.
	dist := map[[2]int]int{{gx, gy}: 0}
	for queue := [][2]int{{gx, gy}}; len(queue) > 0 && queue[0] != [2]int{x, y}; queue = queue[1:] {
		c := queue[0]
		for _, d := range dirs {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if _, seen := dist[n]; !seen && w.passable(w.Level[n[1]][n[0]]) {
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
		nx, ny := x+d[0], y+d[1]
		if n, ok := dist[[2]int{nx, ny}]; ok && n == here-1 && (w.free(nx, ny) || w.Level[ny][nx] == '+' && w.monsterAt(nx, ny) == nil) {
			return d[0], d[1], true
		}
	}
	return 0, 0, false
}

// firstInLine returns the first creature within n tiles of from in direction dx, dy, stopping at walls,
// closed doors and chests.
func (w *World) firstInLine(from *Entity, dx, dy, n int) *Entity {
	for i := 1; i <= n; i++ {
		x, y := from.X+dx*i, from.Y+dy*i
		if solid(w.Level[y][x]) {
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
// over the target's armour, never certain either way, except against a sleeper.
func hitChance(a, t *Entity) int {
	if t.Sleep != 0 {
		return 100
	}
	return min(max(70+5*(a.Atk-t.Def), 5), 95)
}

func (w *World) attack(a, t *Entity) {
	a.Anim = "atk"
	if abs(t.X-a.X)+abs(t.Y-a.Y) > 1 {
		w.Shots = append(w.Shots, Shot{a.X, a.Y, t, a.Missile})
	} else {
		a.Lunge = [2]int{t.X - a.X, t.Y - a.Y}
	}
	if w.rng.IntN(100) >= hitChance(a, t) {
		w.Hits = append(w.Hits, Hit{t, 0, 'm'})
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
		w.hurtBy = article(a.Name)
	}
	w.damage(t, 1+w.rng.IntN(a.Dmg))
	if t.HP <= 0 || t != w.Player {
		return
	}
	if w.rng.IntN(100) < a.Poisons {
		t.Poison += 4 + w.Depth/2
		w.say("You are poisoned!")
	}
	if w.rng.IntN(100) < a.Confuses {
		t.Confused = 4
		w.say("You feel confused!")
	}
}

func (w *World) damage(e *Entity, n int) { w.hurt(e, n, 'd') }

// heal gives a creature back up to n HP, no more than its most.
func (w *World) heal(e *Entity, n int) {
	n = min(n, e.MaxHP-e.HP)
	e.HP += n
	if n > 0 {
		w.Hits = append(w.Hits, Hit{e, n, 'h'})
	}
}

// TakeHits hands the screen the HP changes since it last asked.
func (w *World) TakeHits() []Hit {
	hs := w.Hits
	w.Hits = nil
	return hs
}

func (w *World) hurt(e *Entity, n int, kind byte) {
	w.Hits = append(w.Hits, Hit{e, n, kind})
	e.HP -= n
	e.Sleep = 0
	if e.HP > 0 {
		return
	}
	if e == w.Player {
		w.Over, w.Cause = true, w.hurtBy
		w.say("You die.")
		return
	}
	w.Kills++
	w.gainXP(e.XP)
	if e.Boss { // the sealed stairs open where it fell, and its hoard spills around them
		w.setTile(e.X, e.Y, '>')
		w.say("The %s falls. The stairs open!", e.Name)
		loot := []*Item{rollItem(w.rng, w.Depth+3), rollItem(w.rng, w.Depth+3), {ItemKind: gold, Amount: 50 * w.Depth}}
		for _, d := range dirs {
			if x, y := e.X+d[0], e.Y+d[1]; len(loot) > 0 && !solid(w.Level[y][x]) && !hazard(w.Level[y][x]) {
				loot[0].X, loot[0].Y = x, y
				w.Floor = append(w.Floor, loot[0])
				loot = loot[1:]
			}
		}
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

// free reports whether a creature could step onto a tile now.
func (w *World) free(x, y int) bool {
	return !solid(w.Level[y][x]) && w.monsterAt(x, y) == nil && (w.Player.X != x || w.Player.Y != y)
}

// passable is what paths may cross: anything but solid tiles and hazards, though closed wooden doors
// count, since walking into one opens it.
func (w *World) passable(c byte) bool { return c == '+' || !solid(c) && !hazard(c) }

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
