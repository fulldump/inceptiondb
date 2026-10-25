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
		if t != c.kind {
			return false
		}

		switch c.kind {
		case simdscan.TypeString:
			if bytes.IndexByte(val, '\\') < 0 {
				if string(val) != c.str {
					return false
				}
				continue
			}
			s, err := jsonparser.ParseString(val)
			if err != nil || s != c.str {
				return false
			}
		case simdscan.TypeNumber:
			if c.intRaw != nil && isPlainInteger(val) {
				if !bytes.Equal(val, c.intRaw) {
					return false
				}
				continue
			}
			n, err := jsonparser.ParseFloat(val)
			if err != nil || n != c.num {
				return false
			}
		case simdscan.TypeBoolean:
			if (val[0] == 't') != c.b {
				return false
			}
		}
	}
	return true
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
