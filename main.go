package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var s3Client *s3.Client
var sqsClient *sqs.Client
var bucket = "some bucket"
var queue = "https://some-aws-sqs-queue"

// init generates AWS clients (s3 + sqs)
func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}

	s3Client = s3.NewFromConfig(cfg)
	sqsClient = sqs.NewFromConfig(cfg)
}

// main starts a web server capable of graceful shutdown
func main() {
	// Listen for signals to help gracefully shut down
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Do some work on any received request
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//  handle health checks
		if r.URL.Path == "/up" {
			w.WriteHeader(200)
			fmt.Fprintf(w, "up")
			return
		}

		log.Printf("%s: %s", r.Method, r.URL)

		// Generate the stored request file name
		key := r.Header.Get("Fly-Request-Id")
		if len(key) == 0 {
			log.Printf("no fly request id found, generating one")
			key = uuid.NewString()
		}

		// Save the bits we care about to an s3 (or similar) object
		if err := saveToS3(bucket, key, r); err != nil {
			log.Printf("could not store request: %v", err)
			w.WriteHeader(500)
			fmt.Fprintf(w, "server error")
			return
		}

		// Queue up a message so something else can process the request
		if err := sendToSqs(queue, key); err != nil {
			log.Printf("could not queue request: %v", err)
			w.WriteHeader(500)
			fmt.Fprintf(w, "server error")
			return
		}

		w.WriteHeader(200)
		fmt.Fprintf(w, "done")
	})

	// Handle starting / stopping the web server
	addr := ":8080"
	srv := http.Server{
		Addr: addr,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	log.Printf("listening on %s", addr)

	<-done

	log.Printf("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}

	log.Printf("bye bye")
}

// saveToS3 converts an *http.Request to an S3 object before being
// referenced in an SQS message. SQS messages have a payload limit of 256k
// and so we don't attempt to store the HTTP request data within it.
func saveToS3(bucket, key string, r *http.Request) error {
	uploader := manager.NewUploader(s3Client)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("could not read request body: %w", err)
	}

	jsonBody, jsonErr := json.Marshal(&Request{
		Uri:     r.RequestURI,
		Headers: r.Header,
		Body:    base64.StdEncoding.EncodeToString(body),
	})

	if jsonErr != nil {
		return fmt.Errorf("could not marshal json: %w", jsonErr)
	}

	_, uploadErr := uploader.Upload(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(jsonBody),
	})

	if uploadErr != nil {
		return fmt.Errorf("could not upload to s3: %w", uploadErr)
	}

	return nil
}

// sendToSqs sends a message containing a reference to our latest s3
// object, which stores data about the web request. A worker somewhere else
// will read these messages and process the HTTP request as needed
func sendToSqs(queueUrl, key string) error {
	jsonMessage, err := json.Marshal(&Message{
		Bucket: bucket,
		Key:    key,
	})

	if err != nil {
		return fmt.Errorf("could not marshal JSON: %w", err)
	}

	_, err = sqsClient.SendMessage(context.Background(), &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueUrl),
		MessageBody: aws.String(string(jsonMessage)),
	})

	if err != nil {
		return fmt.Errorf("could not send to sqs: %w", err)
	}

	return nil
}
