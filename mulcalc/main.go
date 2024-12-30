package main

import (
	"fmt"
	"regexp"
	"strconv"
)

type MulReconsiler interface {
	SetInputs(input string)
	ValidateInputs() error
	PrintTotal() int
}

type MulReconsilerImpl struct {
	Input string
	Total int
}

func (mr *MulReconsilerImpl) SetInputs(input string) {
	mr.Input = input
}

func (mr *MulReconsilerImpl) ValidateInputs() error {
	r := regexp.MustCompile(`mul\((\d+),(\d+)\)`)
	matches := r.FindAllStringSubmatch(mr.Input, -1)

	total := 0
	for _, match := range matches {
		x, error := strconv.Atoi(match[1])
		if error != nil {
			return error
		}
		y, error := strconv.Atoi(match[2])
		if error != nil {
			return error
		}
		total += x * y
	}
	mr.Total = total
	return nil
}

func (mr *MulReconsilerImpl) PrintTotal() int {
	return mr.Total
}

func main() {
	mr := MulReconsilerImpl{}
	mr.SetInputs(`xmul(2,4)%&mul[3,7]!@^do_not_mul(5,5)+mul(32,64]then(mul(11,8)mul(8,5))`)
	err := mr.ValidateInputs()
	if err != nil {
		fmt.Println("Error validation inputs:", err)
		return
	}
	fmt.Println(mr.PrintTotal())
}
