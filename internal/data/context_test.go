package data

import "testing"

func TestMapDataContext_Get(t *testing.T) {
	tests := []struct {
		name      string
		dc        *MapDataContext
		key       DependencyKey
		wantOK    bool
		wantValue any
	}{
		{
			name:      "nil receiver returns not found",
			dc:        nil,
			key:       DepRepoMetadata,
			wantOK:    false,
			wantValue: nil,
		},
		{
			name:      "nil map treated as empty",
			dc:        NewMapDataContext(nil),
			key:       DepRepoMetadata,
			wantOK:    false,
			wantValue: nil,
		},
		{
			name:      "missing key returns not found",
			dc:        NewMapDataContext(map[DependencyKey]any{}),
			key:       DepRepoMetadata,
			wantOK:    false,
			wantValue: nil,
		},
		{
			name: "present key returns value",
			dc: NewMapDataContext(map[DependencyKey]any{
				DepRepoMetadata: "value",
			}),
			key:       DepRepoMetadata,
			wantOK:    true,
			wantValue: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.dc.Get(tt.key)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if got != tt.wantValue {
				t.Fatalf("expected value=%v, got %v", tt.wantValue, got)
			}
		})
	}
}
