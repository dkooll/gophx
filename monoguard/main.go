package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type ReportProcessor interface {
	SetInputs(string)
	ParseInputs()
	ValidateReports([]int) bool
	ProcessReport()
	PrintReport()
}

type ReportProcessorImpl struct {
	RawInputs string
	Reports   [][]int
}

func (rp *ReportProcessorImpl) SetInputs(inputs string) {
	rp.RawInputs = inputs
}

func (rp *ReportProcessorImpl) ParseInputs() {
	var reports [][]int

	lines := strings.Split(rp.RawInputs, "\n")
	for _, line := range lines {
		s := strings.Fields(line)
		var tempslice []int
		for _, v := range s {
			d, err := strconv.Atoi(v)
			if err != nil {
				fmt.Println(err)
			}
			tempslice = append(tempslice, d)
		}
		reports = append(reports, tempslice)
	}
	rp.Reports = reports
}

func (rp *ReportProcessorImpl) ValidateReports(report []int) bool {
	isIncreasing := true
	isDecreasing := true

	for i := 0; i < len(report)-1; i++ {
		diff := int(math.Abs(float64(report[i+1] - report[i])))

		// Check if the difference is outside the valid range
		if diff < 1 || diff > 3 {
			return false
		}

		// Update monotonicity flags
		if report[i] > report[i+1] {
			isIncreasing = false
		}
		if report[i] < report[i+1] {
			isDecreasing = false
		}

		// If neither increasing nor decreasing, report is invalid
		if !isIncreasing && !isDecreasing {
			return false
		}
	}

	// Report is valid if it has a consistent trend
	return true
}

func (rp *ReportProcessorImpl) ProcessReport() {
	for _, report := range rp.Reports {
		if rp.ValidateReports(report) {
			fmt.Println(report, "Safe")
		} else {
			fmt.Println(report, "Unsafe")
		}
	}
}

func (rp *ReportProcessorImpl) PrintReport() {
	for _, report := range rp.Reports {
		fmt.Println(report)
	}
}

func main() {
	rp := ReportProcessorImpl{}

	input := `7 6 4 2 1
1 2 7 8 9
9 7 6 2 1
1 3 2 4 5
8 6 4 4 1
1 3 6 7 9`

	rp.SetInputs(input)
	rp.ParseInputs()
	rp.ProcessReport()
}
