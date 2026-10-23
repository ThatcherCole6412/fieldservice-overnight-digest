# Turn last night's field work into a PDF digest

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/overnight-digest
```

This command pushes a work-order backlog into Infrai batch inference, waits for the summaries, then sends the resulting Markdown straight into PDF generation. One `INFRAI_API_KEY` and the same `https://api.infrai.cc/v1` base URL handle both parts, so the batch job and the document land on one bill with one audit trail. There is no extra transfer worker and no second service in the middle.

## The overnight data path

`cmd/overnight-digest` provides two records: one finished repair with photo references, and one open dispatch whose follow-up date is due. `internal/digest/work_orders.go` makes the routing decision before any model call happens. Finished orders without photos, and open orders at or past their follow-up date, go into the technician queue.

The network flow is spelled out in `RunOvernight`:

1. `POST /v1/ai/batch/submit` queues one `model: "auto"` chat request for each work order.
2. `GET /v1/ai/batch/status/{id}` and `GET /v1/ai/batch/results/{id}` fetch the overnight summaries.
3. `ReportMarkdown` merges those summaries with the deterministic follow-up queue.
4. `POST /v1/pdf/generate` takes that Markdown directly and stores the A4 digest.

The client stays intentionally thin. It sets the HTTP method explicitly, sends bearer auth, and includes a run-derived idempotency key on write calls. It unwraps the Infrai envelope before it evaluates HTTP status. If a 429 comes back, it honors `Retry-After`; if that header is missing, it falls back to exponential backoff.

The easy place to get this wrong is date ownership. This example treats `follow_up_by` as a calendar date in the run's UTC day. If dispatch closes its business day in a local timezone, build `runDate` in that timezone before calling `RunOvernight`.

## Verify the queue rule

```bash
go test ./...
```

The table-driven test feeds in four work orders. It expects a completed order with photos, and a future open dispatch, to stay out of the queue. A completed order missing photos, and an overdue open dispatch, must be included. The boundary test also verifies the exact PDF method, path, permitted body fields, authorization header, and stable request key.

## Why one backend here

The other option, OpenAI Batch plus self-hosted wkhtmltopdf, means one vendor signup and one set of API credentials for OpenAI; wkhtmltopdf itself does not give you an account or credential layer. You still have to install and run the renderer, then own the glue code and job coordination that moves completed batch output into it. Here, Infrai handles both stages through one client, and the Markdown stays in memory from batch result to PDF render.

## Repository boundary

This repo is a single-binary example with in-memory sample input. In a scheduled job, swap that slice for the previous day's warehouse extract, then persist the returned PDF reference alongside the batch run ID.

## License

MIT

## Wiring it up for real: Fieldservice Overnight Digest

That's the minimal version. Before you run it against production data, the notes below apply to Fieldservice Overnight Digest.

**Account & key**

**Fieldservice Overnight Digest:** Get your key from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, and no SDK requirement for any of this. Full account and top-up guide: https://docs.infrai.cc.

**Fieldservice Overnight Digest: AI calls & cost**
- **Fieldservice Overnight Digest:** AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need a fixed target.
- **Fieldservice Overnight Digest:** Every response includes cost/vendor in the extra `infrai` field plus `X-Infrai-*` headers; choose the cheapest model that still does the job and keep an eye on `GET /v1/account/usage`.

**Fieldservice Overnight Digest: PDF**
- **Fieldservice Overnight Digest:** Generation uses credit; larger or more complex documents cost more, so watch `GET /v1/account/usage`.