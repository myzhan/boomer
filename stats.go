package boomer

import (
	"time"
)

type requestStats struct {
	entries              map[string]*statsEntry
	errors               map[string]*statsError
	numRequests          int64
	numFailures          int64
	maxRequests          int64
	lastRequestTimestamp int64
	startTime            int64
}

func (s *requestStats) get(name string, method string) (entry *statsEntry) {
	entry, ok := s.entries[name+method]
	if !ok {
		newEntry := &statsEntry{
			stats:         s,
			name:          name,
			method:        method,
			numReqsPerSec: make(map[int64]int64),
			responseTimes: make(map[float64]int64),
		}
		newEntry.reset()
		s.entries[name+method] = newEntry
		return newEntry
	}
	return entry
}

func (s *requestStats) clearAll() {
	s.numRequests = 0
	s.numFailures = 0
	s.entries = make(map[string]*statsEntry)
	s.errors = make(map[string]*statsError)
	s.maxRequests = 0
	s.lastRequestTimestamp = 0
	s.startTime = 0
}

type statsEntry struct {
	stats                *requestStats
	name                 string
	method               string
	numRequests          int64
	numFailures          int64
	totalResponseTime    float64
	minResponseTime      float64
	maxResponseTime      float64
	numReqsPerSec        map[int64]int64
	responseTimes        map[float64]int64
	totalContentLength   int64
	startTime            int64
	lastRequestTimestamp int64
}

func (s *statsEntry) reset() {
	s.startTime = int64(time.Now().Unix())
	s.numRequests = 0
	s.numFailures = 0
	s.totalResponseTime = 0
	s.responseTimes = make(map[float64]int64)
	s.minResponseTime = 0
	s.maxResponseTime = 0
	s.lastRequestTimestamp = int64(time.Now().Unix())
	s.numReqsPerSec = make(map[int64]int64)
	s.totalContentLength = 0
}

func (s *statsEntry) log(responseTime float64, contentLength int64) {

	s.numRequests++

	s.logTimeOfRequest()
	s.logResponseTime(responseTime)

	s.totalContentLength += contentLength

}

func (s *statsEntry) logTimeOfRequest() {

	now := int64(time.Now().Unix())

	_, ok := s.numReqsPerSec[now]
	if !ok {
		s.numReqsPerSec[now] = 1
	} else {
		s.numReqsPerSec[now]++
	}

	s.lastRequestTimestamp = now

}

func (s *statsEntry) logResponseTime(responseTime float64) {
	s.totalResponseTime += responseTime

	if s.minResponseTime == 0 {
		s.minResponseTime = responseTime
	}

	if responseTime < s.minResponseTime {
		s.minResponseTime = responseTime
	}

	if responseTime > s.maxResponseTime {
		s.maxResponseTime = responseTime
	}

	roundedResponseTime := float64(0)

	if responseTime < 100 {
		roundedResponseTime = responseTime
	} else if responseTime < 1000 {
		roundedResponseTime = float64(round(responseTime, .5, -1))
	} else if responseTime < 10000 {
		roundedResponseTime = float64(round(responseTime, .5, -2))
	} else {
		roundedResponseTime = float64(round(responseTime, .5, -3))
	}

	_, ok := s.responseTimes[roundedResponseTime]
	if !ok {
		s.responseTimes[roundedResponseTime] = 1
	} else {
		s.responseTimes[roundedResponseTime]++
	}

}

func (s *statsEntry) logError(err string) {
	s.numFailures++
	key := MD5(s.method, s.name, err)
	entry, ok := s.stats.errors[key]
	if !ok {
		entry = &statsError{
			name:   s.name,
			method: s.method,
			error:  err,
		}
		s.stats.errors[key] = entry
	}
	entry.occured()
}

func (s *statsEntry) serialize() map[string]interface{} {
	result := make(map[string]interface{})
	result["name"] = s.name
	result["method"] = s.method
	result["last_request_timestamp"] = s.lastRequestTimestamp
	result["start_time"] = s.startTime
	result["num_requests"] = s.numRequests
	result["num_failures"] = s.numFailures
	result["total_response_time"] = s.totalResponseTime
	result["max_response_time"] = s.maxResponseTime
	result["min_response_time"] = s.minResponseTime
	result["total_content_length"] = s.totalContentLength
	result["response_times"] = s.responseTimes
	result["num_reqs_per_sec"] = s.numReqsPerSec
	return result
}

func (s *statsEntry) getStrippedReport() map[string]interface{} {
	report := s.serialize()
	s.reset()
	return report
}

type statsError struct {
	name       string
	method     string
	error      string
	occurences int64
}

func (err *statsError) occured() {
	err.occurences++
}

func (err *statsError) toMap() map[string]interface{} {
	m := make(map[string]interface{})
	m["method"] = err.method
	m["name"] = err.name
	m["error"] = err.error
	m["occurences"] = err.occurences
	return m
}

func collectReportData() map[string]interface{} {
	data := make(map[string]interface{})
	entries := make([]interface{}, 0, len(stats.entries))
	for _, v := range stats.entries {
		if !(v.numRequests == 0 && v.numFailures == 0) {
			entries = append(entries, v.getStrippedReport())
		}
	}

	errors := make(map[string]map[string]interface{})
	for k, v := range stats.errors {
		errors[k] = v.toMap()
	}

	data["stats"] = entries
	data["errors"] = errors
	stats.entries = make(map[string]*statsEntry)
	stats.errors = make(map[string]*statsError)

	return data
}

type requestSuccess struct {
	requestType    string
	name           string
	responseTime   float64
	responseLength int64
}

type requestFailure struct {
	requestType  string
	name         string
	responseTime float64
	error        string
}

var stats = new(requestStats)
var requestSuccessChannel = make(chan *requestSuccess, 100)
var requestFailureChannel = make(chan *requestFailure, 100)
var clearStatsChannel = make(chan bool)
var messageToServerChannel = make(chan map[string]interface{}, 10)

func init() {
	stats.entries = make(map[string]*statsEntry)
	stats.errors = make(map[string]*statsError)
	go func() {
		var ticker = time.NewTicker(slaveReportInterval)
		for {
			select {
			case m := <-requestSuccessChannel:
				entry := stats.get(m.name, m.requestType)
				entry.log(m.responseTime, m.responseLength)
			case n := <-requestFailureChannel:
				stats.get(n.name, n.requestType).logError(n.error)
			case <-clearStatsChannel:
				stats.clearAll()
			case <-ticker.C:
				data := collectReportData()
				// send data to channel, no network IO in this goroutine
				messageToServerChannel <- data
			}
		}
	}()
}
