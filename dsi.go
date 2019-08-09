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

func preresponse(writer http.ResponseWriter) {
	writer.Header().Set("content-type", "text/html; charset=utf-8")
	fmt.Fprint(writer, "<html><body>")
	fmt.Fprint(writer, "<a href=\"/\">List All</a> | <a href=\"/?clear=1\">Clear All</a> | <a href=\"/?reload=1\">Reload patch.yaml</a> | <a href=\"/?setconfig=1\">Config</a><br/>")
}

// Traceserver serves data/traces
func Traceserver() {
	log.Print(http.ListenAndServe(":13211", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()

		switch {
		case request.URL.Query().Get("clear") == "1":
			traceMutex.Lock()
			traces = nil
			traceMutex.Unlock()
			http.Redirect(writer, request, "/", http.StatusTemporaryRedirect)
			return

		case request.URL.Query().Get("reload") == "1":
			loadYaml()
			http.Redirect(writer, request, "/", http.StatusTemporaryRedirect)
			return

		case request.URL.Query().Get("setconfig") == "1":
			if request.Method == http.MethodPost {
				yaml.Unmarshal([]byte(request.FormValue("config")), &patchconfig)
				http.Redirect(writer, request, "/", http.StatusTemporaryRedirect)
				return
			}

			preresponse(writer)
			b, _ := yaml.Marshal(patchconfig)
			fmt.Fprintf(writer, `
<form action="?setconfig=1" method="post">
<button type="submit">Set config</button><br/>
<textarea name="config" style="visibility: hidden;" id="configta">%s</textarea>
</form>
<div id="config" style="height: 600px; width: 1000px; position: absolute;">%s</div>`, string(b), string(b))
			fmt.Fprintf(writer, `
<script src="https://pagecdn.io/lib/ace/1.4.5/ace.js" type="text/javascript" charset="utf-8"></script>
<script>
    var editor = ace.edit("config");
    editor.setTheme("ace/theme/monokai");
    editor.session.setMode("ace/mode/yaml");
	var textarea = document.getElementById("configta");
	editor.getSession().setValue(textarea.value);
	editor.getSession().on('change', function(){
	 textarea.value = editor.getSession().getValue();
	});
</script>
`)
			return

		case request.URL.Query().Get("dump") != "":
			preresponse(writer)
			fmt.Fprint(writer, "<pre>")
			var bla []patch
			for _, t := range traces {
				if t.c != request.URL.Query().Get("dump") && request.URL.Query().Get("dump") != "all" {
					continue
				}
				match := []string{"1"}
				for k, v := range t.args {
					for reflect.ValueOf(v).Kind() == reflect.Ptr {
						v = reflect.ValueOf(v).Elem().Interface()
					}
					if reflect.ValueOf(v).Kind() == reflect.Struct {
						continue
					}
					match = append(match, fmt.Sprintf("eq .%s %v", k, v))
				}
				bla = append(bla, patch{
					What:   t.op,
					Match:  "and (" + strings.Join(match, ") (") + ")",
					Return: t.out[0],
					Repeat: 1,
				})
			}
			b, _ := yaml.Marshal(bla)
			fmt.Fprint(writer, string(b))
			return
		}

		preresponse(writer)
		fmt.Fprintf(writer, `<a href="?dump=all">Dump all as yaml</a> | <a href="?dump=%s">Dump as yaml</a><br/>`, request.URL.Query().Get("expand"))
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
	})))
}

type traceEntry struct {
	c    string
	t    time.Time
	op   string
	args map[string]interface{}
	out  []interface{}
}

var (
	traces     []traceEntry
	traceMutex = new(sync.Mutex)
)

func match(check string, args map[string]interface{}) bool {
	// empty matcher
	if check == "" {
		return true
	}

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
	for _, b := range patchconfig {
		if b.What == op {
			if match(b.Match, args) {
				if b.Repeat > 0 && b.repeated >= b.Repeat {
					continue
				}
				b.repeated++

				if b.Return != nil {
					mustload = false
					x, _ := yaml.Marshal(b.Return)
					o := out[0]
					yaml.Unmarshal(x, o)
				} else if b.Patch != nil {
					mustload = false
					load()
					x, _ := yaml.Marshal(b.Patch)
					o := out[0]
					yaml.Unmarshal(x, o)
				}
				break
			}
		}
	}

	if mustload {
		load()
	}

	traceMutex.Lock()
	traces = append([]traceEntry{{
		c:    id,
		t:    time.Now(),
		op:   op,
		args: args,
		out:  out,
	}}, traces...)
	traceMutex.Unlock()
}

type patch struct {
	What  string
	Match string
	//In     string
	Return   interface{} `yaml:",omitempty"`
	Patch    interface{} `yaml:",omitempty"`
	Repeat   int
	repeated int
}

var patchconfig []*patch

func loadYaml() {
	b, _ := ioutil.ReadFile("config/patch.yaml")
	yaml.Unmarshal(b, &patchconfig)
}
func init() {
	loadYaml()
}
