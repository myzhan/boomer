# Implementing influxdb export

install influxdb dependency   

`go get github.com/influxdata/influxdb-client-go/v2`   

```go

// InfluxOutput pushes boomer stats to InfluxDB.
type InfluxOutput struct {
 influx api.WriteAPI
}

// NewInfluxOutput returns a InfluxOutput.
func NewInfluxOutput(influxHost, token, org, bucket string) *InfluxOutput {
 return &InfluxOutput{
  influxdb2.NewClientWithOptions(influxHost, token,
   influxdb2.DefaultOptions().
    SetUseGZip(true).
    SetTLSConfig(&tls.Config{InsecureSkipVerify: true})).WriteAPI(org, bucket),
 }
}

// OnStart will start influxdb write api
func (o *InfluxOutput) OnStart() {
 log.Println("register influx metric collectors")
}

// OnStop of InfluxOutput force all unwritten data to be sent
func (o *InfluxOutput) OnStop() {
 o.influx.Flush()
}

func (o *InfluxOutput) OnEvent(data map[string]interface{}) {
 eventTime := time.Now()
 errorsCh := o.influx.Errors()
 go func() {
  for err := range errorsCh {
   log.Println("could not push to influxdb error: %s\n\n", err.Error())
  }
 }()
 output, err := convertData(data)
 if err != nil {
  log.Println(fmt.Sprintf("convert data error: %s", err))
  return
 }

 for _, stat := range output.Stats {
  method := stat.Method
  name := stat.Name
  point := influxdb2.NewPoint(
   method+name,
   map[string]string{
    "user_count": string(output.UserCount),
    "total_rps":  strconv.FormatInt(output.TotalRPS, 10),
   },
   map[string]interface{}{
    "num_requests":         float64(stat.NumRequests),
    "num_failures":         float64(stat.NumFailures),
    "median_response_time": float64(stat.medianResponseTime),
    "avg_response_time":    stat.avgResponseTime,
    "min_response_time":    float64(stat.MinResponseTime),
    "max_response_time":    float64(stat.MaxResponseTime),
    "avg_content_length":   float64(stat.avgContentLength),
    "current_rps":          float64(stat.currentRps),
    "current_fail_per_sec": float64(stat.currentFailPerSec),
   },
   eventTime)
  o.influx.WritePoint(point)
 }
}

```
