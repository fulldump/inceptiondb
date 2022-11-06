package collection

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/fulldump/biff"
	"github.com/google/btree"
)

func Test_IndexBTree_HappyPath(t *testing.T) {

	index := NewIndexBTree(&IndexBTreeOptions{
		Fields: []string{"id"},
		Sparse: false,
		Unique: true,
	})

	n := 4
	for i := 0; i < n; i++ {
		data, _ := json.Marshal(JSON{
			"id": float64(i),
		})
		index.AddRow(&Row{
			Payload: data,
		})
	}

	{
		expectedPayloads := []string{
			`{"id":0}`, `{"id":1}`, `{"id":2}`, `{"id":3}`,
		}
		payloads := []string{}
		index.Traverse([]byte(`{}`), func(row *Row) bool {
			payloads = append(payloads, string(row.Payload))
			return true
		})
		biff.AssertEqual(payloads, expectedPayloads)
	}

	{
		expectedReversedPayloads := []string{
			`{"id":3}`, `{"id":2}`, `{"id":1}`, `{"id":0}`,
		}
		reversedPayloads := []string{}
		index.Traverse([]byte(`{"reverse":true}`), func(row *Row) bool {
			reversedPayloads = append(reversedPayloads, string(row.Payload))
			return true
		})
		biff.AssertEqual(reversedPayloads, expectedReversedPayloads)
	}

}

// TODO: remove this:
func TestRRRR(t *testing.T) {

	fields := []string{"random"}

	index := btree.NewG(32, func(a, b *RowOrdered) bool {

		for i, valA := range a.Values {
			valB := b.Values[i]
			if reflect.DeepEqual(valA, valB) {
				continue
			}

			switch valA := valA.(type) {
			case string:
				valB, ok := valB.(string)
				if !ok {
					panic("Type B should be string for field " + fields[i])
				}
				return valA < valB

			case float64:
				valB, ok := valB.(float64)
				if !ok {
					panic("Type B should be float64 for field " + fields[i])
				}
				return valA < valB

				// todo: case bool
			default:
				panic("Type A not supported, field " + fields[i])
			}
		}

		return false
	})

	insertRow := func(r *Row) {

		values := []interface{}{}
		data := map[string]interface{}{}
		json.Unmarshal(r.Payload, &data)

		for _, field := range fields {
			values = append(values, data[field])
		}

		index.ReplaceOrInsert(&RowOrdered{
			Row:    r,
			Values: values,
		})
	}

	t0 := time.Now()
	n := 10 * 1000
	for i := 0; i < n; i++ {
		data, _ := json.Marshal(JSON{
			"timestamp": time.Now().UnixNano(),
			"random":    rand.Float64(),
		})
		insertRow(&Row{
			Payload: data,
		})
	}
	fmt.Println("insert:", time.Since(t0))

	t1 := time.Now()
	index.Ascend(func(r *RowOrdered) bool {
		// fmt.Println(string(r.Row.Payload))
		return true
	})
	fmt.Println("traverse:", time.Since(t1))

	// index.AscendRange()

	/**

	collection.find({"$or":[ {"colors":"red"}, {"colors":"blue"}  ]})

	red
	blu 10

	1,2,3,4,5,6,10

	collection.find({"colors":{"$in":["red","blue"]}})

	collection.find({"timestamp":33000, "colors":"red"})




	*/

}

type JSON map[string]interface{}

// insertRow(&Row{
// 	I:       0,
// 	Payload: json.RawMessage(`{"id":1, "timestamp": 100, "name":"kernel panic"}`),
// })
// insertRow(&Row{
// 	I:       0,
// 	Payload: json.RawMessage(`{"id":2, "timestamp": 200, "name":"fulldump"}`),
// })
// insertRow(&Row{
// 	I:       0,
// 	Payload: json.RawMessage(`{"id":3, "timestamp": 300, "name":"willy"}`),
// })
// insertRow(&Row{
// 	I:       0,
// 	Payload: json.RawMessage(`{"id":5, "timestamp": 400, "name":"wonka"}`),
// })
// insertRow(&Row{
// 	I:       0,
// 	Payload: json.RawMessage(`{"id":4, "timestamp": 400, "name":"alpha"}`),
// })
// insertRow(&Row{
// 	I:       0,
// 	Payload: json.RawMessage(`{"id":7, "timestamp": 200, "name":"fulldump"}`),
// })

func TestSomething2(t *testing.T) {

	a := []int{1, 2, 3, 4, 5}

	n := 5

	if len(a) > n {
		fmt.Println("Empty")
	} else {
		fmt.Println(a[n:])
	}

}
