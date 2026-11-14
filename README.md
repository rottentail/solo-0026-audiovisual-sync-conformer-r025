# Audiovisual Sync Conformer

Audiovisual Sync Conformer is a self-hosted Go service for measuring timing
drift between a PCM WAV track and a Y4M picture stream, then producing a
bounded review bundle with corrected audio timing.

The implementation includes:

- strict 16-bit PCM WAV chunk parsing and signal measurement;
- planar 4:2:0 Y4M header, frame, and geometry validation;
- audio-onset and picture-flash detection with a confidence value;
- signed drift analysis and a stable threshold policy;
- sample-accurate audio alignment;
- durable take, analysis, and conform states;
- per-identifier locking and atomic directory publication;
- a deterministic ZIP bundle with a matching receipt.

Automated tests are intentionally deferred. A later task stage will add the
test suite, fixtures, race checks, and test command. This baseline ships
production workflow checks that exercise the implemented routes.

## Requirements

- Go 1.23 or newer.
- A writable data root for admitted media, analysis records, and conform
  bundles.

No third-party Go module, media binary, network service, or database server is
required.

## Build

```bash
go build ./...
```

## Run

```bash
go run ./cmd/syncservice -addr :8096 -data ./var/sync-conformer
```

The health route is `GET http://127.0.0.1:8096/healthz`.

Settings:

- `-addr`: HTTP listen address, default `:8096`.
- `-data`: persistent data root, default `./var/sync-conformer`.

No environment variable is required.

## HTTP interface

### Admit a take

`POST /v1/takes`

```json
{
  "take_id": "studio-take-a",
  "name": "camera a and recorder",
  "audio_base64": "UklGR...",
  "video_base64": "WUVVNE1QRUcy...",
  "note": "first transfer"
}
```

The audio is signed 16-bit PCM WAV with one to eight channels. The picture is
a progressive planar C420 Y4M file. A repeating identical request is
idempotent. Reusing a take identifier with different bytes or metadata returns
`take_conflict`.

### Read a take

`GET /v1/takes/{take_id}`

The response contains audio and picture descriptors, a combined SHA-256 digest,
and the admitted timestamp.

### Analyze sync

`POST /v1/takes/{take_id}/sync-analyses`

```json
{
  "analysis_id": "sync-check-a",
  "min_confidence": 0.55,
  "max_offset_ms": 1000,
  "reference_channel": 1
}
```

The response is a terminal analysis with audio and picture event times, a
signed offset in milliseconds, confidence, and drift in parts per million.
Positive offsets mean the detected audio event is later than the picture
event.

`reference_channel` is optional. When omitted (or zero), onset detection keeps
the legacy behavior and measures every channel together. On a multi-channel
field recording, set it to the 1-based channel that carries the slate clap:
only that channel is analyzed, so louder dialogue on the other channels cannot
determine the offset. A number above the admitted audio channel count returns
`invalid_request`. The reference channel is an analysis selection only; it is
never rendered into a mix and does not alter the stored media.

### Read an analysis

`GET /v1/sync-analyses/{analysis_id}`

### Render a conform

`POST /v1/sync-analyses/{analysis_id}/conforms`

```json
{
  "conform_id": "review-a",
  "max_correction_ms": 250
}
```

The audio is aligned earlier for a positive measured offset and later for a
negative offset. The request fails with `correction_limit_exceeded` when the
measured correction is larger than `max_correction_ms`. The same frame shift is
applied to every interleaved channel, so a multi-channel recording keeps all
of its original channels in their original order with their inter-channel
timing intact; no channel is downmixed, summed, or dropped. The receipt records
`audio_channels` and, when one was requested, `reference_channel`.

### Read a conform

- `GET /v1/conforms/{conform_id}`
- `GET /v1/conforms/{conform_id}/bundle`

The bundle route returns `application/zip` after a job reaches `completed`.
The archive contains:

```text
audio-aligned.wav
video-conformed.y4m
conform-receipt.json
```

## Data layout

```text
var/sync-conformer/
├── takes/
│   └── <take_id>/
│       ├── audio.wav
│       ├── picture.y4m
│       └── record.json
├── analyses/
│   └── <analysis_id>.json
├── conforms/
│   └── <conform_id>/
│       ├── record.json
│       └── bundle.zip
└── tmp/
```

Temporary siblings are written under `tmp/` or next to their destination and
renamed into place after all contents and digests pass validation.

## Source layout

```text
cmd/syncservice/        HTTP server entry point
cmd/syncflowcheck/      production workflow checks
internal/httpapi/       routing, JSON errors, limits, and media responses
internal/service/       workflow orchestration and lifecycle control
internal/domain/        records, validation, IDs, states, and stable errors
internal/media/         WAV/Y4M codecs, signal detection, alignment, and packaging
internal/storage/       locks, atomic writes, and repository operations
```

## Workflow checks

Run one workflow at a time:

```bash
go run ./cmd/syncflowcheck -workflow admit-take
go run ./cmd/syncflowcheck -workflow analyze-sync
go run ./cmd/syncflowcheck -workflow render-conform
go run ./cmd/syncflowcheck -workflow multichannel-sync
```

Each check starts the real loopback HTTP service, generates a bounded
calibration pair through the production media package, completes the named
workflow, and verifies persisted state or artifact bytes. The temporary data
root is removed when the process exits. The `multichannel-sync` check admits a
six-channel recording whose slate clap exists only on the first two channels,
confirms that the reference-channel analysis ignores louder dialogue on the
other channels, and verifies that the conformed WAV preserves every channel
and its inter-channel timing.

## Media boundaries

The baseline accepts one PCM WAV stream and one C420 Y4M stream per take.
Compressed audio, compressed picture, multi-stream containers, captions, and
network storage adapters are intentionally outside this baseline. Their real
extension seams are recorded in `.task-source.json`.
