package main

import (
	"context"
	"log"
	"time"

	"github.com/myzhan/boomer"
	"go.etcd.io/etcd/clientv3"
)

var globalClient *clientv3.Client

func worker() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	start := time.Now()
	resp, err := globalClient.Put(ctx, "hello", "boomer")
	elapsed := time.Since(start)
	if err != nil {
		boomer.RecordFailure("etcd", "put", elapsed.Nanoseconds()/int64(time.Millisecond), err.Error())
	} else {
		boomer.RecordSuccess("etcd", "put", elapsed.Nanoseconds()/int64(time.Millisecond), int64(resp.Header.Size()))
	}

	cancel()
}

func main() {
	client, err := clientv3.NewFromURL("127.0.0.1:2379")
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	globalClient = client

	task := &boomer.Task{
		Name: "etcd/clientv3",
		Fn:   worker,
	}

	boomer.Run(task)
}
