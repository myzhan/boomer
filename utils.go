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

func Round(val float64, roundOn float64, places int) (newVal float64) {
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

func MD5(slice ...string) string {
	h := md5.New()
	for _, v := range slice {
		io.WriteString(h, v)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func GetNodeId() (nodeId string) {
	// generate a random nodeId like locust does, using the same algorithm
	hostname, _ := os.Hostname()
	timestamp := int32(time.Now().Unix())
	randomNum := rand.Intn(10000)
	nodeId = fmt.Sprintf("%s_%s", hostname, MD5(fmt.Sprintf("%d%d", timestamp, randomNum)))
	return
}
