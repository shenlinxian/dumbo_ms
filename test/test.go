package main

import "fmt"

func main() {
	a := make([]int, 0)
	a = append(a, 2)
	fmt.Println(len(a))
}

func test() []byte {
	return nil
}
