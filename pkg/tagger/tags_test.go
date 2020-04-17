package tagger

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewTagger(t *testing.T) {
	const entity = "host"
	for desc, tc := range map[string]struct {
		taggerInit func(tagger *Tagger)
		tags       []string
	}{
		"emptyInit": {
			taggerInit: func(tagger *Tagger) {
			},
			tags: []string{},
		},
		"oneInit": {
			taggerInit: func(tagger *Tagger) {
				tagger.Update(entity, NewTag("1", "1"))
			},
			tags: []string{"1:1"},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tagger := NewTagger()
			tc.taggerInit(tagger)
			assert.Equal(t, tc.tags, tagger.Get(entity))

			tagger.Update(entity)
			assert.Equal(t, tc.tags, tagger.Get(entity))

			tagger.Update(entity, NewTag("3", "3"), NewTag("2", "2"))
			assert.Equal(t, append(tc.tags, "2:2", "3:3"), tagger.Get(entity))

			tagger.Update(entity, NewTag("3", "changed"), NewTag("2", "2"))
			assert.Equal(t, append(tc.tags, "2:2", "3:changed"), tagger.Get(entity))

			tagger.Update(entity, NewTag("3", "3"), NewTag("2", "2"))
			assert.Equal(t, append(tc.tags, "2:2", "3:3"), tagger.Get(entity))

			assert.Equal(t, append(tc.tags, "2:2", "3:3"), tagger.GetWithDefault(entity, NewTag("3", "3")))
			assert.Equal(t, append(tc.tags, "2:2", "3:3"), tagger.GetWithDefault(entity, NewTag("3", "whatever")))

			assert.Equal(t, append(tc.tags, "2:2", "3:3", "4:4"), tagger.GetWithDefault(entity, NewTag("4", "4")))

			tagger.Add(entity, NewTag("3", "3+"))
			assert.Equal(t, append(tc.tags, "2:2", "3:3", "3:3+"), tagger.Get(entity))

			tagger.Update(entity, NewTag("3", "-"))
			assert.Equal(t, append(tc.tags, "2:2", "3:-"), tagger.Get(entity))

			tags, err := CreateTags("1:1")
			require.NoError(t, err)

			tagger.Replace(entity, tags...)
			assert.Equal(t, append([]string{}, "1:1"), tagger.Get(entity))

			_, err = CreateTags("1:")
			assert.Error(t, err)

			_, err = CreateTags("1")
			assert.Error(t, err)

			_, err = CreateTags("")
			assert.Error(t, err)

			assert.Equal(t, []string{"4:4"}, tagger.GetWithDefault("unknownEntity", NewTag("4", "4")))
		})
	}
}
