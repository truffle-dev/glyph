package theme

import "testing"

func TestDefaultHasAllTokens(t *testing.T) {
	tests := []struct {
		name string
		got  string
	}{
		{"Bg", string(Default.Bg)},
		{"Surface", string(Default.Surface)},
		{"Border", string(Default.Border)},
		{"Text", string(Default.Text)},
		{"Primary", string(Default.Primary)},
		{"Success", string(Default.Success)},
		{"Warning", string(Default.Warning)},
		{"Error", string(Default.Error)},
		{"Info", string(Default.Info)},
	}
	for _, tt := range tests {
		if tt.got == "" {
			t.Errorf("Default.%s is empty", tt.name)
		}
	}
}

func TestLightAndDefaultShareAccents(t *testing.T) {
	if Default.Primary != Light.Primary {
		t.Errorf("primary should match across themes, got %q vs %q", Default.Primary, Light.Primary)
	}
	if Default.Success != Light.Success {
		t.Errorf("success should match across themes")
	}
}
