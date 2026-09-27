package main

import (
	"os"
	"strings"
	"testing"
)

// TestBalance plays many seeded runs with two simple bots and logs how they went. It is a tuning
// aid, not a pass/fail check, so it only runs on request:
//
//	NINEDEEP_SIM=1 go test -run Balance -v
func TestBalance(t *testing.T) {
	if os.Getenv("NINEDEEP_SIM") == "" {
		t.Skip("set NINEDEEP_SIM=1 to run the balance simulation")
	}
	const runs = 300
	for _, b := range []struct {
		name string
		play func(*World) (int, int)
	}{{"fighter", fighter}, {"diver", diver}} {
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
		t.Logf("%-7s wins %3d%%  stuck %d  possessions/run %.1f  steps/run %d  deaths by floor %v",
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
		return w.botToward(w.nearest(func(m *Entity) bool { return m.Broken() }))
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
		return w.botToward(w.nearest(func(m *Entity) bool { return m.Broken() }))
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
