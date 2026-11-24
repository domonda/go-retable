# How csvtable reads malformed CSV

RFC 4180, the CSV specification, describes one comma-separated format with
doubled quotes. The files that arrive in practice are different: bank and
accounting exports come in a code page instead of UTF-8, use `;` or tab as
separator, escape quotes in two different ways, and put a title, a period and
a total around the table. `encoding/csv` rejects many of them, and a guess at
the format can silently produce wrong values. This page explains how
`csvtable` gets usable rows out of such files, and what it gives up for that.

For the API, see the [reference](reference.md); for recipes, see
[How to read and write CSV files](howto-read-and-write-csv.md).

## The problem

Strict parsing fails the whole file for one bad field. A user who uploads a
bank statement gets an error about `bare " in non-quoted-field` in line 812,
cannot fix the export of their bank, and gives up. Lenient parsing has the
opposite failure: a wrong guess produces rows that look right and hold wrong
values. Three kinds of guess matter:

- **The separator.** Counting characters picks the comma in a German file
  where every amount is written `1.234,56` and the real separator is `;`.
- **The encoding.** Decoding Windows 1252 as UTF-8 turns `Müller` into
  `M ller`, decoding UTF-8 as Windows 1252 turns it into `MÃ¼ller`.
- **The column titles.** Taking the first line as titles makes the report
  title `Kontoauszug Nr. 4` the only column, and every struct field stays empty.

`csvtable` is built to parse every file without an error, and to make each
guess from the structure of the whole file rather than from a single count.

## Lines first, quotes second

The parser splits the data into lines at the detected newline, then splits
each line at the separator, and only then looks at quotes. A field that starts
a quoted value but doesn't end it was split by a separator or newline inside
the quotes, so it is joined with the following fields, on the same line or on
later lines, until a field closes the quote.

```
"a","{""k"":""v"",""k2"":""v2""}","b"
-> ["a" `{"k":"v","k2":"v2"}` "b"]
```

The alternative, a state machine that tracks quotes byte by byte, is what
`encoding/csv` does. It is exact for valid files, but one unbalanced quote
puts it into the quoted state for the rest of the file, and every following
row becomes part of one field. Splitting first bounds the damage of a bad
quote to the lines it can reach.

### Quoting is decided by parity

A field opens a quoted value when its leading run of quotes is odd, and the
value is still open when the total number of quotes in the field is odd. This
covers every combination of leading and trailing quotes, so no quote pattern
can make the parser fail, and it covers the two escaping conventions real
exporters mix:

```
1997,""Ford"",E350,"Super, luxurious truck"
-> ["1997" `"Ford"` "E350" "Super, luxurious truck"]
```

`""Ford""` is not valid RFC 4180, and `encoding/csv` rejects it with
`bare " in non-quoted-field`. Bank exports write it for a value that contains
quotes, so it is read as the quoted text.

### An unterminated quote costs one field

When no later field closes a quote, the field keeps its literal text and the
following lines are parsed normally:

```
a;"oops
b;c
d;e
-> [["a" `"oops`] ["b" "c"] ["d" "e"]]
```

The file is imported with one wrong value instead of failing or swallowing all
rows after the quote.

## Detection looks at the structure

### The separator that makes the table rectangular

The right separator is the one that splits the rows into the same number of
columns, not the one that occurs most often. `,` `;` tab and `|` are each
scored by the share of records they split into their most common column count:

```
Name;Beschreibung
Meier;Wien, Graz, Linz         3 semicolons vs 4 commas
Huber;Wels, Steyr, Melk

;  ->  2/2/2 columns, uniform      <- chosen
,  ->  1/3/3 columns, not uniform
```

Separators inside quoted fields are not counted, so a quoted
`"Gehalt, Januar"` cannot vote for the comma.

Header and trailer lines are single-column records, and a bank statement often
has more of them than data rows. If they voted, the most common column count
of every candidate would be one, and no separator would be found. So records a
candidate does not split are left out of its vote. That opens the opposite
failure: a single-column file with one `;` inside one value would make `;` the
separator and cut that value in two. A candidate that leaves records unsplit
therefore only counts when it splits at least 2 records and at least one in
five of all records.

### The line ending outside of quotes

`\r\n`, `\n\r` and bare `\n` are counted outside of quoted fields only. A
multi-line address in a quoted field, written with `\r\n` by a Windows client
into an otherwise `\n` file, cannot switch the whole file to `\r\n`.

### The encoding by its characters

A byte order mark is the only reliable statement of an encoding, so it decides
alone, longest mark first, because the UTF-16LE mark `FF FE` is the start of
the UTF-32LE mark `FF FE 00 00`. Without a mark, the data is decoded with every
candidate encoding, and the one that produces the most known characters wins:
umlauts, `ß`, `§`, `€` and Cyrillic letters. Decoding with a wrong code page
does not fail, it produces other characters, so counting characters the
senders actually use is the only signal available. A tie goes to the earlier
candidate, which is why the order of `FormatDetectionConfig.Encodings`
matters.

Excel writes a UTF-8 byte order mark even in front of files it encodes in a
code page. The code page decoders remove it, because decoded as text it would
put `ï»¿` in front of the first column title, which then matches no struct
field.

### No-break spaces become spaces

Every no-break space (U+00A0) is replaced with a plain space. Spreadsheet
exports are full of them, and they look like spaces but are not trimmed by
code that trims spaces or compared equal by code that compares titles. The
cost is that a value which legitimately contains one is changed.

## Finding the table in the file

The column titles are found with the column titles you expect, not by
position. `DetectTableBounds`, which lives in the root package so that rows of
any format use it, takes the column count of the majority of rows
as the table width, with single-column rows only voting when nothing is wider,
then takes the row of that width that matches the most expected titles. The
table ends at the first row of another width. Blank lines inside the table
don't end it, because joining a multi-line field leaves blank rows behind.

The expected titles come from the struct: `ReadStringsToStructSlice` uses its
`requiredCols`. Without them, the first row of the table width without an
empty field is taken, and a header line as wide as the table is wrongly taken
as the titles. `retable.StructFieldNaming.FindTitleRow` is the stricter
alternative that requires every title of the struct.

## Panics are errors

The parse and read functions are called on uploaded files in HTTP handlers and
job workers, where a panic takes down more than the one import. They recover
any panic and return it as a `retable.PanicError` with the message
`csvtable: panic: <value>`. The stack trace is kept in its `Stack` field,
because a panic that reaches this point is a bug and the stack is what locates
it. It is not part of the message, so that the message can go to the person
who uploaded the file while the stack goes to the log.

## Known limits

All three come from the same design: lines are split first and quoted fields
joined afterwards, so the parser has no quote state while splitting.

- **A `\r` directly before a line break inside a quoted field is lost**, as in
  `A;"x\r\ny";B` in a `\n` file. It can't be told apart from the leftover `\r`
  of a file with mixed line endings, which is removed.
- **CR-only line endings**, as written by classic Mac OS, are not supported.
  `\r` alone is not a valid `Format.Newline`, and such a file parses as one line.
- **A wrong encoding is not reported.** Bytes that are invalid after decoding
  become spaces, so a wrong guess turns `Müller` into `M ller` rather than
  into an error. Check the decoded data yourself, or parse with a known format,
  if that distinction matters.

## Trade-offs

| Choice | Gained | Given up |
| --- | --- | --- |
| Parse every file, keep malformed fields literal | Users can import exports they can't fix | A malformed field reaches your code as text instead of as an error |
| Split lines before quotes | One bad quote damages one field, not the rest of the file | The `\r` before a quoted line break, CR-only files |
| Score separators by uniformity | The right separator for German number formats and quoted commas | Small files with few rows can still be guessed wrong |
| Count characters to detect the encoding | Code page files work without configuration | Two code pages that decode a file identically are reported as the earlier one |
| Replace invalid bytes and no-break spaces | Downstream code can treat every value as clean UTF-8 | No error for a wrong encoding, changed no-break spaces |
| Find the table by expected titles | Header and trailer lines are skipped | A trailer row as wide as the table is imported as data |

For files from a known sender, `ParseWithFormat` with a stored `Format` removes
the guessing from encoding and separator detection.

## Writing: the byte order mark and encoders

On the writing side, `Writer.WithUTF8BOM` exists because Excel opens a
double-clicked CSV file in the legacy code page of the system unless the file
starts with a UTF-8 byte order mark. The writer refuses to combine the mark
with an `Encoder`, returning `ErrUTF8BOMWithEncoder`, because the encoder
converts the output to another encoding, and a UTF-8 mark in front of Windows
1252 bytes would tell every reader to decode them wrongly.

The mark is written together with the first row, not when writing starts.
When a formatter fails or panics before any row is written, the destination
is still empty, as it is without the mark, so an `http.ResponseWriter` can still send
an error status.

## Related

- [csvtable reference](reference.md): every function, the detection rules and
  the writer options
- [How to read and write CSV files](howto-read-and-write-csv.md): recipes and
  troubleshooting
- [StructFieldNaming reference](../retable/reference-struct-field-naming.md):
  matching column titles to struct fields
