package datadog

import (
	"log"
	"sort"
	"sync"
)

type Tagger struct {
	tags map[string]map[string]struct{}

	mu *sync.RWMutex
}

func NewTagger() *Tagger {
	return &Tagger{
		tags: make(map[string]map[string]struct{}),
		mu:   &sync.RWMutex{},
	}
}

func (t *Tagger) Upsert(entity string, tags ...string) {
	t.mu.Lock()
	entityTags, ok := t.tags[entity]
	if !ok {
		entityTags = make(map[string]struct{})
	}
	for _, tag := range tags {
		entityTags[tag] = struct{}{}
	}
	t.tags[entity] = entityTags
	t.mu.Unlock()
}

func (t *Tagger) Update(entity string, tags ...string) {
	t.mu.Lock()
	entityTags := make(map[string]struct{})
	for _, tag := range tags {
		entityTags[tag] = struct{}{}
	}
	t.tags[entity] = entityTags
	t.mu.Unlock()
}

func (t *Tagger) GetStable(entity string) []string {
	tags := t.Get(entity)
	sort.Strings(tags)
	return tags
}

func (t *Tagger) Get(entity string) []string {
	tags := make([]string, 0)

	t.mu.RLock()
	defer t.mu.RUnlock()

	entityTags, ok := t.tags[entity]
	if !ok {
		return tags
	}
	for tag := range entityTags {
		tags = append(tags, tag)
	}
	return tags
}

func (t *Tagger) Print() {
	t.mu.RLock()
	for entity := range t.tags {
		log.Printf("%s: %s", entity, t.GetStable(entity))
	}
	t.mu.RUnlock()
}
