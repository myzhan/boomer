package boomer

import (
	"time"
)

type RequestStats struct {
	Entries              map[string]*StatsEntry
	Errors               map[string]*StatsError
	NumRequests          int64
	NumFailures          int64
	MaxRequests          int64
	LastRequestTimestamp int64
	StartTime            int64
}

func (this *RequestStats) get(name string, method string) (entry *StatsEntry) {
	entry, ok := this.Entries[name+method]
	if !ok {
		entry = &StatsEntry{
			Stats:         this,
			Name:          name,
			Method:        method,
			NumReqsPerSec: make(map[int64]int64),
			ResponseTimes: make(map[float64]int64),
		}
		entry.reset()
		this.Entries[name+method] = entry
	}
	return this.Entries[name+method]
}

func (this *RequestStats) clearAll() {
	this.NumRequests = 0
	this.NumFailures = 0
	this.Entries = make(map[string]*StatsEntry)
	this.Errors = make(map[string]*StatsError)
	this.MaxRequests = 0
	this.LastRequestTimestamp = 0
	this.StartTime = 0
}

type StatsEntry struct {
	Stats                *RequestStats
	Name                 string
	Method               string
	NumRequests          int64
	NumFailures          int64
	TotalResponseTime    float64
	MinResponseTime      float64
	MaxResponseTime      float64
	NumReqsPerSec        map[int64]int64
	ResponseTimes        map[float64]int64
	TotalContentLength   int64
	StartTime            int64
	LastRequestTimestamp int64
}

func (this *StatsEntry) reset() {
	this.StartTime = int64(time.Now().Unix())
	this.NumRequests = 0
	this.NumFailures = 0
	this.TotalResponseTime = 0
	this.ResponseTimes = make(map[float64]int64)
	this.MinResponseTime = 0
	this.MaxResponseTime = 0
	this.LastRequestTimestamp = int64(time.Now().Unix())
	this.NumReqsPerSec = make(map[int64]int64)
	this.TotalContentLength = 0
}

func (this *StatsEntry) log(responseTime float64, contentLength int64) {

	this.NumRequests += 1

	this.logTimeOfRequest()
	this.logResponseTime(responseTime)

	this.TotalContentLength += contentLength

}

func (this *StatsEntry) logTimeOfRequest() {

	now := int64(time.Now().Unix())

	_, ok := this.NumReqsPerSec[now]
	if !ok {
		this.NumReqsPerSec[now] = 0
	} else {
		this.NumReqsPerSec[now] += 1
	}

	this.LastRequestTimestamp = now

}

func (this *StatsEntry) logResponseTime(responseTime float64) {
	this.TotalResponseTime += responseTime

	if this.MinResponseTime == 0 {
		this.MinResponseTime = responseTime
	}

	if responseTime < this.MinResponseTime {
		this.MinResponseTime = responseTime
	}

	if responseTime > this.MaxResponseTime {
		this.MaxResponseTime = responseTime
	}

	roundedResponseTime := float64(0)

	if responseTime < 100 {
		roundedResponseTime = responseTime
	} else if responseTime < 1000 {
		roundedResponseTime = float64(Round(responseTime, .5, -1))
	} else if responseTime < 10000 {
		roundedResponseTime = float64(Round(responseTime, .5, -2))
	} else {
		roundedResponseTime = float64(Round(responseTime, .5, -3))
	}

	_, ok := this.ResponseTimes[roundedResponseTime]
	if !ok {
		this.ResponseTimes[roundedResponseTime] = 0
	} else {
		this.ResponseTimes[roundedResponseTime] += 1
	}

}

func (this *StatsEntry) logError(err string) {
	this.NumFailures += 1
	key := MD5(this.Method, this.Name, err)
	entry, ok := this.Stats.Errors[key]
	if !ok {
		entry = &StatsError{
			Name:   this.Name,
			Method: this.Method,
			Error:  err,
		}
		this.Stats.Errors[key] = entry
	}
	entry.occured()
}

func (this *StatsEntry) serialize() map[string]interface{} {
	result := make(map[string]interface{})
	result["name"] = this.Name
	result["method"] = this.Method
	result["last_request_timestamp"] = this.LastRequestTimestamp
	result["start_time"] = this.StartTime
	result["num_requests"] = this.NumRequests
	result["num_failures"] = this.NumFailures
	result["total_response_time"] = this.TotalResponseTime
	result["max_response_time"] = this.MaxResponseTime
	result["min_response_time"] = this.MinResponseTime
	result["total_content_length"] = this.TotalContentLength
	result["response_times"] = this.ResponseTimes
	result["num_reqs_per_sec"] = this.NumReqsPerSec
	return result
}

func (this *StatsEntry) getStrippedReport() map[string]interface{} {
	report := this.serialize()
	this.reset()
	return report
}

type StatsError struct {
	Name       string
	Method     string
	Error      string
	Occurences int64
}

func (this *StatsError) occured() {
	this.Occurences += 1
}

func (this *StatsError) toMap() map[string]interface{} {
	m := make(map[string]interface{})
	m["method"] = this.Method
	m["name"] = this.Name
	m["error"] = this.Error
	m["occurences"] = this.Occurences
	return m
}

func reportToMasterHandler(data *map[string]interface{}) {
	entries := make([]interface{}, 0, len(stats.Entries))
	for _, v := range stats.Entries {
		if !(v.NumRequests == 0 && v.NumFailures == 0) {
			entries = append(entries, v.getStrippedReport())
		}
	}

	errors := make(map[string]map[string]interface{})
	for k, v := range stats.Errors {
		errors[k] = v.toMap()
	}

	(*data)["stats"] = entries
	(*data)["errors"] = errors
	stats.Entries = make(map[string]*StatsEntry)
	stats.Errors = make(map[string]*StatsError)
}

type RequestSuccess struct {
	requestType    string
	name           string
	responseTime   float64
	responseLength int64
}

type RequestFailure struct {
	requestType  string
	name         string
	responseTime float64
	error        string
}

var stats = new(RequestStats)
var RequestSuccessChannel = make(chan *RequestSuccess, 100)
var RequestFailureChannel = make(chan *RequestFailure, 100)
var ClearStatsChannel = make(chan bool)

func init() {
	Events.Subscribe("boomer:report_to_master", reportToMasterHandler)
	stats.Entries = make(map[string]*StatsEntry)
	stats.Errors = make(map[string]*StatsError)
	go func() {
		for {
			select {
			case m := <-RequestSuccessChannel:
				stats.get(m.name, m.requestType).log(m.responseTime, m.responseLength)
			case n := <-RequestFailureChannel:
				stats.get(n.name, n.requestType).logError(n.error)
			case <-ClearStatsChannel:
				stats.clearAll()
			}
		}
	}()
}
