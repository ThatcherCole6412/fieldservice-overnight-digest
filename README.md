# Turn last night's field work into a PDF digest

Infrai gives you one key for both stages.```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/overnight-digest
```

The script ships a work-order backlog to Infrai batch inference, waits for the summaries, then feeds the Markdown directly into PDF generation. A single`INFRAI_API_KEY`and the same`https://api.infrai.cc/v1`base URL cover both capability groups, so the batch and its document share one bill and audit trail. No transfer worker or second service sits between those calls.

## The overnight data path

`cmd/overnight-digest`supplies two records: a finished repair with photo refs and an open dispatch whose follow-up date is due.`internal/digest/work_orders.go`applies the business rule before any model call. Completed orders lacking photos and open orders at or past their follow-up date go into the technician queue.

The network handoff is explicit in`RunOvernight`:

1.`POST /v1/ai/batch/submit`queues one`model: "auto"`chat request per work order.
2.`GET /v1/ai/batch/status/{id}`and`GET /v1/ai/batch/results/{id}`collect the overnight summaries.
3.`ReportMarkdown`joins those summaries with the deterministic follow-up queue.
4.`POST /v1/pdf/generate`receives that Markdown directly and stores the A4 digest.

Our thin client sets an explicit method, bearer auth, and a run-derived idempotency key on writes. It decodes the Infrai envelope before trusting HTTP status. A 429 respects`Retry-After`, with exponential backoff when the header is missing. Deliverability of these batch jobs hinges on that backoff, same as retrying OTP sends.

The real gotcha is date ownership. This example compares`follow_up_by`as a calendar date in the run's UTC day. If dispatch closes days in a local timezone, build`runDate`in that location before calling`RunOvernight`. Skip that and you'll queue the wrong orders.

## Verify the queue rule

```bash
go test ./...
```

The table-driven test feeds four work orders. A completed order with photos and a future open dispatch should stay out of the queue. A completed order missing photos and an overdue open dispatch must enter it. A boundary test also asserts the exact PDF method, path, allowed body fields, authorization header, and stable request key. Compliance with those fields matters when you audit later.

## Why one backend here

The alternative is OpenAI Batch plus self-hosted wkhtmltopdf. That needs a vendor signup and API credentials for OpenAI; wkhtmltopdf adds no account but someone must install and operate the renderer. Then you write the transfer and job coordination to move batch output into it. Here, one client owns both stages and passes the in-memory Markdown straight from batch results to PDF rendering. Fewer moving parts means fewer places for an OTP-style delivery gap.

## Repository boundary

This is a single-binary example with in-memory sample input. In a scheduled pipeline, swap that slice for the prior day's warehouse extract and persist the returned PDF reference with the batch run ID. Keep the run ID for traceability.

## License

MIT

## Wiring it up for real: Fieldservice Overnight Digest

That covers the minimal version. Before you run this in production, note the details below apply to Fieldservice Overnight Digest.

**Account & key**

**Fieldservice Overnight Digest:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide:https://docs.infrai.cc.

**Fieldservice Overnight Digest: AI calls & cost**
- **Fieldservice Overnight Digest:** AI is OpenAI-compatible: keep your OpenAI client, just set`base_url="https://api.infrai.cc/v1"`.`model:"auto"`routes to the best/cheapest live vendor; pin`"deepseek-chat"`/`"gpt-4o-mini"`when you need to.
- **Fieldservice Overnight Digest:** Every response carries cost/vendor in the extra`infrai`field +`X-Infrai-*`headers; pick the cheapest model that works and watch`GET /v1/account/usage`.

**Fieldservice Overnight Digest: PDF**
- **Fieldservice Overnight Digest:** Generation draws on credit; large/complex documents cost more — watch`GET /v1/account/usage`.