# gophx

This repository is dedicated to experimenting with various small Go projects and refactoring existing ones.

It explores different techniques and implementations, focusing on improving code quality and efficiency.

Some projects may grow into ongoing initiatives, but the primary focus is on experimentation.

## Notes

For now, the approach is to define small interfaces with the methods you want to implement.

Next, create structs that store the data and implement these interfaces, ensuring a clear separation of concerns.

Encapsulate core logic within the methods to keep the implementation clean and modular.

Group related behaviors into distinct interfaces to maintain single responsibility.

When broader functionality is needed, combine small, focused interfaces by embedding them into a larger one.

Keep related data and its operations together in the same package to promote cohesion and discoverability.

## Basics

```go
package main

import (
	"fmt"
)

func main() {
	// Arrays & slices, keys are always integers (indices). Elements are stored in order and indexing is fast (O(1) time complexity.
	var array [3]int
	slice := []int{1, 2, 3}

	slice = append(slice, 4, 5)

	for i := 0; i < len(array); i++ {
		fmt.Println("array element", i, ":", array[i])
	}

	for index, value := range slice {
		fmt.Println("slice element", index, ":", value)
	}

	// Maps, keys can be any type (e.g., string, int). Elements are not stored in order.
	// Lookup time is fast on average (O(1)) but slower than slices.
	m := map[string]int{
		"one":   1,
		"two":   2,
		"three": 3,
	}

	m["four"] = 4
	m["two"] = 22
	delete(m, "one")

	// Check if a key exists in a map
	// faster lookup, no need to loop through all keys.
	// prevents unexpected zero values,some types (like *int) can return nil instead of 0.
	// more idiomatic
	value, exists := m["three"]
	if exists {
		fmt.Println("three", value)
	}

	// Nested Maps
	studentss := map[string]map[string]int{
		"alice": {
			"math":    90,
			"science": 85,
		},
		"bob": {
			"math":    70,
			"science": 92,
		},
	}

	// check if bob exists in outer map
	studentGrades, exists := studentss["bob"]
	if exists {
		// check if science exists in inner map
		grade, subject_exists := studentGrades["science"]
		if subject_exists {
			fmt.Println("Bob's science grade:", grade)
		} else {
			fmt.Println("Science grade not found for bob")
		}
	} else {
		fmt.Println("student not found")
	}

	// Iterating over Nested Maps:
	for s, studentGrades := range studentss {
		fmt.Println("Student", s)
		for subject, grade := range studentGrades {
			fmt.Println(subject, ":", grade)
		}
	}

	// Why Use Structs Instead of Nested Maps?
	// More readable – Student{Name, Grades} is clearer than map[string]map[string]int.
	// Easier to extend – Can add more fields (e.g., Age int).
	// More structured – Avoids missing fields or unintentional nil maps.
	type Student struct {
		Name   string
		Grades map[string]int
	}

	students := []Student{
		{
			Name:   "Alice",
			Grades: map[string]int{"math": 90, "science": 85},
		},
		{
			Name:   "Bob",
			Grades: map[string]int{"math": 70, "science": 92},
		},
		{
			Name:   "Charlie",
			Grades: map[string]int{"math": 85, "history": 78},
		},
	}

	// Iterating over nested maps
	for _, student := range students {
		fmt.Println(student.Name)
		for subject, grade := range student.Grades {
			fmt.Println(subject, ":", grade)
		}
	}

	// adding students dynamically
	var name string
	fmt.Print("What is your name:")
	fmt.Scan(&name)

	grade := map[string]int{}
	grade["english"] = 80
	grade["history"] = 75

	students = append(students, Student{Name: name, Grades: grade})

	fmt.Println("Updated student list:")
	for _, student := range students {
		fmt.Println(student.Name)
		for subject, grade := range student.Grades {
			fmt.Println(subject, ":", grade)
		}
	}
}
```
