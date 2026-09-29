package community_service

import (
	"errors"
	"reflect"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestApplyToGroupsSeparatesSuccessFromFailure(t *testing.T) {
	ok1 := "120363000000000001@g.us"
	bad := "120363000000000002@g.us"
	ok2 := "120363000000000003@g.us"

	success, failed := applyToGroups([]string{ok1, bad, ok2}, func(g types.JID) error {
		if g.User == "120363000000000002" {
			return errors.New("boom")
		}
		return nil
	})

	if !reflect.DeepEqual(success, []string{ok1, ok2}) {
		t.Errorf("success = %v, want [%s %s]", success, ok1, ok2)
	}
	if !reflect.DeepEqual(failed, []string{bad}) {
		t.Errorf("failed = %v, want [%s]", failed, bad)
	}
}

func TestApplyToGroupsReportsUnparseableGroupsWithoutCallingTheOperation(t *testing.T) {
	called := 0
	success, failed := applyToGroups([]string{"", "@@"}, func(types.JID) error { called++; return nil })
	if called != 0 || len(success) != 0 || len(failed) != 2 {
		t.Fatalf("called=%d success=%v failed=%v", called, success, failed)
	}
}

func TestApplyToGroupsAllFailedLeavesSuccessEmpty(t *testing.T) {
	success, failed := applyToGroups([]string{"120363000000000001@g.us"}, func(types.JID) error { return errors.New("x") })
	if len(success) != 0 || len(failed) != 1 {
		t.Fatalf("success=%v failed=%v", success, failed)
	}
}
