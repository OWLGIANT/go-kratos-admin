package cmap

import (
	"fmt"
	"testing"
	"time"
)

func TestIter(t *testing.T) {
	m := New[string]()
	m.Set("a289", "fiweo")
	m.Set("b289", "fiweo")

	for i := 0; i < 10000; i++ {
		m.Set(time.Now().GoString(), time.Now().GoString())
		time.Sleep(time.Millisecond)
		for _, key := range m.Keys() {
			_, ok := m.Get(key)
			if !ok {
				panic("not exist key. " + key)
			}
		}
	}
	fmt.Println("done")

}

func TestCopy(t *testing.T) {
	m1 := New[string]()
	m2 := m1
	m1.Set("bb", "cc")

	fmt.Println(m1.Keys())
	fmt.Println(m2.Keys())
	m1.Set("aa", "cc")
	fmt.Println("second")
	fmt.Println(m1.Keys())
	fmt.Println(m2.Keys())
}

func TestCmap(t *testing.T) {
	var c1 ConcurrentMap[string, int]
	// var c2 ConcurrentMap[string, int]
	c2 := c1
	if &c1 == &c2 {
		fmt.Println("same one")
	}
	if c1.Items() == nil {
		fmt.Println("not inited")
	}
	// c1.Set("fwoe", 2)
	// c2.Set("fwoe", 2)
}
