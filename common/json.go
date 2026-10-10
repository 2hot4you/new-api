package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/gin-gonic/gin/binding"
)

// ValidateJsonNoDuplicateKeys rejects duplicate decoded object keys, trailing
// values and nesting beyond 64 containers. It is opt-in; existing codec callers
// retain their original behavior. Callers must bound input bytes separately.
func ValidateJsonNoDuplicateKeys(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("invalid or ambiguous JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := validateJSONValue(decoder, 0); err != nil {
		return errors.New("invalid or ambiguous JSON")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("invalid or ambiguous JSON")
	}
	return nil
}

func validateJSONValue(decoder *json.Decoder, depth int) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, container := token.(json.Delim)
	if !container {
		return nil
	}
	if depth >= 64 || delim != '{' && delim != '[' {
		return errors.New("invalid container")
	}
	keys := make(map[string]bool)
	for decoder.More() {
		if delim == '{' {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || keys[key] {
				return errors.New("duplicate key")
			}
			keys[key] = true
		}
		if err := validateJSONValue(decoder, depth+1); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim == '{' && closing != json.Delim('}') || delim == '[' && closing != json.Delim(']') {
		return errors.New("invalid container")
	}
	return nil
}

type RawMessage = json.RawMessage

// hostJSONCodec is the single place where the host chooses its JSON engine.
// Swap the implementation here (for example to sonic.ConfigStd) and every
// common.* and kitutil.* JSON helper, including relaykit DTO (un)marshalling,
// follows. Injected from init() rather than main() so tests run on the same
// engine as production: common is imported by virtually every root package
// and test binary, while main() never executes under `go test`.
type hostJSONCodec struct{}

func (hostJSONCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (hostJSONCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (hostJSONCodec) Decode(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

func (hostJSONCodec) Valid(data []byte) bool {
	return json.Valid(data)
}

func init() {
	kitutil.SetCodec(hostJSONCodec{})
}

func Unmarshal(data []byte, v any) error {
	return kitutil.Unmarshal(data, v)
}

func UnmarshalJsonStr(data string, v any) error {
	return kitutil.UnmarshalJsonStr(data, v)
}

func DecodeJson(reader io.Reader, v any) error {
	return kitutil.DecodeJson(reader, v)
}

// DecodeJsonWithValidation decodes JSON and applies Gin's configured binding-tag
// validator, including binding:"required" and any registered custom validators.
func DecodeJsonWithValidation(reader io.Reader, v any) error {
	if err := DecodeJson(reader, v); err != nil {
		return err
	}
	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(v)
}

func Marshal(v any) ([]byte, error) {
	return kitutil.Marshal(v)
}

func IndentJson(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	if err := json.Indent(&buffer, data, "", "  "); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func GetJsonType(data RawMessage) string {
	return kitutil.GetJsonType(data)
}

// JsonRawMessageToString returns JSON strings as their decoded value and other JSON values as raw text.
func JsonRawMessageToString(data RawMessage) string {
	return kitutil.JsonRawMessageToString(data)
}
