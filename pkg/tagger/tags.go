package tagger

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	MissingTagValue = "unknown"
	keyValueJoin    = ":"
)

// Tag should be created with CreateTags/NewTag to be safe
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

func NewTagUnsafe(key, value string) *Tag {
	return &Tag{
		key:      tagKey(key),
		value:    tagValue(value),
		keyValue: key + keyValueJoin + value,
	}
}

func NewTag(key, value string) (*Tag, error) {
	if key == "" {
		return nil, errors.New("empty key")
	}
	if value == "" {
		return nil, errors.New("empty value")
	}
	return &Tag{
		key:      tagKey(key),
		value:    tagValue(value),
		keyValue: key + keyValueJoin + value,
	}, nil
}

func CreateTags(s ...string) ([]*Tag, error) {
	var tags []*Tag
	for _, t := range s {
		index := strings.Index(t, keyValueJoin)
		if index == -1 || index+1 >= len(t) {
			return tags, errors.New("invalid tag: " + t)
		}
		tags = append(tags, NewTagUnsafe(t[:index], t[index+1:]))
	}
	return tags, nil
}

type (
	tagKey   string
	tagValue string

	//             "key"      "value"  "key:value"
	entityStore map[tagKey]map[tagValue]string

	//          "host-a"
	tagStore map[string]entityStore
)

type Tagger struct {
	store tagStore

	mu *sync.RWMutex
}

func NewTagger() *Tagger {
	return &Tagger{
		store: make(tagStore),
		mu:    &sync.RWMutex{},
	}
}

func (t *Tagger) Add(entity string, tags ...*Tag) {
	t.mu.Lock()
	entityTags, hasEntity := t.store[entity]
	if !hasEntity {
		entityTags = make(entityStore, 1)
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
	t.store[entity] = entityTags
	t.mu.Unlock()
}

// Update any existing tag key regardless of the value
func (t *Tagger) Update(entity string, tags ...*Tag) {
	t.mu.Lock()
	entityTags, hasEntity := t.store[entity]
	if !hasEntity {
		entityTags = make(entityStore, 1)
	}
	for _, tag := range tags {
		entityTags[tag.key] = map[tagValue]string{
			tag.value: tag.keyValue,
		}
	}
	t.store[entity] = entityTags
	t.mu.Unlock()
}

func (t *Tagger) Replace(entity string, tags ...*Tag) {
	t.mu.Lock()
	entityTags := make(entityStore, 1)
	for _, tag := range tags {
		entityTags[tag.key] = map[tagValue]string{
			tag.value: tag.keyValue,
		}
	}
	t.store[entity] = entityTags
	t.mu.Unlock()
}

func (t *Tagger) Get(entity string) []string {
	tags := t.GetUnstable(entity)
	sort.Strings(tags)
	return tags
}

func (t *Tagger) GetWithDefault(entity string, defaultTags ...*Tag) []string {
	tags := t.GetUnstableWithDefault(entity, defaultTags...)
	sort.Strings(tags)
	return tags
}

func (t *Tagger) GetUnstableWithDefault(entity string, defaultTags ...*Tag) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tags := make([]string, 0)

	entityTags, ok := t.store[entity]
	if !ok {
		for _, t := range defaultTags {
			tags = append(tags, t.keyValue)
		}
		return tags
	}
	meetDefault := make(map[tagKey]*Tag)
	for _, t := range defaultTags {
		meetDefault[t.key] = t
	}
	for tagKey := range entityTags {
		for _, keyValue := range entityTags[tagKey] {
			_, ok := meetDefault[tagKey]
			if ok {
				delete(meetDefault, tagKey)
			}
			tags = append(tags, keyValue)
		}
	}
	for _, t := range meetDefault {
		tags = append(tags, t.keyValue)
	}
	return tags
}

func (t *Tagger) GetUnstable(entity string) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tags := make([]string, 0)

	entityTags, ok := t.store[entity]
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

	entityTags, ok := t.store[entity]
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

func (t *Tagger) Stats() (float64, float64, float64) {
	t.mu.RLock()

	entities, keys, tags := len(t.store), 0, 0
	for _, entityTags := range t.store {
		keys += len(entityTags)
		for tagKey := range entityTags {
			tags += len(entityTags[tagKey])
		}
	}
	t.mu.RUnlock()
	return float64(entities), float64(keys), float64(tags)
}

func (t *Tagger) Print() {
	t.mu.RLock()

	b := bytes.Buffer{}
	entities, tagNumber := 0, len(t.store)
	for entity := range t.store {
		// there is a read lock in Get
		tags := t.Get(entity)
		b.WriteString(fmt.Sprintf("  - %q: %q\n", entity, tags))
		tagNumber += len(tags)
	}
	t.mu.RUnlock()
	fmt.Printf("entities[%d] -> tags[%d]:\n%s", entities, tagNumber, b.String())
}
