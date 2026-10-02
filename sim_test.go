package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// TestBalance plays many seeded runs with two bots and logs how deep they got. It is a tuning
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
	}{{"diver", diver}, {"tactician", tactician}} {
		var depths []int
		var score, turns int
		deaths := make([]int, 16) // by depth; the last bucket counts everything deeper
		for seed := range uint64(runs) {
			w := NewGame(seed)
			n := 0
			for ; !w.Over && n < 20000; n++ {
				w.Step(b.play(w))
			}
			depths = append(depths, w.Depth)
			score += w.Score()
			turns += n
			deaths[min(w.Depth, len(deaths))-1]++
		}
		slices.Sort(depths)
		t.Logf("%-9s depth median %d, best %d  score avg %d  turns/run %d  deaths by depth %v",
			b.name, depths[runs/2], depths[runs-1], score/runs, turns/runs, deaths)
	}
}

// diver runs for the stairs, fighting only what stands in its way or shoots at it.
func diver(w *World) (int, int) {
	if dx, dy := w.botToStairs(); dx != 0 || dy != 0 {
		return dx, dy
	}
	return w.botStrike()
}

// tactician plays the way a careful player would: it strikes whatever it can reach, lets melee
// monsters walk up to it so it hits first, charges shooters rather than standing in their line,
// and rests to regain HP when hurt and nothing is hunting it.
func tactician(w *World) (int, int) {
	p := w.Player
	if dx, dy := w.botStrike(); dx != 0 || dy != 0 {
		return dx, dy
	}
	if s := w.shooterAt(p.X, p.Y); s != nil {
		return w.botToward(s)
	}
	hunter := w.nearest(func(m *Entity) bool { return m.hunting })
	if hunter != nil && hunter.Range == 1 && abs(hunter.X-p.X)+abs(hunter.Y-p.Y) <= hunter.Moves+1 {
		return 0, 0 // it closes in this turn; waiting gives us the first hit
	}
	if hunter == nil && p.HP*3 < p.MaxHP*2 {
		return 0, 0 // rest
	}
	return w.botToStairs()
}

// botStrike attacks the first monster within reach in any direction, or waits.
func (w *World) botStrike() (int, int) {
	for _, d := range dirs {
		if t := w.firstInLine(w.Player, d[0], d[1], w.Player.Range); t != nil {
			return d[0], d[1]
		}
	}
	return 0, 0
}

// shooterAt returns a ranged monster that can shoot tile x, y, if any.
func (w *World) shooterAt(x, y int) *Entity {
	for _, m := range w.Monsters {
		if m.Range > 1 && (m.X == x || m.Y == y) && abs(m.X-x)+abs(m.Y-y) <= m.Range && w.canSee(m.X, m.Y, x, y) {
			return m
		}
	}
	return nil
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

// botToStairs steps toward the stairs, or toward the boss while they are sealed.
func (w *World) botToStairs() (int, int) {
	i := strings.IndexByte(strings.Join(w.Level, ""), '>')
	if i < 0 {
		return w.botToward(w.nearest(func(m *Entity) bool { return m.Boss }))
	}
	dx, dy, _ := w.stepToward(w.Player.X, w.Player.Y, i%mapW, i/mapW)
	return dx, dy
}
