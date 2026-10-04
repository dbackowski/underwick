package main

import (
	"bytes"
	"embed"
	"io"
	"log"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

const sampleRate = 44100

// The effects and music, all CC0 by Juhani Junkala: the effects from The Essential Retro Video Game Sound
// Effects Collection, the music from Chiptune Adventures and the Retro Game Music Pack. An effect may
// have numbered variants (hit1.wav, hit2.wav), one picked at random each time it plays.
//
//go:embed audio
var audioFiles embed.FS

var audioCtx *audio.Context // made in main, so tests need no sound device

// sounds are the effects by name, decoded by loadSounds so playing one costs nothing.
var sounds = map[string][][]byte{}

func loadSounds() {
	entries, err := audioFiles.ReadDir("audio")
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".wav")
		if !ok {
			continue
		}
		b, err := audioFiles.ReadFile("audio/" + e.Name())
		if err != nil {
			log.Fatal(err)
		}
		s, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(b))
		if err != nil {
			log.Fatalf("%s: %v", e.Name(), err)
		}
		pcm, err := io.ReadAll(s)
		if err != nil {
			log.Fatalf("%s: %v", e.Name(), err)
		}
		name = strings.TrimRight(name, "0123456789")
		sounds[name] = append(sounds[name], pcm)
	}
}

// areaTracks take turns floor by floor.
var areaTracks = []string{"area1", "area2", "area3", "area4", "area5"}

// track is the music for what is on screen: the title's, the ending once the hero is dead, the boss
// track while a boss lives on the floor, or else the floor's own.
func (g *Game) track() string {
	w := g.world
	switch {
	case w == nil || g.mode == "title" || g.mode == "class":
		return "title"
	case w.Over:
		return "ending"
	case slices.ContainsFunc(w.Monsters, func(m *Entity) bool { return m.Boss }):
		return "boss"
	}
	return areaTracks[(w.Depth-1)%len(areaTracks)]
}

// playMusic starts the track for what is on screen, looping, if it isn't playing already.
func (g *Game) playMusic() {
	name := g.track()
	if audioCtx == nil || name == g.tune {
		return
	}
	if g.music != nil {
		g.music.Close()
	}
	g.tune, g.music = name, nil
	b, err := audioFiles.ReadFile("audio/" + name + ".ogg")
	if err != nil {
		log.Println("music:", err)
		return
	}
	s, err := vorbis.DecodeWithSampleRate(sampleRate, bytes.NewReader(b))
	if err != nil {
		log.Println("music:", err)
		return
	}
	g.music, err = audioCtx.NewPlayer(audio.NewInfiniteLoop(s, s.Length()))
	if err != nil {
		log.Println("music:", err)
		return
	}
	g.music.SetVolume(0.5) // under the effects
	g.music.Play()
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
		if vs := sounds[name]; on && audioCtx != nil && len(vs) > 0 {
			audioCtx.NewPlayerFromBytes(vs[rand.IntN(len(vs))]).Play()
		}
	}
	return play
}
