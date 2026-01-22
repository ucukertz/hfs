package hfs

import (
	"io"
	"net/http"
	"testing"
	"time"
)

var test_hf_token = "your-token"
var test_name = "zerogpu-aoti-flux-1-kontext-dev" // your HF Space name
var test_endpoint = "/infer"                      // your HF Space endpoint, e.g. "/predict" or "/infer"
var test_jpg_url = "https://i.pinimg.com/474x/c2/c3/d2/c2c3d23c592772cafa4bad0d64d51416.jpg"
var test_png_url = "https://www.pngmart.com/files/10/Thumbs-UP-PNG-Transparent-Image.png"
var test_prompt = "make it smile"

func Test_FileDataFromURL(t *testing.T) {
	t.Parallel()

	hfs := NewHfs[any, any](test_name).
		WithTimeout(300 * time.Second).
		WithBearerToken(test_hf_token)

	fdi, err := NewFileData("").FromUrl(test_jpg_url)
	if err != nil {
		t.Fatalf("ToFileData returned error: %v", err)
	}

	res, err := hfs.Do(test_endpoint, fdi, test_prompt, 0, true, 2.5, 1 /*28*/)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}
	if len(res) == 0 {
		t.Fatalf("expected at least one result from Do()")
	}

	var out []byte
	out, err = GetFileData(res[0])
	if err != nil {
		t.Fatalf("GetFileData() returned error: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected non-empty output")
	}
}

func Test_FileDataFromBytes(t *testing.T) {
	t.Parallel()
	hfs := NewHfs[any, any](test_name).
		WithTimeout(300 * time.Second).
		WithBearerToken(test_hf_token)
	resp, err := http.Get(test_jpg_url)
	if err != nil {
		t.Fatalf("http.Get() returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("http.Get() returned status code %d, expected 200", resp.StatusCode)
	}
	data_reader := resp.Body
	data, err := io.ReadAll(data_reader)
	if err != nil {
		t.Fatalf("io.ReadAll() returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected non-empty input data")
	}

	fdi, err := NewFileData("").FromBytes(data)
	if err != nil {
		t.Fatalf("ToFileData returned error: %v", err)
	}

	res, err := hfs.Do(test_endpoint, fdi, test_prompt, 0, true, 2.5, 1 /*28*/)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}
	if len(res) == 0 {
		t.Fatalf("expected at least one result from Do()")
	}
	var out []byte
	out, err = GetFileData(res[0])
	if err != nil {
		t.Fatalf("GetFileData() returned error: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected non-empty output")
	}
}

func Test_FileDataFromUpload(t *testing.T) {
	t.Parallel()
	hfs := NewHfs[any, any](test_name).
		WithTimeout(300 * time.Second).
		WithBearerToken(test_hf_token)
	// Test with png to check whether /upload cares about the actual file format
	resp, err := http.Get(test_png_url)
	if err != nil {
		t.Fatalf("http.Get() returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("http.Get() returned status code %d, expected 200", resp.StatusCode)
	}
	data_reader := resp.Body
	data, err := io.ReadAll(data_reader)
	if err != nil {
		t.Fatalf("io.ReadAll() returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected non-empty input data")
	}

	fdi, err := NewFileData("").FromUpload(*hfs, data)
	if err != nil {
		t.Fatalf("ToFileData returned error: %v", err)
	}

	res, err := hfs.Do(test_endpoint, fdi, test_prompt, 0, true, 2.5, 1 /*28*/)
	if err != nil {
		t.Fatalf("Do() returned error: %v", err)
	}
	if len(res) == 0 {
		t.Fatalf("expected at least one result from Do()")
	}
	var out []byte
	out, err = GetFileData(res[0])
	if err != nil {
		t.Fatalf("GetFileData() returned error: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected non-empty output")
	}
}
