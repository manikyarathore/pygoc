package main

import (
	"fmt"
)

func main() {
	var x int
	var t1 bool

	x = 0
	goto B2
B2:
	if true {
		goto B3
	} else {
		goto B4
	}
B3:
	x = x + 1
	t1 = x >= 5
	if t1 {
		goto B5
	} else {
		goto B6
	}
B4:
	fmt.Println(x)
	return
B5:
	goto B4
B6:
	goto B2
}
