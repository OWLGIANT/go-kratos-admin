package gcircularqueue_generic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCircular(t *testing.T) {

	cir := NewCircularQueueThreadSafe[int](10)
	for i := 0; i < 5; i++ {
		cir.PushKick(49200 + i)
	}
	require.Equal(t, 49200+1, cir.GetElement(1))
	require.Equal(t, 0, cir.GetElement(8))
	require.Equal(t, 0, cir.GetElement(9))
	require.Equal(t, 5, cir.Len())
	require.False(t, cir.IsFull())

	for i := 5; i < 11; i++ {
		cir.PushKick(49200 + i)
	}
	require.Equal(t, 49200+1, cir.GetElement(0))
	require.Equal(t, 0, cir.GetElement(10))
	require.Equal(t, 49200+10, cir.GetElement(9))
	require.True(t, cir.IsFull())

}
