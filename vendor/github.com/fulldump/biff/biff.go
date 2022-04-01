// Package biff provides support for nested testing, useful for complex business
// logic, APIs, and stateful systems.
//
// Typical usage:
//     biff.Alternative("Initial value", func(a *A) {
//         value := 10
//         a.AssertEqual(value, 10)
//
//         a.Alternative("Plus 50", func(a *A) {
//             // Here value == 10
//             value += 50
//             a.AssertEqual(value, 60)
//         })
//
//         a.Alternative("Multiply by 2", func(a *A) {
//             // Here value == 10 again (it is an alternative from the parent)
//             value *= 2
//             a.AssertEqual(value, 20)
//         })
//     })
//
// Will produce this output:
//     Case: Initial value
//         value is 10
//     Case: Plus 50
//         value is 60
//     -------------------------------
//     Case: Initial value
//         value is 10
//     Case: Multiply by 2
//         value is 20
//     -------------------------------
//     Case: Initial value
//         value is 10
//     -------------------------------
//
// Other example:
//     func TestMyTest(t *testing.T) {
//         Alternative("Login", func(a *A) {
//             user := mySystem.Login("user@email.com", "123456")
//             a.AssertEqual(user.Email, "user@email.com")
//             a.Alternative("Do action 1", func(a *A) {
//                 // Do something
//             })
//             a.Alternative("Do action 2", func(a *A) {
//                 // Do something else
//             })
//             ...
//         }
//     }
//
// If some assertion fails, it will print expected value and `file:line` to get
// direct to the line, like this:
//
//    Expected: []string{"test a", "test a22"}
//    Obtained: []string{"test a", "test a2"}
//    at myservice.Test_isolation.func1.2(0xc420094a80 /.../project/item_test.go:21 +0x18
//
package biff

import (
	"fmt"
	"strings"
)

// An A is a type passed to alternative functions to manage recursion status and
// keep human information information related to the test: `title` and
// `description`.
type A struct {
	skip      int
	f         F
	substatus *[]int
	done      bool

	// Title is the human readable short description for a test case.
	Title string

	// Description is an optional human detailed description for a test case,
	// needs to be filled inside alternative function.
	Description string
}

func newTest(f F) *A {
	return &A{
		f: f,
	}
}

// Alternative describes a new alternative case inside current case. It will be
// executed in a isolated branch.
func (a *A) Alternative(title string, f F) *A {

	if a.skip == 0 {
		n := newTest(f)
		n.Title = title
		a.done = n.run(a.substatus)
	}

	a.skip--

	return a
}

func (a *A) run(status *[]int) (done bool) {

	fmt.Println("Case:", a.Title)

	skip := &(*status)[0]
	substatus := (*status)[1:]

	// Execute
	a.skip = *skip
	a.substatus = &substatus
	a.f(a)

	// There is no more alternatives
	if a.skip == 0 {
		printDescription(a.Description)
		(*status)[0] = 0
		return true
	}

	if a.done {
		(*status)[0]++
	}

	return
}

func trimMultiline(s string) (r string) {

	if s == "" {
		return
	}

	for _, line := range strings.Split(s, "\n") {
		r += strings.TrimSpace(line) + "\n"
	}

	return
}

func printDescription(s string) {
	fmt.Print(trimMultiline(s))
}

// F is a callback alternative function passed to an `Alternative` with testing
// code.
type F func(a *A)

// Alternative is the root use case and the test runner. It will execute all
// test alternatives defined inside.
func Alternative(title string, f F) {

	// Ñap :_(
	status := []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	for {

		t := newTest(f)
		t.Title = title

		done := t.run(&status)

		fmt.Println("-------------------------------")

		if done {
			return
		}

	}

}
