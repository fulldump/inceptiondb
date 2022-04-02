package collection

import (
	"io/ioutil"
	"testing"

	. "github.com/fulldump/biff"
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
		fileContent, readFileErr := ioutil.ReadFile(filename)
		AssertNil(readFileErr)
		AssertEqual(fileContent, []byte(`{"hello":"world"}`+"\n"))
	})
}

func TestFindOne(t *testing.T) {
	Environment(func(filename string) {

		// Setup
		ioutil.WriteFile(filename, []byte("{\"name\":\"Fulanez\"}\n"), 0666)

		// Run
		c, _ := OpenCollection(filename)
		defer c.Close()

		// Check
		r := map[string]interface{}{}
		c.FindOne(&r)
		AssertEqualJson(r, map[string]interface{}{"name": "Fulanez"})
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
		AssertEqual(len(c.rows), n)
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
		c, _ := OpenCollection(filename)

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
		newUser := &User{"1", []string{"pablo@hotmail.com", "p18@yahoo.com"}}
		c, _ := OpenCollection(filename)
		c.Insert(newUser)

		// Run
		indexErr := c.Index("email")

		// Check
		AssertNil(indexErr)
		u := &User{}
		c.FindBy("email", "p18@yahoo.com", u)
		AssertEqual(u.Id, newUser.Id)
	})
}

func TestIndexSparse(t *testing.T) {

	Environment(func(filename string) {

		// Setup
		c, _ := OpenCollection(filename)
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
		c, _ := OpenCollection(filename)
		c.Insert(&User{"1", "Pablo"})
		c.Insert(&User{"1", "Sara"})

		// Run
		err := c.Index("id")

		// Check
		AssertNotNil(err)
	})
}

func TestDoThings(t *testing.T) {

	//c, _ := OpenCollection("users")
	//c.Drop()

	//c.Insert(map[string]interface{}{"id": "1", "name": "Gerardo", "email": []string{"gerardo@email.com", "gerardo@hotmail.com"}})
	//c.Insert(map[string]interface{}{"id": "2", "name": "Pablo", "email": []string{"pablo@email.com", "pablo2018@yahoo.com"}})

	//c.Traverse(func(data []byte) {
	//	u := struct {
	//		Id    string
	//		Email string
	//	}{}
	//
	//	json.Unmarshal(data, &u)
	//
	//	if u.Id != "2" {
	//		return
	//	}
	//
	//	fmt.Println(u)
	//})

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
