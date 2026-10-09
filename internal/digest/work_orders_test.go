package digest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestSubmitReadsBatchID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/ai/batch/submit" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["requests"]; !ok {
			t.Fatal("missing requests")
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"batch_id":"batch-test"},"error":null}`))
	}))
	defer server.Close()
	client := NewClient("test-key")
	client.BaseURL = server.URL + "/v1"
	client.HTTPClient = server.Client()
	id, err := client.Submit(context.Background(), "run-42", []WorkOrder{{ID: "WO-1"}})
	if err != nil || id != "batch-test" {
		t.Fatalf("Submit() = %q, %v", id, err)
	}
}

func TestFollowUps(t *testing.T) {
	runDate := time.Date(2026, 9, 18, 2, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		records []WorkOrder
		want    []FollowUp
	}{
		{"completed with photos", []WorkOrder{{ID: "WO-1", DispatchStatus: "completed", Technician: "Mina", PhotoRefs: []string{"photo-1"}}}, []FollowUp{}},
		{"completed missing photos", []WorkOrder{{ID: "WO-2", DispatchStatus: "completed", Technician: "Jo"}}, []FollowUp{{"WO-2", "Jo", "completed without work-order photos"}}},
		{"overdue open dispatch", []WorkOrder{{ID: "WO-3", DispatchStatus: "dispatched", Technician: "Ari", FollowUpBy: "2026-09-17"}}, []FollowUp{{"WO-3", "Ari", "open past follow-up date"}}},
		{"future open dispatch", []WorkOrder{{ID: "WO-4", DispatchStatus: "dispatched", Technician: "Lee", FollowUpBy: "2026-09-19"}}, []FollowUp{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FollowUps(test.records, runDate); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("FollowUps() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestGeneratePDFRequestBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/pdf/generate" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "fieldservice-pdf-run-42" {
			t.Fatalf("idempotency key = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		wantKeys := []string{"markdown", "orientation", "page_size", "store"}
		for _, key := range wantKeys {
			if _, ok := body[key]; !ok {
				t.Errorf("missing body field %q", key)
			}
		}
		if len(body) != len(wantKeys) {
			t.Fatalf("body has unexpected fields: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"url":"https://example.invalid/digest.pdf"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL + "/v1"
	client.HTTPClient = server.Client()
	result, err := client.GeneratePDF(t.Context(), "run-42", "# Digest")
	if err != nil {
		t.Fatal(err)
	}
	if result["url"] == "" {
		t.Fatalf("result = %#v", result)
	}
}
