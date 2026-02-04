package main

import (
	"fmt"
	"time"

	"github.com/templexxx/tsc"
)

func main() {
	for {
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		for i := 0; i < 5; i++ {
			v1 := tsc.UnixNano()
			v2 := time.Now().UnixNano()
			fmt.Printf("%s: tscns %d, timens %d, diff %d, ratio %.18f\n", nowStr, v1, v2, v1-v2, float64(v1)/float64(v2)-1)
		}
		tsc.Calibrate()
		time.Sleep(time.Minute * 5)
	}
}
