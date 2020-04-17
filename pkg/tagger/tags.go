package tagger

import (
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
)

const (
	MissingTagValue = "unknown"
	keyValueJoin    = ":"
)

type Tag struct {
	key      tagKey
	value    tagValue
	keyValue string
}

func (t *Tag) String() string {
	return t.keyValue
}

/*

Add "entity" -> "keyOne" -> "valueOne" -> "keyOne:valueOne"
Add "entity" -> "keyOne" -> "valueTwo" -> "keyOne:valueTwo"
with the same entity/key we have the following tags "keyOne:valueOne", "keyOne:valueTwo"

Update "entity" -> "keyOne" -> "valueThree" -> "keyOne:valueThree"
with the same entity/key we have the updated following tags "keyOne:valueThree"
*/

func NewTag(key, value string) *Tag {
	return &Tag{
		key:      tagKey(key),
		value:    tagValue(value),
		keyValue: key + keyValueJoin + value,
	}
}

func CreateTags(s ...string) ([]*Tag, error) {
	var tags []*Tag
	for _, t := range s {
		index := strings.Index(t, keyValueJoin)
		if index == -1 || index+1 >= len(t) {
			return tags, errors.New("invalid tag: " + t)
		}
		tags = append(tags, NewTag(t[:index], t[index+1:]))
	}
	return tags, nil
}

type tagKey string
type tagValue string
type entityStore map[tagKey]map[tagValue]string
type tagStore map[string]entityStore

type Tagger struct {
	tags tagStore

	mu *sync.RWMutex
}

func NewTagger() *Tagger {
	return &Tagger{
		tags: make(tagStore),
		mu:   &sync.RWMutex{},
	}
}

func (t *Tagger) Add(entity string, tags ...*Tag) {
	t.mu.Lock()
	entityTags, hasEntity := t.tags[entity]
	if !hasEntity {
		entityTags = make(entityStore)
	}
	for _, tag := range tags {
		if len(entityTags[tag.key]) > 0 {
			entityTags[tag.key][tag.value] = tag.keyValue
			continue
		}
		entityTags[tag.key] = map[tagValue]string{
			tag.value: tag.keyValue,
		}
	}
	t.tags[entity] = entityTags
	t.mu.Unlock()
}

func (t *Tagger) Update(entity string, tags ...*Tag) {
	t.mu.Lock()
	entityTags, hasEntity := t.tags[entity]
	if !hasEntity {
		entityTags = make(entityStore)
	}
	for _, tag := range tags {
		entityTags[tag.key] = map[tagValue]string{
			tag.value: tag.keyValue,
		}
	}
	t.tags[entity] = entityTags
	t.mu.Unlock()
}

func (t *Tagger) Replace(entity string, tags ...*Tag) {
	t.mu.Lock()
	entityTags := make(entityStore)
	for _, tag := range tags {
		entityTags[tag.key] = map[tagValue]string{
			tag.value: tag.keyValue,
		}
	}
	t.tags[entity] = entityTags
	t.mu.Unlock()
}

func (t *Tagger) Get(entity string) []string {
	tags := t.GetUnstable(entity)
	sort.Strings(tags)
	return tags
}

func (t *Tagger) GetWithDefault(entity string, tag *Tag) []string {
	tags := t.GetUnstableWithDefault(entity, tag)
	sort.Strings(tags)
	return tags
}

func (t *Tagger) GetUnstableWithDefault(entity string, defaultTag *Tag) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tags := make([]string, 0)

	entityTags, ok := t.tags[entity]
	if !ok {
		return tags
	}
	hasDefaultKey := false
	for tagKey := range entityTags {
		for _, keyValue := range entityTags[tagKey] {
			if tagKey == defaultTag.key {
				hasDefaultKey = true
			}
			tags = append(tags, keyValue)
		}
	}
	if !hasDefaultKey {
		tags = append(tags, defaultTag.keyValue)
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
	for tagKey := range entityTags {
		for _, keyValue := range entityTags[tagKey] {
			tags = append(tags, keyValue)
		}
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
	for tagKey := range entityTags {
		for _, keyValue := range entityTags[tagKey] {
			tags[keyValue] = struct{}{}
		}
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
