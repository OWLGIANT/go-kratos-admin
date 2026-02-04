package base

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPackage(t *testing.T) {
	fmt.Println(GetExPackageName(3))
}

func TestRegexpMatch(t *testing.T) {
	{
		matched, err := regexp.MatchString("^aws.*", "aws-cn-1")
		fmt.Println(matched, err)
		require.True(t, matched)
		require.Nil(t, err)
	}
	{
		matched, err := regexp.MatchString("aws-", "aws-xxx")
		require.True(t, matched)
		require.Nil(t, err)
	}
	{
		matched, err := regexp.MatchString("aws-xxx", "aws-xxx")
		require.True(t, matched)
		require.Nil(t, err)
	}
	{
		matched, err := regexp.MatchString("aws-*", "aws-xxx")
		require.True(t, matched)
		require.Nil(t, err)
	}
	{
		matched, err := regexp.MatchString("aws-*x*", "aws-xxx")
		require.True(t, matched)
		require.Nil(t, err)
	}
	{
		matched, err := regexp.MatchString("aws*x*", "aws-xxx")
		require.True(t, matched)
		require.Nil(t, err)
	}
	{
		matched, err := regexp.MatchString("aws-*yyy", "aws-xxx")
		require.False(t, matched)
		require.Nil(t, err)
	}
	{
		matched, err := regexp.MatchString(".*aws.*", "aws-xxx")
		require.True(t, matched)
		require.Nil(t, err)
	}
	{
		matched, err := regexp.MatchString(".*aws*", "aws-xxx")
		require.Nil(t, err)
		require.True(t, matched)
	}
	{
		matched, err := regexp.MatchString("*aws*", "")
		fmt.Println("wrong regexp", err)
		require.NotNil(t, err)
		require.False(t, matched)
	}
}
