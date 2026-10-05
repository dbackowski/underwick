package main

import (
	"math/rand/v2"
	"slices"
	"strings"
)

const mapW, mapH = 40, 30

const minions = 3 // monsters placed next to a boss

type room struct{ x, y, w, h int }

func (r room) center() (int, int) { return r.x + r.w/2, r.y + r.h/2 }

// overlaps also counts rooms that touch, so there is always a wall between two rooms.
func (r room) overlaps(o room) bool {
	return r.x <= o.x+o.w && o.x <= r.x+r.w && r.y <= o.y+o.h && o.y <= r.y+r.h
}

// ring lists the tiles just outside a room's edges, where corridors come in (corners excluded), each
// with the direction along the ring, so a doorway's two sides can be checked.
func (r room) ring() [][4]int {
	var ts [][4]int
	for x := r.x; x < r.x+r.w; x++ {
		ts = append(ts, [4]int{x, r.y - 1, 1, 0}, [4]int{x, r.y + r.h, 1, 0})
	}
	for y := r.y; y < r.y+r.h; y++ {
		ts = append(ts, [4]int{r.x - 1, y, 0, 1}, [4]int{r.x + r.w, y, 0, 1})
	}
	return ts
}

// generate builds a floor in the format World.load parses:
//
//	#  wall           .  floor             >  stairs down      @  the hero's start
//	*  an item        (  gold key          )  blue key         &  chest
//	+  door           1  iron door, locked (gold key)          2  magic door, locked (blue key)
//	~  deep water     =  lava              %  acid             ^  pit
//	$  the merchant
//
// and a kind letter for each monster. Rooms are joined by corridors, often through doors. The hero
// starts in the first room and the stairs are in the room farthest from it; with a boss (its kind
// character, or 0 for none) the boss takes the stairs' place, and they open when it dies. A dead-end
// room may be a locked vault with a chest, its key elsewhere on the floor. Other rooms may hold a
// pool of water, acid, lava or pits, always leaving the room's edge walkable. On a shop floor (see
// shopFloor) a merchant stands in the middle of a room with no pool.
func generate(rng *rand.Rand, depth int, boss byte) []string {
	for {
		g := make([][]byte, mapH)
		for y := range g {
			g[y] = []byte(strings.Repeat("#", mapW))
		}

		var rooms []room
		for range 200 {
			r := room{w: 3 + rng.IntN(7), h: 3 + rng.IntN(4)}
			r.x, r.y = 1+rng.IntN(mapW-1-r.w), 1+rng.IntN(mapH-1-r.h)
			if !slices.ContainsFunc(rooms, r.overlaps) {
				rooms = append(rooms, r)
			}
		}
		if len(rooms) < 6 {
			continue // too cramped to be interesting; roll again
		}

		for _, r := range rooms {
			for y := r.y; y < r.y+r.h; y++ {
				for x := r.x; x < r.x+r.w; x++ {
					g[y][x] = '.'
				}
			}
		}
		// Join each room to its nearest already-joined one, so the floor is connected by short
		// corridors, then add a couple of extra ones for loops to run around.
		joined, rest := rooms[:1], slices.Clone(rooms[1:])
		for len(rest) > 0 {
			var bi, bj int
			for i, r := range rest {
				for j, o := range joined {
					if dist(r, o) < dist(rest[bi], joined[bj]) {
						bi, bj = i, j
					}
				}
			}
			link(g, rng, rest[bi], joined[bj])
			joined = append(slices.Clip(joined), rest[bi])
			rest = slices.Delete(rest, bi, bi+1)
		}
		for range 2 {
			link(g, rng, rooms[rng.IntN(len(rooms))], rooms[rng.IntN(len(rooms))])
		}

		sx, sy := rooms[0].center()
		far := slices.IndexFunc(rooms, func(r room) bool {
			return r == slices.MaxFunc(rooms[1:], func(a, b room) int {
				ax, ay := a.center()
				bx, by := b.center()
				return abs(ax-sx) + abs(ay-sy) - abs(bx-sx) - abs(by-sy)
			})
		})

		// Doors: half the proper doorways, where a corridor meets a room between two walls. A room
		// entered only through one doorway is a dead end, and may become the vault.
		vault, key := -1, byte(0)
		for i, r := range rooms {
			var entrances, doorways [][2]int
			for _, t := range r.ring() {
				x, y, dx, dy := t[0], t[1], t[2], t[3]
				if g[y][x] == '#' {
					continue
				}
				entrances = append(entrances, [2]int{x, y})
				if g[y-dy][x-dx] == '#' && g[y+dy][x+dx] == '#' {
					doorways = append(doorways, [2]int{x, y})
					if g[y][x] == '.' && rng.IntN(2) == 0 { // a doorway between two rooms may have its door already
						g[y][x] = '+'
					}
				}
			}
			if vault < 0 && i != 0 && i != far && depth >= 2 && len(entrances) == 1 && len(doorways) == 1 && rng.IntN(2) == 0 {
				vault = i
				d, lock := doorways[0], rng.IntN(2)
				g[d[1]][d[0]], key = "12"[lock], "()"[lock]
			}
		}

		shop := -1
		for i := 1; i < len(rooms) && shop < 0 && shopFloor(depth); i++ {
			if i != far && i != vault {
				shop = i
			}
		}

		// Hazard pools fill parts of rooms' insides, never their edges, so every entrance still
		// connects to every other along the edge.
		for i, r := range rooms {
			if i == 0 || i == far || i == vault || rng.IntN(3) > 0 || i == shop {
				continue
			}
			pool := "~"
			if depth >= 2 && boss == 0 {
				pool += "^" // no pits on a boss floor: they would drop the hero past the boss
			}
			if depth >= 3 {
				pool += "%"
			}
			if depth >= 4 {
				pool += "="
			}
			h := pool[rng.IntN(len(pool))]
			for y := r.y + 1; y < r.y+r.h-1; y++ {
				for x := r.x + 1; x < r.x+r.w-1; x++ {
					if rng.IntN(5) < 3 {
						g[y][x] = h
					}
				}
			}
		}

		g[sy][sx] = '@'
		if shop >= 0 {
			mx, my := rooms[shop].center()
			g[my][mx] = '$'
		}
		fx, fy := rooms[far].center()
		g[fy][fx] = '>'
		if boss != 0 {
			g[fy][fx] = boss
		}

		var spots [][2]int
		for i, r := range rooms[1:] {
			if i+1 == vault {
				continue
			}
			for y := r.y; y < r.y+r.h; y++ {
				for x := r.x; x < r.x+r.w; x++ {
					if g[y][x] == '.' {
						spots = append(spots, [2]int{x, y})
					}
				}
			}
		}
		rng.Shuffle(len(spots), func(i, j int) { spots[i], spots[j] = spots[j], spots[i] })
		// The chest in the vault, and its key out on the floor, where the hero can walk to it without
		// crossing a hazard: pools can leave a dry tile stranded in their middle.
		reach := walkable(func(x, y int) byte { return g[y][x] }, sx, sy)
		if k := slices.IndexFunc(spots, func(s [2]int) bool { return reach[s] }); vault >= 0 && k >= 0 {
			vx, vy := rooms[vault].center()
			g[vy][vx] = '&'
			g[spots[k][1]][spots[k][0]] = key
			spots = slices.Delete(spots, k, k+1)
		}
		items := min(4+depth/2, 12, len(spots)) // '*' marks a spot; the world rolls what lies there
		for _, s := range spots[:items] {
			g[s[1]][s[0]] = '*'
		}
		spots = spots[items:]
		// ponytail: the count grows straight to 20 a floor; tune once the game is played
		pool := spawnable(depth)
		n := min(4+2*depth, 20, len(spots))
		if boss != 0 {
			// Minions stand closest to the boss, so a body dying in the fight has others in reach.
			slices.SortStableFunc(spots, func(a, b [2]int) int {
				return abs(a[0]-fx) + abs(a[1]-fy) - abs(b[0]-fx) - abs(b[1]-fy)
			})
			for _, s := range spots[:min(minions, len(spots))] {
				g[s[1]][s[0]] = pool[rng.IntN(len(pool))]
			}
			spots = spots[min(minions, len(spots)):]
			rng.Shuffle(len(spots), func(i, j int) { spots[i], spots[j] = spots[j], spots[i] })
			n = min(max(0, min(4+2*depth, 20)-minions), len(spots))
		}
		for _, s := range spots[:n] {
			g[s[1]][s[0]] = pool[rng.IntN(len(pool))]
		}

		level := make([]string, mapH)
		for y := range g {
			level[y] = string(g[y])
		}
		return level
	}
}

// walkable maps the tiles reachable from x, y without crossing walls, locked doors, chests, the merchant
// or hazards.
func walkable(tile func(x, y int) byte, x, y int) map[[2]int]bool {
	seen := map[[2]int]bool{{x, y}: true}
	for queue := [][2]int{{x, y}}; len(queue) > 0; queue = queue[1:] {
		for _, d := range dirs {
			n := [2]int{queue[0][0] + d[0], queue[0][1] + d[1]}
			if c := tile(n[0], n[1]); !seen[n] && !hazard(c) && !strings.ContainsRune("#12&$", rune(c)) {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return seen
}

func dist(a, b room) int {
	ax, ay := a.center()
	bx, by := b.center()
	return abs(ax-bx) + abs(ay-by)
}

// link digs an L-shaped corridor between two rooms' centres, turning at a random corner.
func link(g [][]byte, rng *rand.Rand, a, b room) {
	ax, ay := a.center()
	bx, by := b.center()
	if rng.IntN(2) == 0 {
		carve(g, ax, bx, ay, true)
		carve(g, ay, by, bx, false)
	} else {
		carve(g, ay, by, ax, false)
		carve(g, ax, bx, by, true)
	}
}

// carve digs a straight corridor from a to b along one axis, at fixed position at on the other.
func carve(g [][]byte, a, b, at int, horizontal bool) {
	for i := min(a, b); i <= max(a, b); i++ {
		x, y := i, at
		if !horizontal {
			x, y = at, i
		}
		if g[y][x] == '#' {
			g[y][x] = '.'
		}
	}
}
