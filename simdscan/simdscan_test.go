package simdscan

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/buger/jsonparser"
	"github.com/fulldump/inceptiondb/collection/stonejson"
)

// ============================================================================
// Test data
// ============================================================================

var testSimple = []byte(`{"id":10293,"name":"Geometric Gemini","active":true,"balance":4500.67}`)

var testWithEscapes = []byte(`{"msg":"hello \"world\"","path":"C:\\Users\\test","ok":true}`)

var testNested = []byte(`{"user":{"name":"Alice","addr":{"city":"NYC"}},"tags":["go","fast"]}`)

var testLargeValue = []byte(`{"id":1,"data":"` + string(make([]byte, 200)) + `","end":"found"}`)

// ============================================================================
// Correctness Tests — GetField
// ============================================================================

func TestGetField_Simple(t *testing.T) {
	tests := []struct {
		field    string
		wantVal  string
		wantType Type
	}{
		{"id", "10293", TypeNumber},
		{"name", "Geometric Gemini", TypeString},
		{"active", "true", TypeBoolean},
		{"balance", "4500.67", TypeNumber},
	}

	for _, tt := range tests {
		val, typ, err := GetField(testSimple, tt.field)
		if err != nil {
			t.Errorf("GetField(%q): unexpected error: %v", tt.field, err)
			continue
		}
		if typ != tt.wantType {
			t.Errorf("GetField(%q): type = %d, want %d", tt.field, typ, tt.wantType)
		}
		if string(val) != tt.wantVal {
			t.Errorf("GetField(%q): val = %q, want %q", tt.field, string(val), tt.wantVal)
		}
	}
}

func TestGetField_Escapes(t *testing.T) {
	val, typ, err := GetField(testWithEscapes, "msg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != TypeString {
		t.Fatalf("type = %d, want TypeString", typ)
	}
	expected := `hello \"world\"`
	if string(val) != expected {
		t.Errorf("val = %q, want %q", string(val), expected)
	}

	val, _, err = GetField(testWithEscapes, "path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected = `C:\\Users\\test`
	if string(val) != expected {
		t.Errorf("val = %q, want %q", string(val), expected)
	}

	val, typ, err = GetField(testWithEscapes, "ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != TypeBoolean || string(val) != "true" {
		t.Errorf("ok: val=%q type=%d", string(val), typ)
	}
}

func TestGetField_Nested(t *testing.T) {
	val, typ, err := GetField(testNested, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != TypeObject {
		t.Fatalf("type = %d, want TypeObject", typ)
	}
	if string(val) != `{"name":"Alice","addr":{"city":"NYC"}}` {
		t.Errorf("val = %q", string(val))
	}

	val, typ, err = GetField(testNested, "tags")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != TypeArray {
		t.Fatalf("type = %d, want TypeArray", typ)
	}
	if string(val) != `["go","fast"]` {
		t.Errorf("val = %q", string(val))
	}
}

func TestGetField_NotFound(t *testing.T) {
	_, _, err := GetField(testSimple, "missing")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetField_LargeValue(t *testing.T) {
	val, _, err := GetField(testLargeValue, "end")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(val) != "found" {
		t.Errorf("val = %q, want %q", string(val), "found")
	}
}

// ============================================================================
// Correctness Tests — GetPath (nested extraction)
// ============================================================================

func TestGetPath_Simple(t *testing.T) {
	// Single segment = same as GetField
	val, typ, err := GetPath(testSimple, "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != TypeString || string(val) != "Geometric Gemini" {
		t.Errorf("val = %q, type = %d", string(val), typ)
	}
}

func TestGetPath_Nested(t *testing.T) {
	val, typ, err := GetPath(testNested, "user", "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != TypeString || string(val) != "Alice" {
		t.Errorf("val = %q, type = %d", string(val), typ)
	}
}

func TestGetPath_DeepNested(t *testing.T) {
	val, typ, err := GetPath(testNested, "user", "addr", "city")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if typ != TypeString || string(val) != "NYC" {
		t.Errorf("val = %q, type = %d", string(val), typ)
	}
}

func TestGetPath_NotFound(t *testing.T) {
	_, _, err := GetPath(testNested, "user", "phone")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetPath_NonObjectIntermediate(t *testing.T) {
	// "name" is a string, not an object — can't descend
	_, _, err := GetPath(testNested, "user", "name", "first")
	if err == nil {
		t.Errorf("expected error for non-object intermediate")
	}
}

// ============================================================================
// Correctness Tests — ScanObject
// ============================================================================

func TestScanObject_Simple(t *testing.T) {
	obj, err := ScanObject(testSimple)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(obj.Keys) != 4 {
		t.Fatalf("expected 4 keys, got %d: %v", len(obj.Keys), obj.Keys)
	}

	// Check "name" via Get
	val, typ := obj.Get("name")
	if typ != TypeString || string(val) != "Geometric Gemini" {
		t.Errorf("name: val=%q type=%d", string(val), typ)
	}

	// Check "id" via Get
	val, typ = obj.Get("id")
	if typ != TypeNumber || string(val) != "10293" {
		t.Errorf("id: val=%q type=%d", string(val), typ)
	}

	// Check "active" via Get
	val, typ = obj.Get("active")
	if typ != TypeBoolean || string(val) != "true" {
		t.Errorf("active: val=%q type=%d", string(val), typ)
	}

	// GetString convenience
	name, ok := obj.GetString("name")
	if !ok || name != "Geometric Gemini" {
		t.Errorf("GetString(name) = %q, %v", name, ok)
	}
}

func TestScanObject_WithNested(t *testing.T) {
	obj, err := ScanObject(testNested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(obj.Keys) != 2 {
		t.Fatalf("expected 2 keys, got %d: %v", len(obj.Keys), obj.Keys)
	}

	val, typ := obj.Get("user")
	if typ != TypeObject {
		t.Fatalf("user: type=%d, want TypeObject", typ)
	}
	if string(val) != `{"name":"Alice","addr":{"city":"NYC"}}` {
		t.Errorf("user: val=%q", string(val))
	}

	val, typ = obj.Get("tags")
	if typ != TypeArray || string(val) != `["go","fast"]` {
		t.Errorf("tags: val=%q type=%d", string(val), typ)
	}
}

func TestScanObject_NotFound(t *testing.T) {
	obj, err := ScanObject(testSimple)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, typ := obj.Get("missing")
	if typ != TypeNotExist {
		t.Errorf("expected TypeNotExist, got %d", typ)
	}
}

func TestSIMDAvailable(t *testing.T) {
	t.Logf("SIMD (AVX2) available: %v", simdAvailable)
}

// ============================================================================
// Benchmarks: simdscan vs jsonparser vs encoding/json
// ============================================================================

var benchJSON = []byte(`{"id":10293,"name":"Geometric Gemini","active":true,"balance":4500.67,"email":"ai@example.com","address":"123 Silicon Valley","tags":["ai","go","fast"],"version":"1.0.2"}`)

// --- GetField benchmarks ---

func BenchmarkSimdscan_GetField(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		val, _, _ := GetField(benchJSON, "balance")
		_ = val
	}
}

func BenchmarkJsonparser_Get(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		val, _, _, _ := jsonparser.Get(benchJSON, "balance")
		_ = val
	}
}

func BenchmarkStdlib_Unmarshal(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var m map[string]any
		json.Unmarshal(benchJSON, &m)
		_ = m["balance"]
	}
}

// --- GetPath benchmarks (nested extraction) ---

var benchNestedJSON = []byte(`{"user":{"name":"Alice","profile":{"age":30,"city":"NYC","score":99.5}},"active":true}`)

func BenchmarkSimdscan_GetPath(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		val, _, _ := GetPath(benchNestedJSON, "user", "profile", "city")
		_ = val
	}
}

func BenchmarkJsonparser_GetNested(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		val, _, _, _ := jsonparser.Get(benchNestedJSON, "user", "profile", "city")
		_ = val
	}
}

// --- ScanObject benchmarks (full scan) ---

func BenchmarkSimdscan_ScanObject(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		obj, _ := ScanObject(benchJSON)
		_ = obj
	}
}

func BenchmarkStonejson_ParseToOffsets(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		obj, _ := stonejson.ParseToOffsets(benchJSON)
		_ = obj
	}
}

func BenchmarkStdlib_UnmarshalFull(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var m map[string]any
		json.Unmarshal(benchJSON, &m)
		_ = m
	}
}

// --- ScanObject + Get vs GetField ---

func BenchmarkScanObject_ThenGet(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		obj, _ := ScanObject(benchJSON)
		v, _ := obj.Get("balance")
		_ = v
	}
}

// --- Large string skip benchmarks ---

var benchLarge = func() []byte {
	bigVal := make([]byte, 4096)
	for i := range bigVal {
		bigVal[i] = 'x'
	}
	return []byte(fmt.Sprintf(`{"big":"%s","target":"found"}`, string(bigVal)))
}()

func BenchmarkSimdscan_LargeSkip(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		val, _, _ := GetField(benchLarge, "target")
		_ = val
	}
}

func BenchmarkJsonparser_LargeSkip(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		val, _, _, _ := jsonparser.Get(benchLarge, "target")
		_ = val
	}
}

func BenchmarkSimdscan_ScanLarge(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		obj, _ := ScanObject(benchLarge)
		_ = obj
	}
}

func BenchmarkStonejson_ScanLarge(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		obj, _ := stonejson.ParseToOffsets(benchLarge)
		_ = obj
	}
}
