package network

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// Rule is one independently owned rule under a security group. Numeric API IDs
// are represented exactly as decimal strings, never converted through float64.
// Unlike group descriptions, rule name/content responses are not HTML-decoded.
type Rule struct {
	ID, Bound, Protocol, Port, IP, Name string
	Description                         *string
}
type RuleInput struct {
	Bound, Protocol, Port, IP, Name string
	Description                     *string
}
type CreatedRule struct {
	ID   string
	Rule *Rule
}

func ruleCollectionPath(groupID string) (string, error) {
	p, err := groupPath(groupID)
	if err != nil {
		return "", err
	}
	return p + "/rules", nil
}
func ValidateRuleID(id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != id {
		return errors.New("rule ID must be a canonical positive decimal int64")
	}
	return nil
}
func rulePath(groupID, id string) (string, error) {
	p, err := ruleCollectionPath(groupID)
	if err != nil {
		return "", err
	}
	if err = ValidateRuleID(id); err != nil {
		return "", err
	}
	return p + "/" + id, nil
}

// ParseRulePorts preserves the API's one-port/range semantics, with no special
// values invented for ICMP or all protocols. The supported model is TCP/UDP.
func ParseRulePorts(port string) (int64, int64, error) {
	parts := strings.Split(port, "-")
	if len(parts) < 1 || len(parts) > 2 {
		return 0, 0, errors.New("rule port must be a number or a range")
	}
	parse := func(s string) (int64, error) {
		if s == "" {
			return 0, errors.New("rule port is empty")
		}
		for _, c := range s {
			if c < '0' || c > '9' {
				return 0, errors.New("rule port must contain decimal digits")
			}
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 1 || n > 65535 {
			return 0, errors.New("rule port must be between 1 and 65535")
		}
		return n, nil
	}
	from, err := parse(parts[0])
	if err != nil {
		return 0, 0, err
	}
	to := from
	if len(parts) == 2 {
		to, err = parse(parts[1])
		if err != nil {
			return 0, 0, err
		}
	}
	if from > to {
		return 0, 0, errors.New("rule port range must be ascending")
	}
	return from, to, nil
}

// ValidateRuleIPv4 accepts only the verified IPv4 CIDR form. Host bits are
// preserved: the API does not canonicalize them to the network address.
func ValidateRuleIPv4(ip string) error {
	p, err := netip.ParsePrefix(ip)
	if err == nil && p.Addr().Is4() {
		return nil
	}
	return errors.New("rule IP must be an IPv4 CIDR; bare IP and IPv6 are outside the verified contract")
}

func ruleBody(in RuleInput, updating bool) (map[string]string, error) {
	if in.Bound != "IN" && in.Bound != "OUT" {
		return nil, errors.New("rule direction must be IN or OUT")
	}
	if in.Protocol != "TCP" && in.Protocol != "UDP" {
		return nil, errors.New("only verified TCP and UDP rule protocols are supported")
	}
	if _, _, err := ParseRulePorts(in.Port); err != nil {
		return nil, err
	}
	if err := ValidateRuleIPv4(in.IP); err != nil {
		return nil, err
	}
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 25 {
		return nil, errors.New("rule name must contain 1 to 25 Unicode characters")
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > 25 {
		return nil, errors.New("rule description must contain at most 25 Unicode characters")
	}
	if updating && in.Description != nil && *in.Description == "" {
		return nil, errors.New("an existing rule description cannot be cleared by an empty update; recreate the rule to clear it")
	}
	b := map[string]string{"bound": in.Bound, "protocol": in.Protocol, "port": in.Port, "ip": in.IP, "title": in.Name}
	if in.Description != nil {
		b["content"] = *in.Description
	}
	return b, nil
}
func ruleRows(e client.Envelope) ([]Rule, error) {
	if e.Status != 200 || len(e.Page) != 0 || len(e.PageNo) != 0 || len(e.PageSize) != 0 || len(e.Total) != 0 {
		return nil, errors.New("rule response status or unpaginated contract changed")
	}
	var rows []struct {
		ID          *int64          `json:"rule_id"`
		Bound       string          `json:"bound"`
		Protocol    string          `json:"protocol"`
		Port        string          `json:"port"`
		IP          string          `json:"ip"`
		Name        string          `json:"title"`
		Description json.RawMessage `json:"content"`
	}
	if json.Unmarshal(e.Result, &rows) != nil || rows == nil {
		return nil, errors.New("rule result must be an array with integer IDs")
	}
	var count *int
	if json.Unmarshal(e.Count, &count) != nil || count == nil || *count != len(rows) {
		return nil, errors.New("rule count is missing or inconsistent")
	}
	out := make([]Rule, 0, len(rows))
	seen := map[string]bool{}
	for _, r := range rows {
		var desc *string
		if r.ID == nil || *r.ID <= 0 || r.Name == "" || json.Unmarshal(r.Description, &desc) != nil {
			return nil, errors.New("rule has missing or invalid fields")
		}
		id := strconv.FormatInt(*r.ID, 10)
		if seen[id] {
			return nil, errors.New("rule list repeats an ID")
		}
		seen[id] = true
		if r.Bound != "IN" && r.Bound != "OUT" {
			return nil, errors.New("rule direction contract changed")
		}
		if r.Protocol != "TCP" && r.Protocol != "UDP" {
			return nil, errors.New("rule protocol is not supported by the verified contract")
		}
		if _, _, err := ParseRulePorts(r.Port); err != nil {
			return nil, err
		}
		if err := ValidateRuleIPv4(r.IP); err != nil {
			return nil, err
		}
		out = append(out, Rule{ID: id, Bound: r.Bound, Protocol: r.Protocol, Port: r.Port, IP: r.IP, Name: r.Name, Description: desc})
	}
	return out, nil
}

// Rules validates the complete, unpaginated rule result. The API ignores group
// list pagination parameters here. It never treats API errors as an empty list.
func (s *Service) Rules(ctx context.Context, groupID string) ([]Rule, error) {
	p, err := ruleCollectionPath(groupID)
	if err != nil {
		return nil, err
	}
	e, err := s.API.Get(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	return ruleRows(e)
}

// Rule establishes parent existence separately. A deleted parent's rules
// endpoint can return CHECK_PARAM, which alone must never discard rule state.
func (s *Service) Rule(ctx context.Context, groupID, id string) (*Rule, error) {
	if _, err := rulePath(groupID, id); err != nil {
		return nil, err
	}
	group, err := s.Group(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, nil
	}
	rows, err := s.Rules(ctx, groupID)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, nil
}
func (s *Service) CreateRule(ctx context.Context, groupID string, in RuleInput) (CreatedRule, error) {
	p, err := ruleCollectionPath(groupID)
	if err != nil {
		return CreatedRule{}, err
	}
	b, err := ruleBody(in, false)
	if err != nil {
		return CreatedRule{}, err
	}
	e, err := s.API.PostJSON(ctx, p, b)
	if err != nil {
		return CreatedRule{}, err
	}
	if e.Status < 200 || e.Status > 202 {
		return CreatedRule{}, errors.New("rule create returned an unsuccessful status")
	}
	var identities []struct {
		ID *int64 `json:"rule_id"`
	}
	if json.Unmarshal(e.Result, &identities) != nil || len(identities) != 1 || identities[0].ID == nil || *identities[0].ID <= 0 {
		return CreatedRule{}, errors.New("rule create identity unresolved; reconcile without retrying create")
	}
	created := CreatedRule{ID: strconv.FormatInt(*identities[0].ID, 10)}
	rows, err := ruleRows(e)
	if err != nil {
		return created, err
	}
	created.Rule = &rows[0]
	return created, nil
}
func (s *Service) UpdateRule(ctx context.Context, groupID, id string, in RuleInput) (*Rule, error) {
	p, err := rulePath(groupID, id)
	if err != nil {
		return nil, err
	}
	b, err := ruleBody(in, true)
	if err != nil {
		return nil, err
	}
	e, err := s.API.PutJSON(ctx, p, b)
	if err != nil {
		return nil, err
	}
	rows, err := ruleRows(e)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 || rows[0].ID != id {
		return nil, errors.New("rule update did not return exactly the requested ID")
	}
	return &rows[0], nil
}
func (s *Service) DeleteRule(ctx context.Context, groupID, id string) error {
	p, err := rulePath(groupID, id)
	if err != nil {
		return err
	}
	e, err := s.API.Delete(ctx, p)
	if err != nil {
		return err
	}
	var ack *string
	if e.Status != 200 || json.Unmarshal(e.Result, &ack) != nil || ack == nil {
		return errors.New("rule delete acknowledgement contract changed; verify state before another write")
	}
	return nil
}
