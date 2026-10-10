package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

type workOrderPhoto struct {
	WorkOrderID  string `json:"work_order_id"`
	TechnicianID string `json:"technician_id"`
	Filename     string `json:"filename"`
	DataBase64   string `json:"data_base64"`
}
type status struct {
	WorkOrderID string `json:"work_order_id"`
	State       string `json:"state"`
	FollowUp    string `json:"follow_up"`
}

func shouldQueue(data string) bool { return len(data) > 120000 }
func uploadHandler(c *infraiClient, bucket, queue string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in workOrderPhoto
		if json.NewDecoder(r.Body).Decode(&in) != nil || in.WorkOrderID == "" || in.DataBase64 == "" {
			http.Error(w, "invalid photo", 400)
			return
		}
		key := strings.Join([]string{"work-orders", in.WorkOrderID, in.Filename}, "/")
		if err := c.put(bucket, key, in.DataBase64); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		state := "processing"
		if shouldQueue(in.DataBase64) {
			state = "queued"
			if err := c.publish(queue, map[string]any{"work_order_id": in.WorkOrderID, "technician_id": in.TechnicianID, "object_key": key}); err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status{WorkOrderID: in.WorkOrderID, State: state, FollowUp: "technician review"})
	}
}

func main() {
	c := newInfrai()
	bucket := envOrDefault("INFRAI_BUCKET", "fieldservice-photos")
	queue := envOrDefault("INFRAI_QUEUE", "fieldservice-photos")
	if err := c.ensureBucket(bucket); err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/photos", uploadHandler(c, bucket, queue))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// processQueue is the background handoff: queue.consume -> image.process.
func processQueue(c *infraiClient, queue string) error {
	var message map[string]any
	if err := c.consume(queue, &message); err != nil {
		return err
	}
	if payload, ok := message["payload"].(map[string]any); ok {
		_, _ = payload["object_key"], payload["work_order_id"]
		result := map[string]any{}
		return c.process(payload["object_key"].(string), &result)
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
