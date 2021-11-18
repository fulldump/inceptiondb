package box

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
)

func Box2Http(b *B) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()

		// Put box context
		c := &C{
			Parameters: map[string]string{},
			Request:    r,
			Response:   w,
		}
		ctx = setBoxContext(ctx, c)

		// Split url action
		urlResource, urlAction := splitAction(r.URL.EscapedPath())

		// Match resource
		c.Resource = b.Match(urlResource, c.Parameters)
		if nil == c.Resource {
			err := errors.New("RESOURCE " + r.URL.EscapedPath() + " NOT FOUND!!!")
			SetError(ctx, err)
			return
		}

		// Match action
		c.Action = c.Resource.actionsByHttp[r.Method+" "+urlAction]
		if nil == c.Action {
			err := errors.New("Action " + urlAction + " not found!!")
			SetError(ctx, err)
			return
		}

		// Interceptors
		interceptors := []I{}
		{
			ri := c.Resource
			for {
				if nil == ri {
					break
				}
				interceptors = append(ri.Interceptors, interceptors...)
				ri = ri.Parent
			}
		}
		interceptors = append(interceptors, c.Action.Interceptors...)

		hi := func(ctx context.Context) {

			switch h := c.Action.handler.(type) {
			case func(http.ResponseWriter, *http.Request):
				h(c.Response, r)

			case func(context.Context):
				h(ctx)

			default: // Introspection!!!

				v := reflect.ValueOf(h)
				if v.Kind() != reflect.Func {
					// TODO: sendError(w, ErrorMethodNotAllowed("Internal server error: Unsupported handler"))
					return
				}

				objects := map[reflect.Type]interface{}{}
				objects[reflect.TypeOf(ctx).Elem()] = ctx
				objects[reflect.TypeOf(w).Elem()] = c.Response
				objects[reflect.TypeOf(r)] = r

				bodies := []interface{}{}

				in := []reflect.Value{}

				t := v.Type()
				for i := 0; i < t.NumIn(); i++ {
					argument := t.In(i)

					if argument.Kind() == reflect.Interface {
						ctx_type := reflect.TypeOf(ctx)
						w_type := reflect.TypeOf(w)
						if ctx_type.Implements(argument) {
							argument = ctx_type.Elem()
						} else if w_type.Implements(argument) {
							argument = w_type.Elem()
						}
					}

					if object, exists := objects[argument]; exists {
						in = append(in, reflect.ValueOf(object))
					} else {
						body := reflect.New(argument).Interface()
						bodies = append(bodies, body)

						in = append(in, reflect.ValueOf(body).Elem())
					}
				}

				if len(bodies) == 1 {
					err := unserialize(ctx, r.Body, bodies[0])
					if nil != err {
						SetError(ctx, err)
						return
					}
				} else if len(bodies) > 1 {
					// TODO: limit memory usage with io.LimitReader(r.Body, 1024*1024)
					b, err := ioutil.ReadAll(r.Body)
					if nil != err {
						SetError(ctx, err)
						return
					}
					buf := bytes.NewBuffer(b)
					for _, body := range bodies {
						unserialize(ctx, buf, body) // TODO: what if one item fails?
					}
				}

				out := v.Call(in)

				var genericResponse = (interface{})(nil)
				for _, o := range out {
					switch v := o.Interface().(type) {
					case error:
						SetError(ctx, v)
						return
					case nil:
						// do nothing
					default:
						genericResponse = v
					}
				}

				if isNil(genericResponse) {
					// TODO: write empty response
					return
				}

				err := serialize(ctx, c.Response, genericResponse)
				if nil != err {
					// TODO: log serializer error
				}
			}
		}

		interceptorsLen := len(interceptors)
		for i := interceptorsLen - 1; i >= 0; i-- {
			hi = interceptors[i](hi)
		}
		hi(ctx)

	})

}

// TODO: put this into box
func unserialize(c context.Context, r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}

// TODO: put this into box
func serialize(c context.Context, w io.Writer, v interface{}) error {
	return json.NewEncoder(w).Encode(v)
}

func isNil(a interface{}) bool {
	if a == nil {
		return true
	} else {
		switch v := reflect.ValueOf(a); v.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
			return v.IsNil()
		}
	}
	return false
}

func splitAction(url string) (locator, action string) {

	parts := strings.Split(url, ":")
	l := len(parts)
	if l > 1 {
		action = parts[l-1]
		locator = strings.Join(parts[:l-1], ":")
	} else {
		locator = url
	}
	return
}
