package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWorkIDs(t *testing.T) {
	empty, err := parseWorkIDs("  ")
	require.NoError(t, err)
	assert.Nil(t, empty)

	ids, err := parseWorkIDs("3, 55 ,212591,")
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 55, 212591}, ids)

	_, err = parseWorkIDs("3,alice,55")
	require.Error(t, err, "a typo in the list must stop the run, not silently shorten it")
}
