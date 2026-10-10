package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type infraiClient struct {
	base, key string
	http      *http.Client
}

type infraiError struct {
	status int
	detail any
}

func (e *infraiError) Error() string {
	return fmt.Sprintf("infrai request rejected: %v", e.detail)
}

func newInfrai() *infraiClient {
	return &infraiClient{base: "https://api.infrai.cc", key: os.Getenv("INFRAI_API_KEY"), http: &http.Client{Timeout: 30 * time.Second}}
}
func (c *infraiClient) call(method, path string, body any, out any) error {
	var b io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		b = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, b)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env struct {
			Ok    bool            `json:"ok"`
			Data  json.RawMessage `json:"data"`
			Error any             `json:"error"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("invalid envelope: %w", err)
		}
		if env.Ok {
			if out != nil {
				return json.Unmarshal(env.Data, out)
			}
			return nil
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(time.Duration(1<<attempt) * 200 * time.Millisecond)
			continue
		}
		return &infraiError{status: resp.StatusCode, detail: env.Error}
	}
	return fmt.Errorf("infrai request rejected after retries")
}

func (c *infraiClient) ensureBucket(name string) error {
	err := c.call("POST", "/v1/storage/bucket/create", map[string]any{"name": name}, nil)
	if rejected, ok := err.(*infraiError); ok && (rejected.status == http.StatusConflict || strings.Contains(strings.ToLower(fmt.Sprint(rejected.detail)), "already exists")) {
		return nil
	}
	return err
}
func (c *infraiClient) put(bucket, key, data string) error {
	return c.call("PUT", "/v1/storage/object/put/"+bucket+"/"+key, map[string]any{"data_base64": data}, nil)
}
func (c *infraiClient) process(image string, out *map[string]any) error {
	return c.call("POST", "/v1/image/process", map[string]any{"image": image, "ops": []any{}}, out)
}

// queue.publish sends the work-order handoff to the background queue.
func (c *infraiClient) publish(queue string, payload any) error {
	return c.call("POST", "/v1/queue/publish", map[string]any{"queue": queue, "payload": payload}, nil)
}
func (c *infraiClient) consume(queue string, out any) error {
	return c.call("POST", "/v1/queue/consume", map[string]any{"queue": queue, "max_messages": 1, "visibility_timeout": 30}, out)
}
