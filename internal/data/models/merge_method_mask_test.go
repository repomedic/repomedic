package models

import "testing"

func TestMergeMethodMask_String_DeterministicOrder(t *testing.T) {
	tests := []struct {
		name string
		mask MergeMethodMask
		want string
	}{
		{name: "empty", mask: 0, want: ""},
		{name: "merge", mask: MergeMethodMerge, want: "merge"},
		{name: "squash", mask: MergeMethodSquash, want: "squash"},
		{name: "rebase", mask: MergeMethodRebase, want: "rebase"},
		{name: "merge+rebase", mask: MergeMethodMerge | MergeMethodRebase, want: "merge,rebase"},
		{name: "all", mask: MergeMethodMerge | MergeMethodSquash | MergeMethodRebase, want: "merge,squash,rebase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mask.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeMethodMask_Size_Has_Intersect(t *testing.T) {
	m := MergeMethodMerge | MergeMethodRebase
	if got, want := m.Size(), 2; got != want {
		t.Fatalf("Size() = %d, want %d", got, want)
	}
	if !m.Has(MergeMethodMerge) {
		t.Fatalf("expected mask to have merge")
	}
	if m.Has(MergeMethodSquash) {
		t.Fatalf("expected mask to not have squash")
	}
	if got := m.Intersect(MergeMethodMerge | MergeMethodSquash); got != MergeMethodMerge {
		t.Fatalf("Intersect() = %v, want %v", got, MergeMethodMerge)
	}
}
