package genx

import (
	"sort"

	"github.com/pkg/errors"
)

type Replacement struct {
	Start int
	End   int
	Text  string
}

type Replacements []*Replacement

func (rs Replacements) CheckNoOverlap() error {
	sort.Slice(rs, func(i, j int) bool {
		return rs[i].Start < rs[j].Start
	})

	for i := 1; i < len(rs); i++ {
		prev := rs[i-1]
		current := rs[i]
		if current.Start < prev.End {
			return errors.Errorf("overlap: %v and %v", prev, current)
		}
	}
	return nil
}

func (rs Replacements) Apply(text string) (string, error) {
	if err := rs.CheckNoOverlap(); err != nil {
		return "", err
	}

	sort.Slice(rs, func(i, j int) bool {
		return rs[i].Start > rs[j].Start
	})

	for _, rep := range rs {
		if rep.Start < 0 || rep.End > len(text) || rep.Start > rep.End {
			return "", errors.Errorf("invalid replacement: %v", rep)
		}
		text = text[:rep.Start] + rep.Text + text[rep.End:]
	}

	return text, nil
}
