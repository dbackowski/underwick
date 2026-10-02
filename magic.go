package main

import "slices"

// Class is a kind of hero to play: its stats before gear, what it starts with in hand and the spells
// it knows. Name doubles as the sprite prefix.
type Class struct {
	Kind
	About   string
	Start   []string // items it starts with, all in use
	Spells  []string
	Mana    int
	Stealth bool // sleeping monsters wake half as often
}

var classes = []*Class{
	{Kind: Kind{Name: "warrior", MaxHP: 20, Atk: 2, Def: 1, Dmg: 2, Moves: 1, Range: 1},
		Start: []string{"sword"}, About: "Tough, and good with a sword."},
	{Kind: Kind{Name: "archer", MaxHP: 16, Atk: 3, Dmg: 2, Moves: 1, Range: 1},
		Start: []string{"bow"}, About: "Shoots down straight lines with a bow."},
	{Kind: Kind{Name: "mage", MaxHP: 14, Atk: 1, Dmg: 1, Moves: 1, Range: 1},
		Start: []string{"staff"}, Spells: []string{"fire bolt"}, Mana: 15, About: "Frail, but casts Fire Bolt."},
	{Kind: Kind{Name: "thief", MaxHP: 16, Atk: 3, Def: 1, Dmg: 2, Moves: 1, Range: 1},
		Start: []string{"dagger", "boots"}, Stealth: true, About: "Quiet: sleepers wake half as often."},
	{Kind: Kind{Name: "cleric", MaxHP: 16, Def: 2, Dmg: 1, Moves: 1, Range: 1},
		Start: []string{"hammer"}, Spells: []string{"heal"}, Mana: 8, About: "Armoured, and casts Heal."},
}

// Spell is magic the hero can learn from a tome and cast for mana.
type Spell struct {
	Name  string
	Cost  int
	Aimed bool // cast in a direction
	cast  func(w *World, dx, dy int)
}

var spells = []*Spell{
	{Name: "fire bolt", Cost: 3, Aimed: true, cast: func(w *World, dx, dy int) {
		p := w.Player
		t := w.firstInLine(p, dx, dy, 6)
		if t == nil {
			w.say("The fire bolt flies off and fizzles.")
			return
		}
		w.Shots = append(w.Shots, Shot{p.X, p.Y, t, "proj_orange_ball"})
		w.say("The fire bolt hits the %s.", t.Name)
		w.damage(t, 2+w.rng.IntN(4)+w.ExpLevel/2)
	}},
	{Name: "heal", Cost: 6, cast: func(w *World, dx, dy int) {
		p := w.Player
		p.HP = min(p.MaxHP, p.HP+4+w.ExpLevel)
		p.Poison = 0
		w.say("You feel better.")
	}},
	{Name: "sleep", Cost: 5, cast: func(w *World, dx, dy int) { w.lull(8) }},
}

func spellNamed(name string) *Spell {
	return spells[slices.IndexFunc(spells, func(s *Spell) bool { return s.Name == name })]
}

// manaEvery is how many turns it takes to regain 1 mana.
const manaEvery = 5

// Cast casts a known spell, in direction dx, dy if it is aimed. It takes a turn if there was the mana.
func (w *World) Cast(i, dx, dy int) {
	w.record(Action{Do: 'c', I: i, X: dx, Y: dy})
	if i < 0 || i >= len(w.Spells) {
		return
	}
	s := w.Spells[i]
	if w.Mana < s.Cost {
		w.Log = []string{"You don't have the mana for that."}
		return
	}
	w.turn(func() {
		w.Mana -= s.Cost
		w.Player.Anim = "atk"
		if s.Aimed {
			w.Player.Dir = dirName(dx, dy)
		}
		s.cast(w, dx, dy)
	})
}

// learn is what reading a tome does: teaches its spell, and gives a hero without magic some mana.
func learn(name string) func(*World) {
	return func(w *World) {
		s := spellNamed(name)
		if slices.Contains(w.Spells, s) {
			w.say("You know this spell already.")
			return
		}
		w.Spells = append(w.Spells, s)
		if w.MaxMana == 0 {
			w.MaxMana, w.Mana = 6, 6
		}
		w.say("You learn to cast %s.", name)
	}
}

// lull puts every monster the hero can see within 6 tiles to sleep for some turns.
func (w *World) lull(turns int) {
	n := 0
	for _, m := range w.Monsters {
		if !m.Boss && w.Visible[m.Y][m.X] && abs(m.X-w.Player.X)+abs(m.Y-w.Player.Y) <= 6 {
			m.Sleep, m.hunting = turns, false
			w.say("The %s falls asleep.", m.Name)
			n++
		}
	}
	if n == 0 {
		w.say("You feel drowsy for a moment.")
	}
}
