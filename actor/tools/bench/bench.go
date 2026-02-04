package bench

import (
	"fmt"
	"time"
)

type Node struct {
	Name string
	Tsus int64
}

func (n *Node) String() string {
	return fmt.Sprintf("[%s,%d]", n.Name, n.Tsus)
}

type Row []Node

type Table []Row

var table Table

func init() {
	table = make(Table, 0, 10000)
}

// must call before AddOneNode
func StartOneRow(len int) {
	row := make([]Node, 0, len)
	table = append(table, row)
}

func AddOneNode(name string) {
	row := table[len(table)-1]
	row = append(row, Node{name, time.Now().UnixMicro()})
	table[len(table)-1] = row
}

func EndAllRecording(printType string) {
	fmt.Printf("\nbench record:")
	for i, row := range table {
		fmt.Printf("%d: ", i)
		startTs := row[0].Tsus
		lastTs := row[0].Tsus
		for _, node := range row {
			fmt.Printf(" [%s", node.Name)
			if printType == "offset" {
				fmt.Printf(",%d,%d] ", node.Tsus-startTs, node.Tsus-lastTs)
			} else {
				fmt.Printf(",%d] ", node.Tsus)
			}
			lastTs = node.Tsus
		}
	}
	fmt.Printf("\n\n")
	table = table[:0]
}
