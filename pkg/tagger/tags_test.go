package tagger

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGet(t *testing.T) {
	const entity = "host"
	for desc, tc := range map[string]struct {
		taggerInit func(tagger *Tagger)
		tags       []string
	}{
		"empty": {
			taggerInit: func(tagger *Tagger) {
			},
			tags: []string{},
		},
		"one": {
			taggerInit: func(tagger *Tagger) {
				tagger.Upsert(entity, "1:1")
			},
			tags: []string{"1:1"},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tagger := NewTagger()
			tc.taggerInit(tagger)
			assert.Equal(t, tagger.Get(entity), tc.tags)
			tagger.Upsert(entity)
			tagger.Upsert(entity, "3:3", "2:2")
			assert.Equal(t, tagger.Get(entity), append(tc.tags, "2:2", "3:3"))
			assert.Equal(t, tagger.GetWithDefault(entity, "3", "3"), append(tc.tags, "2:2", "3:3"))
			assert.Equal(t, tagger.GetWithDefault(entity, "3", "whatever"), append(tc.tags, "2:2", "3:3"))
			assert.Equal(t, tagger.GetWithDefault(entity, "4", "4"), append(tc.tags, "2:2", "3:3", "4:4"))
		})
	}
}
