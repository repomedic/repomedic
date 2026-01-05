package rules

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	registry = make(map[string]Rule)
	mu       sync.RWMutex
)

func Register(r Rule) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registry[r.ID()]; exists {
		panic(fmt.Sprintf("rule %s already registered", r.ID()))
	}
	// Wrap the rule with AllowListWrapper to provide automatic allowlist support
	registry[r.ID()] = &AllowListWrapper{Rule: r}
}

func List() []Rule {
	mu.RLock()
	defer mu.RUnlock()
	var rules []Rule
	for _, r := range registry {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].ID() < rules[j].ID()
	})
	return rules
}

func Resolve(selector string) ([]Rule, error) {
	mu.RLock()
	defer mu.RUnlock()

	if selector == "" {
		return List(), nil
	}

	// Simple comma-separated list for now
	// TODO: Implement groups, negation, etc.
	ids := strings.Split(selector, ",")
	var selected []Rule
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if r, ok := registry[id]; ok {
			selected = append(selected, r)
		} else {
			return nil, fmt.Errorf("rule not found: %s", id)
		}
	}
	return selected, nil
}
