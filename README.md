# regexkit

Agentic-first regex testing and pattern matching service. Test patterns, extract matches, validate input, replace, and split text. Plain text API, agent-driven, single Go binary with JSON file storage.

## Quick Start

```bash
# Build and run
make build && ./regexkit

# Or with Go
go run ./cmd/regexkit
```

## Auth

```bash
# 1. Request OTP
curl -X POST http://localhost:7700/auth/request -d '{"email":"you@example.com"}' -H 'Content-Type: application/json'
# status=otp_sent code=123456

# 2. Verify OTP → get token
curl -X POST http://localhost:7700/auth/verify -d '{"email":"you@example.com","code":"123456"}' -H 'Content-Type: application/json'
# token=abc123... workspace=ws_xxxxx
```

## Usage

```bash
# Test a pattern
curl -X POST http://localhost:7700/test \
  -H "Authorization: Bearer <token>" \
  -d 'pattern=\d+&input=hello123world456'
# matched=true count=2 match[0]=123 start=5 end=8 match[1]=456 start=14 end=17

# Find first match with capture groups
curl -X POST http://localhost:7700/match \
  -H "Authorization: Bearer <token>" \
  -d 'pattern=(\w+)@(\w+)\.(\w+)&input=user@example.com'
# matched=true full=user@example.com start=0 end=15 1=user 2=example 3=com

# Replace matches
curl -X POST http://localhost:7700/replace \
  -H "Authorization: Bearer <token>" \
  -d 'pattern=\d+&input=hello123&replacement=NUM'
# result=helloNUM count=1

# Split text
curl -X POST http://localhost:7700/split \
  -H "Authorization: Bearer <token>" \
  -d 'pattern=[,;]&input=a,b;c'
# count=3 [0]=a [1]=b [2]=c

# Validate a pattern
curl -X POST http://localhost:7700/validate \
  -H "Authorization: Bearer <token>" \
  -d 'pattern=\d+\w+'
# valid=true

# Save a pattern
curl -X POST http://localhost:7700/patterns \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"email","pattern":"\\w+@\\w+\\.\\w+"}' \
  -H 'Content-Type: application/json'
# handle=pat_a1b2c name=email pattern=\w+@\w+\.\w+ flags=

# Get JSON response
curl -X POST 'http://localhost:7700/test?format=json' \
  -H "Authorization: Bearer <token>" \
  -d 'pattern=\d+&input=hello123'
```

## Configuration

| Flag    | Env             | Default         | Description              |
|---------|-----------------|-----------------|--------------------------|
| -addr   | REGEXKIT_ADDR   | :7700           | Listen address           |
| -db     | REGEXKIT_DB     | regexkit.json   | Data file path           |
| -secret | REGEXKIT_SECRET | (random)        | Auth token signing secret|

## Build

```bash
make build    # Build binary
make test     # Run tests with race detector
make vet      # Run go vet
make run      # Build and run
```

## License

MIT
