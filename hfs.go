package hfs

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// HFSpace represents a client to a Hugging Face Space.
// I is the input type, O is the output type. Use `any` if there are different types.
// Use NewHfs() to create an instance.
type HFSpace[I any, O any] struct {
	BaseURL string
	Headers map[string]string
	client  *http.Client
}

// NewHfs creates a new HFSpace with a default HTTP client.
// I is the input type, O is the output type. Use `any` if there are different types.
func NewHfs[I, O any](name string) *HFSpace[I, O] {
	return &HFSpace[I, O]{
		BaseURL: "https://" + name + ".hf.space/gradio_api/call",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		client: http.DefaultClient,
	}
}

// NewHfsRaw creates a new HFSpace for non-standard spaces or not hosted on Hugging Face with a default HTTP client.
// I is the input type, O is the output type. Use `any` if there are different types.
func NewHfsRaw[I, O any](url string) *HFSpace[I, O] {
	return &HFSpace[I, O]{
		BaseURL: strings.TrimRight(url, "/"),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		client: http.DefaultClient,
	}
}

// WithHeader sets a custom header.
func (h *HFSpace[I, O]) WithHeader(key, value string) *HFSpace[I, O] {
	h.Headers[key] = value
	return h
}

// WithBearerToken adds an Authorization Bearer token.
func (h *HFSpace[I, O]) WithBearerToken(token string) *HFSpace[I, O] {
	return h.WithHeader("Authorization", "Bearer "+token)
}

// WithTimeout sets a custom timeout on the underlying HTTP client.
// Applies to both POST and GET requests.
func (h *HFSpace[I, O]) WithTimeout(d time.Duration) *HFSpace[I, O] {
	h.client.Timeout = d
	return h
}

// WithUserAgent sets a custom User-Agent.
func (h *HFSpace[I, O]) WithUserAgent(agent string) *HFSpace[I, O] {
	return h.WithHeader("User-Agent", agent)
}

// WithHTTPClient allows setting a custom http.Client.
func (h *HFSpace[I, O]) WithHTTPClient(client *http.Client) *HFSpace[I, O] {
	h.client = client
	return h
}

// Upload performs data upload to the space.
// Returns server path of the uploaded data.
func (h *HFSpace[I, O]) Upload(data []byte) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("files", "image.jpg")
	if err != nil {
		return "", err
	}

	_, err = io.Copy(part, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	writer.Close()

	url := strings.TrimSuffix(h.BaseURL, "/call") + "/upload"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("hfs bad status: %s", resp.Status)
	}

	var resultPaths []string
	if err := json.NewDecoder(resp.Body).Decode(&resultPaths); err != nil {
		return "", err
	}

	if len(resultPaths) == 0 {
		return "", fmt.Errorf("hfs server returned empty path list")
	}

	return resultPaths[0], nil
}

// Do performs the full request + follow-up GET using the event ID.
func (h *HFSpace[I, O]) Do(endpoint string, params ...I) ([]O, error) {
	fullURL := fmt.Sprintf("%s/%s", h.BaseURL, strings.TrimLeft(endpoint, "/"))

	// Step 1: POST request
	payload := map[string]any{
		"data": params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("hfs req body marshall: %w", err)
	}

	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("hfs post req create: %w", err)
	}
	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hfs post req exec: %w", err)
	}
	defer resp.Body.Close()

	// Decode event ID
	var idResp struct {
		Eventid string `json:"event_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&idResp); err != nil {
		return nil, fmt.Errorf("hfs event ID decode: %w", err)
	}
	eventID := idResp.Eventid

	// Step 2: GET request to fetch final result
	streamURL := fmt.Sprintf("%s/%s", fullURL, eventID)

	getReq, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		return nil, fmt.Errorf("hfs get req create: %w", err)
	}
	for k, v := range h.Headers {
		getReq.Header.Set(k, v)
	}

	resp2, err := h.client.Do(getReq)
	if err != nil {
		return nil, fmt.Errorf("hfs get req exec: %w", err)
	}
	defer resp2.Body.Close()

	res2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, fmt.Errorf("hfs get resp read: %w", err)
	}

	lines := strings.Split(string(res2), "\n")

	EventCompleted := false
	var data string
	for idx, line := range lines {
		if strings.HasPrefix(line, "event:") {
			if strings.Contains(line, "error") {
				hferr := ""
				errline := idx
				hb_count := 0
				for pos, l := range lines {
					if strings.Contains(l, "heartbeat") {
						hb_count++
						continue
					}
					if pos >= errline {
						hferr += l + " -- "
					}
				}
				hferr = fmt.Sprint(hferr, "hbcount: ", hb_count)
				return nil, fmt.Errorf("%s", hferr)
			}
			if strings.Contains(line, "complete") {
				EventCompleted = true
			}
		}
		if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(line[len("data:"):])
			if EventCompleted {
				break
			}
		}
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("hfs no data in resp")
	}

	// Final result
	var Result []O
	if err := json.Unmarshal([]byte(data), &Result); err != nil {
		return nil, fmt.Errorf("hfs decode final resp: %w", err)
	}

	return Result, nil
}

func (h *HFSpace[I, O]) DoFD(endpoint string, params ...I) ([]byte, error) {
	res, err := h.Do(endpoint, params...)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("hfs no results from Do()")
	}
	return GetFileData(res[0])
}

// Gradio-compatible FileData structure.
// Usually used for images, audio, video, etc.
type FileData struct {
	Path     string         `json:"path,omitempty"`
	URL      string         `json:"url,omitempty"`
	Size     int64          `json:"size,omitempty"`
	OrigName string         `json:"orig_name,omitempty"`
	MimeType *string        `json:"mime_type"`
	IsStream bool           `json:"is_stream"`
	Meta     map[string]any `json:"meta,omitempty"`
}

func NewFileData(name string) *FileData {
	return &FileData{
		OrigName: name,
		IsStream: false,
		MimeType: nil,
		Meta:     map[string]any{"_type": "gradio.FileData"},
	}
}

func (fd *FileData) FromUrl(url string) (*FileData, error) {
	fd.URL = url
	fd.Path = ""
	fd.Size = 0
	return fd, nil
}

func (fd *FileData) FromBytes(data []byte) (*FileData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("hfs empty data")
	}

	mimeType := http.DetectContentType(data)
	encodedData := base64.StdEncoding.EncodeToString(data)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encodedData)

	fd.URL = dataURI
	fd.Size = int64(len(data))
	fd.Path = ""

	return fd, nil
}

func (fd *FileData) FromBase64(b64 string) (*FileData, error) {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("hfs base64 decode: %w", err)
	}
	return fd.FromBytes(decoded)
}

func (fd *FileData) FromUpload(hfs HFSpace[any, any], data []byte) (*FileData, error) {
	path, err := hfs.Upload(data)
	if err != nil {
		return nil, err
	}

	fd.Path = path
	fd.URL = ""
	fd.Size = 0
	return fd, nil
}

// Parse src into a FileData struct.
func ParseFileData(src any) (*FileData, error) {
	var fd FileData

	switch v := src.(type) {
	case FileData:
		fd = v
	case *FileData:
		if v == nil {
			return nil, fmt.Errorf("hfs nil *FileData")
		}
		fd = *v
	case map[string]any:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("hfs filedata map json encode: %w", err)
		}
		if err := json.Unmarshal(b, &fd); err != nil {
			return nil, fmt.Errorf("hfs filedata map json decode: %w", err)
		}
	default:
		b, err := json.Marshal(src)
		if err != nil {
			return nil, fmt.Errorf("hfs filedata json encode: %w", err)
		}
		if err := json.Unmarshal(b, &fd); err != nil {
			return nil, fmt.Errorf("hfs filedata json decode: %w", err)
		}
	}

	return &fd, nil
}

// Download content from a FileData's HTTPS URL.
// Use on output FileData.
func FileDataDownload(fileData *FileData, timeout time.Duration) ([]byte, error) {
	// Validate input
	if fileData == nil {
		return nil, fmt.Errorf("hfs filedata is nil")
	}

	if fileData.URL == "" {
		return nil, fmt.Errorf("hfs filedata URL is empty")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout * time.Second,
	}

	// Create the request
	req, err := http.NewRequest("GET", fileData.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("hfs filedata get req create: %w", err)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hfs filedata get req exec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hfs filedata get resp status: %d %s", resp.StatusCode, resp.Status)
	}

	// Read the response body
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hfs filedata get resp read: %w", err)
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("hfs downloaded content is empty")
	}

	return content, nil
}

// Check if src is a FileData.
// Download content from FileData's URL if so.
func GetFileData(src any) ([]byte, error) {
	fd, err := ParseFileData(src)
	if err != nil {
		return nil, fmt.Errorf("hfs get filedata parse: %w", err)
	}
	return FileDataDownload(fd, 60*time.Second)
}

// Check if src is gallery.
// Download content image on gallery idx.
func GetGalleryImage(src any, idx int) ([]byte, error) {
	garr, ok := src.([]any)
	if !ok {
		return nil, fmt.Errorf("hfs not gallery")
	}
	if len(garr) < idx+1 {
		return nil, fmt.Errorf("hfs no idx in gallery")
	}
	json := garr[idx].(map[string]any)
	fd, err := ParseFileData(json["image"])
	if err != nil {
		return nil, err
	}

	fd.URL = strings.ReplaceAll(fd.URL, "space/ca", "space")
	img, err := FileDataDownload(fd, 60*time.Second)
	if err != nil {
		return nil, err
	}
	return img, err
}
