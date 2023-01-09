package collection

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"sync"
	"testing"
	"time"

	. "github.com/fulldump/biff"
	"github.com/google/uuid"
)

func TestInsert(t *testing.T) {
	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
		defer c.Close()

		// Run
		c.Insert(map[string]interface{}{
			"hello": "world",
		})

		// Check
		fileContent, _ := ioutil.ReadFile(filename)
		command := &Command{}
		json.Unmarshal(fileContent, command)
		AssertEqual(string(command.Payload), `{"hello":"world"}`)
	})
}

func TestCollection_Insert_Concurrency(t *testing.T) {
	Environment(func(filename string) {

		c, _ := OpenCollection(filename)

		n := 100

		wg := &sync.WaitGroup{}
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c.Insert(map[string]interface{}{"hello": "world"})
			}()
		}

		wg.Wait()

		AssertEqual(len(c.Rows), n)
	})
}

func TestFindOne(t *testing.T) {
	Environment(func(filename string) {

		// Setup
		ioutil.WriteFile(filename, []byte(`{"name":"insert","uuid":"ec59a0e6-8fcb-4c1c-91e5-3dd7df6a0b80","timestamp":1648937091073939741,"start_byte":0,"payload":{"name": "Fulanez"}}`), 0666)

		// Run
		c, _ := OpenCollection(filename)
		defer c.Close()

		// Check
		row := map[string]interface{}{}
		c.FindOne(&row)
		AssertEqualJson(row, map[string]interface{}{"name": "Fulanez"})
	})
}

func TestInsert100K(t *testing.T) {
	Environment(func(filename string) {
		// Setup
		c, _ := OpenCollection(filename)
		defer c.Close()

		// Run
		n := 100 * 1000
		for i := 0; i < n; i++ {
			c.Insert(map[string]interface{}{"hello": "world", "n": i})
		}

		// Check
		AssertEqual(len(c.Rows), n)
	})
}

func TestIndex(t *testing.T) {
	type User struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	Environment(func(filename string) {
		// Setup
		c, _ := OpenCollection(filename)
		c.Insert(&User{"1", "Pablo"})
		c.Insert(&User{"2", "Sara"})

		// Run
		c.Index("my-index", &IndexMapOptions{
			Field: "id",
		})

		// Check
		user := &User{}
		c.Indexes["my-index"].Traverse([]byte(`{"value":"2"}`), func(row *Row) bool {
			json.Unmarshal(row.Payload, &user)
			return false
		})
		AssertEqual(user.Name, "Sara")
	})
}

func findByIndex(index Index, options string, value interface{}) (n int) {
	index.Traverse([]byte(options), func(row *Row) bool {
		n++
		json.Unmarshal(row.Payload, &value)
		return false
	})
	return
}

func TestInsertAfterIndex(t *testing.T) {
	type User struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)

		// Run
		c.Index("my-index", &IndexMapOptions{
			Field: "id",
		})
		c.Insert(&User{"1", "Pablo"})

		// Check
		user := &User{}
		findByIndex(c.Indexes["my-index"], `{"value":"1"}`, user)
		AssertEqual(user.Name, "Pablo")
	})
}

func TestIndexMultiValue(t *testing.T) {
	type User struct {
		Id    string   `json:"id"`
		Email []string `json:"email"`
	}
	Environment(func(filename string) {

		// Setup
		newUser := &User{"1", []string{"pablo@hotmail.com", "p18@yahoo.com"}}
		c, _ := OpenCollection(filename)
		c.Insert(newUser)

		// Run
		indexErr := c.Index("my-index", &IndexMapOptions{
			Field: "email",
		})

		// Check
		AssertNil(indexErr)
		u := &User{}
		findByIndex(c.Indexes["my-index"], `{"value":"p18@yahoo.com"}`, u)
		AssertEqual(u.Id, newUser.Id)
	})
}

// TODO: this should be a unit test for IndexMap
func TestIndexSparse(t *testing.T) {
	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
		row, err := c.Insert(map[string]interface{}{"id": "1"})

		// Run
		errIndex := c.Index("my-index", &IndexMapOptions{
			Field:  "email",
			Sparse: true,
		})

		// Check
		AssertNil(errIndex)
		AssertNotNil(row)
		AssertNil(err)
		AssertEqual(len(c.Indexes["my-index"].Index.(*IndexMap).Entries), 0)
	})
}

func TestIndexNonSparse(t *testing.T) {
	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
		c.Insert(map[string]interface{}{"id": "1"})

		// Run
		errIndex := c.Index("my-index", &IndexMapOptions{
			Field:  "email",
			Sparse: false,
		})

		// Check
		AssertNotNil(errIndex)
		AssertEqual(errIndex.Error(), "index row: field `email` is indexed and mandatory, data: {\"id\":\"1\"}")
	})
}

func TestCollection_Index_Collision(t *testing.T) {
	type User struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
		c.Insert(&User{"1", "Pablo"})
		c.Insert(&User{"1", "Sara"})

		// Run
		errIndex := c.Index("my-index", &IndexMapOptions{
			Field: "id",
		})

		// Check
		AssertNotNil(errIndex)
		AssertEqual(errIndex.Error(), `index row: index conflict: field 'id' with value '1', data: {"id":"1","name":"Sara"}`)
	})
}

func TestPersistenceInsertAndIndex(t *testing.T) {
	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
		c.Insert(map[string]interface{}{"id": "1", "name": "Pablo", "email": []string{"pablo@email.com", "pablo2018@yahoo.com"}})
		c.Index("my-index", &IndexMapOptions{
			Field: "email",
		})
		c.Insert(map[string]interface{}{"id": "2", "name": "Sara", "email": []string{"sara@email.com", "sara.jimenez8@yahoo.com"}})
		c.Close()

		// Run
		c, _ = OpenCollection(filename)
		user := struct {
			Id    string
			Name  string
			Email []string
		}{}
		findByIndex(c.Indexes["my-index"], `{"value":"sara@email.com"}`, &user)

		// Check
		AssertEqual(user.Id, "2")

	})
}

func TestPersistenceDelete(t *testing.T) {
	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
		c.Index("my-index", &IndexMapOptions{
			Field: "email",
		})
		c.Insert(map[string]interface{}{"id": "1", "name": "Pablo", "email": []string{"pablo@email.com", "pablo2018@yahoo.com"}})
		row, _ := c.Insert(map[string]interface{}{"id": "2", "name": "Sara", "email": []string{"sara@email.com", "sara.jimenez8@yahoo.com"}})
		c.Insert(map[string]interface{}{"id": "3", "name": "Ana", "email": []string{"ana@email.com", "ana@yahoo.com"}})
		err := c.Remove(row)
		AssertNil(err)
		c.Close()

		// Run
		c, _ = OpenCollection(filename)
		user := struct {
			Id    string
			Name  string
			Email []string
		}{}
		n := findByIndex(c.Indexes["my-index"], `{"value":"sara@email.com"}`, &user)

		// Check
		AssertEqual(n, 0)
		AssertEqual(len(c.Rows), 2)
	})
}

// TestPersistenceDeleteTwice check if the same command is persisted twice (or more) when the collection is open
func TestPersistenceDeleteTwice(t *testing.T) {
	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
		c.Index("my-index", &IndexMapOptions{
			Field: "id",
		})
		row, _ := c.Insert(map[string]interface{}{"id": "1"})
		c.Remove(row)
		c.Close()

		// Run
		c, _ = OpenCollection(filename)

		AssertEqual(len(c.Rows), 0)

		// TODO: assert this somehow!

	})
}

func TestPersistenceUpdate(t *testing.T) {
	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
		c.Index("my-index", &IndexMapOptions{
			Field: "id",
		})
		row, _ := c.Insert(map[string]interface{}{"id": "1", "name": "Pablo", "email": []string{"pablo@email.com", "pablo2018@yahoo.com"}})
		c.Patch(row, map[string]interface{}{"name": "Jaime"})
		c.Close()

		// Run
		c, _ = OpenCollection(filename)
		user := struct {
			Id    string
			Name  string
			Email []string
		}{}
		n := findByIndex(c.Indexes["my-index"], `{"value":"1"}`, &user)

		// Check
		AssertEqual(n, 1)
		AssertEqual(user.Name, "Jaime")

		AssertEqual(len(c.Rows), 1)
	})
}

type MockIndex struct {
	AddRowCallback    func(row *Row) error
	RemoveRowCallback func(row *Row) error
}

func (m *MockIndex) AddRow(row *Row) error {
	return m.AddRowCallback(row)
}

func (m *MockIndex) RemoveRow(row *Row) error {
	return m.RemoveRowCallback(row)
}

func (m *MockIndex) Traverse(options []byte, f func(row *Row) bool) {
	// TODO implement me
	panic("implement me")
}

func TestIndexInsert_Rollback(t *testing.T) {

	adds := []string{}
	removes := []string{}

	newMock := func(name string) Index {
		return &MockIndex{
			AddRowCallback: func(row *Row) error {
				if len(adds) == 2 {
					return errors.New("mock error")
				}
				adds = append(adds, name)
				return nil
			},
			RemoveRowCallback: func(row *Row) error {
				removes = append(removes, name)
				return nil
			},
		}
	}

	indexes := map[string]*collectionIndex{
		"a": &collectionIndex{
			Index: newMock("a"),
		},
		"b": &collectionIndex{
			Index: newMock("b"),
		},
		"c": &collectionIndex{
			Index: newMock("c"),
		},
	}

	row := &Row{}

	indexInsert(indexes, row)

	AssertEqual(removes, adds)
	AssertEqual(len(removes), 2)
}

func TestIndexInsert_Rollback_BlackBox(t *testing.T) {

	indexes := map[string]*collectionIndex{
		"a": &collectionIndex{
			Index: NewIndexMap(&IndexMapOptions{
				Field: "id",
			}),
		},
	}

	row := &Row{
		Payload: []byte(`{"id":"my-id"}`),
	}

	err1 := indexInsert(indexes, row)
	AssertNil(err1)

	err2 := indexInsert(indexes, row)
	AssertNotNil(err2)

	err3 := indexInsert(indexes, row)
	AssertNotNil(err3)

}

func TestInsert1M_serial(t *testing.T) {

	t.Skip()

	Environment(func(filename string) {
		// Setup
		c, _ := OpenCollection(filename)
		defer c.Close()

		c.Index("index1", &IndexMapOptions{
			Field: "uuid",
		})
		c.Index("index2", &IndexMapOptions{
			Field: "i",
		})

		// Run
		n := 1000 * 1000
		for i := 0; i < n; i++ {
			c.Insert(map[string]interface{}{"uuid": uuid.New().String(), "hello": "world", "i": strconv.Itoa(i)})
		}

		// Check
		AssertEqual(len(c.Rows), n)
	})

}

func TestInsert1M_concurrent(t *testing.T) {

	t.Skip()

	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
		defer c.Close()

		c.Index("index1", &IndexMapOptions{
			Field: "uuid",
		})
		c.Index("index2", &IndexMapOptions{
			Field: "i",
		})

		// Run
		t0 := time.Now()
		wg := &sync.WaitGroup{}
		workers := 128
		n := 2 * 1000 * 1000 / workers
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				for i := 0; i < n; i++ {
					c.Insert(map[string]interface{}{"uuid": uuid.New().String(), "hello": "world", "i": strconv.Itoa(i + n*w)})
				}
			}(w)
		}

		wg.Wait()
		delay := time.Since(t0)

		// Check
		AssertEqual(len(c.Rows), n*workers)
		fmt.Println("delay", delay)
		fmt.Println("throughput (inserts/second)", float64(n*workers)/delay.Seconds())
	})

}

// TODO: test concurrent delete
