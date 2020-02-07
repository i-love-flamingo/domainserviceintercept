package dsi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opencensus.io/trace"

	"flamingo.me/flamingo/v3/framework/flamingo"
)

type logentry struct {
	level string
	msg   string
	ts    time.Time
}

var (
	logs map[string][]logentry
	lm   sync.Mutex
)

type PdsiLogger struct {
	flamingo.Logger
	ctx context.Context
}

func (l *PdsiLogger) add(entry logentry) {
	entry.ts = time.Now()
	ctx := l.ctx

	id := "0000000000000000"
	if ctx == nil {
		ctx = lastctx
	}
	if span := trace.FromContext(ctx); span != nil {
		id = span.SpanContext().TraceID.String()
	}

	//lm.Lock()
	//logs[id] = append(logs[id], entry)
	//lm.Unlock()

	traceMutex.Lock()
	traces = append([]traceEntry{{
		c: id,
		t: entry.ts,
		//op: entry.level + ": " + entry.msg,
		logLevel: entry.level,
		logMsg:   entry.msg,
	}}, traces...)
	traceMutex.Unlock()
}

func (l *PdsiLogger) WithContext(ctx context.Context) flamingo.Logger {
	return &PdsiLogger{l.Logger.WithContext(ctx), ctx}
}

func (l *PdsiLogger) Debug(args ...interface{}) {
	l.add(logentry{level: "Debug", msg: fmt.Sprint(args...)})
	l.Logger.Debug(args...)
}
func (l *PdsiLogger) Info(args ...interface{}) {
	l.add(logentry{level: "Info", msg: fmt.Sprint(args...)})
	l.Logger.Info(args...)
}
func (l *PdsiLogger) Warn(args ...interface{}) {
	l.add(logentry{level: "Warn", msg: fmt.Sprint(args...)})
	l.Logger.Warn(args...)
}
func (l *PdsiLogger) Error(args ...interface{}) {
	l.add(logentry{level: "Error", msg: fmt.Sprint(args...)})
	l.Logger.Error(args...)
}
func (l *PdsiLogger) Fatal(args ...interface{}) {
	l.add(logentry{level: "Fatal", msg: fmt.Sprint(args...)})
	l.Logger.Fatal(args...)
}
func (l *PdsiLogger) Panic(args ...interface{}) {
	l.add(logentry{level: "Panic", msg: fmt.Sprint(args...)})
	l.Logger.Panic(args...)
}

func (l *PdsiLogger) Debugf(logf string, args ...interface{}) {
	l.add(logentry{level: "Debug", msg: fmt.Sprintf(logf, args...)})
	l.Logger.Debugf(logf, args...)
}
