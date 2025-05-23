package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Metric struct {
	SessionID            string   `json:"session_id"`
	FPS                  float64  `json:"fps"`
	Latency              float64  `json:"latency_ms"`
	Bitrate              float64  `json:"bitrate_kbps"`
	Resolution           string   `json:"resolution"`
	Outliers             []string `json:"outliers,omitempty"`
	PredictedDegradation []string `json:"predicted_degradation,omitempty"`
}

var (
	recentFPS     []float64
	recentLatency []float64
	recentBitrate []float64
	mu            sync.Mutex
	windowSize    = 10
)

func updateWindow(slice *[]float64, value float64) {
	*slice = append(*slice, value)
	if len(*slice) > windowSize {
		*slice = (*slice)[1:]
	}
}

func isAnomalous(value float64, history []float64) bool {
	if len(history) < windowSize {
		return false
	}
	var sum, sqSum float64
	for _, v := range history {
		sum += v
		sqSum += v * v
	}
	mean := sum / float64(len(history))
	stddev := math.Sqrt((sqSum / float64(len(history))) - (mean * mean))
	return math.Abs(value-mean) > 2*stddev
}

func computeTrendSlope(values []float64) float64 {
	n := len(values)
	if n < 2 {
		return 0
	}
	var sumX, sumY, sumXY, sumXX float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	denominator := float64(n)*sumXX - sumX*sumX
	if denominator == 0 {
		return 0
	}
	return (float64(n)*sumXY - sumX*sumY) / denominator
}

func detectDegradation() []string {
	mu.Lock()
	defer mu.Unlock()
	var flags []string
	fpsSlope := computeTrendSlope(recentFPS)
	latencySlope := computeTrendSlope(recentLatency)
	bitrateSlope := computeTrendSlope(recentBitrate)

	if fpsSlope < -0.5 {
		flags = append(flags, "fps")
	}
	if latencySlope > 0.5 {
		flags = append(flags, "latency")
	}
	if bitrateSlope < -100 {
		flags = append(flags, "bitrate")
	}
	return flags
}

func detectOutliers(m Metric) []string {
	mu.Lock()
	defer mu.Unlock()

	var alerts []string

	updateWindow(&recentFPS, m.FPS)
	updateWindow(&recentLatency, m.Latency)
	updateWindow(&recentBitrate, m.Bitrate)

	if m.FPS < 45 || isAnomalous(m.FPS, recentFPS) {
		alerts = append(alerts, "fps")
	}
	if m.Latency > 100 || isAnomalous(m.Latency, recentLatency) {
		alerts = append(alerts, "latency")
	}
	if m.Bitrate < 2000 || isAnomalous(m.Bitrate, recentBitrate) {
		alerts = append(alerts, "bitrate")
	}
	return alerts
}

func saveMetricToFile(sessionID string, metric Metric) {
	folder := "data"
	_ = os.MkdirAll(folder, os.ModePerm)
	filePath := filepath.Join(folder, sessionID+".json")

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("❌ Failed to open file:", err)
		return
	}
	defer f.Close()

	data, err := json.Marshal(metric)
	if err != nil {
		log.Println("❌ Failed to marshal metric:", err)
		return
	}
	f.WriteString(string(data) + "\n")
}

func main() {
	sessionID := "simulator-" + strconv.FormatInt(time.Now().Unix(), 10)
	log.Printf("🟢 Starting simulator with session ID: %s", sessionID)

	ticker := time.NewTicker(1 * time.Second)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	client := &http.Client{Timeout: 2 * time.Second}

	resolutions := []string{"480p", "720p", "1080p", "1440p", "4K"}

	for {
		select {
		case <-ticker.C:
			resolution := resolutions[rand.Intn(len(resolutions))]

			fpsBase := map[string]float64{"480p": 50, "720p": 55, "1080p": 60, "1440p": 60, "4K": 60}[resolution]
			bitrateBase := map[string]float64{"480p": 1500, "720p": 3000, "1080p": 5000, "1440p": 8000, "4K": 15000}[resolution]

			metric := Metric{
				SessionID:  sessionID,
				FPS:        fpsBase + rand.Float64()*5,
				Latency:    30 + rand.Float64()*100,
				Bitrate:    bitrateBase + rand.Float64()*1000,
				Resolution: resolution,
			}

			metric.Outliers = detectOutliers(metric)
			metric.PredictedDegradation = detectDegradation()
			saveMetricToFile(sessionID, metric)

			body, _ := json.Marshal(metric)
			req, err := http.NewRequest("POST", "http://profiler:8080/ingest", bytes.NewBuffer(body))

			if err != nil {
				log.Println("❌ Failed to create request:", err)
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				log.Println("❌ Failed to send metric:", err)
				continue
			}
			resp.Body.Close()

			log.Printf("📡 Sent [%s] FPS=%.1f, Latency=%.1fms, Bitrate=%.1fkbps%s%s",
				resolution, metric.FPS, metric.Latency, metric.Bitrate,
				func() string {
					if len(metric.Outliers) > 0 {
						return " [⚠️ Outliers: " + strconv.Quote(strings.Join(metric.Outliers, ", ")) + "]"
					}
					return ""
				}(),
				func() string {
					if len(metric.PredictedDegradation) > 0 {
						return " [🔮 Predicted: " + strconv.Quote(strings.Join(metric.PredictedDegradation, ", ")) + "]"
					}
					return ""
				}())

		case <-sigs:
			log.Println("🛑 Simulator shutting down.")
			return
		}
	}
}
