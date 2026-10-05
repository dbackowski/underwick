package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// TestBalance plays many seeded runs with bots and logs how deep they got: the diver as a warrior, and
// the tactician as every class. It is a tuning
// aid, not a pass/fail check, so it only runs on request:
//
//	UNDERWICK_SIM=1 go test -run Balance -v
func TestBalance(t *testing.T) {
	if os.Getenv("UNDERWICK_SIM") == "" {
		t.Skip("set UNDERWICK_SIM=1 to run the balance simulation")
	}
	const runs = 300
	type bot struct {
		name  string
		class *Class
		play  func(*World) (int, int)
	}
	bots := []bot{{"diver", classes[0], diver}}
	for _, c := range classes {
		bots = append(bots, bot{"tact " + c.Name, c, tactician})
	}
	for _, b := range bots {
		var gear int
		potionsDrunk, goldSpent, waresBought, healsBought = 0, 0, 0, 0
		var depths []int
		var score, turns, stuck int
		deaths := make([]int, 16) // by depth; the last bucket counts everything deeper
		for seed := range uint64(runs) {
			w := NewGame(seed, b.class)
			n := 0
			for ; !w.Over && n < 20000; n++ {
				if b.name != "diver" && w.botItems() {
					continue // an item action used the turn
				}
				w.Step(b.play(w))
			}
			for _, it := range w.Inventory {
				if it.Worn && it.Slot != "weapon" {
					gear++
				}
			}

			if !w.Over {
				stuck++
			}
			depths = append(depths, w.Depth)
			score += w.Score()
			turns += n
			deaths[min(w.Depth, len(deaths))-1]++
		}
		slices.Sort(depths)
		t.Logf("%-14s stuck %d  depth median %d, best %d  score avg %d  turns/run %d  potions/run %.1f  armour worn at death %.1f  deaths by depth %v",
			b.name, stuck, depths[runs/2], depths[runs-1], score/runs, turns/runs, float64(potionsDrunk)/runs, float64(gear)/runs, deaths)
		if b.name != "diver" {
			var visits []string
			for d := range 30 {
				if gs := shopVisits[d]; len(gs) > 0 {
					slices.Sort(gs)
					visits = append(visits, fmt.Sprintf("floor %d: %d runs, median %d gold", d, len(gs), gs[len(gs)/2]))
				}
			}
			clear(shopVisits)
			t.Logf("%-14s shops: gold spent/run %d  wares/run %.1f  heals/run %.1f  on arrival %s", "", goldSpent/runs, float64(waresBought)/runs, float64(healsBought)/runs, strings.Join(visits, "; "))
		}
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
// and, knowing Heal, rests for the mana to cast it when hurt and nothing is hunting it.
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
	sealed := !strings.Contains(strings.Join(w.Level, ""), ">")
	healer := slices.ContainsFunc(w.Spells, func(s *Spell) bool { return s.Name == "heal" })
	if hunter == nil && healer && (p.HP*3 < p.MaxHP*2 || sealed && p.HP < p.MaxHP) {
		return 0, 0 // rest for the mana to heal, and before a boss heal to full
	}
	if hunter == nil && len(w.Inventory) < maxItems {
		// Keep going for the item it set out for, even once it drops out of sight; otherwise pick
		// the first one in view.
		it := botTarget[w]
		if !slices.Contains(w.Floor, it) {
			it = nil
			for _, f := range w.Floor {
				if w.Visible[f.Y][f.X] && w.monsterAt(f.X, f.Y) == nil {
					it = f
					break
				}
			}
		}
		botTarget[w] = it
		if it != nil {
			if dx, dy, ok := w.stepToward(p.X, p.Y, it.X, it.Y); ok {
				return dx, dy
			}
			delete(botTarget, w) // can't get there; forget it
		}
	}
	// With the floor's items gathered, visit the merchant on the way to the stairs; botItems shops.
	if i := strings.IndexByte(strings.Join(w.Level, ""), '$'); i >= 0 && hunter == nil && botShopped[w] != w.Depth {
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
	return w.botToward(w.nearest(func(m *Entity) bool { return true }))
}

var (
	potionsDrunk int                  // counted across a bot's runs
	botTarget    = map[*World]*Item{} // the item the tactician is walking to
	botShopped   = map[*World]int{}   // the floor the tactician last finished shopping on
	botVisited   = map[*World]int{}   // the floor the tactician last reached a merchant on

	goldSpent, waresBought, healsBought int               // counted across a bot's runs
	shopVisits                          = map[int][]int{} // gold on reaching a merchant, by floor
)

// botShop has the tactician spend its gold beside a merchant: healing first, then the ware that improves
// its gear most, then known healing potions. It reports whether it bought anything; once it can't, it is
// done with this floor's shop.
func (w *World) botShop() bool {
	p := w.Player
	if !w.nextToShop() || botShopped[w] == w.Depth {
		return false
	}
	if botVisited[w] != w.Depth {
		botVisited[w] = w.Depth
		shopVisits[w.Depth] = append(shopVisits[w.Depth], w.Gold)
	}
	gold := w.Gold
	switch best, gain := -1, 0; {
	case p.HP < p.MaxHP && w.HealPrice() <= w.Gold:
		w.Buy(buyHeal)
		healsBought++
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
			waresBought++
		}
	}
	if w.Gold == gold {
		botShopped[w] = w.Depth
		return false
	}
	goldSpent += gold - w.Gold
	return true
}

// botItems has the tactician use its gear the way a careful player would. It reports whether it took
// the turn: drinking a potion when hurt (known healing first), picking up what it stands on, wearing
// anything better than what it has on, and reading unknown scrolls when nothing is hunting it.
func (w *World) botItems() bool {
	p := w.Player
	// Magic first: heal when hurt, and bolt anything in line.
	for i, s := range w.Spells {
		if w.Mana < s.Cost {
			continue
		}
		if s.Name == "heal" && p.HP*2 < p.MaxHP {
			w.Cast(i, 0, 0)
			return true
		}
		if s.Name == "fire bolt" {
			for _, d := range dirs {
				if t := w.firstInLine(p, d[0], d[1], 6); t != nil && t != p {
					w.Cast(i, d[0], d[1])
					return true
				}
			}
		}
	}
	safe := w.nearest(func(m *Entity) bool { return m.hunting }) == nil
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
			potionsDrunk++
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
		if it.Class == 's' && !w.known[it.ItemKind] {
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
