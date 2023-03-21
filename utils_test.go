package retable

import (
	"testing"
)

func TestSpacePascalCase(t *testing.T) {
	tests := []struct {
		testName string
		name     string
		want     string
	}{
		{testName: "", name: "", want: ""},
		{testName: "HelloWorld", name: "HelloWorld", want: "Hello World"},
		{testName: "_Hello_World", name: "_Hello_World", want: "Hello World"},
		{testName: "helloWorld", name: "helloWorld", want: "hello World"},
		{testName: "helloWorld_", name: "helloWorld_", want: "hello World"},
		{testName: "ThisHasMoreSpacesForSure", name: "ThisHasMoreSpacesForSure", want: "This Has More Spaces For Sure"},
		{testName: "ThisHasMore_Spaces__ForSure", name: "ThisHasMore_Spaces__ForSure", want: "This Has More Spaces For Sure"},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			if got := SpacePascalCase(tt.name); got != tt.want {
				t.Errorf("SpacePascalCase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStructFieldIndex(t *testing.T) {
	type innerStruct struct {
		B int
		C bool
	}
	type testStruct struct {
		A int
		innerStruct
		private string
		D       bool
	}
	var s testStruct

	tests := []struct {
		name      string
		structPtr any
		fieldPtr  any
		want      int
		wantErr   bool
	}{
		{name: "A", structPtr: &s, fieldPtr: &s.A, want: 0},
		{name: "B", structPtr: &s, fieldPtr: &s.B, want: 1},
		{name: "C", structPtr: &s, fieldPtr: &s.C, want: 2},
		{name: "D", structPtr: &s, fieldPtr: &s.D, want: 3},

		// Errors
		{name: "nil, nil", structPtr: nil, fieldPtr: nil, wantErr: true},
		{name: "unexported field", structPtr: &s, fieldPtr: &s.private, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StructFieldIndex(tt.structPtr, tt.fieldPtr)
			if (err != nil) != tt.wantErr {
				t.Errorf("StructFieldIndex() error = '%s', wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("StructFieldIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}
