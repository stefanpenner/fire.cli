package firewalla

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRules(t *testing.T) {
	rules := parseRules(fixture(t, "rules.txt"))
	require.Len(t, rules, 3)

	// Sorted newest-first by timestamp: policy:2 (900), policy:3 (600), policy:1 (300).
	assert.Equal(t, "2", rules[0].ID)
	assert.Equal(t, "block", rules[0].Action) // action omitted → defaults to block
	assert.Equal(t, "games", rules[0].Target)
	assert.Equal(t, "intf:adf9e2ed-d9f6-40f3-81cc-4da924d9f15d", rules[0].Scope) // unwrapped from JSON array
	assert.True(t, rules[0].Disabled)

	assert.Equal(t, "3", rules[1].ID)
	assert.Equal(t, "allow", rules[1].Action)
	assert.Equal(t, int64(301686), rules[1].HitCount)
	assert.Equal(t, time.Unix(1700009999, 0).UTC(), rules[1].LastHit)

	assert.Equal(t, "1", rules[2].ID)
	assert.Equal(t, "dns", rules[2].Type)
	assert.Equal(t, "bidirection", rules[2].Direction)
	assert.False(t, rules[2].Disabled)
	assert.Equal(t, int64(4210), rules[2].HitCount)
	assert.Equal(t, time.Unix(1700000300, 500000000).UTC(), rules[2].Created)
}

func TestParseRules_Empty(t *testing.T) {
	assert.Nil(t, parseRules(""))
}
