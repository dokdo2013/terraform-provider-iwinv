package compute

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
)

const sshKeyPageSize = 10
const sshKeyMaxPages = 1000

// SSHKey contains reference metadata only, never key material. The API exposes
// a list, not a detail endpoint; exact lookup must finish validating all pages.
type SSHKey struct {
	ID   string `json:"ssh_key_id"`
	Name string `json:"name"`
}

func (s *Service) SSHKeys(ctx context.Context) ([]SSHKey, error) {
	keys := make([]SSHKey, 0)
	seen := map[string]bool{}
	for page := 1; page <= sshKeyMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e, err := s.API.Get(ctx, "/v1/auth/ssh_key", url.Values{"page_no": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(sshKeyPageSize)}})
		if err != nil {
			return nil, err
		}
		if e.Status != 200 || len(e.Page) != 0 || len(e.Total) != 0 {
			return nil, errors.New("SSH key list returned an unverified response contract")
		}
		var rows []struct {
			ID   string  `json:"ssh_key_id"`
			Name *string `json:"name"`
		}
		if json.Unmarshal(e.Result, &rows) != nil || rows == nil {
			return nil, errors.New("SSH key list must be an array")
		}
		count, cerr := integer(e.Count)
		n, nerr := integer(e.PageNo)
		size, serr := integer(e.PageSize)
		if cerr != nil || nerr != nil || serr != nil || count != len(rows) || n != page || size != sshKeyPageSize || count > size {
			return nil, errors.New("SSH key pagination metadata is missing or inconsistent")
		}
		for _, row := range rows {
			if row.ID == "" || row.Name == nil || seen[row.ID] {
				return nil, errors.New("SSH key list has missing fields or duplicate IDs")
			}
			seen[row.ID] = true
			keys = append(keys, SSHKey{ID: row.ID, Name: *row.Name})
		}
		if count < size {
			sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
			return keys, nil
		}
	}
	return nil, errors.New("SSH key list exceeded the bounded pagination limit")
}

func (s *Service) SSHKey(ctx context.Context, id string) (SSHKey, error) {
	if id == "" {
		return SSHKey{}, errors.New("a non-empty exact SSH key ID is required")
	}
	rows, err := s.SSHKeys(ctx)
	if err != nil {
		return SSHKey{}, err
	}
	for _, row := range rows {
		if row.ID == id {
			return row, nil
		}
	}
	return SSHKey{}, errors.New("SSH key lookup did not find the exact requested ID")
}
