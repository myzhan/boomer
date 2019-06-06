package boomer

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"
)

// Output is primarily responsible for printing test results to different destinations
// such as consoles, files. You can write you own output and add to boomer.
// When running in standalone mode, the default output is ConsoleOutput, you can add more.
// When running in distribute mode, test results will be reported to master with or without
// an output.
// All the OnXXX function will be call in a separated goroutine, just in case some output will block.
// But it will wait for all outputs return to avoid data lost.
type Output interface {
	// OnStart will be call before the test starts.
	OnStart()

	// By default, each output receive stats data from runner every three seconds.
	// OnEvent is responsible for dealing with the data.
	OnEvent(data map[string]interface{})

	// OnStop will be called before the test ends.
	OnStop()
}

// ConsoleOutput is the default output for standalone mode.
type ConsoleOutput struct {
}

// NewConsoleOutput returns a ConsoleOutput.
func NewConsoleOutput() *ConsoleOutput {
	return &ConsoleOutput{}
}

func getMedianResponseTime(numRequests int64, responseTimes map[int64]int64) int64 {
	medianResponseTime := int64(0)
	if len(responseTimes) != 0 {
		pos := (numRequests - 1) / 2
		var sortedKeys []int64
		for k := range responseTimes {
			sortedKeys = append(sortedKeys, k)
		}
		sort.SliceStable(sortedKeys, func(i, j int) bool {
			return sortedKeys[i] < sortedKeys[j]
		})
		for _, k := range sortedKeys {
			if pos < responseTimes[k] {
				medianResponseTime = k
				break
			}
			pos -= responseTimes[k]
		}
	}
	return medianResponseTime
}

func getAvgResponseTime(numRequests int64, totalResponseTime int64) (avgResponseTime float64) {
	avgResponseTime = float64(0)
	if numRequests != 0 {
		avgResponseTime = float64(totalResponseTime / numRequests)
	}
	return avgResponseTime
}

func getAvgContentLength(numRequests int64, totalContentLength int64) (avgContentLength int64) {
	avgContentLength = int64(0)
	if numRequests != 0 {
		avgContentLength = totalContentLength / numRequests
	}
	return avgContentLength
}

func getCurrentRps(numRequests int64, numReqsPerSecond map[int64]int64) (currentRps int64) {
	currentRps = int64(0)
	numReqsPerSecondLength := int64(len(numReqsPerSecond))
	if numReqsPerSecondLength != 0 {
		currentRps = numRequests / numReqsPerSecondLength
	}
	return currentRps
}

// OnStart of ConsoleOutput has nothing to do.
func (o *ConsoleOutput) OnStart() {

}

// OnStop of ConsoleOutput has nothing to do.
func (o *ConsoleOutput) OnStop() {

}

// OnEvent will print to the console.
func (o *ConsoleOutput) OnEvent(data map[string]interface{}) {
	currentTime := time.Now()
	println(fmt.Sprintf("Current time: %s", currentTime.Format("2006/01/02 15:04:05")))
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Type", "Name", "# requests", "# fails", "Median", "Average", "Min", "Max", "Content Size", "# reqs/sec"})
	stats := data["stats"].([]interface{})
	for _, stat := range stats {
		s := stat.(map[string]interface{})
		row := make([]string, 10)
		row[0], row[1] = s["name"].(string), s["method"].(string)

		numRequests := s["num_requests"].(int64)
		row[2] = strconv.FormatInt(numRequests, 10)

		numFailures := s["num_failures"].(int64)
		row[3] = strconv.FormatInt(numFailures, 10)

		medianResponseTime := getMedianResponseTime(numRequests, s["response_times"].(map[int64]int64))
		row[4] = strconv.FormatInt(medianResponseTime, 10)

		totalResponseTime := s["total_response_time"].(int64)
		avgResponseTime := getAvgResponseTime(numRequests, totalResponseTime)
		row[5] = strconv.FormatFloat(avgResponseTime, 'f', 2, 64)

		minResponseTime := s["min_response_time"].(int64)
		row[6] = strconv.FormatInt(minResponseTime, 10)

		maxResponseTime := s["max_response_time"].(int64)
		row[7] = strconv.FormatInt(maxResponseTime, 10)

		totalContentLength := s["total_content_length"].(int64)
		avgContentLength := getAvgContentLength(numRequests, totalContentLength)
		row[8] = strconv.FormatInt(avgContentLength, 10)

		numReqsPerSecond := s["num_reqs_per_sec"].(map[int64]int64)
		currentRps := getCurrentRps(numRequests, numReqsPerSecond)
		row[9] = strconv.FormatInt(currentRps, 10)

		table.Append(row)
	}
	table.Render()
	println()
}
