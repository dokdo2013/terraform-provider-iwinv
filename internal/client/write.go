package client

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"sort"
)

const maxRequestBytes = 1 << 20

var formFieldName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.]*$`)

// PostJSON and PutJSON make exactly one attempt. A failed write can have taken
// effect remotely: the caller must reconcile its outcome, never blindly repeat
// it. Successful HTTP 202 is preserved and does not imply operation completion.
func (c *Client) PostJSON(ctx context.Context, path string, body any) (Envelope, error) {
	return c.writeJSON(ctx, http.MethodPost, path, body)
}
func (c *Client) PutJSON(ctx context.Context, path string, body any) (Envelope, error) {
	return c.writeJSON(ctx, http.MethodPut, path, body)
}
func (c *Client) writeJSON(ctx context.Context, method, path string, body any) (Envelope, error) {
	if !validPath(path) {
		return Envelope{}, &Error{Kind: "invalid_path"}
	}
	if err := ctx.Err(); err != nil {
		return Envelope{}, err
	}
	encoded, err := json.Marshal(body)
	// Control-plane write contracts take JSON objects, not null or scalars.
	if err != nil || len(encoded) == 0 || encoded[0] != '{' {
		return Envelope{}, &Error{Kind: "invalid_json_request"}
	}
	if len(encoded) > maxRequestBytes {
		return Envelope{}, &Error{Kind: "request_too_large"}
	}
	return c.request(ctx, method, path, nil, "application/json", bytes.NewReader(encoded))
}

// PostForm and PutForm implement multipart/form-data. Values are transmitted
// verbatim: any endpoint-specific percent encoding belongs in its service layer.
// An empty string is sent; absence requires omitting the field from the map.
func (c *Client) PostForm(ctx context.Context, path string, fields map[string]string) (Envelope, error) {
	return c.writeForm(ctx, http.MethodPost, path, fields)
}
func (c *Client) PutForm(ctx context.Context, path string, fields map[string]string) (Envelope, error) {
	return c.writeForm(ctx, http.MethodPut, path, fields)
}

// PutFormWithQuery allows a service to select a safe response projection on a
// multipart update. Query values are URL-encoded, never included in the HMAC
// path, and cannot be supplied by embedding a query in path.
func (c *Client) PutFormWithQuery(ctx context.Context, path string, query url.Values, fields map[string]string) (Envelope, error) {
	return c.writeFormQuery(ctx, http.MethodPut, path, query, fields)
}
func (c *Client) writeForm(ctx context.Context, method, path string, fields map[string]string) (Envelope, error) {
	return c.writeFormQuery(ctx, method, path, nil, fields)
}
func (c *Client) writeFormQuery(ctx context.Context, method, path string, query url.Values, fields map[string]string) (Envelope, error) {
	if !validPath(path) {
		return Envelope{}, &Error{Kind: "invalid_path"}
	}
	if err := ctx.Err(); err != nil {
		return Envelope{}, err
	}
	names := make([]string, 0, len(fields))
	size := 0
	for name, value := range fields {
		if !formFieldName.MatchString(name) {
			return Envelope{}, &Error{Kind: "invalid_form_field"}
		}
		size += len(name) + len(value)
		if size > maxRequestBytes {
			return Envelope{}, &Error{Kind: "request_too_large"}
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range names {
		if err := writer.WriteField(name, fields[name]); err != nil {
			return Envelope{}, &Error{Kind: "invalid_form_request"}
		}
		if body.Len() > maxRequestBytes {
			return Envelope{}, &Error{Kind: "request_too_large"}
		}
	}
	if err := writer.Close(); err != nil {
		return Envelope{}, &Error{Kind: "invalid_form_request"}
	}
	if body.Len() > maxRequestBytes {
		return Envelope{}, &Error{Kind: "request_too_large"}
	}
	return c.request(ctx, method, path, query, writer.FormDataContentType(), bytes.NewReader(body.Bytes()))
}

// Delete performs one signed DELETE with no body and no automatic retries.
// Absence and completion must be established by a verified service Read.
func (c *Client) Delete(ctx context.Context, path string) (Envelope, error) {
	return c.request(ctx, http.MethodDelete, path, nil, "", nil)
}
