package csv

type TextTransformer interface {
	Bytes([]byte) ([]byte, error)
}
