package retable

import "testing"

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
