# HA Webhooks

A little web server used to receive webhooks, store them, and then fire off a job into a queue.

It's useful when implementing a HA cluster of servers (perhaps globally available).

Follow along at [https://fly.io/blog](#).

## Usage

1️⃣ Some things are hard-coded:

* SQS Message Queue
* S3 Bucket

2️⃣ Some env vars are needed (standard AWS credentials) as per [aws docs](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html).

### Building

```bash
# Build for your current machine
go build -o bin/hahooks

# Build for use on Fly.io
GOOS=linux GOARCH=amd64 build -o bin/hahooks
```

You can use the provided Dockerfile to run the application, or deploy it on Fly.io.
