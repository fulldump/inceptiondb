package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fulldump/apitest"
	"github.com/fulldump/biff"
)

type JSON = map[string]interface{}

func Acceptance(a *biff.A, apiRequest func(method, path string) *apitest.Request) {

	a.Alternative("Create collection", func(a *biff.A) {
		resp := apiRequest("POST", "/collections").
			WithBodyJson(JSON{
				"name": "my-collection",
			}).Do()
		Save(resp, "Create collection", ``)

		biff.AssertEqual(resp.StatusCode, http.StatusCreated)
		expectedBody := JSON{
			"name":  "my-collection",
			"total": 0,
		}
		biff.AssertEqualJson(resp.BodyJson(), expectedBody)

		a.Alternative("Retrieve collection", func(a *biff.A) {
			resp := apiRequest("GET", "/collections/my-collection").
				WithBodyJson(JSON{
					"name": "my-collection", // TODO: remove
				}).Do()
			Save(resp, "Retrieve collection", ``)

			biff.AssertEqual(resp.StatusCode, http.StatusOK)
			expectedBody := JSON{
				"name":  "my-collection",
				"total": 0,
			}
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)
		})

		a.Alternative("List collections", func(a *biff.A) {
			resp := apiRequest("GET", "/collections").Do()
			Save(resp, "List collections", ``)

			biff.AssertEqual(resp.StatusCode, http.StatusOK)
			expectedBody := []JSON{
				{
					"name":  "my-collection",
					"total": 0,
				},
			}
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)
		})

		a.Alternative("Drop collection", func(a *biff.A) {
			resp := apiRequest("POST", "/collections/my-collection:dropCollection").
				Do()
			Save(resp, "Drop collection", ``)

			biff.AssertEqual(resp.StatusCode, http.StatusOK)

			a.Alternative("Get dropped collection", func(a *biff.A) {
				resp := apiRequest("GET", "/collections/my-collection").
					Do()
				Save(resp, "Get collection - not found", ``)

				biff.AssertEqual(resp.StatusCode, http.StatusNotFound)
			})
		})

		a.Alternative("Insert one operation", func(a *biff.A) {
			myDocument := JSON{
				"id":      "my-id",
				"name":    "Fulanez",
				"address": "Elm Street 11",
			}
			resp := apiRequest("POST", "/collections/my-collection:insert").
				WithBodyJson(myDocument).Do()
			Save(resp, "Insert one", ``)

			biff.AssertEqual(resp.StatusCode, http.StatusCreated)
			biff.AssertEqual(resp.BodyString(), "")

			a.Alternative("Find with fullscan", func(a *biff.A) {
				resp := apiRequest("POST", "/collections/my-collection:find").
					WithBodyJson(JSON{
						"mode":  "fullscan",
						"limit": 0,
						"skip":  0,
						"filter": JSON{
							"name": "Fulanez",
						},
					}).Do()
				Save(resp, "Find - fullscan", ``)

				biff.AssertEqual(resp.StatusCode, http.StatusOK)
				biff.AssertEqual(resp.BodyJson(), myDocument)
			})

		})

		a.Alternative("Insert many", func(a *biff.A) {

			myDocuments := []JSON{
				{"id": "1", "name": "Alfonso"},
				{"id": "2", "name": "Gerardo"},
				{"id": "3", "name": "Alfonso"},
			}

			body := ""
			for _, myDocument := range myDocuments {
				myDocument, _ := json.Marshal(myDocument)
				body += string(myDocument) + "\n"
			}
			resp := apiRequest("POST", "/collections/my-collection:insert").
				WithBodyString(body).Do()
			Save(resp, "Insert many", ``)

			a.Alternative("Create index", func(a *biff.A) {
				resp := apiRequest("POST", "/collections/my-collection:createIndex").
					WithBodyJson(JSON{"name": "my-index", "type": "map", "options": JSON{"field": "id"}}).Do()
				Save(resp, "Create index", ``)

				a.Alternative("Delete by index", func(a *biff.A) {
					resp := apiRequest("POST", "/collections/my-collection:remove").
						WithBodyJson(JSON{
							"index": "my-index",
							"value": "2",
						}).Do()
					Save(resp, "Delete - by index", ``)

					biff.AssertEqualJson(resp.BodyJson(), myDocuments[1])
					biff.AssertEqual(resp.StatusCode, http.StatusOK)
				})
				a.Alternative("Patch by index", func(a *biff.A) {
					resp := apiRequest("POST", "/collections/my-collection:patch").
						WithBodyJson(JSON{
							"index": "my-index",
							"value": "3",
							"patch": JSON{
								"name": "Pedro",
							},
						}).Do()
					Save(resp, "Patch - by index", ``)

					expectedBody := JSON{
						"id":   "3",
						"name": "Pedro",
					}

					biff.AssertEqualJson(resp.BodyJson(), expectedBody)
					biff.AssertEqual(resp.StatusCode, http.StatusOK)

					{
						resp = apiRequest("POST", "/collections/my-collection:find").
							WithBodyJson(JSON{"limit": 10}).Do()
						Save(resp, "Find - fullscan with limit 10", ``)

						dec := json.NewDecoder(strings.NewReader(resp.BodyString()))
						expectedDocuments := []JSON{
							myDocuments[0],
							myDocuments[1],
							{"id": "3", "name": "Pedro"},
						}
						for _, expectedDocument := range expectedDocuments {
							var bodyRow interface{}
							dec.Decode(&bodyRow)
							biff.AssertEqualJson(bodyRow, expectedDocument)
						}
						biff.AssertEqual(resp.StatusCode, http.StatusOK)
					}

				})

			})

			a.Alternative("Delete by fullscan", func(a *biff.A) {

				{
					resp := apiRequest("POST", "/collections/my-collection:remove").
						WithBodyJson(JSON{
							"limit": 10,
							"filter": JSON{
								"name": "Alfonso",
							},
						}).Do()
					Save(resp, "Delete - fullscan", ``)

					dec := json.NewDecoder(strings.NewReader(resp.BodyString()))
					expectedDocuments := []JSON{
						myDocuments[0],
						myDocuments[2],
					}
					for _, expectedDocument := range expectedDocuments {
						var bodyRow interface{}
						dec.Decode(&bodyRow)
						biff.AssertEqualJson(bodyRow, expectedDocument)
					}
					biff.AssertEqual(resp.StatusCode, http.StatusOK)
				}

				{
					resp = apiRequest("POST", "/collections/my-collection:find").
						WithBodyJson(JSON{}).Do()

					dec := json.NewDecoder(strings.NewReader(resp.BodyString()))
					expectedDocuments := []JSON{
						myDocuments[1],
					}
					for _, expectedDocument := range expectedDocuments {
						var bodyRow interface{}
						dec.Decode(&bodyRow)
						biff.AssertEqualJson(bodyRow, expectedDocument)
					}
					biff.AssertEqual(resp.StatusCode, http.StatusOK)
				}

			})

			a.Alternative("Patch by fullscan", func(a *biff.A) {

				{
					resp := apiRequest("POST", "/collections/my-collection:patch").
						WithBodyJson(JSON{
							"limit": 10,
							"filter": JSON{
								"name": "Alfonso",
							},
							"patch": JSON{
								"country": "es",
							},
						}).Do()
					Save(resp, "Patch - by fullscan", ``)

					//					biff.AssertEqual(resp.BodyString(), "") // todo: assert body
					biff.AssertEqual(resp.StatusCode, http.StatusOK)
				}

				{
					resp = apiRequest("POST", "/collections/my-collection:find").
						WithBodyJson(JSON{"limit": 10}).Do()

					dec := json.NewDecoder(strings.NewReader(resp.BodyString()))
					expectedDocuments := []JSON{
						{"id": "1", "name": "Alfonso", "country": "es"},
						myDocuments[1],
						{"id": "3", "name": "Alfonso", "country": "es"},
					}
					for _, expectedDocument := range expectedDocuments {
						var bodyRow interface{}
						dec.Decode(&bodyRow)
						biff.AssertEqualJson(bodyRow, expectedDocument)
					}
					biff.AssertEqual(resp.StatusCode, http.StatusOK)
				}

			})

		})

		a.Alternative("Create index - map", func(a *biff.A) {
			resp := apiRequest("POST", "/collections/my-collection:createIndex").
				WithBodyJson(JSON{"name": "my-index", "type": "map", "options": JSON{"field": "id", "sparse": true}}).Do()

			expectedBody := JSON{"type": "map", "name": "my-index", "options": JSON{"field": "id", "sparse": true}}
			biff.AssertEqual(resp.StatusCode, http.StatusCreated)
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)

			a.Alternative("Get index", func(a *biff.A) {
				resp := apiRequest("POST", "/collections/my-collection:getIndex").
					WithBodyJson(JSON{
						"name": "my-index",
					}).Do()
				Save(resp, "Retrieve index", ``)

				biff.AssertEqual(resp.StatusCode, http.StatusOK)
				biff.AssertEqualJson(resp.BodyJson(), expectedBody)
			})

			a.Alternative("List indexes", func(a *biff.A) {
				resp := apiRequest("POST", "/collections/my-collection:listIndexes").Do()
				Save(resp, "List indexes", ``)

				expectedBody := []JSON{{"kind": "", "name": "my-index", "parameters": interface{}(nil)}}
				biff.AssertEqual(resp.StatusCode, http.StatusOK)
				biff.AssertEqualJson(resp.BodyJson(), expectedBody)
			})

			a.Alternative("Insert twice", func(a *biff.A) {
				myDocument := JSON{
					"id":      "my-id",
					"name":    "Fulanez",
					"address": "Elm Street 11",
				}

				apiRequest("POST", "/collections/my-collection:insert").
					WithBodyJson(myDocument).Do()
				resp := apiRequest("POST", "/collections/my-collection:insert").
					WithBodyJson(myDocument).Do()
				Save(resp, "Insert - unique index conflict", ``)

				expectedBody := JSON{
					"error": JSON{
						"description": "Unexpected error",
						"message":     "index add 'my-index': index conflict: field 'id' with value 'my-id'",
					},
				}
				biff.AssertEqual(resp.StatusCode, http.StatusConflict)
				biff.AssertEqual(resp.BodyJson(), expectedBody)
			})

			a.Alternative("Find with unique index", func(a *biff.A) {

				myDocument := JSON{
					"id":      "my-id",
					"name":    "Fulanez",
					"address": "Elm Street 11",
				}
				apiRequest("POST", "/collections/my-collection:insert").
					WithBodyJson(myDocument).Do()

				resp := apiRequest("POST", "/collections/my-collection:find").
					WithBodyJson(JSON{
						"index": "my-index",
						"value": "my-id",
					}).Do()
				Save(resp, "Find - by unique index", ``)

				biff.AssertEqual(resp.BodyJson(), myDocument)
				biff.AssertEqual(resp.StatusCode, http.StatusOK)
			})

		})

		a.Alternative("Create index - btree", func(a *biff.A) {
			resp := apiRequest("POST", "/collections/my-collection:createIndex").
				WithBodyJson(JSON{"name": "my-index", "kind": "btree", "parameters": JSON{"fields": []string{"category", "product"}}}).Do()
			Save(resp, "Create index - btree", ``)

			expectedBody := JSON{"kind": "btree", "name": "my-index", "parameters": JSON{"fields": []interface{}{"category", "product"}}}
			biff.AssertEqual(resp.StatusCode, http.StatusCreated)
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)

			a.Alternative("Insert some documents", func(a *biff.A) {

				documents := []JSON{
					{"id": "1", "category": "fruit", "product": "orange"},
					{"id": "2", "category": "drink", "product": "water"},
					{"id": "3", "category": "drink", "product": "milk"},
					{"id": "4", "category": "fruit", "product": "apple"},
				}

				for _, document := range documents {
					resp := apiRequest("POST", "/collections/my-collection:insert").
						WithBodyJson(document).Do()
					fmt.Println(resp.StatusCode, resp.BodyString())
				}

				a.Alternative("Find with BTree", func(a *biff.A) {
					resp := apiRequest("POST", "/collections/my-collection:find").
						WithBodyJson(JSON{
							"index": "my-index",
						}).Do()
					Save(resp, "Find - by BTree", ``)

					expectedOrderIDs := []string{"3", "2", "4", "1"}

					d := json.NewDecoder(bytes.NewReader(resp.BodyBytes()))
					i := 0
					for {
						item := JSON{}
						err := d.Decode(&item)
						if err == io.EOF {
							break
						}
						biff.AssertEqual(item["id"], expectedOrderIDs[i])
						i++
					}
				})

				a.Alternative("Find with BTree - reverse order", func(a *biff.A) {
					resp := apiRequest("POST", "/collections/my-collection:find").
						WithBodyJson(JSON{
							"index":   "my-index",
							"reverse": true,
						}).Do()
					Save(resp, "Find - by BTree reverse order", ``)

					expectedOrderIDs := []string{"1", "4", "2", "3"}

					d := json.NewDecoder(bytes.NewReader(resp.BodyBytes()))
					i := 0
					for {
						item := JSON{}
						err := d.Decode(&item)
						if err == io.EOF {
							break
						}
						biff.AssertEqual(item["id"], expectedOrderIDs[i])
						i++
					}
				})

			})

		})

		a.Alternative("Find with collection not found", func(a *biff.A) {

			resp := apiRequest("POST", "/collections/your-collection:find").
				WithBodyJson(JSON{}).Do()

			Save(resp, "Find - collection not found", ``)

			errorMessage := resp.BodyJson().(JSON)["error"].(JSON)["message"].(string)
			biff.AssertEqual(errorMessage, "collection not found")
			biff.AssertEqual(resp.StatusCode, http.StatusInternalServerError) // todo: it should return 404
		})

	})

	a.Alternative("Insert on not existing collection", func(a *biff.A) {

		myDocument := JSON{
			"id": "my-id",
		}
		resp := apiRequest("POST", "/collections/my-collection:insert").
			WithBodyJson(myDocument).Do()

		biff.AssertEqual(resp.BodyString(), "")
		biff.AssertEqual(resp.StatusCode, http.StatusCreated)

		a.Alternative("List collection", func(a *biff.A) {

			resp := apiRequest("POST", "/collections/my-collection:find").
				WithBodyJson(JSON{}).Do()

			biff.AssertEqual(resp.BodyString(), "{\"id\":\"my-id\"}\n")
			biff.AssertEqual(resp.StatusCode, http.StatusOK)

		})

	})

	a.Alternative("Create index on not existing collection", func(a *biff.A) {

		resp := apiRequest("POST", "/collections/my-collection:createIndex").
			WithBodyJson(JSON{
				"kind":  "map",
				"field": "id",
			}).Do()

		biff.AssertEqual(resp.StatusCode, http.StatusCreated)
	})

}
