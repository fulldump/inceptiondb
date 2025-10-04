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
			"name":     "my-collection",
			"total":    0,
			"indexes":  0,
			"defaults": map[string]any{"id": "uuid()"},
		}
		zzz := biff.AssertEqualJson(resp.BodyJson(), expectedBody)
		if !zzz {
			fmt.Println("JODERRRRRRR")
		}

		a.Alternative("Retrieve collection", func(a *biff.A) {
			resp := apiRequest("GET", "/collections/my-collection").Do()
			Save(resp, "Retrieve collection", ``)

			biff.AssertEqual(resp.StatusCode, http.StatusOK)
			expectedBody := JSON{
				"name":     "my-collection",
				"total":    0,
				"indexes":  0,
				"defaults": map[string]any{"id": "uuid()"},
			}
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)
		})

		a.Alternative("List collections", func(a *biff.A) {
			resp := apiRequest("GET", "/collections").Do()
			Save(resp, "List collections", ``)

			biff.AssertEqual(resp.StatusCode, http.StatusOK)
			expectedBody := []JSON{
				{
					"name":     "my-collection",
					"total":    0,
					"indexes":  0,
					"defaults": map[string]any{"id": "uuid()"},
				},
			}
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)
		})

		a.Alternative("Drop collection", func(a *biff.A) {
			resp := apiRequest("POST", "/collections/my-collection:dropCollection").
				Do()
			Save(resp, "Drop collection", ``)

			biff.AssertEqual(resp.StatusCode, http.StatusNoContent)

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

			expectedBody := map[string]any{
				"address": "Elm Street 11",
				"id":      "my-id",
				"name":    "Fulanez",
			}
			biff.AssertEqual(resp.BodyJson(), expectedBody)

			a.Alternative("Find with fullscan", func(a *biff.A) {
				resp := apiRequest("POST", "/collections/my-collection:find").
					WithBodyJson(JSON{
						"mode":  "fullscan",
						"skip":  0,
						"limit": 1,
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
					WithBodyJson(JSON{"name": "my-index", "type": "map", "field": "id"}).Do()
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
				a.Alternative("Size", func(a *biff.A) {
					resp := apiRequest("POST", "/collections/my-collection:size").Do()
					Save(resp, "Size - experimental", `
						EXPERIMENTAL!!!

						This will probably be removed, it is extremely inefficient.
					`)
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
				WithBodyJson(JSON{"name": "my-index", "type": "map", "field": "id", "sparse": true}).Do()

			expectedBody := JSON{"type": "map", "name": "my-index", "field": "id", "sparse": true}
			biff.AssertEqual(resp.StatusCode, http.StatusCreated)
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)

			a.Alternative("Drop index", func(a *biff.A) {

				resp := apiRequest("POST", "/collections/my-collection:dropIndex").
					WithBodyJson(JSON{"name": "my-index"}).Do()

				Save(resp, "Drop index", ``)

				biff.AssertEqual(resp.StatusCode, http.StatusNoContent)

				a.Alternative("Insert twice", func(a *biff.A) {

					{
						resp := apiRequest("GET", "/collections/my-collection").Do()
						biff.AssertEqualJson(resp.BodyJson().(JSON)["total"], 0)
					}

					myDocument := JSON{"id": "duplicated-id"}

					apiRequest("POST", "/collections/my-collection:insert").
						WithBodyJson(myDocument).Do()
					apiRequest("POST", "/collections/my-collection:insert").
						WithBodyJson(myDocument).Do()

					{
						resp := apiRequest("GET", "/collections/my-collection").Do()
						biff.AssertEqualJson(resp.BodyJson().(JSON)["total"], 2)
					}
				})

			})

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

				expectedBody := []JSON{{"type": "map", "name": "my-index", "field": "id", "sparse": true}}
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

			a.Alternative("Find - index not found", func(a *biff.A) {
				resp := apiRequest("POST", "/collections/my-collection:find").
					WithBodyJson(JSON{
						"index": "invented",
						"value": "my-id",
					}).Do()
				Save(resp, "Find - index not found", ``)

				expectedBody := JSON{
					"error": JSON{
						"description": "Unexpected error",
						"message":     "index 'invented' not found, available indexes [my-index]",
					},
				}

				biff.AssertEqualJson(resp.BodyJson(), expectedBody)
				biff.AssertEqual(resp.StatusCode, 500) // todo: should be 400
			})

		})

		a.Alternative("Create index - btree compound", func(a *biff.A) {
			resp := apiRequest("POST", "/collections/my-collection:createIndex").
				WithBodyJson(JSON{"name": "my-index", "type": "btree", "fields": []string{"category", "-product"}}).Do()
			Save(resp, "Create index - btree compound", ``)

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
							"skip":  0,
							"limit": 10,
						}).Do()
					Save(resp, "Find - by BTree", ``)

					expectedOrderIDs := []string{"2", "3", "1", "4"}

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
					biff.AssertEqual(i, len(expectedOrderIDs))
				})

			})
		})

		a.Alternative("Create index - btree", func(a *biff.A) {
			resp := apiRequest("POST", "/collections/my-collection:createIndex").
				WithBodyJson(JSON{"name": "my-index", "type": "btree", "fields": []string{"category", "product"}}).Do()
			Save(resp, "Create index - btree", ``)

			expectedBody := JSON{"name": "my-index", "type": "btree", "fields": []interface{}{"category", "product"}, "sparse": false, "unique": false}
			biff.AssertEqual(resp.StatusCode, http.StatusCreated)
			biff.AssertEqual(resp.BodyJson(), expectedBody)

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
							"skip":  0,
							"limit": 10,
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
					biff.AssertEqual(i, len(expectedOrderIDs))
				})

				a.Alternative("Find with BTree with filter", func(a *biff.A) {
					resp := apiRequest("POST", "/collections/my-collection:find").
						WithBodyJson(JSON{
							"index": "my-index",
							"skip":  0,
							"limit": 10,
							"filter": JSON{
								"category": "fruit",
							},
						}).Do()
					Save(resp, "Find - by BTree with filter", ``)

					expectedOrderIDs := []string{"4", "1"}

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
					biff.AssertEqual(i, len(expectedOrderIDs))
				})

				a.Alternative("Remove - BTree ", func(a *biff.A) {
					resp := apiRequest("POST", "/collections/my-collection:find").
						WithBodyJson(JSON{
							"index": "my-index",
							"skip":  0,
							"limit": 10,
						}).Do()
					Save(resp, "Remove - by BTree with filter", ``)

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
					biff.AssertEqual(i, len(expectedOrderIDs))
				})

				a.Alternative("Remove - BTree with filter", func(a *biff.A) {
					resp := apiRequest("POST", "/collections/my-collection:find").
						WithBodyJson(JSON{
							"index": "my-index",
							"skip":  0,
							"limit": 10,
							"filter": JSON{
								"category": "fruit",
							},
						}).Do()
					Save(resp, "Remove - by BTree with filter", ``)

					expectedOrderIDs := []string{"4", "1"}

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
					biff.AssertEqual(i, len(expectedOrderIDs))
				})

				a.Alternative("Find with BTree - reverse order", func(a *biff.A) {
					resp := apiRequest("POST", "/collections/my-collection:find").
						WithBodyJson(JSON{
							"index":   "my-index",
							"skip":    0,
							"limit":   10,
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

		a.Alternative("Set defaults", func(a *biff.A) {
			resp := apiRequest("POST", "/collections/my-collection:setDefaults").
				WithBodyJson(JSON{
					"c":        "auto()",
					"n":        "auto()",
					"name":     "",
					"street":   "",
					"verified": false,
				}).Do()

			expectedBody := JSON{
				"id":       "uuid()",
				"c":        "auto()",
				"n":        "auto()",
				"name":     "",
				"street":   "",
				"verified": false,
			}
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)

			a.Alternative("Insert with defaults - overwrite field", func(a *biff.A) {
				resp := apiRequest("POST", "/collections/my-collection:insert").
					WithBodyJson(JSON{
						"name": "fulanez",
					}).Do()
				expectedBody := JSON{
					"id":       resp.BodyJson().(JSON)["id"],
					"c":        1,
					"n":        1,
					"name":     "fulanez",
					"street":   "",
					"verified": false,
				}
				biff.AssertEqualJson(resp.BodyJson(), expectedBody)
			})

			a.Alternative("Insert with defaults - new field", func(a *biff.A) {
				resp := apiRequest("POST", "/collections/my-collection:insert").
					WithBodyJson(JSON{
						"new": "field",
					}).Do()
				expectedBody := JSON{
					"id":       resp.BodyJson().(JSON)["id"],
					"c":        1,
					"n":        1,
					"name":     "",
					"street":   "",
					"verified": false,
					"new":      "field",
				}
				biff.AssertEqualJson(resp.BodyJson(), expectedBody)
			})

		})

		a.Alternative("Set defaults - example", func(a *biff.A) {
			resp := apiRequest("POST", "/collections/my-collection:setDefaults").
				WithBodyJson(JSON{
					"created_on": "unixnano()",
					"name":       "",
					"street":     "",
					"verified":   false,
				}).Do()

			Save(resp, "Set defaults", `

				The ´SetDefaults´ function is designed to automatically assign predefined default values to specific
				fields in a document when a new entry is added to a database collection. This ensures consistency and 
				completeness of data, especially for fields that require a default state or value.

				## Overview

				When you insert a new document into the collection, ´SetDefaults´ intervenes by checking for any fields 
				that have not been explicitly provided in the input document. For such fields, if default values have 
				been predefined using SetDefaults, those values are automatically added to the document before it is 
				inserted into the collection. This process is seamless and ensures that every new document adheres to 
				a defined structure and contains all necessary information.

				## Example usage

				Consider a scenario where you are adding a new user record to a collection but only provide the user's
				name. If ´SetDefaults´ has been configured for the collection, it will automatically fill in any missing
				fields that have default values defined.

				### Input Document

				When you attempt to insert a document with just the user's name:

				´´´json
				{
				  "name": "Fulanez"
				}
				´´´

				### Predefined Defaults

				Assume the following default values have been set for the collection:

				´´´json
				{
					"id": "uuid()",      // A function generating a unique identifier
					"verified": false    // A boolean flag set to false by default
				}
				´´´
				
				### Resulting Document
				
				With ´SetDefaults´ applied, the document that gets inserted into the collection will include the missing
				fields with their default values:
				
				´´´json
				{
				  "id": "3bb5afae-c7b7-11ee-86b0-4f000ceb9a36", // Generated unique ID
				  "name": "Fulanez",                             // Provided by the user
				  "verified": false                              // Default value
				}
				´´´

				## Benefits

				* **Consistency**: Ensures that all documents in the collection follow a consistent structure, even when
				some data points are not provided during insertion.
				* **Completeness**: Guarantees that essential fields are always populated, either by the user or through
				default values, ensuring data integrity.
				* **Efficiency**: Saves time and effort by automating the assignment of common default values, reducing 
				the need for manual data entry or post-insertion updates.

				## Configuration

				To utilize ´SetDefaults´, you must first define the default values for the desired fields in your 
				collection's configuration. This typically involves specifying a field name and its corresponding 
				default value or function (e.g., uuid() for generating unique identifiers).

				It's important to note that ´SetDefaults´ only applies to new documents being inserted into the 
				collection. It does not affect documents that are already present in the collection or those being 
				updated.

				## Generative Functions in ´SetDefaults´

				´SetDefaults´ supports a variety of generative functions to automatically assign dynamic values to 
				fields in new documents. These functions are executed at the time of document insertion, ensuring that 
				each entry receives a unique or contextually appropriate value based on the specified function. Below is
				a list of supported generative functions:

				### 1. ´uuid()´

				**Description**: Generates a Universally Unique Identifier (UUID) for the document. This is particularly
				useful for assigning a unique identifier to each entry, ensuring that each document can be distinctly 
				identified within the collection.
				
				**Example Usage**: Ideal for fields requiring a unique ID, such as user identifiers, transaction IDs, etc.
				
				**Output Example**: ´"id": "3bb5afae-c7b7-11ee-86b0-4f000ceb9a36"´
				
				### 2. ´unixnano()´
				**Description**: Produces a numerical value representing the current time in Unix nanoseconds. This 
				function is handy for timestamping documents at the exact time of their creation, providing 
				high-resolution time tracking.
				
				**Example Usage**: Suitable for fields that need to record the precise time of document insertion, 
				like creation timestamps, log entries, etc.
				
				**Output Example**: ´"created_at": 16180339887467395´ (represents the number of nanoseconds since 
				January 1, 1970, 00:00:00 UTC)
				
				### 3. ´auto()´
				**Description**: Implements an automatic row counter that increments with each insert, starting from 
				the first insertion. This function is beneficial for maintaining a sequential order or count of the
				documents added to the collection.
				
				**Example Usage**: Useful for auto-increment fields, such as a serial number, order number, or any
				scenario where a simple, incrementing counter is needed.
				
				**Output Example**: ´"serial_number": 1023´ (where 1023 is the current count of documents inserted 
				since the first one)
				
				### Implementation Considerations

				When integrating generative functions with ´SetDefaults´, consider the following:
				
				**Uniqueness**: Functions like uuid() guarantee uniqueness, making them ideal for identifiers.

				**Temporal Precision**: unixnano() provides high-precision timestamps, useful for time-sensitive data.

				**Sequential Integrity**: auto() ensures a consistent, incremental sequence, beneficial for ordering or 
				numbering entries.

				Ensure that the chosen generative function aligns with the field's purpose and the overall data model's 
				requirements. Proper configuration of ´SetDefaults´ with these functions enhances data integrity, 
				consistency, and utility within your application.

			`)

			expectedBody := JSON{
				"id":         "uuid()",
				"created_on": "unixnano()",
				"name":       "",
				"street":     "",
				"verified":   false,
			}
			biff.AssertEqualJson(resp.BodyJson(), expectedBody)
		})

		a.Alternative("Set defaults - auto", func(a *biff.A) {
			apiRequest("POST", "/collections/my-collection:setDefaults").
				WithBodyJson(JSON{
					"id": nil,
					"n":  "auto()",
				}).Do()

			a.Alternative("Insert multiple", func(a *biff.A) {
				for i := 1; i <= 4; i++ {
					resp := apiRequest("POST", "/collections/my-collection:insert").
						WithBodyJson(JSON{}).Do()

					expectedBody := JSON{
						"n": i,
					}
					biff.AssertEqualJson(resp.BodyJson(), expectedBody)
				}
			})

		})

	})

	a.Alternative("Insert on not existing collection", func(a *biff.A) {

		myDocument := JSON{
			"id": "my-id",
		}
		resp := apiRequest("POST", "/collections/my-collection:insert").
			WithBodyJson(myDocument).Do()

		expectedBody := map[string]any{
			"id": "my-id",
		}
		biff.AssertEqual(resp.BodyJson(), expectedBody)
		biff.AssertEqual(resp.StatusCode, http.StatusCreated)

		a.Alternative("List collection", func(a *biff.A) {

			resp := apiRequest("POST", "/collections/my-collection:find").
				WithBodyJson(JSON{}).Do()

			biff.AssertEqual(resp.BodyString(), "{\"id\":\"my-id\"}\n")
			biff.AssertEqual(resp.StatusCode, http.StatusOK)

		})

	})

	// todo review this alternative
	a.Alternative("Create index on not existing collection", func(a *biff.A) {

		resp := apiRequest("POST", "/collections/my-collection:createIndex").
			WithBodyJson(JSON{
				"kind":  "map",
				"field": "id",
			}).Do()

		biff.AssertEqual(resp.StatusCode, http.StatusInternalServerError)
	})

}
