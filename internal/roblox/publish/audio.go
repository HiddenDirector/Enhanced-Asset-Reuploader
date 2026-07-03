package publish

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kartFr/Asset-Reuploader/internal/roblox"
)

var UploadAudioErrors = struct {
	ErrModerated        error
	ErrTokenInvalid     error
	ErrNotAuthenticated error
	ErrQuotaExceeded    error
}{
	ErrModerated:        errors.New("moderated name or description"),
	ErrTokenInvalid:     errors.New("XSRF token validation failed"),
	ErrNotAuthenticated: errors.New("user is not authenticated"),
	ErrQuotaExceeded:    errors.New("user audio limit exceeded"),
}

type uploadAudioRequest struct {
	Name              string  `json:"name"`
	File              string  `json:"file"`
	GroupID           int64   `json:"groupId,omitempty"`
	PaymentSource     string  `json:"paymentSource,omitempty"`
	EstimatedFileSize int64   `json:"estimatedFileSize"`
	EstimatedDuration float64 `json:"estimatedDuration"`
	AssetPrivacy      int32   `json:"assetPrivacy"`
}

type publishAudioResponse struct {
	ID     int64  `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

func xorBytes(data []byte, key byte) []byte {
	out := make([]byte, len(data))
	for i := range data {
		out[i] = data[i] ^ key
	}
	return out
}

func newUploadAudioRequest(name string, file string, size int64, groupID int64) (*http.Request, error) {
	body := uploadAudioRequest{
		Name:              name,
		File:              file,
		EstimatedFileSize: size,
		GroupID:           groupID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://publish.roblox.com/v1/audio", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "RobloxStudio/WinInet")
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func NewUploadAudioHandler(c *roblox.Client, name string, data *bytes.Buffer, groupID ...int64) (func() (*publishAudioResponse, error), error) {
	// Encode once without draining data: the old io.Copy from the buffer emptied it,
	// so the moderated-rename retry rebuilt the request with an empty file.
	file := base64.StdEncoding.EncodeToString(data.Bytes())
	size := int64(data.Len())
	var group int64
	if len(groupID) > 0 {
		group = groupID[0]
	}
	currentName := name

	return func() (*publishAudioResponse, error) {
		// Fresh request per attempt: a shared *http.Request would resend a consumed
		// body on retry and stack duplicate Cookie headers.
		req, err := newUploadAudioRequest(currentName, file, size, group)
		if err != nil {
			return nil, err
		}
		req.AddCookie(&http.Cookie{
			Name:  ".ROBLOSECURITY",
			Value: c.Cookie,
		})
		req.Header.Set("x-csrf-token", c.GetToken())

		resp, err := c.DoRequest(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var response publishAudioResponse
		json.NewDecoder(resp.Body).Decode(&response)

		switch resp.StatusCode {
		case http.StatusOK:
			return &response, nil
		case http.StatusBadRequest:
			if response.Errors == nil {
				return nil, errors.New(resp.Status)
			}

			message := response.Errors[0].Message
			if message == "Audio name or description is moderated." {
				currentName = "[Censored]"
				return nil, UploadAudioErrors.ErrModerated
			}

			return nil, errors.New(message)
		case http.StatusUnauthorized:
			if response.Errors == nil {
				return nil, errors.New(resp.Status)
			}

			message := response.Errors[0].Message
			if message == "User is not authenticated" {
				return nil, UploadAudioErrors.ErrNotAuthenticated
			}

			return nil, errors.New(message)
		case http.StatusForbidden:
			c.SetToken(resp.Header.Get("x-csrf-token"))
			return nil, UploadAudioErrors.ErrTokenInvalid
		case http.StatusTooManyRequests:
			if response.Errors == nil {
				return nil, errors.New(resp.Status)
			}

			message := response.Errors[0].Message
			if message == "Audio upload has exceeded user's quota." {
				return nil, UploadAudioErrors.ErrQuotaExceeded
			}

			return nil, errors.New(message)
		default:
			if response.Errors == nil {
				return nil, errors.New(resp.Status)
			}

			return nil, errors.New(response.Errors[0].Message)
		}
	}, nil
}
