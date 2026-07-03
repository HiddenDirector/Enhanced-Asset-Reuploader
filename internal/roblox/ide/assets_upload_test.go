package ide

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"testing"
)

func TestNewCreateAssetRequestMultipartBody(t *testing.T) {
	data := bytes.NewBufferString("FAKE_RBXM_DATA")
	req, err := newCreateAssetRequest("Animation", "MyAnim", "", data, "model/x-rbxm", 42, false)
	if err != nil {
		t.Fatal(err)
	}

	if req.ContentLength <= 0 {
		t.Errorf("ContentLength = %d, want > 0 (no chunked encoding)", req.ContentLength)
	}
	if req.GetBody == nil {
		t.Error("GetBody is nil; redirects/HTTP2 retries can't replay the body")
	}

	_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	mr := multipart.NewReader(req.Body, params["boundary"])

	part1, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if part1.FormName() != "request" {
		t.Errorf("part 1 = %q, want request", part1.FormName())
	}
	j, _ := io.ReadAll(part1)
	if !bytes.Contains(j, []byte(`"displayName":"MyAnim"`)) || !bytes.Contains(j, []byte(`"userId":42`)) {
		t.Errorf("request JSON missing fields: %s", j)
	}

	part2, err := mr.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	if part2.FormName() != "fileContent" {
		t.Errorf("part 2 = %q, want fileContent", part2.FormName())
	}
	if ct := part2.Header.Get("Content-Type"); ct != "model/x-rbxm" {
		t.Errorf("file part Content-Type = %q", ct)
	}
	fileData, _ := io.ReadAll(part2)
	if string(fileData) != "FAKE_RBXM_DATA" {
		t.Errorf("file data = %q", fileData)
	}

	if _, err := mr.NextPart(); err != io.EOF {
		t.Errorf("expected exactly 2 parts, got extra (err=%v)", err)
	}

	// data buffer must not be drained (handlers rebuild the request on retry).
	if data.Len() == 0 {
		t.Error("input buffer was drained; retries would upload empty data")
	}
}
