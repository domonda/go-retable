package retable

// func TestReflectColumnTitles_ColumnTitlesAndRowReflector(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		n           *StructRowsViewer
// 		structSlice any
// 		wantTitles  []string
// 		// wantRowReflector RowReflector
// 	}{
// 		{
// 			name:        "DefaultReflectColumnTitles empty",
// 			n:           DefaultStructRowsViewer,
// 			structSlice: []struct{}{},
// 			wantTitles:  nil,
// 		},
// 		{
// 			name:        "DefaultReflectColumnTitles empty",
// 			n:           DefaultStructRowsViewer,
// 			structSlice: []struct{ OneTitle int }{{}},
// 			wantTitles:  []string{"One Title"},
// 		},

// 		// TODO
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			structType := reflect.TypeOf(tt.structSlice).Elem()
// 			gotTitles, gotRowReflector := tt.n.ColumnTitlesAndRowReflector(structType)
// 			if !reflect.DeepEqual(gotTitles, tt.wantTitles) {
// 				t.Errorf("ReflectColumnTitles.ColumnTitlesAndRowReflector() gotTitles = %v, want %v", gotTitles, tt.wantTitles)
// 			}
// 			fmt.Println("TODO test", gotRowReflector)
// 		})
// 	}
// }
