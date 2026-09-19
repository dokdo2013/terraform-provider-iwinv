package compute

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// InstanceWriteAPI is separate from API so catalog/read adapters cannot write.
// These documentation-derived methods are not registered Terraform capabilities.
type InstanceWriteAPI interface {
	PostForm(context.Context, string, map[string]string) (client.Envelope, error)
	PutFormWithQuery(context.Context, string, url.Values, map[string]string) (client.Envelope, error)
	Delete(context.Context, string) (client.Envelope, error)
}

type InstanceWriter struct{ API InstanceWriteAPI }

// No count, inline volumes or deprecated network inputs: one owned instance per
// request, with separate attachment ownership still gated on live contracts.
type InstanceCreateInput struct {
	ZoneID, ImageID, FlavorID string
	Name, Description         *string
	SSHKeyIDs                 []string
	UserScriptID              *string
}

// Nil omits an input; a non-nil empty description explicitly sends empty.
// Whether that clears the server value remains unverified, not promised here.
type InstanceUpdateInput struct{ Name, Description *string }

// CreateReceipt must be handled even when CreateInstance returns an error.
// ID is set only for one identifiable result row, before remaining validation.
// RecoveryIDs preserves all valid unique IDs from an unexpected multi-create;
// callers must journal them, never select the first or blindly replay a create.
type CreateReceipt struct {
	ID          string
	RecoveryIDs []string
}

// DeletionReceipt acknowledges a request, not absence or billing termination.
// These volume IDs are reported as retained by the documented delete response;
// they are not automatically deleted or adopted by the instance owner.
type DeletionReceipt struct{ RetainedBlockStorageIDs []string }

func instanceWritePath(id string) (string, error) {
	if !catalogID.MatchString(id) {
		return "", errors.New("instance ID must be a single supported path segment")
	}
	return "/v1/instances/" + id, nil
}

func instanceTextFields(name, description *string) (map[string]string, error) {
	fields := make(map[string]string)
	if name != nil {
		if *name == "" || !utf8.ValidString(*name) {
			return nil, errors.New("instance name must be nonempty valid UTF-8 when supplied")
		}
		fields["name"] = url.QueryEscape(*name)
	}
	if description != nil {
		if !utf8.ValidString(*description) {
			return nil, errors.New("instance description must be valid UTF-8")
		}
		fields["description"] = url.QueryEscape(*description)
	}
	return fields, nil
}

func createInstanceFields(in InstanceCreateInput) (map[string]string, error) {
	if !catalogID.MatchString(in.ZoneID) || !catalogID.MatchString(in.ImageID) || !catalogID.MatchString(in.FlavorID) {
		return nil, errors.New("instance create requires valid zone, image and flavor identifiers")
	}
	fields, err := instanceTextFields(in.Name, in.Description)
	if err != nil {
		return nil, err
	}
	fields["zone_id"], fields["image_id"], fields["flavor_id"] = in.ZoneID, in.ImageID, in.FlavorID
	fields["count"] = "1"
	keys := append([]string(nil), in.SSHKeyIDs...)
	sort.Strings(keys)
	for i, key := range keys {
		if !catalogID.MatchString(key) || (i > 0 && keys[i-1] == key) {
			return nil, errors.New("SSH key identifiers must be valid and unique")
		}
	}
	if len(keys) > 0 {
		fields["ssh_key_id"] = strings.Join(keys, ",")
	}
	if in.UserScriptID != nil {
		if !catalogID.MatchString(*in.UserScriptID) {
			return nil, errors.New("user script identifier must be valid when supplied")
		}
		fields["user_script_id"] = *in.UserScriptID
	}
	return fields, nil
}

func instanceCreateReceipt(e client.Envelope) (CreateReceipt, error) {
	var receipt CreateReceipt
	if e.Status < 200 || e.Status >= 300 {
		return receipt, errors.New("instance create returned an unsuccessful HTTP status")
	}
	var rows []json.RawMessage
	if json.Unmarshal(e.Result, &rows) != nil || rows == nil {
		return receipt, errors.New("instance create result must be an array")
	}
	seen := map[string]bool{}
	for _, raw := range rows {
		var row struct {
			ID string `json:"instance_id"`
		}
		if json.Unmarshal(raw, &row) == nil && catalogID.MatchString(row.ID) && !seen[row.ID] {
			seen[row.ID] = true
			receipt.RecoveryIDs = append(receipt.RecoveryIDs, row.ID)
		}
	}
	sort.Strings(receipt.RecoveryIDs)
	if len(rows) == 1 && len(receipt.RecoveryIDs) == 1 {
		receipt.ID = receipt.RecoveryIDs[0]
	}
	count, err := integer(e.Count)
	if e.Status != 202 || err != nil || count != 1 || len(rows) != 1 || receipt.ID == "" {
		return receipt, errors.New("instance create acknowledgement is inconsistent; preserve received identities and reconcile without replay")
	}
	var row struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(rows[0], &row) != nil || row.Status == "" {
		return receipt, errors.New("instance create status is missing; preserve received identity and reconcile without replay")
	}
	return receipt, nil
}

// CreateInstance submits one request. A successful receipt does not mean ready.
// On transport/business errors no response identity is trusted or synthesized.
func (s *InstanceWriter) CreateInstance(ctx context.Context, in InstanceCreateInput) (CreateReceipt, error) {
	fields, err := createInstanceFields(in)
	if err != nil {
		return CreateReceipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return CreateReceipt{}, err
	}
	e, err := s.API.PostForm(ctx, "/v1/instances", fields)
	if err != nil {
		return CreateReceipt{}, err
	}
	return instanceCreateReceipt(e)
}

// UpdateInstance returns the fixed safe projection. A post-write decode failure
// leaves reconciliation to the caller; it must not trigger another write.
func (s *InstanceWriter) UpdateInstance(ctx context.Context, id string, in InstanceUpdateInput) (Instance, error) {
	path, err := instanceWritePath(id)
	if err != nil {
		return Instance{}, err
	}
	fields, err := instanceTextFields(in.Name, in.Description)
	if err != nil {
		return Instance{}, err
	}
	if len(fields) == 0 {
		return Instance{}, errors.New("instance update requires at least one supplied attribute")
	}
	if err := ctx.Err(); err != nil {
		return Instance{}, err
	}
	e, err := s.API.PutFormWithQuery(ctx, path, url.Values{"fields": {strconv.Itoa(instanceReadFields)}}, fields)
	if err != nil {
		return Instance{}, err
	}
	rows, err := instanceRows(e)
	if err != nil {
		return Instance{}, err
	}
	if len(rows) != 1 || rows[0].ID != id {
		return Instance{}, errors.New("instance update did not return exactly one matching identifier; reconcile without replay")
	}
	return rows[0], nil
}

func (s *InstanceWriter) DeleteInstance(ctx context.Context, id string) (DeletionReceipt, error) {
	path, err := instanceWritePath(id)
	if err != nil {
		return DeletionReceipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return DeletionReceipt{}, err
	}
	e, err := s.API.Delete(ctx, path)
	if err != nil {
		return DeletionReceipt{}, err
	}
	var rows []struct {
		Status  string   `json:"status"`
		Volumes []string `json:"block_storage"`
	}
	count, countErr := integer(e.Count)
	if e.Status != 202 || json.Unmarshal(e.Result, &rows) != nil || len(rows) != 1 ||
		countErr != nil || count != 1 || rows[0].Status != "deleting" || rows[0].Volumes == nil {
		return DeletionReceipt{}, errors.New("instance deletion acknowledgement is inconsistent; reconcile without replay")
	}
	ids := rows[0].Volumes
	sort.Strings(ids)
	for i, id := range ids {
		if !catalogID.MatchString(id) || (i > 0 && ids[i-1] == id) {
			return DeletionReceipt{}, errors.New("instance deletion returned invalid retained-volume identifiers; reconcile without replay")
		}
	}
	return DeletionReceipt{RetainedBlockStorageIDs: ids}, nil
}
