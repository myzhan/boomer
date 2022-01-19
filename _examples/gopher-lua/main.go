package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/yuin/gopher-lua/parse"

	"github.com/myzhan/boomer"
	lua "github.com/yuin/gopher-lua"
)

// This example uses gopher-lua to run lua scripts.
// It shows the possibility that we can put test logics in lua scripts.
// It's not ready to be used directly.
// More features and optimizations need to be added.

var script string
var compiledScript *lua.FunctionProto
var httpClient *http.Client
var globalBoomer = boomer.NewBoomer("127.0.0.1", 5557)

func initHTTPClient() {
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 2000
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		MaxIdleConnsPerHost: 2000,
	}
	httpClient = &http.Client{
		Transport: tr,
	}
}

func httpGet(l *lua.LState) int {
	url := l.ToString(1)
	resp, err := httpClient.Get(url)
	if err != nil {
		r := l.CreateTable(0, 1)
		l.SetField(r, "error", lua.LString(fmt.Sprintf("%v", err)))
		l.Push(r)
		return 1
	}
	defer resp.Body.Close()

	s, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		r := l.CreateTable(0, 1)
		l.SetField(r, "error", lua.LString(fmt.Sprintf("%v", err)))
		l.Push(r)
		return 1
	}

	r := l.CreateTable(0, 1)
	l.SetField(r, "body", lua.LString(s))
	l.Push(r)
	return 1
}

var httpExports = map[string]lua.LGFunction{
	"get": httpGet,
}

func recordSuccess(l *lua.LState) int {
	requestType := l.ToString(1)
	name := l.ToString(2)
	responseTime := l.ToInt64(3)
	responseLength := l.ToInt64(4)
	globalBoomer.RecordSuccess(requestType, name, responseTime, responseLength)
	return 0
}

func recordFailure(l *lua.LState) int {
	requestType := l.ToString(1)
	name := l.ToString(2)
	responseTime := l.ToInt64(3)
	exception := l.ToString(4)
	globalBoomer.RecordFailure(requestType, name, responseTime, exception)
	return 0
}

func getMillis(l *lua.LState) int {
	millis := time.Now().UnixNano() / int64(time.Millisecond)
	l.Push(lua.LNumber(millis))
	return 1
}

func injectGoModules(l *lua.LState) int {
	l.SetGlobal("record_success", l.NewFunction(recordSuccess))
	l.SetGlobal("record_failure", l.NewFunction(recordFailure))
	l.SetGlobal("get_millis", l.NewFunction(getMillis))

	httpMod := l.SetFuncs(l.NewTable(), httpExports)
	l.Push(httpMod)
	return 1
}

func luaHTTP() {
	l := lua.NewState()
	defer l.Close()
	l.PreloadModule("http", injectGoModules)

	lfunc := l.NewFunctionFromProto(compiledScript)
	l.Push(lfunc)
	l.PCall(0, lua.MultRet, nil)

	if err := l.CallByParam(lua.P{
		Fn:      l.GetGlobal("execute"),
		NRet:    0,
		Protect: true,
	}, lua.LNil); err != nil {
		log.Printf("%v\n", err)
	}
}

func waitForQuit() {
	wg := sync.WaitGroup{}
	wg.Add(1)

	quitByMe := false
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		quitByMe = true
		globalBoomer.Quit()
		wg.Done()
	}()

	boomer.Events.Subscribe(EVENT_QUIT, func() {
		if !quitByMe {
			wg.Done()
		}
	})

	wg.Wait()
}

func compileFile(filePath string) (*lua.FunctionProto, error) {
	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	chunk, err := parse.Parse(reader, filePath)
	if err != nil {
		return nil, err
	}
	proto, err := lua.Compile(chunk, filePath)
	if err != nil {
		return nil, err
	}
	return proto, nil
}

func main() {
	var err error
	compiledScript, err = compileFile(script)
	if err != nil {
		log.Printf("Failed to compile lua script: %s, %v", script, err)
		os.Exit(1)
	}

	initHTTPClient()

	task := &boomer.Task{
		Name: "lua",
		Fn:   luaHTTP,
	}

	globalBoomer.Run(task)

	waitForQuit()
	log.Println("shut down")
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.StringVar(&script, "script", "demo.lua", "Path of lua script")
	flag.Parse()
}
