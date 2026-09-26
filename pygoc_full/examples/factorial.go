package main

import (
	"fmt"
)

func fact(n int) int {
	var t1 bool
	var t2 int
	var t3 int
	var t4 int

	t1 = n <= 1
	if t1 {
		goto B3
	} else {
		goto B4
	}
B3:
	return 1
B4:
	t2 = n - 1
	t3 = fact(t2)
	t4 = n * t3
	return t4
}

func main() {
	var t5 int

	t5 = fact(5)
	fmt.Println(t5)
	return
}
