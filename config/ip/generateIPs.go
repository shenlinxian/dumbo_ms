package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	// 从命令行获取参数 n
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run main.go <n>")
		return
	}

	// 转换命令行参数为整数
	n, err := strconv.Atoi(os.Args[1])
	if err != nil || n <= 0 {
		fmt.Println("Invalid value for n. Please provide a positive integer.")
		return
	}

	//creat dir
	for i := 0; i < n; i++ {
		err := os.MkdirAll(fmt.Sprintf("%s%d", "ip", i+1), 0755)
		if err != nil {
			fmt.Println("Error creating directory:", err)
			return
		}
	}

	baseIP := "127.0.0.1:"
	basePort := 6000
	tmpport := basePort
	DrtIPs := make([]string, n)
	for i := 0; i < n; i++ {
		tmpport++
		DrtIPs[i] = fmt.Sprintf("%s%d", baseIP, tmpport)
	}

	//write direction ips
	file, err := os.Create("drtips.txt")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	for i := 0; i < n; i++ {
		_, err := file.WriteString(DrtIPs[i] + "\n")
		if err != nil {
			panic(err)
		}
	}

	file.Close()

	SrcIPS := make([][]string, n)
	for i := 0; i < n; i++ {
		SrcIPS[i] = make([]string, n)
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			tmpport++
			tmpIP := fmt.Sprintf("%s%d", baseIP, tmpport)
			SrcIPS[i][j] = tmpIP
		}
	}

	fmt.Println(SrcIPS)

	for i := 0; i < n; i++ {
		file, err := os.Create(fmt.Sprintf("%s%d/myips.txt", "ip", i+1))
		if err != nil {
			fmt.Println("Error creating file:", err)
			return
		}
		for j := 0; j < n; j++ {
			_, err := file.WriteString(SrcIPS[i][j] + "\n")
			if err != nil {
				panic(err)
			}
		}
		file.Close()

	}

	for j := 0; j < n; j++ {
		file, err := os.Create(fmt.Sprintf("%s%d/otherips.txt", "ip", j+1))
		if err != nil {
			fmt.Println("Error creating file:", err)
			return
		}
		for i := 0; i < n; i++ {
			_, err := file.WriteString(SrcIPS[i][j] + "\n")
			if err != nil {
				panic(err)
			}
		}
		file.Close()
	}

}
