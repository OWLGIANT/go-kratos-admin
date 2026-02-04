package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLineFactors(t *testing.T) {
	// lf := LineFactors{
	// Zone: "aws-22",
	// Uid:  "wis",
	// }
	lf2 := LineFactors{
		Zone: "aws-22",
		Uid:  "wis",
	}
	lf3 := LineFactors{
		Zone: "aws-22",
		Uid:  "wis2",
	}
	lf4 := LineFactors{}

	require.Equal(t, 2, CalcWeight(lf2))
	require.Equal(t, 1, CalcWeight(lf3))
	require.Equal(t, 0, CalcWeight(lf4))
}
