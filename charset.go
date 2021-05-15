package retable

type Charset interface {
	Encode(utf8Str []byte) (encodedStr []byte, err error)
}
