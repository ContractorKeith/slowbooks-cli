package client

import (
	"context"
	"io"
	"mime"
	"net/http"

	"github.com/ContractorKeith/slowbooks-cli/internal/output"
)

// DownloadResponse describes the successful GET body written to the destination.
type DownloadResponse struct {
	Status      int
	ContentType string
	Filename    string
	Bytes       int64
}

// Download streams successful bodies without the JSON size cap. Once bytes have
// reached an arbitrary writer, replay would corrupt its output, so do not retry.
func (c *Client) Download(ctx context.Context, path string, dst io.Writer) (*DownloadResponse, error) {
	type rewindable interface {
		io.WriteSeeker
		Truncate(int64) error
	}
	rewind, canRewind := dst.(rewindable)
	var offset int64
	if canRewind {
		var err error
		offset, err = rewind.Seek(0, io.SeekCurrent)
		if err != nil {
			canRewind = false
		}
	}
	for attempt := 0; ; attempt++ {
		result, err := c.downloadAttempt(ctx, path, dst)
		if err == nil {
			return result, nil
		}
		if result.Bytes > 0 && !canRewind {
			output.AsError(err).Retryable = false
			return result, err
		}
		if ctx.Err() != nil {
			return result, &output.Error{Code: "network", Message: "download cancelled", Retryable: false}
		}
		if attempt == 1 || !output.AsError(err).Retryable {
			return result, err
		}
		if result.Bytes > 0 {
			if _, resetErr := rewind.Seek(offset, io.SeekStart); resetErr != nil {
				return result, resetErr
			}
			if resetErr := rewind.Truncate(offset); resetErr != nil {
				return result, resetErr
			}
		}
	}
}

func (c *Client) downloadAttempt(ctx context.Context, path string, dst io.Writer) (*DownloadResponse, error) {
	result := &DownloadResponse{}
	target, err := requestURL(c.base, path)
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return result, usage("invalid download path")
	}
	if c.options.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.options.Token)
	}
	if c.options.Company != "" {
		req.Header.Set("X-SlowBooks-Company", c.options.Company)
	}
	req.Header.Set("User-Agent", "sbp/"+c.options.Version)
	resp, err := c.http.Do(req)
	if err != nil {
		return result, networkError("request failed or timed out", true)
	}
	defer resp.Body.Close()
	result.Status = resp.StatusCode
	result.ContentType = resp.Header.Get("Content-Type")
	_, params, _ := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	result.Filename = params["filename"]
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseBytes+1))
		if err != nil {
			return result, networkError("could not read server response", true)
		}
		if int64(len(data)) > c.maxResponseBytes {
			e := networkError("response too large", true)
			e.Retryable = false
			return result, e
		}
		return result, responseError(resp.StatusCode, data, true)
	}
	// Separate reader failures from local disk/stdout failures.
	reader := &downloadReader{Reader: resp.Body}
	result.Bytes, err = io.Copy(dst, reader)
	if err != nil {
		if reader.err != nil {
			e := networkError("could not read server response", true)
			return result, e
		}
		return result, err
	}
	return result, nil
}

type downloadReader struct {
	io.Reader
	err error
}

func (r *downloadReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}
