package main

import (
	"fmt"
)

func main() {
	var total int
	var i int
	var t1 int
	var t2 bool
	var t3 int

	total = 0
	i = 0
	t1 = 5
	goto B2
B2:
	t2 = i < t1
	if t2 {
		goto B3
	} else {
		goto B5
	}
B3:
	total = total + i
	goto B4
B4:
	t3 = i + 1
	i = t3
	goto B2
B5:
	fmt.Println(total)
	return
}
