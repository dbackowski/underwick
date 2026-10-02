package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// TestBalance plays many seeded runs with three bots and logs how they went. It is a tuning
// aid, not a pass/fail check, so it only runs on request:
//
//	UNDERWICK_SIM=1 go test -run Balance -v
func TestBalance(t *testing.T) {
	if os.Getenv("UNDERWICK_SIM") == "" {
		t.Skip("set UNDERWICK_SIM=1 to run the balance simulation")
	}
	const runs = 300
	for _, b := range []struct {
		name string
		play func(*World) (int, int)
	}{{"fighter", fighter}, {"diver", diver}, {"tactician", tactician}} {
		var wins, stuck, possessions, turns int
		deaths := make([]int, floors+1)
		for seed := range uint64(runs) {
			w := NewGame(seed)
			n := 0
			for ; !w.Over && n < 3000; n++ {
				body := w.Player
				w.Step(b.play(w))
				if w.Player != body && !w.IsSpark() {
					possessions++
				}
			}
			turns += n
			switch {
			case w.Won:
				wins++
			case !w.Over:
				stuck++
			default:
				deaths[w.Depth]++
			}
		}
		t.Logf("%-9s wins %3d%%  stuck %d  possessions/run %.1f  steps/run %d  deaths by floor %v",
			b.name, wins*100/runs, stuck, float64(possessions)/runs, turns/runs, deaths[1:])
	}
}

// fighter fights whatever hunts it, swaps into broken bodies when hurt or offered a bigger one,
// and otherwise heads for the stairs.
func fighter(w *World) (int, int) {
	p := w.Player
	hurt := p.HP*2 <= p.MaxHP
	for _, d := range dirs {
		if m := w.monsterAt(p.X+d[0], p.Y+d[1]); m != nil && m.Broken() && (w.IsSpark() || hurt || m.MaxHP > p.MaxHP) {
			return d[0], d[1]
		}
	}
	if w.IsSpark() {
		return w.sparkMove()
	}
	for _, d := range dirs {
		if t := w.firstInLine(p, d[0], d[1], p.Range); t != nil && !t.Broken() {
			return d[0], d[1]
		}
	}
	if m := w.nearest(func(m *Entity) bool { return m.Broken() }); hurt && m != nil {
		return w.botToward(m)
	}
	if m := w.nearest(func(m *Entity) bool { return m.hunting && !m.Broken() }); m != nil {
		return w.botToward(m)
	}
	if dx, dy := w.botToStairs(); dx != 0 || dy != 0 {
		return dx, dy
	}
	for _, d := range dirs { // the way is blocked by a broken body: take it rather than wait
		if m := w.monsterAt(p.X+d[0], p.Y+d[1]); m != nil && m.Broken() {
			return d[0], d[1]
		}
	}
	return 0, 0
}

// diver runs for the stairs, fighting only what blocks the way and possessing only to survive.
func diver(w *World) (int, int) {
	p := w.Player
	for _, d := range dirs {
		if m := w.monsterAt(p.X+d[0], p.Y+d[1]); m != nil && m.Broken() && (w.IsSpark() || p.HP*2 <= p.MaxHP) {
			return d[0], d[1]
		}
	}
	if w.IsSpark() {
		return w.sparkMove()
	}
	if dx, dy := w.botToStairs(); dx != 0 || dy != 0 {
		return dx, dy
	}
	for _, d := range dirs {
		if t := w.firstInLine(p, d[0], d[1], p.Range); t != nil && !t.Broken() {
			return d[0], d[1]
		}
	}
	return 0, 0
}

// sparkMove heads for the nearest broken body it can reach, or else the nearest monster to haunt.
func (w *World) sparkMove() (int, int) {
	p := w.Player
	targets := slices.DeleteFunc(slices.Clone(w.Monsters), func(m *Entity) bool { return m.Boss })
	slices.SortStableFunc(targets, func(a, b *Entity) int {
		ra, rb := abs(a.X-p.X)+abs(a.Y-p.Y), abs(b.X-p.X)+abs(b.Y-p.Y)
		if a.Broken() { // broken bodies first: taking one is a single move
			ra -= 100
		}
		if b.Broken() {
			rb -= 100
		}
		return ra - rb
	})
	for _, m := range targets {
		if abs(m.X-p.X)+abs(m.Y-p.Y) == 1 {
			return m.X - p.X, m.Y - p.Y
		}
		if dx, dy, ok := w.stepToward(p.X, p.Y, m.X, m.Y); ok {
			return dx, dy
		}
	}
	return 0, 0
}

func (w *World) nearest(keep func(*Entity) bool) *Entity {
	var best *Entity
	for _, m := range w.Monsters {
		if keep(m) && (best == nil || abs(m.X-w.Player.X)+abs(m.Y-w.Player.Y) < abs(best.X-w.Player.X)+abs(best.Y-w.Player.Y)) {
			best = m
		}
	}
	return best
}

func (w *World) botToward(m *Entity) (int, int) {
	if m == nil {
		return 0, 0
	}
	dx, dy, _ := w.stepToward(w.Player.X, w.Player.Y, m.X, m.Y)
	return dx, dy
}

func (w *World) botToStairs() (int, int) {
	i := strings.IndexByte(strings.Join(w.Level, ""), '>')
	if i < 0 { // a boss floor: the stairs open when the boss dies
		return w.botToward(w.nearest(func(m *Entity) bool { return m.Boss }))
	}
	dx, dy, _ := w.stepToward(w.Player.X, w.Player.Y, i%mapW, i/mapW)
	return dx, dy
}

// tactician plays the way a careful player would: it strikes whatever it can reach, lets melee
// monsters walk up to it so it hits first, steps out of ranged lines it can't answer, keeps its body
// fresh by breaking monsters and taking them, and on a boss floor clears the minions before facing
// the boss.
func tactician(w *World) (int, int) {
	p := w.Player
	hurt := p.HP*2 <= p.MaxHP
	broken := func(m *Entity) bool { return m.Broken() }
	unbroken := func(m *Entity) bool { return !m.Broken() && !m.Boss }

	for _, d := range dirs { // take a broken body when bare, hurt, or offered a stronger one
		if m := w.monsterAt(p.X+d[0], p.Y+d[1]); m != nil && m.Broken() && (w.IsSpark() || hurt || m.MaxHP > p.MaxHP) {
			return d[0], d[1]
		}
	}
	if w.IsSpark() {
		return w.sparkMove()
	}
	for _, d := range dirs { // strike
		if t := w.firstInLine(p, d[0], d[1], p.Range); t != nil && !t.Broken() {
			return d[0], d[1]
		}
	}
	if s := w.shooterAt(p.X, p.Y); s != nil { // under fire we can't answer: charge the shooter
		return w.botToward(s)
	}
	if m := w.nearest(func(m *Entity) bool { return m.hunting && !m.Broken() && m.Range == 1 }); m != nil &&
		abs(m.X-p.X)+abs(m.Y-p.Y) <= m.Moves+1 {
		return 0, 0 // it closes in this turn; waiting gives us the first hit
	}
	if hurt { // head for a spare body, or go and make one
		if b := w.nearest(broken); b != nil {
			return w.botToward(b)
		}
		if m := w.nearest(unbroken); m != nil {
			return w.botToward(m)
		}
	}
	if strings.IndexByte(strings.Join(w.Level, ""), '>') < 0 { // boss floor: clear the minions first
		if m := w.nearest(unbroken); m != nil {
			return w.botToward(m)
		}
	}
	if dx, dy := w.botToStairs(); dx != 0 || dy != 0 {
		return dx, dy
	}
	for _, d := range dirs { // the way is blocked by a broken body: take it rather than wait
		if m := w.monsterAt(p.X+d[0], p.Y+d[1]); m != nil && m.Broken() {
			return d[0], d[1]
		}
	}
	return 0, 0
}

// shooterAt returns a ranged monster that can shoot tile x, y, if any.
func (w *World) shooterAt(x, y int) *Entity {
	for _, m := range w.Monsters {
		if m.Range > 1 && !m.Broken() && (m.X == x || m.Y == y) && abs(m.X-x)+abs(m.Y-y) <= m.Range && w.canSee(m.X, m.Y, x, y) {
			return m
		}
	}
	return nil
}
