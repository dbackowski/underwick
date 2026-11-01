package main

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestBalance plays many seeded runs with bots and logs how deep they got: the diver as a warrior, and
// the tactician as every class. It is a tuning aid, not a pass/fail check, so it only runs on request,
// with 300 runs a bot unless given another count:
//
//	UNDERWICK_SIM=1 go test -run Balance -v
//	UNDERWICK_SIM=1000 go test -run Balance -v
//
// Each average score comes with its 95% margin: two results whose margins overlap may differ by chance.
func TestBalance(t *testing.T) {
	if os.Getenv("UNDERWICK_SIM") == "" {
		t.Skip("set UNDERWICK_SIM=1 to run the balance simulation")
	}
	runs := 300
	if n, err := strconv.Atoi(os.Getenv("UNDERWICK_SIM")); err == nil && n > 1 {
		runs = n
	}
	type bot struct {
		name  string
		class *Class
		play  func(*World) (int, int)
	}
	bots := []bot{{"diver", classes[0], diver}}
	for _, c := range classes {
		bots = append(bots, bot{"tact " + c.Name, c, tactician})
	}
	// Each bot plays in its own goroutine; the logs wait for all of them, to come out in order.
	logs := make([][]string, len(bots))
	var wg sync.WaitGroup
	for bi, b := range bots {
		wg.Go(func() { logs[bi] = simulate(b.name, b.class, b.play, runs) })
	}
	wg.Wait()
	for _, l := range logs {
		for _, line := range l {
			t.Log(line)
		}
	}
}

// simulate plays one bot's runs and describes how they went.
func simulate(name string, class *Class, play func(*World) (int, int), runs int) []string {
	tl := &tally{shopVisits: map[int][]int{}}
	var gear, turns, stuck int
	var depths []int
	var scores []float64
	deaths := make([]int, 16) // by depth; the last bucket counts everything deeper
	// What ends a run on floors 1-3, and in what state, to tell a bad start from a bad fight.
	early, earlyLevel, earlyAround := map[string]int{}, 0, 0
	for seed := range uint64(runs) {
		w := NewGame(seed, class)
		st := &botState{tally: tl}
		botStates.Store(w, st)
		n := 0
		for ; !w.Over && n < 20000; n++ {
			if name != "diver" && w.botItems() {
				continue // an item action used the turn
			}
			w.Step(play(w))
		}
		botStates.Delete(w)
		for _, it := range w.Inventory {
			if it.Worn && it.Slot != "weapon" {
				gear++
			}
		}

		if !w.Over {
			stuck++
		}
		depths = append(depths, w.Depth)
		scores = append(scores, float64(w.Score()))
		turns += n
		deaths[min(w.Depth, len(deaths))-1]++
		if w.Over && w.Depth <= 3 {
			if w.Cause == "poison" && st.drunk == "potion of poison" {
				early["a potion of poison"]++
			} else if w.Player.Confused > 0 && st.drunk == "potion of confusion" {
				early["confused by a potion"]++
			} else {
				early[w.Cause]++
			}
			earlyLevel += w.ExpLevel
			for _, m := range w.Monsters {
				if m.HP > 0 && max(abs(m.X-w.Player.X), abs(m.Y-w.Player.Y)) <= 1 {
					earlyAround++
				}
			}
		}
	}
	slices.Sort(depths)
	mean, spread := 0.0, 0.0
	for _, s := range scores {
		mean += s / float64(runs)
	}
	for _, s := range scores {
		spread += (s - mean) * (s - mean) / float64(runs-1)
	}
	margin := 1.96 * math.Sqrt(spread/float64(runs))
	logs := []string{fmt.Sprintf("%-14s stuck %d  depth median %d, best %d  score avg %.0f ±%.0f  turns/run %d  potions/run %.1f  armour worn at death %.1f  deaths by depth %v",
		name, stuck, depths[runs/2], depths[runs-1], mean, margin, turns/runs, float64(tl.potionsDrunk)/float64(runs), float64(gear)/float64(runs), deaths)}
	if len(early) > 0 {
		var causes []string
		for c, k := range early {
			causes = append(causes, fmt.Sprintf("%s %d", c, k))
		}
		slices.Sort(causes) // map order would shuffle the log between runs
		dead := 0
		for _, k := range early {
			dead += k
		}
		logs = append(logs, fmt.Sprintf("%-14s floors 1-3: %d deaths, level %.1f, %.1f monsters adjacent; by %s", "", dead,
			float64(earlyLevel)/float64(dead), float64(earlyAround)/float64(dead), strings.Join(causes, ", ")))
	}
	if name != "diver" {
		var visits []string
		for d := range 30 {
			if gs := tl.shopVisits[d]; len(gs) > 0 {
				slices.Sort(gs)
				visits = append(visits, fmt.Sprintf("floor %d: %d runs, median %d gold", d, len(gs), gs[len(gs)/2]))
			}
		}
		logs = append(logs, fmt.Sprintf("%-14s shops: gold spent/run %d  wares/run %.1f  heals/run %.1f  on arrival %s", "",
			tl.goldSpent/runs, float64(tl.waresBought)/float64(runs), float64(tl.healsBought)/float64(runs), strings.Join(visits, "; ")))
	}
	return logs
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
// and, knowing Heal, casts it when hurt (botItems).
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
		if _, _, ok := w.stepToward(hunter.X, hunter.Y, p.X, p.Y); ok {
			return 0, 0 // it closes in this turn; waiting gives us the first hit
		}
	}
	if hunter == nil && len(w.Inventory) < maxItems {
		// Keep going for the item it set out for, even once it drops out of sight; otherwise pick
		// the first one in view.
		it := w.bot().target
		if !slices.Contains(w.Floor, it) {
			it = nil
			for _, f := range w.Floor {
				if w.Visible[f.Y][f.X] && w.monsterAt(f.X, f.Y) == nil {
					it = f
					break
				}
			}
		}
		w.bot().target = it
		if it != nil {
			if dx, dy, ok := w.stepToward(p.X, p.Y, it.X, it.Y); ok {
				return dx, dy
			}
			w.bot().target = nil // can't get there; forget it
		}
	}
	// Then smash the barrels and crates and search the bookcases in sight, those alongside first.
	if hunter == nil {
		for _, d := range dirs {
			if c := w.Level[p.Y+d[1]][p.X+d[0]]; c == '6' || c == '7' || c == '8' {
				return d[0], d[1]
			}
		}
		for y, row := range w.Level {
			for x := range row {
				if c := row[x]; (c == '6' || c == '7' || c == '8') && w.Visible[y][x] {
					if dx, dy, ok := w.stepToward(p.X, p.Y, x, y); ok {
						return dx, dy
					}
				}
			}
		}
	}
	// With the floor's items gathered, visit the merchant on the way to the stairs; botItems shops.
	if i := strings.IndexByte(strings.Join(w.Level, ""), '$'); i >= 0 && hunter == nil && w.bot().shopped != w.Depth {
		if dx, dy, ok := w.stepToward(p.X, p.Y, i%mapW, i/mapW); ok {
			return dx, dy
		}
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

// botToStairs steps toward the stairs, or toward the boss while they are sealed. When a monster that
// hasn't noticed the hero blocks the way, it goes for the nearest monster instead, to clear it.
func (w *World) botToStairs() (int, int) {
	i := strings.IndexByte(strings.Join(w.Level, ""), '>')
	if i < 0 {
		return w.botToward(w.nearest(func(m *Entity) bool { return m.Boss }))
	}
	if dx, dy, ok := w.stepToward(w.Player.X, w.Player.Y, i%mapW, i/mapW); ok {
		return dx, dy
	}
	if dx, dy := w.botToward(w.nearest(func(m *Entity) bool { return true })); dx != 0 || dy != 0 {
		return dx, dy
	}
	// Stranded on a dry tile among pools, where no monster can come either: step into a pit, water or
	// acid alongside, as a player would, but never lava.
	for _, d := range dirs {
		if c := w.Level[w.Player.Y+d[1]][w.Player.X+d[0]]; hazard(c) && c != '=' {
			return d[0], d[1]
		}
	}
	return 0, 0
}

// botState is what a bot keeps between turns of one run, and the tally it adds to for its kind of bot.
type botState struct {
	target  *Item  // the item the tactician is walking to
	shopped int    // the floor it last finished shopping on
	visited int    // the floor it last reached a merchant on
	drunk   string // the potion it drank last, to blame poison or confusion on
	*tally
}

// tally counts across all of one bot's runs.
type tally struct {
	potionsDrunk, goldSpent, waresBought, healsBought int
	shopVisits                                        map[int][]int // gold on reaching a merchant, by floor
}

// botStates holds each run's botState: the bots play at once, so this is no plain map.
var botStates sync.Map

// bot is the state of the bot playing w, made fresh for a run TestBalance didn't start.
func (w *World) bot() *botState {
	st, _ := botStates.LoadOrStore(w, &botState{tally: &tally{shopVisits: map[int][]int{}}})
	return st.(*botState)
}

// botShop has the tactician spend its gold beside a merchant: healing first, then the ware that improves
// its gear most, then known healing potions. It reports whether it bought anything; once it can't, it is
// done with this floor's shop.
func (w *World) botShop() bool {
	p := w.Player
	st := w.bot()
	if !w.nextToShop() || st.shopped == w.Depth {
		return false
	}
	if st.visited != w.Depth {
		st.visited = w.Depth
		st.shopVisits[w.Depth] = append(st.shopVisits[w.Depth], w.Gold)
	}
	gold := w.Gold
	switch best, gain := -1, 0; {
	case p.HP < p.MaxHP && w.HealPrice() <= w.Gold:
		w.Buy(buyHeal)
		st.healsBought++
	case len(w.Inventory) >= maxItems:
	default:
		for i, it := range w.Wares {
			if g := worth(it) - worth(w.wornIn(it.Slot)); it.Slot != "" && it.Slot != "ring" && g > gain && it.Price() <= w.Gold {
				best, gain = i, g
			}
			if best < 0 && w.known[it.ItemKind] && it.Name == "potion of healing" && it.Price() <= w.Gold {
				best = i
			}
		}
		if best >= 0 {
			w.Buy(best)
			st.waresBought++
		}
	}
	if w.Gold == gold {
		st.shopped = w.Depth
		return false
	}
	st.goldSpent += gold - w.Gold
	return true
}

// botItems has the tactician use its gear the way a careful player would. It reports whether it took
// the turn: drinking a potion when hurt (known healing first), picking up what it stands on, wearing
// anything better than what it has on, and reading unknown scrolls and tomes when nothing is hunting it.
func (w *World) botItems() bool {
	p := w.Player
	hunter := w.nearest(func(m *Entity) bool { return m.hunting })
	// Magic first: heal when hurt and bolt anything in line; when hurt in a fight, put the monsters to
	// sleep or blink away from the one alongside; with a bow, net a monster coming to fight hand to hand.
	hurt := p.HP*2 < p.MaxHP
	for i, s := range w.Spells {
		if w.Mana < w.cost(s) {
			continue
		}
		switch s.Name {
		case "heal":
			// Heal takes all the mana, so spend it when none of it would spill: in a fight once under
			// half HP, or as soon as nothing is hunting.
			if p.MaxHP-p.HP >= w.healing() && (hurt || hunter == nil) {
				w.Cast(i, 0, 0)
				return true
			}
		case "fire bolt", "lightning":
			for _, d := range dirs {
				if t := w.firstInLine(p, d[0], d[1], 6); t != nil && t != p {
					w.Cast(i, d[0], d[1])
					return true
				}
			}
		case "sleep":
			if hurt && hunter != nil && abs(hunter.X-p.X)+abs(hunter.Y-p.Y) <= 2 {
				w.Cast(i, 0, 0)
				return true
			}
		case "blink":
			if hurt && hunter != nil && abs(hunter.X-p.X)+abs(hunter.Y-p.Y) == 1 {
				w.Cast(i, sign(p.X-hunter.X), sign(p.Y-hunter.Y))
				return true
			}
		case "net":
			for _, d := range dirs {
				t := w.firstInLine(p, d[0], d[1], 6)
				if p.Range > 1 && t != nil && t != p && t.hunting && t.Range == 1 && t.Held == 0 && abs(t.X-p.X)+abs(t.Y-p.Y) >= 2 {
					w.Cast(i, d[0], d[1])
					return true
				}
			}
		}
	}
	safe := hunter == nil
	if safe && w.botShop() {
		return true
	}
	if p.HP*5 < p.MaxHP*2 {
		best := -1
		for i, it := range w.Inventory {
			if it.Class == 'p' && (best < 0 || w.known[it.ItemKind] && it.Name == "potion of healing") {
				if !w.known[it.ItemKind] || it.Name == "potion of healing" {
					best = i
				}
			}
		}
		if best >= 0 {
			w.bot().potionsDrunk++
			w.bot().drunk = w.Inventory[best].Name
			w.Use(best)
			return true
		}
	}
	if its := w.ItemsAt(p.X, p.Y); len(its) > 0 && len(w.Inventory) < maxItems {
		w.PickUp()
		return true
	}
	if !safe {
		return false
	}
	for i, it := range w.Inventory {
		if it.Slot != "" && !it.Worn && worth(it) > worth(w.wornIn(it.Slot)) {
			w.Use(i)
			return true
		}
		if it.Class == 's' && !w.known[it.ItemKind] || it.Class == 't' {
			w.Use(i)
			return true
		}
	}
	return false
}

// worth rates a piece of gear for the bot: everything it adds, roughly in HP-sized units.
func worth(it *Item) int {
	if it == nil {
		return 0
	}
	if it.Slot == "weapon" {
		return 2*(it.Dmg+it.Plus) + it.Atk + it.Def
	}
	return 2*(it.Atk+it.Def+it.Plus) + it.MaxHP/3
}

func (w *World) wornIn(slot string) *Item {
	for _, it := range w.Inventory {
		if it.Worn && it.Slot == slot {
			return it
		}
	}
	return nil
}
