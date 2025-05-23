package main

import (
	"log"
	"net/http"
	"os"

	"cloud-game-stream-profiler/internal/metrics"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/ingest", metrics.HandleIngest)
	http.HandleFunc("/dashboard", metrics.HandleDashboard)
	http.HandleFunc("/metrics", metrics.HandleJSON)
	http.HandleFunc("/sessions", metrics.HandleSessions)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("webui/static"))))

	log.Printf("Profiler listening on http://localhost:%s/dashboard", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
