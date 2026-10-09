package digest

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type WorkOrder struct {
	ID             string   `json:"id"`
	Site           string   `json:"site"`
	DispatchStatus string   `json:"dispatch_status"`
	Technician     string   `json:"technician"`
	PhotoRefs      []string `json:"photo_refs"`
	TechnicianNote string   `json:"technician_note"`
	FollowUpBy     string   `json:"follow_up_by"`
}

type FollowUp struct {
	WorkOrderID string `json:"work_order_id"`
	Technician  string `json:"technician"`
	Reason      string `json:"reason"`
}

// FollowUps is the deterministic operational decision made before inference.
func FollowUps(records []WorkOrder, runDate time.Time) []FollowUp {
	day := runDate.Format("2006-01-02")
	items := make([]FollowUp, 0)
	for _, record := range records {
		reasons := make([]string, 0, 2)
		if record.DispatchStatus == "completed" && len(record.PhotoRefs) == 0 {
			reasons = append(reasons, "completed without work-order photos")
		}
		if record.DispatchStatus != "completed" && record.FollowUpBy != "" && record.FollowUpBy <= day {
			reasons = append(reasons, "open past follow-up date")
		}
		if len(reasons) > 0 {
			items = append(items, FollowUp{record.ID, record.Technician, strings.Join(reasons, "; ")})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].WorkOrderID < items[j].WorkOrderID })
	return items
}

func Prompt(record WorkOrder) string {
	return fmt.Sprintf("Summarise this field-service work order in two factual sentences. State dispatch status, photo count, technician note, and required follow-up. ID: %s; site: %s; status: %s; technician: %s; photo_count: %d; note: %s; follow_up_by: %s", record.ID, record.Site, record.DispatchStatus, record.Technician, len(record.PhotoRefs), record.TechnicianNote, record.FollowUpBy)
}

func ReportMarkdown(runDate time.Time, records []WorkOrder, summaries map[string]string) string {
	followUps := FollowUps(records, runDate)
	var b strings.Builder
	fmt.Fprintf(&b, "# Field service overnight digest: %s\n\n", runDate.Format("2006-01-02"))
	fmt.Fprintf(&b, "Processed: %d work orders  \nTechnician follow-ups: %d\n\n", len(records), len(followUps))
	b.WriteString("## Work-order summaries\n\n")
	for _, record := range records {
		fmt.Fprintf(&b, "### %s - %s\n\n%s\n\n", record.ID, record.Site, summaries[record.ID])
	}
	b.WriteString("## Follow-up queue\n\n")
	if len(followUps) == 0 {
		b.WriteString("No follow-up items.\n")
	}
	for _, item := range followUps {
		fmt.Fprintf(&b, "- %s: %s (%s)\n", item.WorkOrderID, item.Reason, item.Technician)
	}
	return b.String()
}
