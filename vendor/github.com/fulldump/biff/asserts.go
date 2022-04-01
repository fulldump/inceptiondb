package biff

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
)

var exit = func() {
	os.Exit(1)
}

// deprecated
// AssertNotEqual return true if `obtained` is not equal to `expected` otherwise
// it will print trace and exit.
func (a *A) AssertNotEqual(obtained, expected interface{}) bool {
	if !reflect.DeepEqual(expected, obtained) {
		l, r := printShould(expected)
		fmt.Printf("    %s is not equal %s\n", l, r)
		return true
	}

	printExpectedObtained(expected, obtained)

	return false
}

// AssertNotEqual return true if `obtained` is not equal to `expected` otherwise
// it will print trace and exit.
func AssertNotEqual(obtained, expected interface{}) bool {
	if !reflect.DeepEqual(expected, obtained) {
		l, r := printShould(expected)
		fmt.Printf("    %s is not equal %s\n", l, r)
		return true
	}

	printExpectedObtained(expected, obtained)

	return false
}

// deprecated
// AssertEqual return true if `obtained` is equal to `expected` otherwise it
// will print trace and exit.
func (a *A) AssertEqual(obtained, expected interface{}) bool {
	if reflect.DeepEqual(expected, obtained) {
		l, r := printShould(expected)
		fmt.Printf("    %s is %s\n", l, r)
		return true
	}

	printExpectedObtained(expected, obtained)

	return false
}

// AssertEqual return true if `obtained` is equal to `expected` otherwise it
// will print trace and exit.
func AssertEqual(obtained, expected interface{}) bool {

	if reflect.DeepEqual(expected, obtained) {
		l, r := printShould(expected)
		fmt.Printf("    %s is %s\n", l, r)
		return true
	}

	printExpectedObtained(expected, obtained)

	return false
}

func readFileLine(filename string, line int) string {

	data, _ := ioutil.ReadFile(filename)
	lines := strings.Split(string(data), "\n")

	return lines[line-1]
}

// deprecated
// AssertEqualJson return true if `obtained` is equal to `expected`. Prior to
// comparison, both values are JSON Marshaled/Unmarshaled to avoid JSON type
// issues like int vs float etc. Otherwise it will print trace and exit.
func (a *A) AssertEqualJson(obtained, expected interface{}) bool {
	e := interface{}(nil)
	{
		b, _ := json.Marshal(expected)
		json.Unmarshal(b, &e)
	}

	o := interface{}(nil)
	{
		b, _ := json.Marshal(obtained)
		json.Unmarshal(b, &o)
	}

	if reflect.DeepEqual(e, o) {
		l, r := printShould(expected)
		fmt.Printf("    %s is same JSON as %s\n", l, r)
		return true
	}

	printExpectedObtained(e, o)

	return false
}

// AssertEqualJson return true if `obtained` is equal to `expected`. Prior to
// comparison, both values are JSON Marshaled/Unmarshaled to avoid JSON type
// issues like int vs float etc. Otherwise it will print trace and exit.
func AssertEqualJson(obtained, expected interface{}) bool {

	e := interface{}(nil)
	{
		b, _ := json.Marshal(expected)
		json.Unmarshal(b, &e)
	}

	o := interface{}(nil)
	{
		b, _ := json.Marshal(obtained)
		json.Unmarshal(b, &o)
	}

	if reflect.DeepEqual(e, o) {
		l, r := printShould(expected)
		fmt.Printf("    %s is same JSON as %s\n", l, r)
		return true
	}

	printExpectedObtained(e, o)

	return false
}

// deprecated
// AssertNil return true if `obtained` is nil, otherwise it will print trace and
// exit.
func (a *A) AssertNil(obtained interface{}) bool {

	if nil == obtained || reflect.ValueOf(obtained).IsNil() {
		l, _ := printShould(nil)
		fmt.Printf("    %s is nil\n", l)
		return true
	}

	printExpectedObtained(nil, obtained)

	return false
}

// AssertNil return true if `obtained` is nil, otherwise it will print trace and
// exit.
func AssertNil(obtained interface{}) bool {

	if nil == obtained || reflect.ValueOf(obtained).IsNil() {
		l, _ := printShould(nil)
		fmt.Printf("    %s is nil\n", l)
		return true
	}

	printExpectedObtained(nil, obtained)

	return false
}

// deprecated
// AssertNotNil return true if `obtained` is NOT nil, otherwise it will print trace
// and exit.
func (a *A) AssertNotNil(obtained interface{}) bool {

	if isNil(obtained) {
		line := getStackLine(2)
		fmt.Printf(""+
			"    Expected: not nil\n"+
			"    Obtained: %#v\n"+
			"    at %s\n", obtained, line)
		exit()
		return false
	}

	l, _ := printShould(nil)
	v := fmt.Sprintf("%#v", obtained)
	if v != l {
		v = " (" + v + ")"
	}
	fmt.Printf("    %s is not nil%s\n", l, v)

	return true
}

// AssertNotNil return true if `obtained` is NOT nil, otherwise it will print trace
// and exit.
func AssertNotNil(obtained interface{}) bool {

	if isNil(obtained) {
		line := getStackLine(2)
		fmt.Printf(""+
			"    Expected: not nil\n"+
			"    Obtained: %#v\n"+
			"    at %s\n", obtained, line)
		exit()
		return false
	}

	l, _ := printShould(nil)
	v := fmt.Sprintf("%#v", obtained)
	if v != l {
		v = " (" + v + ")"
	}
	fmt.Printf("    %s is not nil%s\n", l, v)

	return true
}

// deprecated
// AssertTrue return true if `obtained` is true, otherwise it will print trace
// and exit.
func (a *A) AssertTrue(obtained interface{}) bool {

	if reflect.DeepEqual(true, obtained) {
		l, _ := printShould(nil)
		fmt.Printf("    %s is true\n", l)
		return true
	}

	printExpectedObtained(true, obtained)

	return false
}

// AssertTrue return true if `obtained` is true, otherwise it will print trace
// and exit.
func AssertTrue(obtained interface{}) bool {

	if reflect.DeepEqual(true, obtained) {
		l, _ := printShould(nil)
		fmt.Printf("    %s is true\n", l)
		return true
	}

	printExpectedObtained(true, obtained)

	return false
}

// deprecated
// AssertFalse return true if `obtained` is false, otherwise it will print trace
// and exit.
func (a *A) AssertFalse(obtained interface{}) bool {

	if reflect.DeepEqual(false, obtained) {
		l, _ := printShould(nil)
		fmt.Printf("    %s is false\n", l)
		return true
	}

	printExpectedObtained(true, obtained)

	return false
}

// AssertFalse return true if `obtained` is false, otherwise it will print trace
// and exit.
func AssertFalse(obtained interface{}) bool {

	if reflect.DeepEqual(false, obtained) {
		l, _ := printShould(nil)
		fmt.Printf("    %s is false\n", l)
		return true
	}

	printExpectedObtained(true, obtained)

	return false
}

// deprecated
// AssertInArray return true if `item` match at least with one element of the
// array. Otherwise it will print trace and exit.
func (a *A) AssertInArray(array interface{}, item interface{}) bool {

	v := reflect.ValueOf(array)
	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		line := getStackLine(2)
		fmt.Printf("Expected second argument to be array:\n"+
			"    Obtained: %#v\n"+
			"    at %s\n", array, line)
		exit()
	}

	l := v.Len()
	for i := 0; i < l; i++ {
		e := v.Index(i)
		if reflect.DeepEqual(e.Interface(), item) {
			l, r := printShould(item)
			fmt.Printf("    %s[%d] is %s\n", l, i, r)
			return true
		}
	}

	line := getStackLine(2)
	fmt.Printf(""+
		"    Expected item to be in array.\n"+
		"    Item: %#v\n"+
		"    Array: %#v\n"+
		"    at %s\n", item, array, line)

	exit()

	return false
}

// AssertInArray return true if `item` match at least with one element of the
// array. Otherwise it will print trace and exit.
func AssertInArray(array interface{}, item interface{}) bool {

	v := reflect.ValueOf(array)
	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		line := getStackLine(2)
		fmt.Printf("Expected second argument to be array:\n"+
			"    Obtained: %#v\n"+
			"    at %s\n", array, line)
		exit()
	}

	l := v.Len()
	for i := 0; i < l; i++ {
		e := v.Index(i)
		if reflect.DeepEqual(e.Interface(), item) {
			l, r := printShould(item)
			fmt.Printf("    %s[%d] is %s\n", l, i, r)
			return true
		}
	}

	line := getStackLine(2)
	fmt.Printf(""+
		"    Expected item to be in array.\n"+
		"    Item: %#v\n"+
		"    Array: %#v\n"+
		"    at %s\n", item, array, line)

	exit()

	return false
}

func getStackLine(linesToSkip int) string {

	stack := debug.Stack()
	lines := make([]string, 0)
	index := 0
	for i := 0; i < len(stack); i++ {
		if stack[i] == []byte("\n")[0] {
			lines = append(lines, string(stack[index:i-1]))
			index = i + 1
		}
	}
	return lines[linesToSkip*2+3] + " " + lines[linesToSkip*2+4]
}

func printExpectedObtained(expected, obtained interface{}) {

	line := getStackLine(3)
	fmt.Printf(""+
		"    Expected: %#v\n"+
		"    Obtained: %#v\n"+
		"    at %s\n", expected, obtained, line)

	exit()
}

func printShould(value interface{}) (arg0, arg1 string) {
	arg0 = "It"
	arg1 = fmt.Sprintf("%#v", value)

	func() {

		p := make([]runtime.StackRecord, 50)

		_, ok := runtime.GoroutineProfile(p)
		if !ok {
			return
		}

		frames := runtime.CallersFrames(p[0].Stack())

		// Make it compatible with latests golang versions (1.14 on)
		frame, more := frames.Next()
		for ; more; frame, more = frames.Next() {
			if frame.Function == "github.com/fulldump/biff.printShould" {
				break
			}
		}

		frame, _ = frames.Next()
		frame, _ = frames.Next()

		l := readFileLine(frame.File, frame.Line)

		a, err := parser.ParseExpr(l)
		if nil != err {
			return
		}

		aFunc, ok := a.(*ast.CallExpr)
		if !ok {
			return
		}

		a0 := aFunc.Args[0]
		arg0 = l[a0.Pos()-1 : a0.End()-1]

		if len(aFunc.Args) > 1 {
			a1 := aFunc.Args[1]
			arg1 = l[a1.Pos()-1 : a1.End()-1]
		}

	}()

	v := fmt.Sprintf("%#v", value)

	if v != arg1 {
		arg1 = arg1 + " (" + v + ")"
	}

	return
}

// Source: https://sourcegraph.com/github.com/stretchr/testify/-/blob/assert/assertions.go#L520:6
// isNil checks if a specified object is nil or not, without Failing.
func isNil(object interface{}) bool {
	if object == nil {
		return true
	}

	value := reflect.ValueOf(object)
	kind := value.Kind()
	isNilableKind := containsKind(
		[]reflect.Kind{
			reflect.Chan, reflect.Func,
			reflect.Interface, reflect.Map,
			reflect.Ptr, reflect.Slice},
		kind)

	if isNilableKind && value.IsNil() {
		return true
	}

	return false
}

// source: github.com/stretchr/testify/-/blob/assert/assertions.go#L524
// containsKind checks if a specified kind in the slice of kinds.
func containsKind(kinds []reflect.Kind, kind reflect.Kind) bool {
	for i := 0; i < len(kinds); i++ {
		if kind == kinds[i] {
			return true
		}
	}

	return false
}
