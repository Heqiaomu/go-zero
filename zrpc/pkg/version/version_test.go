package version

import "testing"

func TestGetString(t *testing.T) {
	tests := []struct {
		name    string
		wantV   string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotV, err := GetString()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotV != tt.wantV {
				t.Errorf("GetString() gotV = %v, want %v", gotV, tt.wantV)
			}
		})
	}
}
