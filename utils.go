package boomer

import (
	"crypto/md5"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"time"
)

func round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

// MD5 hash of strings
func MD5(slice ...string) string {
	h := md5.New()
	for _, v := range slice {
		io.WriteString(h, v)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// generate a random nodeID like locust does, using the same algorithm.
func getNodeID() string {
	hostname, _ := os.Hostname()
	timestamp := time.Now().Unix()
	randomNum := int64(rand.Intn(10000))
	return fmt.Sprintf("%s_%s", hostname, MD5(fmt.Sprint(timestamp+randomNum)))
}

// Now get current timestamp in milliseconds.
func Now() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
