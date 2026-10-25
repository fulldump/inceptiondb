package simdscan

import "bytes"

// Field describes a top-level field of a JSON object by its byte positions
// in the original data.
type Field struct {
	Key        []byte // raw key: without quotes and not unescaped
	KeyStart   int    // index of the opening quote of the key
	ValueStart int
	ValueEnd   int // exclusive; strings include their quotes
	Type       Type
}

// EachField calls fn for every top-level field of the JSON object in data, in
// order, without allocating. It returns the index of the closing brace. If fn
// returns false the scan stops and EachField returns -1.
func EachField(data []byte, fn func(f Field) bool) (int, error) {
	n := len(data)
	i := skipWhitespace(data, 0)
	if i >= n || data[i] != '{' {
		return 0, ErrBadJSON
	}
	i = skipWhitespace(data, i+1)
	if i < n && data[i] == '}' {
		return i, nil
	}

	for i < n {
		// Key
		if data[i] != '"' {
			return 0, ErrBadJSON
		}
		keyStart := i
		keyEnd := findStringEnd(data, i+1)
		if keyEnd < 0 {
			return 0, ErrBadJSON
		}
		key := data[i+1 : keyEnd]

		// Colon
		i = skipWhitespace(data, keyEnd+1)
		if i >= n || data[i] != ':' {
			return 0, ErrBadJSON
		}
		i = skipWhitespace(data, i+1)
		if i >= n {
			return 0, ErrBadJSON
		}

		// Value
		valueStart := i
		var t Type
		switch data[i] {
		case '"':
			t = TypeString
			end := findStringEnd(data, i+1)
			if end < 0 {
				return 0, ErrBadJSON
			}
			i = end + 1
		case '{', '[':
			t = TypeArray
			if data[i] == '{' {
				t = TypeObject
			}
			i = findValueEnd(data, i)
			if i < 0 {
				return 0, ErrBadJSON
			}
		case 't':
			t, i = TypeBoolean, i+4
			if i > n || !bytes.Equal(data[valueStart:i], []byte("true")) {
				return 0, ErrBadJSON
			}
		case 'f':
			t, i = TypeBoolean, i+5
			if i > n || !bytes.Equal(data[valueStart:i], []byte("false")) {
				return 0, ErrBadJSON
			}
		case 'n':
			t, i = TypeNull, i+4
			if i > n || !bytes.Equal(data[valueStart:i], []byte("null")) {
				return 0, ErrBadJSON
			}
		default:
			t = TypeNumber
			for i < n && !isStructOrWS(data[i]) {
				i++
			}
			if i == valueStart {
				return 0, ErrBadJSON
			}
		}

		if !fn(Field{Key: key, KeyStart: keyStart, ValueStart: valueStart, ValueEnd: i, Type: t}) {
			return -1, nil
		}

		i = skipWhitespace(data, i)
		if i >= n {
			return 0, ErrBadJSON
		}
		switch data[i] {
		case '}':
			return i, nil
		case ',':
			i = skipWhitespace(data, i+1)
		default:
			return 0, ErrBadJSON
		}
	}
	return 0, ErrBadJSON
}
