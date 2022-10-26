package service

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fulldump/apitest"
	"github.com/fulldump/biff"
)

type JSON = map[string]interface{}

func Acceptance(a *biff.A, base string, apiRequest func(method, path string) *apitest.Request) {

	a.Alternative("Create collection", func(a *biff.A) {
		resp := apiRequest("POST", base+"/collections").
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
			resp := apiRequest("GET", base+"/collections/my-collection").
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
			resp := apiRequest("GET", base+"/collections").Do()
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
			resp := apiRequest("POST", base+"/collections/my-collection:dropCollection").
				Do()
			Save(resp, "Drop collection", ``)

			biff.AssertEqual(resp.StatusCode, http.StatusOK)

			a.Alternative("Get dropped collection", func(a *biff.A) {
				resp := apiRequest("GET", base+"/collections/my-collection").
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
			resp := apiRequest("POST", base+"/collections/my-collection:insert").
				WithBodyJson(myDocument).Do()
			Save(resp, "Insert one", ``)

			biff.AssertEqual(resp.StatusCode, http.StatusCreated)
			biff.AssertEqual(resp.BodyString(), "")

			a.Alternative("Find with fullscan", func(a *biff.A) {
				resp := apiRequest("POST", base+"/collections/my-collection:find").
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
			resp := apiRequest("POST", base+"/collections/my-collection:insert").
				WithBodyString(body).Do()
			Save(resp, "Insert many", ``)

			a.Alternative("Create index", func(a *biff.A) {
				resp := apiRequest("POST", base+"/collections/my-collection:createIndex").
					WithBodyJson(JSON{"field": "id", "sparse": true}).Do()
				Save(resp, "Create index", ``)

				a.Alternative("Delete by index", func(a *biff.A) {
					resp := apiRequest("POST", base+"/collections/my-collection:remove").
						WithBodyJson(JSON{
							"mode":  "unique",
							"field": "id",
							"value": "1",
						}).Do()
					Save(resp, "Delete - by index", ``)

					biff.AssertEqualJson(resp.BodyJson(), myDocuments[0])
					biff.AssertEqual(resp.StatusCode, http.StatusOK)
				})
				a.Alternative("Patch by index", func(a *biff.A) {
					resp := apiRequest("POST", base+"/collections/my-collection:patch").
						WithBodyJson(JSON{
							"mode":  "unique",
							"field": "id",
							"value": "3",
							"patch": JSON{
								"name": "Pedro",
							},
						}).Do()
					Save(resp, "Patch - by index", ``)

					biff.AssertEqualJson(resp.BodyString(), "")
					biff.AssertEqual(resp.StatusCode, http.StatusOK)

					{
						resp = apiRequest("POST", base+"/collections/my-collection:find").
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
					resp := apiRequest("POST", base+"/collections/my-collection:remove").
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
					resp = apiRequest("POST", base+"/collections/my-collection:find").
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
					resp := apiRequest("POST", base+"/collections/my-collection:patch").
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

					biff.AssertEqual(resp.BodyString(), "")
					biff.AssertEqual(resp.StatusCode, http.StatusOK)
				}

				{
					resp = apiRequest("POST", base+"/collections/my-collection:find").
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

		a.Alternative("Create index", func(a *biff.A) {
			resp := apiRequest("POST", base+"/collections/my-collection:createIndex").
				WithBodyJson(JSON{"field": "id", "sparse": true}).Do()

			expectedBody := JSON{"field": "id", "name": "id", "sparse": true}
			biff.AssertEqual(resp.StatusCode, http.StatusCreated)
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)

			a.Alternative("Get index", func(a *biff.A) {
				resp := apiRequest("POST", base+"/collections/my-collection:getIndex").
					WithBodyJson(JSON{
						"name": "id",
					}).Do()
				Save(resp, "Retrieve index", ``)

				biff.AssertEqual(resp.StatusCode, http.StatusOK)
				biff.AssertEqualJson(resp.BodyJson(), expectedBody)
			})

			a.Alternative("List indexes", func(a *biff.A) {
				resp := apiRequest("POST", base+"/collections/my-collection:listIndexes").Do()
				Save(resp, "List indexes", ``)

				expectedBody := []JSON{{"field": "id", "name": "id", "sparse": true}}
				biff.AssertEqual(resp.StatusCode, http.StatusOK)
				biff.AssertEqualJson(resp.BodyJson(), expectedBody)
			})

			a.Alternative("Insert twice", func(a *biff.A) {
				myDocument := JSON{
					"id":      "my-id",
					"name":    "Fulanez",
					"address": "Elm Street 11",
				}

				apiRequest("POST", base+"/collections/my-collection:insert").
					WithBodyJson(myDocument).Do()
				resp := apiRequest("POST", base+"/collections/my-collection:insert").
					WithBodyJson(myDocument).Do()
				Save(resp, "Insert - unique index conflict", ``)

				expectedBody := JSON{
					"error": JSON{
						"description": "Unexpected error",
						"message":     "index conflict: field 'id' with value 'my-id'",
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
				apiRequest("POST", base+"/collections/my-collection:insert").
					WithBodyJson(myDocument).Do()

				resp := apiRequest("POST", base+"/collections/my-collection:find").
					WithBodyJson(JSON{
						"mode":  "unique",
						"field": "id",
						"value": "my-id",
					}).Do()
				Save(resp, "Find - by unique index", ``)

				biff.AssertEqual(resp.BodyJson(), myDocument)
				biff.AssertEqual(resp.StatusCode, http.StatusOK)
			})

		})

		a.Alternative("Find with {invalid} mode", func(a *biff.A) {

			resp := apiRequest("POST", base+"/collections/my-collection:find").
				WithBodyJson(JSON{
					"mode": "{invalid}",
				}).Do()

			Save(resp, "Find - bad request", ``)

			errorMessage := resp.BodyJson().(JSON)["error"].(JSON)["message"].(string)
			biff.AssertEqual(errorMessage, "bad mode '{invalid}', must be [fullscan|unique]. See docs: TODO")
			biff.AssertEqual(resp.StatusCode, http.StatusBadRequest)
		})

	})

}
