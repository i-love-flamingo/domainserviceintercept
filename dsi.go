package dsi

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"

	"go.opencensus.io/trace"
	"gopkg.in/yaml.v2"
)

// Traceserver serves data/traces
func Traceserver() {
	http.ListenAndServe(":13211", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("content-type", "text/html; charset=utf-8")

		fmt.Fprint(writer, "<html><body>")
		if request.URL.Query().Get("expand") != "" {
			fmt.Fprint(writer, "<a href=\"?\">List All</a><br/>")
		} else {
			//fmt.Fprint(writer, "<a href=\"?expand=1\">Expand</a><br/>")
		}

		fmt.Fprint(writer, "<pre>")

		pt := ""
		for _, t := range traces {
			if request.URL.Query().Get("expand") != "" && request.URL.Query().Get("expand") != t.c {
				continue
			}

			if pt != "" && pt != t.c {
				fmt.Fprint(writer, "\n")
			}
			var args []string
			for k, v := range t.args {
				for reflect.ValueOf(v).Kind() == reflect.Ptr {
					v = reflect.ValueOf(v).Elem().Interface()
				}
				args = append(args, k+"="+fmt.Sprintf("%#v", v))
			}
			fmt.Fprintf(writer, "<a href=\"?expand=%s\">%s</a> @ %s: %s(%v)\n", t.c, t.c, t.t.Format("15:04:05.000000000"), t.op, strings.Join(args, ", "))
			if request.URL.Query().Get("expand") != "" {
				b, _ := yaml.Marshal(t.out)
				fmt.Fprintf(writer, "%s\n", string(b))
			}
			pt = t.c
		}

		loadYaml()
	}))
}

type t struct {
	c    string
	t    time.Time
	op   string
	args map[string]interface{}
	out  []interface{}
}

var traces []t
var tracelock = new(sync.Mutex)

func match(check string, args map[string]interface{}) bool {
	t, err := template.New("").Parse(`{{if (` + check + `)}}ok{{end}}`)
	if err != nil {
		panic(err)
	}
	res := new(bytes.Buffer)
	myargs := make(map[string]interface{}, len(args))
	for k, v := range args {
		var prev interface{}
		for reflect.ValueOf(v).Kind() == reflect.Ptr {
			prev = v
			v = reflect.ValueOf(v).Elem().Interface()
		}
		if prev != nil && reflect.TypeOf(v).Kind() == reflect.Struct {
			v = prev
		}
		myargs[k] = v
	}
	err = t.Execute(res, myargs)
	if err != nil {
		panic(err)
	}
	return res.String() == "ok"
}

// Traceme collects available data
func Traceme(ctx context.Context, op string, args map[string]interface{}, load func(), out ...interface{}) {
	id := "0000000000000000"
	if span := trace.FromContext(ctx); span != nil {
		id = span.SpanContext().TraceID.String()
	}

	mustload := true
	for _, b := range fakedata {
		if b.What == op {
			if match(b.Match, args) {
				if b.Return != nil {
					mustload = false
					x, _ := yaml.Marshal(b.Return)
					o := out[0]
					yaml.Unmarshal(x, o)
					log.Println("replaced", b.Return)
				} else if b.Patch != nil {
					mustload = false
					load()
					x, _ := yaml.Marshal(b.Patch)
					o := out[0]
					yaml.Unmarshal(x, o)
					log.Println("patched", b.Patch)
				}
			}
		}
	}

	if mustload {
		load()
	}

	tracelock.Lock()
	traces = append([]t{{
		c:    id,
		t:    time.Now(),
		op:   op,
		args: args,
		out:  out,
	}}, traces...)
	tracelock.Unlock()
}

type bla struct {
	What  string
	Match string
	//In     string
	Return interface{}
	Patch  interface{}
}

var fakedata []bla

func loadYaml() {
	b, _ := ioutil.ReadFile("config/patch.yaml")
	yaml.Unmarshal(b, &fakedata)
}
func init() {
	loadYaml()
}
