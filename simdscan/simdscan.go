// Package simdscan provides SIMD-accelerated JSON field extraction.
//
// It uses AVX2 instructions to scan 32 bytes at a time, classifying
// structural characters (quotes, backslashes, braces, brackets, colons,
// commas) in parallel. This enables finding JSON field values ~5-10x
// faster than traditional byte-by-byte parsers.
//
// The package handles escaped characters correctly — backslash sequences
// like \" and \\ are properly tracked across SIMD chunk boundaries.
//
// On non-amd64 architectures or CPUs without AVX2, a scalar fallback
// is used automatically.
package simdscan

import (
	"errors"
	"math/bits"
)

// Type represents the JSON value type.
type Type int

const (
	TypeNotExist Type = iota
	TypeString
	TypeNumber
	TypeObject
	TypeArray
	TypeBoolean
	TypeNull
)

var (
	ErrNotFound = errors.New("field not found")
	ErrBadJSON  = errors.New("invalid JSON")
)

// ValueCoord stores the position and type of a JSON value within the
// original byte slice — no copies, just coordinates.
type ValueCoord struct {
	Offset uint32
	Length uint32
	Type   Type
}

// ObjectScan holds the result of scanning a JSON object.
// Keys and Coords are parallel slices — Coords[i] describes the value for Keys[i].
// Data is a reference to the original JSON bytes (zero-copy).
type ObjectScan struct {
	Data   []byte
	Keys   []string
	Coords []ValueCoord
}

// Get retrieves the raw bytes and type for a given key.
// The value is extracted on-demand from the original data using coordinates.
func (o *ObjectScan) Get(key string) ([]byte, Type) {
	for i, k := range o.Keys {
		if k == key {
			c := o.Coords[i]
			return o.Data[c.Offset : c.Offset+c.Length], c.Type
		}
	}
	return nil, TypeNotExist
}

// GetString is a convenience method that returns the string value for a key.
// For string types, the returned value does NOT include JSON quotes.
func (o *ObjectScan) GetString(key string) (string, bool) {
	val, t := o.Get(key)
	if t != TypeString {
		return "", false
	}
	return string(val), true
}

// GetField extracts a top-level field value from a JSON object.
// For string values, the returned bytes do NOT include the surrounding quotes.
func GetField(json []byte, field string) ([]byte, Type, error) {
	if len(json) == 0 {
		return nil, TypeNotExist, ErrBadJSON
	}
	return getFieldAt(json, 0, field)
}

// GetPath extracts a nested field value following the given path.
// Example: GetPath(data, "user", "address", "city") extracts the
// "city" field from the nested object at user.address.
func GetPath(json []byte, path ...string) ([]byte, Type, error) {
	if len(json) == 0 || len(path) == 0 {
		return nil, TypeNotExist, ErrBadJSON
	}

	data := json
	for i, segment := range path {
		val, t, err := getFieldAt(data, 0, segment)
		if err != nil {
			return nil, TypeNotExist, err
		}

		// If this is the last path segment, return the value
		if i == len(path)-1 {
			return val, t, nil
		}

		// Otherwise, the value must be an object to descend into
		if t != TypeObject {
			return nil, TypeNotExist, ErrNotFound
		}
		data = val
	}

	return nil, TypeNotExist, ErrNotFound
}

// ScanObject scans a JSON object and returns coordinates for all top-level
// fields. This is the SIMD-accelerated equivalent of stonejson.ParseToOffsets.
// No values are copied — only their positions within the original data.
func ScanObject(data []byte) (*ObjectScan, error) {
	n := len(data)
	i := skipWhitespace(data, 0)
	if i >= n || data[i] != '{' {
		return nil, ErrBadJSON
	}
	i++

	obj := &ObjectScan{
		Data:   data,
		Keys:   make([]string, 0, 8),
		Coords: make([]ValueCoord, 0, 8),
	}

	for i < n {
		i = skipWhitespace(data, i)
		if i >= n {
			break
		}
		if data[i] == '}' {
			break
		}
		if data[i] == ',' {
			i++
			continue
		}

		// Key
		if data[i] != '"' {
			return nil, ErrBadJSON
		}
		i++
		keyStart := i
		keyEnd := findStringEnd(data, i)
		if keyEnd < 0 {
			return nil, ErrBadJSON
		}
		key := string(data[keyStart:keyEnd])
		i = keyEnd + 1

		// Colon
		i = skipWhitespace(data, i)
		if i >= n || data[i] != ':' {
			return nil, ErrBadJSON
		}
		i++

		// Value
		i = skipWhitespace(data, i)
		if i >= n {
			return nil, ErrBadJSON
		}

		valueStart := i
		var t Type

		switch data[i] {
		case '"':
			t = TypeString
			i++
			// For strings, the coord points to the content INSIDE quotes
			contentStart := i
			end := findStringEnd(data, i)
			if end < 0 {
				return nil, ErrBadJSON
			}
			obj.Keys = append(obj.Keys, key)
			obj.Coords = append(obj.Coords, ValueCoord{
				Offset: uint32(contentStart),
				Length: uint32(end - contentStart),
				Type:   TypeString,
			})
			i = end + 1

		case '{', '[':
			if data[i] == '{' {
				t = TypeObject
			} else {
				t = TypeArray
			}
			end := findValueEnd(data, i)
			if end < 0 {
				return nil, ErrBadJSON
			}
			obj.Keys = append(obj.Keys, key)
			obj.Coords = append(obj.Coords, ValueCoord{
				Offset: uint32(valueStart),
				Length: uint32(end - valueStart),
				Type:   t,
			})
			i = end

		case 't':
			i += 4
			obj.Keys = append(obj.Keys, key)
			obj.Coords = append(obj.Coords, ValueCoord{
				Offset: uint32(valueStart),
				Length: uint32(i - valueStart),
				Type:   TypeBoolean,
			})
		case 'f':
			i += 5
			obj.Keys = append(obj.Keys, key)
			obj.Coords = append(obj.Coords, ValueCoord{
				Offset: uint32(valueStart),
				Length: uint32(i - valueStart),
				Type:   TypeBoolean,
			})
		case 'n':
			i += 4
			obj.Keys = append(obj.Keys, key)
			obj.Coords = append(obj.Coords, ValueCoord{
				Offset: uint32(valueStart),
				Length: uint32(i - valueStart),
				Type:   TypeNull,
			})
		default:
			// Number
			for i < n && !isStructOrWS(data[i]) {
				i++
			}
			obj.Keys = append(obj.Keys, key)
			obj.Coords = append(obj.Coords, ValueCoord{
				Offset: uint32(valueStart),
				Length: uint32(i - valueStart),
				Type:   TypeNumber,
			})
		}
	}

	return obj, nil
}

// getFieldAt is the internal implementation that searches for a field
// starting at a given offset within the data.
func getFieldAt(data []byte, offset int, field string) ([]byte, Type, error) {
	n := len(data)
	i := skipWhitespace(data, offset)
	if i >= n || data[i] != '{' {
		return nil, TypeNotExist, ErrBadJSON
	}
	i++

	fieldBytes := []byte(field)

	for i < n {
		i = skipWhitespace(data, i)
		if i >= n {
			break
		}
		if data[i] == '}' {
			break
		}
		if data[i] == ',' {
			i++
			continue
		}

		// Expect opening quote for key
		if data[i] != '"' {
			return nil, TypeNotExist, ErrBadJSON
		}
		i++

		// Find end of key string (SIMD-accelerated)
		keyStart := i
		keyEnd := findStringEnd(data, i)
		if keyEnd < 0 {
			return nil, TypeNotExist, ErrBadJSON
		}
		i = keyEnd + 1 // skip closing quote

		// Check if key matches
		keyMatch := bytesEqualSimple(data[keyStart:keyEnd], fieldBytes)

		// Skip whitespace, expect ':'
		i = skipWhitespace(data, i)
		if i >= n || data[i] != ':' {
			return nil, TypeNotExist, ErrBadJSON
		}
		i++

		// Skip whitespace before value
		i = skipWhitespace(data, i)
		if i >= n {
			return nil, TypeNotExist, ErrBadJSON
		}

		// Read value
		valueStart := i
		switch data[i] {
		case '"':
			i++
			end := findStringEnd(data, i)
			if end < 0 {
				return nil, TypeNotExist, ErrBadJSON
			}
			if keyMatch {
				return data[i:end], TypeString, nil
			}
			i = end + 1

		case '{', '[':
			end := findValueEnd(data, i)
			if end < 0 {
				return nil, TypeNotExist, ErrBadJSON
			}
			t := TypeObject
			if data[valueStart] == '[' {
				t = TypeArray
			}
			if keyMatch {
				return data[valueStart:end], t, nil
			}
			i = end

		case 't':
			i += 4
			if keyMatch {
				return data[valueStart:i], TypeBoolean, nil
			}
		case 'f':
			i += 5
			if keyMatch {
				return data[valueStart:i], TypeBoolean, nil
			}
		case 'n':
			i += 4
			if keyMatch {
				return data[valueStart:i], TypeNull, nil
			}
		default:
			// Number
			for i < n && !isStructOrWS(data[i]) {
				i++
			}
			if keyMatch {
				return data[valueStart:i], TypeNumber, nil
			}
		}
	}

	return nil, TypeNotExist, ErrNotFound
}

// findStringEnd finds the closing unescaped " starting from position start.
// Returns the index of the closing quote, or -1 if not found.
func findStringEnd(data []byte, start int) int {
	i := start
	n := len(data)

	// SIMD fast path: scan 32 bytes at a time
	if simdAvailable && n-i >= 32 {
		for i+32 <= n {
			q, b, _ := classify32(&data[i])

			if q == 0 && b == 0 {
				// No quotes or backslashes — skip entire 32-byte chunk
				i += 32
				continue
			}

			if b == 0 {
				// Quotes but no backslashes — first quote is unescaped
				return i + bits.TrailingZeros32(q)
			}

			// Both backslashes and quotes present in this chunk.
			// Process byte-by-byte for correctness.
			break
		}
	}

	// Scalar fallback (also handles tail < 32 bytes)
	for i < n {
		if data[i] == '\\' {
			i += 2 // skip escaped character
			continue
		}
		if data[i] == '"' {
			return i
		}
		i++
	}
	return -1
}

// findValueEnd finds the end of a JSON value starting at data[start].
// For objects/arrays, it tracks nesting depth using SIMD acceleration.
func findValueEnd(data []byte, start int) int {
	i := start
	n := len(data)
	depth := 0
	inString := false

	for i < n {
		// SIMD fast path: skip chunks with no structural chars
		if !inString && simdAvailable && i+32 <= n {
			_, _, s := classify32(&data[i])
			if s == 0 {
				i += 32
				continue
			}
		}

		c := data[i]
		if inString {
			if c == '\\' {
				i += 2
				continue
			}
			if c == '"' {
				inString = false
			}
			i++
			continue
		}

		switch c {
		case '"':
			inString = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return -1
}

func skipWhitespace(data []byte, i int) int {
	for i < len(data) {
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

func isStructOrWS(c byte) bool {
	return c == ',' || c == '}' || c == ']' || c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func bytesEqualSimple(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
