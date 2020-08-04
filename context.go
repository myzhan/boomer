package boomer

import (
	"encoding/json"
	"reflect"
)

type Context struct {
	items map[interface{}]interface{}
}

func NewContext() *Context {
	return &Context{items: make(map[interface{}]interface{})}
}

func (ctx *Context) MSet(data map[interface{}]interface{}) {
	for key, value := range data {
		if key == nil {
			panic("nil key")
		}
		if !reflect.TypeOf(key).Comparable() {
			panic("key is not comparable")
		}
		ctx.items[key] = value
	}
}

func (ctx *Context) Set(key, value interface{}) {
	if key == nil {
		panic("nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}
	ctx.items[key] = value
}

func (ctx *Context) Get(key interface{}) (interface{}, bool) {
	value, ok := ctx.items[key]
	return value, ok
}

func (ctx *Context) Count() int {
	return len(ctx.items)
}

func (ctx *Context) Has(key interface{}) bool {
	_, ok := ctx.items[key]
	return ok
}

func (ctx *Context) Remove(key interface{}) {
	delete(ctx.items, key)
}

func (ctx *Context) Pop(key interface{}) (value interface{}, exists bool) {
	value, exists = ctx.items[key]
	ctx.Remove(key)
	return
}

func (ctx *Context) IsEmpty() bool {
	return ctx.Count() == 0
}

func (ctx *Context) MarshalJSON() ([]byte, error) {
	return json.Marshal(ctx.items)
}
