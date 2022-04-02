package collection

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/fulldump/biff"
)

func TestInsert(t *testing.T) {

	Environment(func(filename string) {

		c := OpenCollection(filename)
		c.Insert(map[string]interface{}{
			"hello": "world",
		})
		c.Close()

		fileContent, readFileErr := ioutil.ReadFile(filename)
		AssertNil(readFileErr)
		AssertEqual(fileContent, []byte(`{"hello":"world"}`+"\n"))

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

func TestInsert100K(t *testing.T) {

	Environment(func(filename string) {

		c := OpenCollection(filename)
		n := 100 * 1000
		for i := 0; i < n; i++ {
			c.Insert(map[string]interface{}{"hello": "world", "n": i})
		}
		c.Close()

	})

}

func TestIndex(t *testing.T) {

	type User struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}

	Environment(func(filename string) {

		// Setup
		c := OpenCollection(filename)
		c.Insert(&User{"1", "Pablo"})
		c.Insert(&User{"2", "Sara"})

		// Run
		c.Index("id")

		// Check
		user := &User{}
		errFindBy := c.FindBy("id", "2", user)
		AssertNil(errFindBy)
		AssertEqual(user.Name, "Sara")
	})
}

func TestInsertAfterIndex(t *testing.T) {

	type User struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}

	Environment(func(filename string) {

		// Setup
		c := OpenCollection(filename)

		// Run
		c.Index("id")
		c.Insert(&User{"1", "Pablo"})

		// Check
		user := &User{}
		errFindBy := c.FindBy("id", "1", user)
		AssertNil(errFindBy)
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
		c := OpenCollection(filename)
		c.Insert(&User{"1", []string{"pablo@hotmail.com", "p18@yahoo.com"}})

		// Run
		c.Index("email")

		// Check
		u := &User{}
		errFindBy := c.FindBy("email", "p18@yahoo.com", u)
		AssertNil(errFindBy)
		AssertEqual(u.Id, "1")
	})
}

func TestIndexSparse(t *testing.T) {

	Environment(func(filename string) {

		// Setup
		c := OpenCollection(filename)
		c.Insert(map[string]interface{}{"id": "1"})

		// Run
		errIndex := c.Index("email")

		// Check
		AssertNil(errIndex)
		AssertEqual(len(c.indexes["email"]), 0)
	})
}

func TestCollection_Index_Collision(t *testing.T) {

	type User struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}

	Environment(func(filename string) {

		// Setup
		c := OpenCollection(filename)
		c.Insert(&User{"1", "Pablo"})
		c.Insert(&User{"1", "Sara"})

		// Run
		err := c.Index("id")

		// Check
		AssertNotNil(err)
	})
}

func TestDoThings(t *testing.T) {

	c := OpenCollection("users")
	defer c.Close()

	//c.Drop()
	//c.Index("id")
	c.Insert(map[string]interface{}{"id": "1", "name": "Gerardo", "email": []string{"gerardo@email.com", "gerardo@hotmail.com"}})
	c.Insert(map[string]interface{}{"id": "2", "name": "Pablo", "email": []string{"pablo@email.com", "pablo2018@yahoo.com"}})

	c.Traverse(func(data []byte) {
		u := struct {
			Id    string
			Email []string
		}{}

		json.Unmarshal(data, &u)

		if u.Id != "2" {
			return
		}

		fmt.Println(u)
	})

	//err := c.Index("email")
	//AssertNil(err)
	//
	//u := struct {
	//	Id    string
	//	Name  string
	//	Email []string
	//}{}
	//
	//fmt.Println(c.FindBy("email", "gerardo@email.com", &u), u)
}

func TestConcurrentInsertions(t *testing.T) {

	c := OpenCollection("data/concurrency")

	longstring := strings.Repeat("a", 4*1024)

	wg := &sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100000; j++ {
				c.Insert(map[string]interface{}{
					"hello": longstring,
					"n":     j,
				})
			}
		}()
	}

	wg.Wait()

	c.Close()
}

func TestConcurrentInsertions2(t *testing.T) {

	t0 := time.Now()
	c := OpenCollection("data/concurrency")
	fmt.Println("Load time:", time.Since(t0))

	t1 := time.Now()
	s := int64(0)
	q := make(chan []byte, 320)
	wg := sync.WaitGroup{}
	for w := 0; w < 32; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for data := range q {
				v := struct {
					N int `json:"n"`
				}{}
				json.Unmarshal(data, &v)

				atomic.AddInt64(&s, int64(v.N))
			}
		}()
	}
	i := 0
	c.Traverse(func(data []byte) {
		q <- data
		i++
	})
	close(q)

	wg.Wait()

	fmt.Println("Traverse time:", time.Since(t1))
	fmt.Println("records:", i)
	fmt.Println("sum:", s)
}
