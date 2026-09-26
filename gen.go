package main

import (
	"math/rand/v2"
	"slices"
	"strings"
)

const (
	mapW, mapH = 20, 15
	floors     = 9
)

type room struct{ x, y, w, h int }

func (r room) center() (int, int) { return r.x + r.w/2, r.y + r.h/2 }

// overlaps also counts rooms that touch, so there is always a wall between two rooms.
func (r room) overlaps(o room) bool {
	return r.x <= o.x+o.w && o.x <= r.x+r.w && r.y <= o.y+o.h && o.y <= r.y+r.h
}

// generate builds a floor in the format NewWorld parses: rooms joined by corridors, the player in
// the first room, stairs down ('>') in the room farthest from it, and monsters in the other rooms.
func generate(rng *rand.Rand, depth int) []string {
	for {
		g := make([][]byte, mapH)
		for y := range g {
			g[y] = []byte(strings.Repeat("#", mapW))
		}

		var rooms []room
		for range 50 {
			r := room{w: 3 + rng.IntN(5), h: 3 + rng.IntN(3)}
			r.x, r.y = 1+rng.IntN(mapW-1-r.w), 1+rng.IntN(mapH-1-r.h)
			if !slices.ContainsFunc(rooms, r.overlaps) {
				rooms = append(rooms, r)
			}
		}
		if len(rooms) < 3 {
			continue // too cramped to be interesting; roll again
		}

		for _, r := range rooms {
			for y := r.y; y < r.y+r.h; y++ {
				for x := r.x; x < r.x+r.w; x++ {
					g[y][x] = '.'
				}
			}
		}
		// Chaining each room to the previous one keeps the whole floor connected.
		for i := 1; i < len(rooms); i++ {
			ax, ay := rooms[i-1].center()
			bx, by := rooms[i].center()
			if rng.IntN(2) == 0 {
				carve(g, ax, bx, ay, true)
				carve(g, ay, by, bx, false)
			} else {
				carve(g, ay, by, ax, false)
				carve(g, ax, bx, by, true)
			}
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
		for _, s := range spots[:min(2+depth, len(spots))] {
			g[s[1]][s[0]] = pool[rng.IntN(len(pool))]
		}

		level := make([]string, mapH)
		for y := range g {
			level[y] = string(g[y])
		}
		return level
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
