package unifyport

import "testing"

func TestGeneratedOperationCatalogIsCompleteAndDefensive(t *testing.T) {
	operations := Operations()
	if len(operations) != 78 {
		t.Fatalf("len(Operations()) = %d, want 78", len(operations))
	}
	counts := struct {
		retryable    int
		destructive  int
		secretInput  int
		secretOutput int
	}{}
	seen := make(map[string]struct{}, len(operations))
	for _, operation := range operations {
		if operation.ID == "" || operation.Method == "" || operation.Path == "" || operation.Tag == "" {
			t.Fatalf("operation metadata is incomplete: %#v", operation)
		}
		if _, duplicate := seen[operation.ID]; duplicate {
			t.Fatalf("duplicate operation ID %q", operation.ID)
		}
		seen[operation.ID] = struct{}{}
		if operation.Retryable {
			counts.retryable++
		}
		if operation.Mutability == MutabilityDestructive {
			counts.destructive++
		}
		if operation.SecretInput {
			counts.secretInput++
		}
		if operation.SecretOutput {
			counts.secretOutput++
		}
		lookedUp, ok := LookupOperation(operation.ID)
		if !ok || lookedUp != operation {
			t.Fatalf("LookupOperation(%q) = %#v, %t", operation.ID, lookedUp, ok)
		}
	}
	if counts.retryable != 19 || counts.destructive != 8 || counts.secretInput != 9 || counts.secretOutput != 16 {
		t.Fatalf("operation policy counts = %#v", counts)
	}

	// 修改返回切片不得影响内部 catalog，避免调用方改变后续调用的安全策略。
	operations[0].ID = "mutated"
	if refreshed := Operations(); refreshed[0].ID == "mutated" {
		t.Fatal("Operations() returned mutable catalog storage")
	}
	if _, ok := LookupOperation("unknown"); ok {
		t.Fatal("LookupOperation returned true for an unknown ID")
	}
}
