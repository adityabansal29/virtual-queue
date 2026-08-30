# virtual-queue
Virtual waiting room to handle traffic surge during peak events

## AWS dev validation

Validated on 2026-08-30 against the deployed CloudFront endpoints:

- Queue page: `https://d3ef9uo0iwz4l3.cloudfront.net/queue/index.html`
- Queue API: `https://d20ahx254u6i0j.cloudfront.net`
- ECS services: `dev-queueserver:3`, `dev-scheduler:3`, `dev-stuborigin:2`
- SSM business configuration: `EVENT_ID=evt-001`, `DEFAULT_ADMIT_RATE=1`, `SSE_THRESHOLD=200`, `SCHEDULER_TICK_SECS=20`

Flow evidence:

1. `GET /health` through Queue API CloudFront returned `200` and `{"ok":true}`.
2. `GET /queue/join?eventId=evt-001` returned `302` to `/queue/index.html?ticket=...`.
3. With 20 seeded users ahead, the browser status request returned `{"type":"position","value":20,"upgrade_to_sse":true}`.
4. The browser then opened `GET /queue/status/{ticketId}?mode=sse` and received a position event followed by an admitted event.
5. Browser navigation completed at the configured target URL.

Browser evidence:

- [Polling state](test-screenshots/aws-queue-polling.png)
- [SSE state](test-screenshots/aws-queue-sse.png)
- Progress screenshots: [20](test-screenshots/aws-queue-progress-20.png), [19](test-screenshots/aws-queue-progress-19.png), [18](test-screenshots/aws-queue-progress-18.png), [16](test-screenshots/aws-queue-progress-16.png), [10](test-screenshots/aws-queue-progress-10.png)
- [JavaScript and CloudWatch evidence](test-screenshots/aws-queue-evidence.txt)

The repeatable capture command is `./scripts/capture-aws-evidence.sh`. It seeds 20 users in parallel, captures one timestamped screenshot per rank through `0` (set `PROGRESS_STOP_RANK` to choose another endpoint), records browser console/network events, and collects filtered `/ecs/dev/queueserver` and `/ecs/dev/scheduler` CloudWatch logs after the 20-second scheduler tick. The current captured run shows ranks `20` through `10`; the JavaScript network trace proves `poll -> 200 -> sse -> 200`, and the scheduler log records admissions for the isolated evidence event.
