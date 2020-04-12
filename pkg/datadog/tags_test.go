package datadog

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
				tagger.Upsert(entity, "1")
			},
			tags: []string{"1"},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tagger := NewTagger()
			tc.taggerInit(tagger)
			assert.Equal(t, tagger.GetStable(entity), tc.tags)
			tagger.Upsert(entity)
			tagger.Upsert(entity, "3", "2")
			assert.Equal(t, tagger.GetStable(entity), append(tc.tags, "2", "3"))
		})
	}
}
