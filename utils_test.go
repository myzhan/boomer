package boomer

import (
	"os"
	"regexp"
	"testing"
	"time"
)

func TestRound(t *testing.T) {

	if int(round(float64(147.5002), .5, -1)) != 150 {
		t.Error("147.5002 should be rounded to 150")
	}

	if int(round(float64(3432.5002), .5, -2)) != 3400 {
		t.Error("3432.5002 should be rounded to 3400")
	}

	roundOne := round(float64(58760.5002), .5, -3)
	roundTwo := round(float64(58960.6003), .5, -3)
	if roundOne != roundTwo {
		t.Error("round(58760.5002) should be equal to round(58960.6003)")
	}

	roundOne = round(float64(58360.5002), .5, -3)
	roundTwo = round(float64(58460.6003), .5, -3)
	if roundOne != roundTwo {
		t.Error("round(58360.5002) should be equal to round(58460.6003)")
	}

	roundOne = round(float64(58360), .5, -3)
	roundTwo = round(float64(58460), .5, -3)
	if roundOne != roundTwo {
		t.Error("round(58360) should be equal to round(58460)")
	}

}

func TestMD5(t *testing.T) {
	hashValue := MD5("Hello", "World!")
	if hashValue != "06e0e6637d27b2622ab52022db713ce2" {
		t.Error("Expected: 06e0e6637d27b2622ab52022db713ce2, Got: ", hashValue)
	}
}

func TestGetNodeID(t *testing.T) {
	nodeID := getNodeID()
	hostname, _ := os.Hostname()
	regex := hostname + "_[a-f0-9]{32}$"
	validNodeID := regexp.MustCompile(regex)
	if !validNodeID.MatchString(nodeID) {
		t.Error("Invalid format of nodeID")
	}
}

func TestNow(t *testing.T) {
	now := Now()
	if now < 1000000000000 || now > 2000000000000 {
		t.Error("Invalid format of timestamp in milliseconds")
	}
}

func TestStartMemoryProfile(t *testing.T) {
	if _, err := os.Stat("mem.pprof"); os.IsExist(err) {
		os.Remove("mem.pprof")
	}
	StartMemoryProfile("mem.pprof", 2*time.Second)
	time.Sleep(2100 * time.Millisecond)
	if _, err := os.Stat("mem.pprof"); os.IsNotExist(err) {
		t.Error("File mem.pprof is not generated")
	} else {
		os.Remove("mem.pprof")
	}
}

func TestStartCPUProfile(t *testing.T) {
	if _, err := os.Stat("cpu.pprof"); os.IsExist(err) {
		os.Remove("cpu.pprof")
	}
	StartCPUProfile("cpu.pprof", 2*time.Second)
	time.Sleep(2100 * time.Millisecond)
	if _, err := os.Stat("cpu.pprof"); os.IsNotExist(err) {
		t.Error("File cpu.pprof is not generated")
	} else {
		os.Remove("cpu.pprof")
	}
}
