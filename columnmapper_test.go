package retable

import (
	"reflect"
	"testing"
)

func TestReflectColumnTitles_ColumnTitlesAndRowReflector(t *testing.T) {
	type fields struct {
		Tag                string
		IgnoreTitle        string
		UntaggedFieldTitle func(fieldName string) (columnTitle string)
		MapIndices         map[int]int
	}
	type args struct {
		structType reflect.Type
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantTitles       []string
		wantRowReflector RowReflector
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &ReflectColumnTitles{
				Tag:                tt.fields.Tag,
				IgnoreTitle:        tt.fields.IgnoreTitle,
				UntaggedFieldTitle: tt.fields.UntaggedFieldTitle,
				MapIndices:         tt.fields.MapIndices,
			}
			gotTitles, gotRowReflector := n.ColumnTitlesAndRowReflector(tt.args.structType)
			if !reflect.DeepEqual(gotTitles, tt.wantTitles) {
				t.Errorf("ReflectColumnTitles.ColumnTitlesAndRowReflector() gotTitles = %v, want %v", gotTitles, tt.wantTitles)
			}
			if !reflect.DeepEqual(gotRowReflector, tt.wantRowReflector) {
				t.Errorf("ReflectColumnTitles.ColumnTitlesAndRowReflector() gotRowReflector = %v, want %v", gotRowReflector, tt.wantRowReflector)
			}
		})
	}
}

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
