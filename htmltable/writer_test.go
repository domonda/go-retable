package htmltable

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
)

func ExampleWriter() {
	type Row struct {
		Status        json.RawMessage `db:"-"              col:"Status"`
		CompanyName   string          `db:"company_name"   col:"Company"`
		InternalNames []string        `db:"internal_names" col:"-"`
		CompanyID     uint64          `db:"company_id"     col:"Company ID"`
	}
	table := []Row{
		{Status: nil, CompanyName: "Company 1", InternalNames: []string{"Company 1a"}, CompanyID: 1},
		{Status: json.RawMessage(`{"ok":true}`), CompanyName: "Company 2", InternalNames: nil, CompanyID: 2},
	}

	NewWriter[[]Row]().
		WithHeaderRow(true).
		WithTypeFormatter(reflect.TypeOf(json.RawMessage(nil)), JSONCellFormatter("")).
		Write(context.Background(), os.Stdout, table, "Table Title")

	// Output:
	// <table>
	//   <caption>Table Title</caption>
	//   <tr><th>Status</th><th>Company</th><th>Company ID</th></tr>
	//   <tr><td></td><td>Company 1</td><td>1</td></tr>
	//   <tr><td><pre>{"ok":true}</pre></td><td>Company 2</td><td>2</td></tr>
	// </table>
}
