package tagger

import (
	"log"
	"sort"
	"strings"
	"sync"
)

const (
	MissingTagValue = "unknown"
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

func (t *Tagger) Get(entity string) []string {
	tags := t.GetUnstable(entity)
	sort.Strings(tags)
	return tags
}

func (t *Tagger) GetWithDefault(entity string, tagKey, tagValue string) []string {
	tags := t.GetUnstableWithDefault(entity, tagKey, tagValue)
	sort.Strings(tags)
	return tags
}

func (t *Tagger) GetUnstableWithDefault(entity string, tagKey, tagValue string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tags := make([]string, 0)

	entityTags, ok := t.tags[entity]
	if !ok {
		return tags
	}
	withTag := false
	for tag := range entityTags {
		index := strings.Index(tag, ":")
		if index != -1 && tag[:index] == tagKey {
			withTag = true
		}
		tags = append(tags, tag)
	}
	if !withTag {
		tags = append(tags, tagKey+":"+tagValue)
	}
	return tags
}

func (t *Tagger) GetUnstable(entity string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tags := make([]string, 0)

	entityTags, ok := t.tags[entity]
	if !ok {
		return tags
	}
	for tag := range entityTags {
		tags = append(tags, tag)
	}
	return tags
}

func (t *Tagger) GetIndexed(entity string) map[string]struct{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tags := make(map[string]struct{})

	entityTags, ok := t.tags[entity]
	if !ok {
		return tags
	}
	for tag, s := range entityTags {
		tags[tag] = s
	}
	return tags
}

func (t *Tagger) Print() {
	t.mu.RLock()
	tags := make([]string, 0, len(t.tags))

	for entity := range t.tags {
		tags = append(tags, entity)
	}
	t.mu.RUnlock()
	for _, entity := range tags {
		log.Printf("%s: %s", entity, t.Get(entity))
	}
}
