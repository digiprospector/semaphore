package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	originalVer := Ver
	originalCommit := Commit
	originalDate := Date
	t.Cleanup(func() {
		Ver = originalVer
		Commit = originalCommit
		Date = originalDate
	})

	Ver = "2.18.1"
	Commit = "dac3cdc"
	Date = "1777928283"

	assert.Equal(t, "2.18.1-dac3cdc-1777928283-dp", Version())
}
