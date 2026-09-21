package logexport

import (
	"reflect"
	"testing"
)

// @sk-test log-export#T4.1: resolver maps tenants to sinks and defaults to none (AC-003)
func TestResolverSinksFor(t *testing.T) {
	r := NewResolver(map[string][]string{
		"alpha": {"webhook-a", "s3-a"},
		"beta":  {"s3-b"},
		"empty": {},
	})

	if got := r.SinksFor("alpha"); !reflect.DeepEqual(got, []string{"s3-a", "webhook-a"}) {
		t.Errorf("alpha = %v, want sorted [s3-a webhook-a]", got)
	}
	if got := r.SinksFor("beta"); !reflect.DeepEqual(got, []string{"s3-b"}) {
		t.Errorf("beta = %v, want [s3-b]", got)
	}
	if got := r.SinksFor("unknown"); got != nil {
		t.Errorf("unknown = %v, want nil", got)
	}
	if got := r.SinksFor("empty"); got != nil {
		t.Errorf("empty route = %v, want nil", got)
	}
}

// @sk-test log-export#T4.1: a nil resolver never routes (AC-001)
func TestResolverNil(t *testing.T) {
	var r *Resolver
	if got := r.SinksFor("alpha"); got != nil {
		t.Errorf("nil resolver = %v, want nil", got)
	}
}
