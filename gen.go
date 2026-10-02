package main

import (
	"math/rand/v2"
	"slices"
	"strings"
)

const (
	mapW, mapH = 40, 30
	floors     = 9
)

const minions = 3 // monsters placed next to a boss

type room struct{ x, y, w, h int }

func (r room) center() (int, int) { return r.x + r.w/2, r.y + r.h/2 }

// overlaps also counts rooms that touch, so there is always a wall between two rooms.
func (r room) overlaps(o room) bool {
	return r.x <= o.x+o.w && o.x <= r.x+r.w && r.y <= o.y+o.h && o.y <= r.y+r.h
}

// generate builds a floor in the format NewWorld parses: rooms joined by corridors, the player in
// the first room, stairs down ('>') in the room farthest from it, and monsters in the other rooms.
// With a boss (its kind character, or 0 for none), the boss takes the stairs' place; they open when it dies.
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
		g[sy][sx] = '@'
		far := slices.MaxFunc(rooms[1:], func(a, b room) int {
			ax, ay := a.center()
			bx, by := b.center()
			return abs(ax-sx) + abs(ay-sy) - abs(bx-sx) - abs(by-sy)
		})
		fx, fy := far.center()
		g[fy][fx] = '>'
		if boss != 0 {
			g[fy][fx] = boss
		}

		var spots [][2]int
		for _, r := range rooms[1:] {
			for y := r.y; y < r.y+r.h; y++ {
				for x := r.x; x < r.x+r.w; x++ {
					if g[y][x] == '.' {
						spots = append(spots, [2]int{x, y})
					}
				}
			}
		}
		rng.Shuffle(len(spots), func(i, j int) { spots[i], spots[j] = spots[j], spots[i] })
		// ponytail: flat difficulty curve, more monsters and more orcs per floor; tune once the game is played
		pool := "rrraa" + strings.Repeat("o", depth/2)
		n := min(4+2*depth, len(spots))
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
			n = min(max(0, 4+2*depth-minions), len(spots))
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
