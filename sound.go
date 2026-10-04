package main

import (
	"math"
	"math/rand/v2"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const sampleRate = 44100

var audioCtx *audio.Context // made in main, so tests need no sound device

// note is one part of a sound: a square wave sliding from one pitch to another, or noise when from is 0.
type note struct{ from, to, secs float64 }

// sounds are the game's effects as PCM, made at start in an 8-bit style, the art pack having no sounds.
var sounds = map[string][]byte{}

func init() {
	for name, notes := range map[string][]note{
		"hit":    {{0, 0, 0.04}, {300, 120, 0.06}},
		"hurt":   {{160, 60, 0.15}},
		"miss":   {{800, 1200, 0.04}},
		"kill":   {{0, 0, 0.05}, {400, 60, 0.2}},
		"die":    {{440, 220, 0.25}, {330, 165, 0.25}, {220, 55, 0.6}},
		"shoot":  {{1200, 400, 0.08}},
		"magic":  {{300, 900, 0.12}, {900, 1500, 0.1}},
		"heal":   {{523, 523, 0.08}, {659, 659, 0.08}, {784, 784, 0.12}},
		"stairs": {{392, 392, 0.1}, {330, 330, 0.1}, {262, 262, 0.1}, {196, 196, 0.2}},
		"level":  {{523, 523, 0.08}, {659, 659, 0.08}, {784, 784, 0.08}, {1047, 1047, 0.2}},
		"coin":   {{988, 988, 0.05}, {1319, 1319, 0.15}},
		"pickup": {{600, 900, 0.06}},
	} {
		sounds[name] = synth(notes)
	}
}

// synth renders notes one after another as 16-bit little-endian stereo, each fading out as it plays.
func synth(notes []note) []byte {
	var b []byte
	phase := 0.0
	for _, n := range notes {
		count := int(n.secs * sampleRate)
		for i := range count {
			t := float64(i) / float64(count)
			v := rand.Float64()*2 - 1
			if n.from != 0 {
				phase += (n.from + (n.to-n.from)*t) / sampleRate
				v = 1
				if math.Mod(phase, 1) >= 0.5 {
					v = -1
				}
			}
			s := int16(v * (1 - t) * 0.15 * math.MaxInt16)
			b = append(b, byte(s), byte(s>>8), byte(s), byte(s>>8))
		}
	}
	return b
}

// heard is what the hero had when the last turn's sounds played, to hear what changed since.
type heard struct {
	w                               *World
	depth, level, gold, items, mana int
	over                            bool
}

// playTurn plays the sounds of the turn just played: its hits and missiles, and what it changed.
// Each sound plays once a turn, however often it happened. It returns them, for tests.
func (g *Game) playTurn(hs []Hit) map[string]bool {
	w := g.world
	was := g.heard
	g.heard = heard{w, w.Depth, w.ExpLevel, w.Gold, len(w.Inventory), w.Mana, w.Over}
	if was.w != w { // a new or continued run: nothing to compare with yet
		was = g.heard
	}
	play := map[string]bool{
		"shoot":  len(w.Shots) > 0,
		"magic":  w.Mana < was.mana,
		"stairs": w.Depth > was.depth,
		"level":  w.ExpLevel > was.level,
		"coin":   w.Gold > was.gold,
		"pickup": len(w.Inventory) > was.items,
		"die":    w.Over && !was.over,
	}
	for _, h := range hs {
		switch {
		case h.Kind == 'm':
			play["miss"] = true
		case h.Kind == 'h':
			play["heal"] = true
		case h.Kind == 'p' || h.To == w.Player && h.To.HP <= 0: // poison ticks every turn; death has its own
		case h.To == w.Player:
			play["hurt"] = true
		case h.To.HP <= 0:
			play["kill"] = true
		default:
			play["hit"] = true
		}
	}
	for name, on := range play {
		if on && audioCtx != nil {
			audioCtx.NewPlayerFromBytes(sounds[name]).Play()
		}
	}
	return play
}
