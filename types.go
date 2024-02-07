package main

type Request struct {
	// Base64 encoded string of bytes
	Body string `json:"body"`

	// Map of key=value1,value2 headers
	Headers map[string][]string `json:"headers"`

	// URI with parameters
	Uri string `json:"uri"`
}

type Message struct {
	Bucket string `json:bucket`
	Key    string `json:key`
}
