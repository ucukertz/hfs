package hfs

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// The tests below are offline: they exercise parsing and error handling against
// a local server, so they need no token and no network.

func Test_parseSSE(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		body    string
		want    string
		wantErr string
	}{
		{
			name: "heartbeat then complete",
			body: "event: heartbeat\ndata: null\n\nevent: complete\ndata: [1,2]\n\n",
			want: "[1,2]",
		},
		{
			name: "complete then heartbeat",
			body: "event: complete\ndata: [1,2]\n\nevent: heartbeat\ndata: null\n\n",
			want: "[1,2]",
		},
		{
			name: "data split over several lines",
			body: "event: complete\ndata: [1,\ndata: 2]\n\n",
			want: "[1,\n2]",
		},
		{
			name: "no trailing blank line",
			body: "event: complete\ndata: [1,2]",
			want: "[1,2]",
		},
		{
			name: "crlf",
			body: "event: complete\r\ndata: [1,2]\r\n\r\n",
			want: "[1,2]",
		},
		{
			name: "comment is ignored",
			body: "event: complete\n: keep-alive\ndata: [1,2]\n\n",
			want: "[1,2]",
		},
		{
			name:    "error event",
			body:    "event: error\ndata: {\"error\":\"nope\",\"title\":\"Error\"}\n\n",
			wantErr: "Error: nope",
		},
		{
			name:    "no complete event",
			body:    "event: heartbeat\ndata: null\n\n",
			wantErr: "no complete event",
		},
	}

	for _, c := range cases {
		got, err := parseSSE(c.body)
		if c.wantErr != "" {
			if err == nil {
				t.Errorf("%s: expected error %q, got data %q", c.name, c.wantErr, got)
			} else if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s: error = %q, want it to contain %q", c.name, err, c.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: data = %q, want %q", c.name, got, c.want)
		}
	}
}

func Test_eventErrorKeepsRaw(t *testing.T) {
	t.Parallel()

	body := "event: error\ndata: {\"error\":\"Value 1 is less than minimum value 8.\",\"title\":\"Error\"}\n\n"
	_, err := parseSSE(body)

	var ee *EventError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *EventError, got %T: %v", err, err)
	}
	if ee.Title != "Error" || ee.Message != "Value 1 is less than minimum value 8." {
		t.Fatalf("unexpected EventError: %+v", ee)
	}
	if err.Error() != "Error: Value 1 is less than minimum value 8." {
		t.Fatalf("unexpected Error() text: %q", err.Error())
	}
	if ee.Raw != body {
		t.Fatal("EventError.Raw should hold the whole stream")
	}
}

func Test_apiDetail(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		want string
	}{
		{"string detail", `{"detail":"Queue is full. Max size is 20 and size is 20."}`, "Queue is full. Max size is 20 and size is 20."},
		{"list detail", `{"detail":[{"type":"json_invalid","msg":"JSON decode error"}]}`, `[{"type":"json_invalid","msg":"JSON decode error"}]`},
		{"plain body", `upstream connect error`, "upstream connect error"},
		{"empty body", ``, "no response body"},
	}

	for _, c := range cases {
		if got := apiDetail([]byte(c.body)); got != c.want {
			t.Errorf("%s: apiDetail() = %q, want %q", c.name, got, c.want)
		}
	}
}

// A full queue answers on the POST with 503 and no event_id. That used to be
// swallowed, and the follow-up GET to ".../generate/" reported "Method Not
// Allowed" as "hfs no data in resp".
func Test_Do_queueFull(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"detail":"Queue is full. Max size is 20 and size is 20."}`)
	}))
	defer srv.Close()

	s := NewHfsRaw[any, any](srv.URL + "/gradio_api/call")
	_, err := s.Do("/generate", "hi")
	if err == nil {
		t.Fatal("expected an error from a full queue")
	}
	if !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), "Queue is full") {
		t.Fatalf("error should carry the status and the space's message, got: %v", err)
	}
}

func Test_Do_eventError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			fmt.Fprint(w, `{"event_id":"evt1"}`)
			return
		}
		fmt.Fprint(w, "event: heartbeat\ndata: null\n\n"+
			"event: error\ndata: {\"error\":\"Expired ZeroGPU proxy token\",\"title\":\"ZeroGPU client error\"}\n\n")
	}))
	defer srv.Close()

	s := NewHfsRaw[any, any](srv.URL + "/gradio_api/call")
	_, err := s.Do("/generate", "hi")

	var ee *EventError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *EventError, got %T: %v", err, err)
	}
	if err.Error() != "ZeroGPU client error: Expired ZeroGPU proxy token" {
		t.Fatalf("unexpected Error() text: %q", err.Error())
	}
}

func Test_Do_emptyEventID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"detail":"something odd"}`)
	}))
	defer srv.Close()

	s := NewHfsRaw[any, any](srv.URL + "/gradio_api/call")
	if _, err := s.Do("/generate", "hi"); err == nil {
		t.Fatal("expected an error when the space returns no event_id")
	}
}

// Upload used a fresh http.Client with no headers, so a gated space never saw
// the bearer token and the request had no timeout.
func Test_Upload_sendsAuthAndMultipart(t *testing.T) {
	t.Parallel()

	var gotAuth, gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		fmt.Fprint(w, `["/tmp/gradio/abc/image.jpg"]`)
	}))
	defer srv.Close()

	s := NewHfsRaw[any, any](srv.URL + "/gradio_api/call").WithBearerToken("tok")
	path, err := s.Upload([]byte("jpegbytes"))
	if err != nil {
		t.Fatalf("Upload() returned error: %v", err)
	}
	if path != "/tmp/gradio/abc/image.jpg" {
		t.Fatalf("Upload() path = %q", path)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer tok")
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data; boundary=") {
		t.Fatalf("Content-Type = %q, want multipart with a boundary", gotCT)
	}
}

// WithTimeout wrote to client.Timeout on http.DefaultClient, so one space's
// timeout silently applied to every other space and to net/http at large.
func Test_WithTimeout_doesNotTouchDefaultClient(t *testing.T) {
	t.Parallel()

	before := http.DefaultClient.Timeout
	NewHfs[any, any]("some-space").WithTimeout(300 * time.Second)
	if http.DefaultClient.Timeout != before {
		t.Fatalf("http.DefaultClient.Timeout changed from %v to %v", before, http.DefaultClient.Timeout)
	}
}

func Test_GetGalleryImage_badInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		src  any
		idx  int
	}{
		{"not a gallery", "nope", 0},
		{"empty gallery", []any{}, 0},
		{"index past the end", []any{map[string]any{}}, 1},
		{"negative index", []any{map[string]any{}}, -1},
		{"item is not a map", []any{"nope"}, 0},
		{"item has no image", []any{map[string]any{}}, 0},
	}

	for _, c := range cases {
		if _, err := GetGalleryImage(c.src, c.idx); err == nil {
			t.Errorf("%s: expected an error, got none", c.name)
		}
	}
}

func Test_FileDataDownload_validation(t *testing.T) {
	t.Parallel()

	if _, err := FileDataDownload(nil, time.Second); err == nil {
		t.Error("expected an error for a nil FileData")
	}
	if _, err := FileDataDownload(&FileData{}, time.Second); err == nil {
		t.Error("expected an error for an empty URL")
	}
}

func Test_ParseFileData_nil(t *testing.T) {
	t.Parallel()

	if _, err := ParseFileData(nil); err == nil {
		t.Error("expected an error for nil input")
	}
}
