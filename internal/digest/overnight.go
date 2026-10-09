package digest

import (
	"context"
	"time"
)

type OvernightResult struct {
	BatchID  string
	Markdown string
	PDF      PDFResult
}

func RunOvernight(ctx context.Context, client *Client, runID string, runDate time.Time, records []WorkOrder) (OvernightResult, error) {
	batchID, err := client.Submit(ctx, runID, records)
	if err != nil {
		return OvernightResult{}, err
	}
	if err := client.Wait(ctx, batchID, 5*time.Second); err != nil {
		return OvernightResult{}, err
	}
	summaries, err := client.Results(ctx, batchID)
	if err != nil {
		return OvernightResult{}, err
	}
	markdown := ReportMarkdown(runDate, records, summaries)
	pdf, err := client.GeneratePDF(ctx, runID, markdown)
	if err != nil {
		return OvernightResult{}, err
	}
	return OvernightResult{BatchID: batchID, Markdown: markdown, PDF: pdf}, nil
}
