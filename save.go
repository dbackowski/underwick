package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// dataDir holds the save and the high scores, in the user's config folder. Tests point it elsewhere.
var dataDir = func() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(d, "underwick")
}()

// saveVersion goes up whenever a change to the rules would replay an old save differently.
const saveVersion = 6

// Action is one thing the hero did, enough to do it again: a save is the run's seed, class and actions,
// replayed on load. The game draws all its chance from the seed, so a replay ends where the run left.
type Action struct {
	Do      byte // 'm' move or wait, 'g' pick up, 'u' use, 'd' drop, 'c' cast, 'b' buy
	I, X, Y int  // the item or spell, and the direction
}

type saved struct {
	Version int
	Seed    uint64
	Class   string
	Actions []Action
}

func savePath() string { return filepath.Join(dataDir, "save.json") }

// Save writes the run so it can be continued. A dead hero's run is not saved.
func (w *World) Save() error {
	if w.Over {
		return nil
	}
	data, err := json.Marshal(saved{saveVersion, w.seed, w.Class.Name, w.Actions})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(savePath(), data, 0o644)
}

// HasSave reports whether there is a run to continue.
func HasSave() bool {
	_, err := os.Stat(savePath())
	return err == nil
}

// Continue loads the saved run by replaying it, and deletes the save: a run can't be loaded twice, so
// death stays final. A save from an older version of the rules is thrown away.
func Continue() (*World, error) {
	data, err := os.ReadFile(savePath())
	if err != nil {
		return nil, err
	}
	DeleteSave()
	var s saved
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	i := slices.IndexFunc(classes, func(c *Class) bool { return c.Name == s.Class })
	if s.Version != saveVersion || i < 0 {
		return nil, errors.New("the save is from an older version of the game")
	}
	w := NewGame(s.Seed, classes[i])
	for _, a := range s.Actions {
		switch a.Do {
		case 'm':
			w.Step(a.X, a.Y)
		case 'g':
			w.PickUp()
		case 'u':
			w.Use(a.I)
		case 'd':
			w.Drop(a.I)
		case 'c':
			w.Cast(a.I, a.X, a.Y)
		case 'b':
			w.Buy(a.I)
		}
	}
	w.Log = []string{"Welcome back."}
	return w, nil
}

func DeleteSave() { os.Remove(savePath()) }

// Score is one finished run on the high-score list.
type Score struct {
	Points, Depth, Level int
	Class, Cause, Date   string
}

const maxScores = 10

func scoresPath() string { return filepath.Join(dataDir, "scores.json") }

// LoadScores reads the high-score list, best first. A missing or broken file is an empty list.
func LoadScores() []Score {
	var ss []Score
	if data, err := os.ReadFile(scoresPath()); err == nil {
		json.Unmarshal(data, &ss)
	}
	return ss
}

// RecordScore adds a finished run to the high-score list and returns its place, from 0, or -1 if it
// didn't make the list.
func (w *World) RecordScore() int {
	s := Score{w.Score(), w.Depth, w.ExpLevel, w.Class.Name, w.Cause, time.Now().Format("2006-01-02")}
	ss := append(LoadScores(), s)
	slices.SortStableFunc(ss, func(a, b Score) int { return cmp.Compare(b.Points, a.Points) })
	place := slices.Index(ss, s)
	ss = ss[:min(len(ss), maxScores)]
	if place >= maxScores {
		place = -1
	}
	if data, err := json.Marshal(ss); err == nil && os.MkdirAll(dataDir, 0o755) == nil {
		os.WriteFile(scoresPath(), data, 0o644)
	}
	return place
}
