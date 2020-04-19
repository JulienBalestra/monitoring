package tagger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	entity   = "some-host"
	noEntity = "another-host"
)

func TestNewTagger(t *testing.T) {
	for desc, tc := range map[string]struct {
		taggerInit func(tagger *Tagger)
		exp        []string
	}{
		"emptyInit": {
			taggerInit: func(tagger *Tagger) {
			},
			exp: []string{},
		},
		"oneInit": {
			taggerInit: func(tagger *Tagger) {
				tagger.Add(entity, NewTagUnsafe("1", "1"))
			},
			exp: []string{"1:1"},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tagger := NewTagger()
			tc.taggerInit(tagger)
			assert.Equal(t, tc.exp, tagger.Get(entity))
		})
	}
}

func TestGetWithDefault(t *testing.T) {
	for desc, tc := range map[string]struct {
		tags        []*Tag
		getWithtags []*Tag

		exp         []string
		expNoEntity []string
	}{
		"empty": {
			[]*Tag{},
			[]*Tag{},
			[]string{},
			[]string{},
		},
		"1:1": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
			},
			[]*Tag{
				NewTagUnsafe("1", "2"),
			},
			[]string{
				"1:1",
			},
			[]string{
				"1:2",
			},
		},
		"1:1 2:2": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
			},
			[]*Tag{
				NewTagUnsafe("2", "2"),
			},
			[]string{
				"1:1",
				"2:2",
			},
			[]string{
				"2:2",
			},
		},
		"1:1 2:2 2:2": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
			},
			[]*Tag{
				NewTagUnsafe("2", "2"),
				NewTagUnsafe("2", "2"),
			},
			[]string{
				"1:1",
				"2:2",
			},
			[]string{
				"2:2",
				"2:2",
			},
		},
		"1:1 2:2 3:3": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
			},
			[]*Tag{
				NewTagUnsafe("2", "2"),
				NewTagUnsafe("3", "3"),
			},
			[]string{
				"1:1",
				"2:2",
				"3:3",
			},
			[]string{
				"2:2",
				"3:3",
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tagger := NewTagger()
			tagger.Add(entity, tc.tags...)

			result := tagger.GetWithDefault(entity, tc.getWithtags...)
			assert.Equal(t, tc.exp, result)

			result = tagger.GetWithDefault(noEntity, tc.getWithtags...)
			assert.Equal(t, tc.expNoEntity, result)
		})
	}
}

func TestGetIndexed(t *testing.T) {
	for desc, tc := range map[string]struct {
		tags []*Tag

		exp map[string]struct{}
	}{
		"empty": {
			[]*Tag{},
			map[string]struct{}{},
		},
		"1:1": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
			},
			map[string]struct{}{
				"1:1": {},
			},
		},
		"1:1 2:2": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
				NewTagUnsafe("1", "2"),
				NewTagUnsafe("2", "2"),
			},
			map[string]struct{}{
				"1:1": {},
				"1:2": {},
				"2:2": {},
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tagger := NewTagger()
			tagger.Add(entity, tc.tags...)

			result := tagger.GetIndexed(entity)
			assert.Equal(t, tc.exp, result)

			result = tagger.GetIndexed(noEntity)
			assert.Equal(t, map[string]struct{}{}, result)
		})
	}
}

func TestUpdate(t *testing.T) {
	for desc, tc := range map[string]struct {
		tags []*Tag
		exp  []string
	}{
		"empty": {
			[]*Tag{},
			[]string{},
		},
		"1:1": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
			},
			[]string{
				"1:1",
			},
		},
		"1:1 1:2 2:2": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
				NewTagUnsafe("1", "2"),
				NewTagUnsafe("2", "2"),
			},
			[]string{
				"1:2",
				"2:2",
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tagger := NewTagger()
			tagger.Update(entity, tc.tags...)
			result := tagger.Get(entity)
			assert.Equal(t, tc.exp, result)
			tagger.Update(entity)
			assert.Equal(t, tc.exp, result)
			tagger.Update(entity, []*Tag{}...)
			assert.Equal(t, tc.exp, result)
		})
	}
}

func TestReplace(t *testing.T) {
	for desc, tc := range map[string]struct {
		tags []*Tag
		exp  []string
	}{
		"empty": {
			[]*Tag{},
			[]string{},
		},
		"1:1": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
			},
			[]string{
				"1:1",
			},
		},
		"1:1 1:2 2:2": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
				NewTagUnsafe("1", "2"),
				NewTagUnsafe("2", "2"),
			},
			[]string{
				"1:2",
				"2:2",
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tagger := NewTagger()
			tagger.Replace(entity, tc.tags...)
			result := tagger.Get(entity)
			assert.Equal(t, tc.exp, result)

			tagger.Replace(entity)
			result = tagger.Get(entity)
			assert.Equal(t, []string{}, result)

			tagger.Replace(entity, []*Tag{}...)
			result = tagger.Get(entity)
			assert.Equal(t, []string{}, result)

			tagger.Replace(entity, tc.tags...)
			result = tagger.Get(entity)
			assert.Equal(t, tc.exp, result)
		})
	}
}

func TestAdd(t *testing.T) {
	for desc, tc := range map[string]struct {
		tags []*Tag
		exp  []string
	}{
		"empty": {
			[]*Tag{},
			[]string{},
		},
		"1:1": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
			},
			[]string{
				"1:1",
			},
		},
		"1:1 1:2 2:2": {
			[]*Tag{
				NewTagUnsafe("1", "1"),
				NewTagUnsafe("1", "2"),
				NewTagUnsafe("2", "2"),
			},
			[]string{
				"1:1",
				"1:2",
				"2:2",
			},
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tagger := NewTagger()
			tagger.Add(entity, tc.tags...)
			result := tagger.Get(entity)
			assert.Equal(t, tc.exp, result)

			tagger.Add(entity)
			result = tagger.Get(entity)
			assert.Equal(t, tc.exp, result)

			tagger.Add(entity, []*Tag{}...)
			result = tagger.Get(entity)
			assert.Equal(t, tc.exp, result)

			tagger.Add(entity, tc.tags...)
			result = tagger.Get(entity)
			assert.Equal(t, tc.exp, result)
		})
	}
}

func TestCreateTags(t *testing.T) {
	for desc, tc := range map[string]struct {
		tagStrings []string
		expTags    []*Tag
		hasError   bool
	}{
		"error missing value": {
			[]string{"1:"},
			[]*Tag{},
			true,
		},
		"error empty": {
			[]string{""},
			[]*Tag{},
			true,
		},
		"error 2nd empty": {
			[]string{"1:1", ""},
			[]*Tag{},
			true,
		},
		"error 1st empty": {
			[]string{"", "1:1"},
			[]*Tag{},
			true,
		},
		"1:1": {
			[]string{"1:1"},
			[]*Tag{
				{
					key:      "1",
					value:    "1",
					keyValue: "1:1",
				},
			},
			false,
		},
		"1:1 * 2": {
			[]string{"1:1", "1:1"},
			[]*Tag{
				{
					key:      "1",
					value:    "1",
					keyValue: "1:1",
				},
				{
					key:      "1",
					value:    "1",
					keyValue: "1:1",
				},
			},
			false,
		},
		"1:1 2:2": {
			[]string{"1:1", "2:2"},
			[]*Tag{
				{
					key:      "1",
					value:    "1",
					keyValue: "1:1",
				},
				{
					key:      "2",
					value:    "2",
					keyValue: "2:2",
				},
			},
			false,
		},
		"1:1 1:2": {
			[]string{"1:1", "1:2"},
			[]*Tag{
				{
					key:      "1",
					value:    "1",
					keyValue: "1:1",
				},
				{
					key:      "1",
					value:    "2",
					keyValue: "1:2",
				},
			},
			false,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tags, err := CreateTags(tc.tagStrings...)
			if tc.hasError {
				assert.Error(t, err)
				return
			}
			assert.Equal(t, tc.expTags, tags)
		})
	}
}

func TestNewTag(t *testing.T) {
	for desc, tc := range map[string]struct {
		key   string
		value string

		exTag    *Tag
		hasError bool
	}{
		"1:1": {
			"1",
			"1",
			&Tag{
				"1",
				"1",
				"1:1",
			},
			false,
		},
		"1:": {
			"1",
			"",
			nil,
			true,
		},
		":1": {
			"",
			"1",
			nil,
			true,
		},
		":": {
			"",
			"",
			nil,
			true,
		},
	} {
		t.Run(desc, func(t *testing.T) {
			tag, err := NewTag(tc.key, tc.value)
			if tc.hasError {
				assert.Error(t, err)
			}
			assert.Equal(t, tc.exTag, tag)
		})
	}
}
