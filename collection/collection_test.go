package collection

import (
	"io/ioutil"
	"reflect"
	"testing"
)

func TestInsert(t *testing.T) {

	Environment(func(filename string) {

		c := OpenCollection(filename)
		c.Insert(map[string]interface{}{
			"hello": "world",
		})
		c.Close()

		b, err := ioutil.ReadFile(filename)
		if err != nil {
			t.Error("should not happen!")
		}
		if string(b) != "{\"hello\":\"world\"}\n" {
			t.Error("Unexpected data file content", string(b))
		}

	})

}

func TestFindOne(t *testing.T) {

	Environment(func(filename string) {

		ioutil.WriteFile(filename, []byte("{\"name\":\"Fulanez\"}\n"), 0666)

		c := OpenCollection(filename)

		r := map[string]interface{}{}
		c.FindOne(&r)

		c.Close()

		if !reflect.DeepEqual(r, map[string]interface{}{"name": "Fulanez"}) {
			t.Error("Unexpected retrieved information")
		}

	})
}

func TestInsert1000(t *testing.T) {

	Environment(func(filename string) {

		c := OpenCollection(filename)
		for i := 0; i < 1000; i++ {
			c.Insert(map[string]interface{}{"hello": "world", "n": i})
		}
		c.Close()

	})

}
