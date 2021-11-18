package box

import (
	"net/url"
	"reflect"
	"runtime"
	"strings"
)

// R stands for Resource
type R struct {
	Attr

	// Path is a literal or placehoder that matches with a portion of the path
	Path string

	// Parent is a reference to parent resource
	Parent *R

	// Children is the list of desdendent resources
	Children []*R

	// Interceptors is the list of actions that will be executed before each
	// action or resource under this resource
	Interceptors []I

	// actions contains all actions (bound and unbound)
	actionsByName map[string]*A

	// bound contains only bound actions
	actionsByHttp map[string]*A
}

func NewResource() *R {
	return &R{
		Attr:          Attr{},
		Path:          "",
		Children:      []*R{},
		actionsByName: map[string]*A{},
		actionsByHttp: map[string]*A{},
		Interceptors:  []I{},
	}
}

// TODO: maybe parameters should be a type `P`
func (r *R) Match(path string, parameters map[string]string) (result *R) {

	parts := strings.SplitN(path, "/", 2)

	current, err := url.QueryUnescape(parts[0])
	if nil != err {
		// TODO: maybe log debug "unescape" error ??
		return
	}

	if strings.HasPrefix(r.Path, "{") && strings.HasSuffix(r.Path, "}") {
		// Match with pattern
		parameter := r.Path
		parameter = strings.TrimPrefix(parameter, "{")
		parameter = strings.TrimSuffix(parameter, "}")
		parameters[parameter] = current
	} else if r.Path == current {
		// Exact match
	} else if r.Path == "*" {
		// Delegation match
		parameters["*"] = path
		return r
	} else {
		// No match
		return nil // TODO: maybe return error no match ?¿?¿?
	}

	if len(parts) == 1 {
		return r
	}

	for _, c := range r.Children {
		if result := c.Match(parts[1], parameters); nil != result {
			return result
		}
	}

	return
}

func (r *R) resourceParts(parts []string) *R {

	if len(parts) == 0 {
		return r
	}

	part := parts[0]
	for _, child := range r.Children {
		if child.Path == part {
			return child.resourceParts(parts[1:])
		}
	}

	child := NewResource()
	child.Path = part
	child.Parent = r
	r.Children = append(r.Children, child)

	return child.resourceParts(parts[1:])
}

// Resource defines a new resource below current resource
func (r *R) Resource(locator string) *R {

	if locator == "" {
		return r
	}

	locator = strings.TrimPrefix(locator, "/")
	parts := strings.Split(locator, "/")

	return r.resourceParts(parts)
}

// Add action to this resource
func (r *R) WithActions(action ...*A) *R {

	for _, a := range action {
		if "" == a.Name {
			a.Name = getFunctionName(a.handler)
			a.Name = actionNameNormalizer(a.Name)
		}
		a.resource = r
		r.actionsByName[a.Name] = a

		h := a.HttpMethod + " "
		if !a.Bound {
			h += a.Name
		}
		r.actionsByHttp[h] = a
	}

	return r
}

// GetActions retrieve the slice of actions defined in this resource
func (r *R) GetActions() []*A {

	var actions []*A
	for _, a := range r.actionsByName {
		actions = append(actions, a)
	}

	return actions
}

// Add interceptor to this resource
func (r *R) WithInterceptors(interceptor ...I) *R {

	for _, i := range interceptor {
		r.Interceptors = append(r.Interceptors, i)
	}

	return r
}

func (r *R) WithAttribute(key string, value interface{}) *R {
	r.SetAttribute(key, value)
	return r
}

func actionNameNormalizer(u string) string {
	if len(u) == 0 {
		return u
	}

	return strings.ToLower(u[0:1]) + u[1:]
}

func getFunctionName(i interface{}) string {
	name := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()

	parts := strings.Split(name, ".")

	return parts[1]
}
