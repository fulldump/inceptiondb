package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/fulldump/apitest"
)

// Save generates the MD of the tests.
func Save(response *apitest.Response, title, description string) {

	request := response.Request

	s := ""

	s += "# " + title + "\n"
	s += md_description(description) + "\n"

	s += "Curl example:\n\n"

	s += "```sh\n"

	method := request.Method
	if "GET" == method {
		method = ""
	} else {
		method = "-X " + method + " "
	}

	query := request.URL.RawQuery
	if "" != query {
		query = "?" + query
	}

	s += "curl " + method + "\"https://example.com" + request.URL.Path + query + "\""
	for k, l := range request.Header {
		for _, v := range l {
			s += " \\\n-H \"" + k + ": " + v + "\""
		}
	}
	requestBody := formatJSON(response.BodyRequestString())
	if "" != requestBody {
		s += " \\\n-d '" + requestBody + "'"
	}

	s += "\n```\n\n\n"

	s += "HTTP request/response example:\n\n"

	s += "```http\n"

	// Request
	s += request.Method + " " + request.URL.Path + query + " " + request.Proto + "\n"
	s += "Host: " + "example.com" + "\n"
	for k, l := range request.Header {
		for _, v := range l {
			s += k + ": " + v + "\n"
		}
	}
	s += "\n"

	s += formatJSON(response.BodyRequestString()) + "\n\n"

	// Response
	s += response.Proto + " " + response.Status + "\n"

	headerKeys := []string{}
	for k := range response.Header {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)

	for _, k := range headerKeys {
		if k == "Date" {
			s += "Date: Mon, 15 Aug 2022 02:08:13 GMT\n"
		} else {
			for _, v := range response.Header[k] {
				s += k + ": " + v + "\n"
			}
		}
	}
	s += "\n"

	// Response body
	responseBody := formatJSON(response.BodyString())

	s += responseBody + "\n" // response body

	s += "```\n\n\n"

	// Save markdown
	writeFile(strings.ToLower(title)+".md", s)
}

func formatJSON(body string) string {

	var i interface{}

	err := json.Unmarshal([]byte(body), &i)
	if nil != err {
		return body
	}

	bytes, err := json.MarshalIndent(i, "", "    ")
	if nil != err {
		return body
	}

	return string(bytes)
}

func writeFile(filename, text string) {
	if text == "" {
		return
	}
	filename = strings.Replace(filename, " ", "_", -1)
	examplesPath := os.Getenv("API_EXAMPLES_PATH")
	if examplesPath != "" {
		p := path.Join(examplesPath, path.Clean(filename))
		fmt.Println("Saving", p)
		err := os.WriteFile(p, []byte(text), 0666)
		if nil != err {
			fmt.Println("Saving err:", err)
		}
	}
}

func md_description(d string) string {
	d = md_crop_tabs(d)
	d = strings.Replace(d, "\n´´´", "\n```", -1)
	// d = strings.Replace(d, "´", "`", -1)
	return d
}

func md_crop_tabs(d string) string {
	// Split lines
	lines := strings.Split(d, "\n")

	first := 0
	last := len(lines)
	if len(lines) > 2 {
		first++
		last--
	}

	// Get min tabs
	min_tabs := 99999
	for _, line := range lines[first:last] {
		// if 0 == i {
		// 	continue
		// }
		if strings.TrimSpace(line) != "" {
			c := md_count_tabs(line)
			if min_tabs > c {
				min_tabs = c
			}
		}
	}

	// Prefix
	prefix := strings.Repeat("\t", min_tabs)

	// Do the work
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, prefix)
	}

	return strings.Join(lines, "\n")
}

func md_count_tabs(d string) int {
	i := 0
	for _, c := range d {
		if c != '\t' {
			break
		}
		i++
	}

	return i
}
