package main

import (
	"fmt"
	"math"
	"sort"
)

type ListReconciler interface {
	SetInputs(left, right []int)
	ValidateInputs() error
	SortLists()
	ComputeDifferences()
	PrintDifferences()
}

type ListReconsilerImpl struct {
	LeftList  []int
	RightList []int
	Diffs     []int
	TotalDiff int
}

func (lr *ListReconsilerImpl) SetInputs(left, right []int) {
	lr.LeftList = left
	lr.RightList = right
}

func (lr *ListReconsilerImpl) SortLists() {
	sort.Ints(lr.LeftList)
	sort.Ints(lr.RightList)
}

func (lr *ListReconsilerImpl) ValidateInputs() error {
	if len(lr.LeftList) != len(lr.RightList) {
		return fmt.Errorf("error: left and right lists must have the same length")
	}
	return nil
}

func (lr *ListReconsilerImpl) ComputeDifferences() {
	lr.Diffs = make([]int, len(lr.LeftList))
	total := 0
	for i := 0; i < len(lr.LeftList); i++ {
		diff := int(math.Abs(float64(lr.LeftList[i] - lr.RightList[i])))
		lr.Diffs[i] = diff
		total += diff
	}
	lr.TotalDiff = total
}

func (lr *ListReconsilerImpl) DisplayResults() {
	for i := 0; i < len(lr.LeftList); i++ {
		fmt.Printf("%d %d %d\n", lr.LeftList[i], lr.RightList[i], lr.Diffs[i])
	}
	fmt.Printf("Total: %d\n", lr.TotalDiff)
}

func main() {
	lr := ListReconsilerImpl{}
	lr.SetInputs([]int{3, 4, 2, 1, 3, 3}, []int{4, 3, 5, 3, 9, 7})

	if err := lr.ValidateInputs(); err != nil {
		fmt.Println(err)
		return
	}

	lr.SortLists()
	lr.ComputeDifferences()
	lr.DisplayResults()
}

// quick and dirty working one
//package main

//import (
//"fmt"
//"math"
//"sort"
//)

//func main() {

//s1 := []int{3, 4, 2, 1, 3, 3}
//s2 := []int{4, 3, 5, 3, 9, 3}

//sort.Ints(s1)
//sort.Ints(s2)

//total := 0

//for i := 0; i < len(s1); i++ {
//diff := math.Abs(float64(s1[i] - s2[i]))
//total += int(diff)
//fmt.Printf("%d %d %d\n", s1[i], s2[i], int(diff))
//}

//fmt.Printf("Total: %d\n", total)
//}
