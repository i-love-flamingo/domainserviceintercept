package dsi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"

	"go.opencensus.io/trace"
	"gopkg.in/yaml.v2"
)

var statevars = new(sync.Map)

func Vars() *sync.Map {
	return statevars
}

func preresponse(writer http.ResponseWriter) {
	writer.Header().Set("content-type", "text/html; charset=utf-8")
	fmt.Fprint(writer, "<html><body>")
	fmt.Fprint(writer, `
DomainServiceIntercept
| <a href="/">List All</a>
| <a href="/?clear=1">Clear All</a>
| <a href="/?setconfig=1">Config</a>
| <a href="/?vars=1">Vars</a>
| <a href="/?scenarios=show">Scenarios</a>
<br/>`)
}

func clear(w http.ResponseWriter, r *http.Request) {
	traceMutex.Lock()
	traces = nil
	traceMutex.Unlock()
	loadYaml([]byte(`[]`))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func setconfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		loadYaml([]byte(r.FormValue("config")))
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	preresponse(w)
	b, _ := yaml.Marshal(patchconfig)
	fmt.Fprintf(w, `
<form action="?setconfig=1" method="post">
<button type="submit">Set config</button><br/>
<textarea name="config" style="visibility: hidden;" id="configta">%s</textarea>
</form>
<div id="config" style="height: 600px; width: 1000px; position: absolute;">%s</div>`, string(b), string(b))
	fmt.Fprintf(w, `
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
}

func vars(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("json") == "1" {
		w.Header().Set("Content-Type", "application/json")
		v := make(map[string]interface{})
		statevars.Range(func(key, value interface{}) bool {
			v[fmt.Sprintf("%v", key)] = value
			return true
		})
		log.Println(json.NewEncoder(w).Encode(v))
		return
	}
	preresponse(w)
	fmt.Fprint(w, "<pre>")
	statevars.Range(func(key, value interface{}) bool {
		fmt.Fprintf(w, "%q: %#v\n", key, value)
		return true
	})
}

func dump(w http.ResponseWriter, r *http.Request) {
	preresponse(w)
	fmt.Fprint(w, "<pre>")
	var bla []patch
	for _, t := range traces {
		if t.c != r.URL.Query().Get("dump") && r.URL.Query().Get("dump") != "all" {
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
			Return: t.out,
			Repeat: 1,
		})
	}
	b, _ := yaml.Marshal(bla)
	fmt.Fprint(w, string(b))
}

func scenarios(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("scenarios") == "show" {
		files, _ := ioutil.ReadDir("dsi")
		preresponse(w)
		for _, file := range files {
			fmt.Fprintf(w, `<a href="?scenarios=%s">%s</a><br/>`, file.Name(), file.Name())
		}
		return
	}

	filename := filepath.Clean("/" + r.URL.Query().Get("scenarios"))
	loadFile("dsi" + filename)

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// Traceserver serves data/traces
func Traceserver() {
	log.Print(http.ListenAndServe(":13211", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		switch {
		case r.URL.Query().Get("clear") == "1":
			clear(w, r)
			return

		case r.URL.Query().Get("setconfig") == "1":
			setconfig(w, r)
			return

		case r.URL.Query().Get("vars") == "1":
			vars(w, r)
			return

		case r.URL.Query().Get("dump") != "":
			dump(w, r)
			return

		case r.URL.Query().Get("scenarios") != "":
			scenarios(w, r)
			return
		}

		preresponse(w)
		fmt.Fprintf(w, `<a href="?dump=all">Dump all as yaml</a> | <a href="?dump=%s">Dump as yaml</a><br/>`, r.URL.Query().Get("expand"))
		fmt.Fprint(w, "<pre>")

		pt := ""
		for _, t := range traces {
			if r.URL.Query().Get("expand") != "" && r.URL.Query().Get("expand") != t.c {
				continue
			}

			if pt != "" && pt != t.c {
				fmt.Fprint(w, "\n")
			}
			var args []string
			for k, v := range t.args {
				for reflect.ValueOf(v).Kind() == reflect.Ptr {
					v = reflect.ValueOf(v).Elem().Interface()
				}
				args = append(args, k+"="+fmt.Sprintf("%#v", v))
			}
			fmt.Fprintf(w, "<a href=\"?expand=%s\">%s</a> @ %s: %s%s(%v)\n", t.c, t.c, t.t.Format("15:04:05.000000000"), strings.Repeat("| ", t.depth), t.op, strings.Join(args, ", "))
			if r.URL.Query().Get("expand") != "" {
				b, _ := yaml.Marshal(t.out)
				fmt.Fprintf(w, "%s\n", string(b))
			}
			pt = t.c
		}
	})))
}

type traceEntry struct {
	c     string
	t     time.Time
	op    string
	args  A
	out   A
	depth int
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

	t := template.New("")
	t.Funcs(template.FuncMap{
		"get": func(key interface{}) interface{} {
			v, _ := statevars.Load(key)
			return v
		},
	})
	t, err := t.Parse(`{{if (` + check + `)}}ok{{end}}`)
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

var depth = 0

type A map[string]interface{}

// Traceme collects available data
func Traceme(ctx context.Context, op string, args A, load func(), out A, vars A) {
	id := "0000000000000000"
	if span := trace.FromContext(ctx); span != nil {
		id = span.SpanContext().TraceID.String()
	}

	depth++

	mustload := true
	for _, b := range patchconfig {
		if b.What == op {
			if match(b.Match, args) {
				if b.Repeat > 0 && b.repeated >= b.Repeat {
					continue
				}
				b.repeated++

				for outName := range out {
					if ret, ok := b.Return[outName]; ok {
						mustload = false
						if s, ok := ret.(string); ok && vars[s] != nil {
							reflect.ValueOf(out[outName]).Elem().Set(reflect.ValueOf(vars[s]))
						} else {
							x, _ := yaml.Marshal(ret)
							yaml.Unmarshal(x, out[outName])
						}
					} else if patch, ok := b.Patch[outName]; ok {
						mustload = false
						load()
						x, _ := yaml.Marshal(patch)
						yaml.Unmarshal(x, out[outName])
					}
				}

				for k, v := range b.Set {
					statevars.Store(k, v)
				}
				if !b.Continue {
					break
				}
			}
		}
	}

	if mustload {
		load()
	}

	depth--

	traceMutex.Lock()
	traces = append([]traceEntry{{
		c:     id,
		t:     time.Now(),
		op:    op,
		args:  args,
		out:   out,
		depth: depth,
	}}, traces...)
	traceMutex.Unlock()
}

type patch struct {
	What  string
	Match string
	//In     string
	Return   map[string]interface{} `yaml:",omitempty"`
	Patch    map[string]interface{} `yaml:",omitempty"`
	Repeat   int
	repeated int
	Set      map[string]interface{} `yaml:",omitempty"`
	Continue bool                   `yaml:",omitempty"`
}

var patchconfig []*patch

func loadFile(filename string) {
	b, _ := ioutil.ReadFile(filename)
	loadYaml(b)
}

func loadYaml(b []byte) {
	yaml.Unmarshal(b, &patchconfig)
	statevars = new(sync.Map)
}
