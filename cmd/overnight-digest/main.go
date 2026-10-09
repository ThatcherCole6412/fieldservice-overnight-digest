package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/infrai-examples/fieldservice-overnight-digest/internal/digest"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "INFRAI_API_KEY is required")
		os.Exit(2)
	}
	records := []digest.WorkOrder{
		{ID: "WO-2048", Site: "North pump room", DispatchStatus: "completed", Technician: "Mina", PhotoRefs: []string{"arrival.jpg", "repair.jpg"}, TechnicianNote: "Replaced the failed pressure switch.", FollowUpBy: "2026-09-18"},
		{ID: "WO-2051", Site: "West loading bay", DispatchStatus: "dispatched", Technician: "Jo", TechnicianNote: "Access contact did not answer.", FollowUpBy: "2026-09-18"},
	}
	runDate := time.Now().UTC()
	runID := runDate.Format("20060102")
	result, err := digest.RunOvernight(context.Background(), digest.NewClient(key), runID, runDate, records)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output := map[string]any{"run_id": runID, "batch_id": result.BatchID, "follow_up_count": len(digest.FollowUps(records, runDate)), "pdf": result.PDF}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
