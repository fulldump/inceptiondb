package apicollectionv1

import (
	"bytes"
	"math"
	"strconv"

	"github.com/buger/jsonparser"

	"github.com/fulldump/inceptiondb/simdscan"
)

// equalityFilter is a compiled filter of top-level fields compared by
// equality with a string, number, boolean or null. It is evaluated on the raw
// JSON without decoding the document.
//
// It matches like connor (the general matcher): a field that is an array
// matches when any of its elements (at any depth) is equal to the value.
type equalityFilter []equalityCond

type equalityCond struct {
	key  string
	kind simdscan.Type
	str  string
	num  float64
	// intRaw is the JSON literal of num when it is an integer that float64
	// represents exactly, so plain integer literals compare as bytes.
	intRaw []byte
	b      bool
}

func compileEqualityFilter(filter map[string]interface{}) equalityFilter {
	conds := make(equalityFilter, 0, len(filter))
	for k, v := range filter {
		c := equalityCond{key: k}
		switch exp := v.(type) {
		case string:
			c.kind, c.str = simdscan.TypeString, exp
		case float64:
			c.kind, c.num = simdscan.TypeNumber, exp
			if exp == math.Trunc(exp) && math.Abs(exp) < 1<<53 {
				c.intRaw = strconv.AppendInt(nil, int64(exp), 10)
			}
		case bool:
			c.kind, c.b = simdscan.TypeBoolean, exp
		case nil:
			c.kind = simdscan.TypeNull
		}
		conds = append(conds, c)
	}
	return conds
}

func (f equalityFilter) match(payload []byte) bool {
	for i := range f {
		c := &f[i]
		val, t, err := simdscan.GetField(payload, c.key)
		if err != nil {
			if c.kind == simdscan.TypeNull {
				continue // a missing field matches null
			}
			return false
		}
		if !c.equal(val, t) {
			return false
		}
	}
	return true
}

// equal reports whether a JSON value (val of type t) is equal to the value of
// the condition, or is an array with an element that is.
func (c *equalityCond) equal(val []byte, t simdscan.Type) bool {
	if t == simdscan.TypeArray {
		return c.anyElement(val)
	}
	if t != c.kind {
		return false
	}

	switch c.kind {
	case simdscan.TypeString:
		if bytes.IndexByte(val, '\\') < 0 {
			return string(val) == c.str
		}
		s, err := jsonparser.ParseString(val)
		return err == nil && s == c.str
	case simdscan.TypeNumber:
		if c.intRaw != nil && isPlainInteger(val) {
			return bytes.Equal(val, c.intRaw)
		}
		n, err := jsonparser.ParseFloat(val)
		return err == nil && n == c.num
	case simdscan.TypeBoolean:
		return (val[0] == 't') == c.b
	}
	return true // null
}

func (c *equalityCond) anyElement(array []byte) bool {
	found := false
	jsonparser.ArrayEach(array, func(value []byte, dataType jsonparser.ValueType, _ int, _ error) {
		if !found {
			found = c.equal(value, simdscanType(dataType))
		}
	})
	return found
}

func simdscanType(t jsonparser.ValueType) simdscan.Type {
	switch t {
	case jsonparser.String:
		return simdscan.TypeString
	case jsonparser.Number:
		return simdscan.TypeNumber
	case jsonparser.Boolean:
		return simdscan.TypeBoolean
	case jsonparser.Null:
		return simdscan.TypeNull
	case jsonparser.Array:
		return simdscan.TypeArray
	case jsonparser.Object:
		return simdscan.TypeObject
	}
	return simdscan.TypeNotExist
}

// isPlainInteger reports whether a JSON number literal has no fraction or
// exponent (and is not -0, which equals 0 but has different bytes).
func isPlainInteger(val []byte) bool {
	digits := val
	if len(digits) > 0 && digits[0] == '-' {
		digits = digits[1:]
		if len(digits) == 1 && digits[0] == '0' {
			return false
		}
	}
	if len(digits) == 0 {
		return false
	}
	for _, d := range digits {
		if d < '0' || d > '9' {
			return false
		}
	}
	return true
}
