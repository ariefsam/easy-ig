package ariefjson

import (
	"bytes"
	"encoding/json"
)

func Marshal(input any) (ret string) {
	bf := bytes.NewBuffer([]byte{})
	jsonEncoder := json.NewEncoder(bf)
	jsonEncoder.SetEscapeHTML(false)
	jsonEncoder.Encode(input)
	ret = bf.String()
	return
}

func MarshalIndent(input any) (ret string) {
	bf := bytes.NewBuffer([]byte{})
	jsonEncoder := json.NewEncoder(bf)
	jsonEncoder.SetEscapeHTML(false)
	jsonEncoder.SetIndent("", " ")
	jsonEncoder.Encode(input)
	ret = bf.String()
	return
}
