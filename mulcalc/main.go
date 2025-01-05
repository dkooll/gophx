package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

// Constants
const (
	mulPattern = `mul\((\d+),(\d+)\)`
)

// Custom errors
var (
	ErrEmptyInput = errors.New("input string is empty")
	ErrNoMatches  = errors.New("no valid multiplication expressions found")
)

// MulReconciler interface defines the contract for multiplication reconciliation
type MulReconciler interface {
	SetInput(input string) error
	Process() error
	GetResults() []MultiplicationResult
	GetTotal() int
}

// MultiplicationResult represents a single multiplication operation
type MultiplicationResult struct {
	X        int
	Y        int
	Product  int
	Original string
}

// MulReconcilerImpl implements the MulReconciler interface
type MulReconcilerImpl struct {
	input   string
	results []MultiplicationResult
	total   int
	regex   *regexp.Regexp
}

// NewMulReconciler creates a new instance of MulReconcilerImpl
func NewMulReconciler() *MulReconcilerImpl {
	return &MulReconcilerImpl{
		regex: regexp.MustCompile(mulPattern),
	}
}

// SetInput validates and sets the input string
func (mr *MulReconcilerImpl) SetInput(input string) error {
	if input == "" {
		return ErrEmptyInput
	}
	mr.input = input
	return nil
}

// Process handles the multiplication expressions
func (mr *MulReconcilerImpl) Process() error {
	matches := mr.regex.FindAllStringSubmatch(mr.input, -1)
	if len(matches) == 0 {
		return ErrNoMatches
	}

	mr.results = make([]MultiplicationResult, 0, len(matches))
	mr.total = 0

	for _, match := range matches {
		x, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("invalid first number: %w", err)
		}

		y, err := strconv.Atoi(match[2])
		if err != nil {
			return fmt.Errorf("invalid second number: %w", err)
		}

		product := x * y
		mr.total += product

		mr.results = append(mr.results, MultiplicationResult{
			X:        x,
			Y:        y,
			Product:  product,
			Original: match[0],
		})
	}

	return nil
}

// GetResults returns all multiplication results
func (mr *MulReconcilerImpl) GetResults() []MultiplicationResult {
	return mr.results
}

// GetTotal returns the sum of all multiplications
func (mr *MulReconcilerImpl) GetTotal() int {
	return mr.total
}

func main() {
	mr := NewMulReconciler()
	input := `xmul(2,4)%&mul[3,7]!@^do_not_mul(5,5)+mul(32,64]then(mul(11,8)mul(8,5))`

	if err := mr.SetInput(input); err != nil {
		fmt.Printf("Error setting input: %v\n", err)
		return
	}

	if err := mr.Process(); err != nil {
		fmt.Printf("Error processing input: %v\n", err)
		return
	}

	// Print individual results
	fmt.Println("Individual multiplication results:")
	for _, result := range mr.GetResults() {
		fmt.Printf("%s: %d × %d = %d\n",
			result.Original, result.X, result.Y, result.Product)
	}

	fmt.Printf("\nTotal sum: %d\n", mr.GetTotal())
}
