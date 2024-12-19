package boxopenapi

import (
	"context"
	"net/http"
	"reflect"
	"strings"

	"github.com/fulldump/box"
)

type JSON = map[string]any

// Spec returns a valid OpenAPI spec from a box router ready to be used.
func Spec(api *box.B) OpenAPI {

	paths := JSON{}
	schemas := JSON{}

	walk(api.R, func(r *box.R) {

		if len(r.GetActions()) == 0 {
			return
		}

		methods := JSON{}

		path := getPath(r)

		parameters := getParams(r)

		for _, action := range r.GetActions() {
			method := strings.ToLower(action.HttpMethod)

			inputs, outputs, _ := describeHandler(action.GetHandler())

			X := JSON{
				"operationId": action.Name,
				"responses":   JSON{},
			}

			if parameters != nil {
				X["parameters"] = parameters
			}

			if len(inputs) == 1 {
				X["requestBody"] = JSON{
					"description": "TODO",
					"required":    true,
					"content": JSON{
						"application/json": JSON{
							"schema": schema(schemas, inputs[0]),
						},
					},
				}
			}

			if len(outputs) == 1 {
				X["responses"] = JSON{
					"default": JSON{
						"description": "some human description",
						"content": JSON{
							"application/json": JSON{
								"schema": schema(schemas, outputs[0]),
							},
						},
					},
				}
			}

			if action.Bound {
				methods[method] = X
			} else {
				paths[path+":"+action.Name] = X
			}
		}

		paths[path] = methods
	})

	return OpenAPI{
		Openapi: "3.1.0",
		Info: Info{
			Title:   "BoxOpenAPI",
			Version: "1",
		},
		Servers: []Server{
			{
				Url: "http://localhost:8080",
			},
		},
		Paths: paths,
		Components: JSON{
			"schemas": schemas,
		},
	}
}

func schemaStruct(schemas JSON, item reflect.Type) JSON {

	name := item.Name()
	{
		// Normalize name:
		name = strings.TrimRight(name, "]")
		parts := strings.Split(name, ".")
		name = parts[len(parts)-1]
	}

	result := JSON{
		"$ref": "#/components/schemas/" + name,
	}

	if _, exists := schemas[name]; exists {
		return result
	}

	properties := JSON{}
	definition := JSON{
		"type":       "object",
		"required":   []string{},
		"properties": properties,
	}
	schemas[name] = definition

	// follow pointers
	for item.Kind() == reflect.Ptr {
		item = item.Elem()
	}

	for i := 0; i < item.NumField(); i++ {
		f := item.Field(i)

		definition := schemaAny(schemas, f.Type)

		if v, ok := f.Tag.Lookup("description"); ok {
			definition["description"] = v
		}

		name := f.Tag.Get("json")
		if name == "" {
			name = f.Name
		}

		properties[name] = definition
	}

	return result
}

func schemaAny(schemas JSON, item reflect.Type) JSON {

	// follow pointers
	for item.Kind() == reflect.Ptr {
		item = item.Elem()
	}

	if item.String() == "time.Time" {
		return JSON{
			"type":     "string",
			"format":   "date-time",
			"examples": []any{"2006-01-02T15:04:05Z07:00"},
		}
	}

	switch item.Kind() {
	case reflect.Struct:
		return schemaStruct(schemas, item)

	case reflect.Array, reflect.Slice:

		item = item.Elem()

		// follow pointers
		for item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		return JSON{
			"type":  "array",
			"items": schemaAny(schemas, item),
		}

	case reflect.String:
		return JSON{
			"type": "string",
		}
	case reflect.Bool:
		return JSON{
			"type": "boolean",
		}
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8,
		reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8,
		reflect.Float64, reflect.Float32:
		return JSON{
			"type": "number",
		}
	}

	return JSON{}
}

func schema(schemas JSON, item reflect.Type) any {

	// follow pointers
	for item.Kind() == reflect.Ptr {
		item = item.Elem()
	}

	switch item.Kind() {
	case reflect.Struct:
		return schemaStruct(schemas, item)

	case reflect.Array, reflect.Slice:

		item = item.Elem()

		// follow pointers
		for item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		return JSON{
			"type":  "array",
			"items": schemaAny(schemas, item),
		}
	default:
		return schemaAny(schemas, item)
	}

}

func getPath(r *box.R) string {
	result := []string{}

	for r != nil {
		result = append([]string{r.Path}, result...)
		r = r.Parent
	}

	return strings.Join(result, "/")
}

func getParams(r *box.R) []JSON {

	var result []JSON

	for r != nil {
		if strings.HasPrefix(r.Path, "{") && strings.HasSuffix(r.Path, "}") {
			name := r.Path
			name = strings.TrimLeft(name, "{")
			name = strings.TrimRight(name, "}")
			result = append([]JSON{JSON{
				"name":     name,
				"in":       "path",
				"required": true,
				// "description": "",
				"schema": JSON{
					"type": "string",
				},
			}}, result...)
		}
		r = r.Parent
	}

	return result
}

func walk(n *box.R, f func(r *box.R)) {

	f(n)

	for _, child := range n.Children {
		walk(child, f)
	}

}

func getReflectObjectTypes() map[reflect.Type]bool {
	objects := map[reflect.Type]bool{}
	objects[reflect.TypeOf(context.Context(nil))] = true
	objects[reflect.TypeOf(http.ResponseWriter(nil))] = true
	objects[reflect.TypeOf(&http.Request{})] = true
	return objects
}

func describeHandler(handler interface{}) (inputs, outputs, errors []reflect.Type) {
	// inputs = make([]reflect.Type, 0)
	// outputs = make([]reflect.Type, 0)
	// errors = make([]reflect.Type, 0)
	v := reflect.ValueOf(handler)
	if v.Kind() != reflect.Func {
		return
	}
	objects := getReflectObjectTypes()
	t := v.Type()
	for i := 0; i < t.NumIn(); i++ {
		argument := t.In(i)
		if argument.Kind() == reflect.Interface {
			if argument.String() == "context.Context" {
				continue
			}
			if argument.String() == "http.ResponseWriter" {
				continue
			}
		}
		_, exists := objects[argument]
		if !exists {
			inputs = append(inputs, argument)
		}
	}
	for i := 0; i < t.NumOut(); i++ {
		argument := t.Out(i)
		if argument.String() == "error" {
			errors = append(errors, argument)
		} else {
			outputs = append(outputs, argument)
		}
	}
	return
}

func mergeLeft(a, b *JSON) {

	for k, vb := range *b {

		va := (*a)[k]

		ca, caOk := isRec(va)
		cb, cbOk := isRec(vb)

		if caOk && cbOk {
			mergeLeft(ca, cb)
		} else {
			(*a)[k] = vb
		}

	}

}

func isRec(i interface{}) (*JSON, bool) {

	switch r := i.(type) {
	case JSON:
		return &r, true
	case *JSON:
		return r, true
	default:
		return nil, false
	}

	return nil, false
}
