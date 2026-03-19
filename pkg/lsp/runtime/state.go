package runtime

import (
	"fmt"
	"strings"
)

type PossibleValue struct {
	Value      string
	SourceLine int
	SourceType string
}

func (pv PossibleValue) String() string {
	return fmt.Sprintf("%s@%d: %s", pv.SourceType, pv.SourceLine, pv.Value)
}

type VarState struct {
	Name   string
	Values []PossibleValue
}

func (v *VarState) AddValue(pv PossibleValue) {
	v.Values = append(v.Values, pv)
}

func (v *VarState) LastValue() *PossibleValue {
	if len(v.Values) == 0 {
		return nil
	}
	return &v.Values[len(v.Values)-1]
}

func (v *VarState) UniqueValues() []string {
	seen := make(map[string]bool)
	var result []string
	for _, pv := range v.Values {
		if !seen[pv.Value] {
			seen[pv.Value] = true
			result = append(result, pv.Value)
		}
	}
	return result
}

type State struct {
	variables        map[string]*VarState
	forVars          map[string]string
	delayedExpansion bool
	scopeDepth       int
}

func NewState() *State {
	return &State{
		variables: make(map[string]*VarState),
		forVars:   make(map[string]string),
	}
}

func (s *State) SetForVar(name, value string) {
	s.forVars[strings.ToUpper(name)] = value
}

func (s *State) GetForVar(name string) (string, bool) {
	val, ok := s.forVars[strings.ToUpper(name)]
	return val, ok
}

func (s *State) Fork() *State {
	newState := &State{
		variables:        make(map[string]*VarState),
		forVars:          make(map[string]string),
		delayedExpansion: s.delayedExpansion,
		scopeDepth:       s.scopeDepth,
	}
	for name, v := range s.variables {
		newV := &VarState{
			Name:   v.Name,
			Values: make([]PossibleValue, len(v.Values)),
		}
		copy(newV.Values, v.Values)
		newState.variables[name] = newV
	}
	for name, val := range s.forVars {
		newState.forVars[name] = val
	}
	return newState
}

func (s *State) Merge(other *State) *State {
	if other == nil {
		return s.Fork()
	}

	merged := s.Fork()

    for name, v := range other.variables {
        existing := merged.variables[name]
        if existing == nil {
            newV := &VarState{
                Name:   v.Name,
                Values: make([]PossibleValue, len(v.Values)),
            }
            copy(newV.Values, v.Values)
            merged.variables[name] = newV
        } else {
            existingVals := make(map[string]bool)
            for _, pv := range existing.Values {
                key := fmt.Sprintf("%s:%d", pv.Value, pv.SourceLine)
                existingVals[key] = true
            }

            for _, pv := range v.Values {
                key := fmt.Sprintf("%s:%d", pv.Value, pv.SourceLine)
                if !existingVals[key] {
                    existing.Values = append(existing.Values, pv)
                }
            }
        }
    }
    }

    return merged
}

	merged := s.Fork()

	for name, v := range other.variables {
		existing := merged.variables[name]
		if existing == nil {
			newV := &VarState{
				Name:   v.Name,
				Values: make([]PossibleValue, len(v.Values)),
			}
			copy(newV.Values, v.Values)
			merged.variables[name] = newV
		} else {
			existingVals := make(map[string]bool)
			for _, pv := range existing.Values {
				key := fmt.Sprintf("%s:%d", pv.Value, pv.SourceLine)
				existingVals[key] = true
			}
			for _, pv := range v.Values {
				key := fmt.Sprintf("%s:%d", pv.Value, pv.SourceLine)
				if !existingVals[key] {
					existing.Values = append(existing.Values, pv)
				}
			}
		}
	}

	return merged
}

func (s *State) SetVar(name string, value PossibleValue) {
	upperName := strings.ToUpper(name)
	v := s.variables[upperName]
	if v == nil {
		v = &VarState{Name: upperName}
		s.variables[upperName] = v
	}
	v.Values = []PossibleValue{value}
}

func (s *State) GetVariable(name string) *VarState {
	return s.variables[strings.ToUpper(name)]
}

func (s *State) DelayedExpansion() bool {
	return s.delayedExpansion
}

func (s *State) SetDelayedExpansion(enabled bool) {
	s.delayedExpansion = enabled
}

func (s *State) ScopeDepth() int {
	return s.scopeDepth
}

func (s *State) SetScopeDepth(depth int) {
	s.scopeDepth = depth
}
