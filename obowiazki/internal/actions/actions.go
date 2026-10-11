// Package actions koduje i dekoduje identyfikatory akcji powiadomień: CHORES_<RODZAJ>_<wystąpienie>_p<osoba>.
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

// Parse zwraca ok=false dla akcji, które nie należą do appki (np. z innych automatyzacji).
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
