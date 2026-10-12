// Package actions encodes and decodes notification action IDs: CHORES_<KIND>_<occurrence>_p<person>.
package actions

import (
	"fmt"
	"strconv"
	"strings"
)

type Kind string

const (
	Done   Kind = "DONE"
	Snooze Kind = "SNOOZE"
	Me     Kind = "ME"
)

const prefix = "CHORES_"

type Action struct {
	Kind         Kind
	OccurrenceID int64
	PersonID     int64
}

func (a Action) String() string {
	return fmt.Sprintf("%s%s_%d_p%d", prefix, a.Kind, a.OccurrenceID, a.PersonID)
}

// Parse returns ok=false for actions that do not belong to this app (e.g. from other automations).
func Parse(s string) (Action, bool) {
	rest, found := strings.CutPrefix(s, prefix)
	if !found {
		return Action{}, false
	}
	parts := strings.Split(rest, "_")
	if len(parts) != 3 {
		return Action{}, false
	}
	kind := Kind(parts[0])
	switch kind {
	case Done, Snooze, Me:
	default:
		return Action{}, false
	}
	occ, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || occ < 0 {
		return Action{}, false
	}
	p, found := strings.CutPrefix(parts[2], "p")
	if !found {
		return Action{}, false
	}
	person, err := strconv.ParseInt(p, 10, 64)
	if err != nil || person < 0 {
		return Action{}, false
	}
	return Action{Kind: kind, OccurrenceID: occ, PersonID: person}, true
}
