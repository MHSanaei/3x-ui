package duration

import (
	"encoding/json"
	"fmt"
	"time"
)

type Duration int64

// MarshalJSON implements encoding/json.Marshaler.MarshalJSON
func (d *Duration) MarshalJSON() ([]byte, error) {
	dr := time.Duration(*d)
	return json.Marshal(dr.String())
}

// UnmarshalJSON implements encoding/json.Unmarshaler.UnmarshalJSON
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case string:
		var err error
		dr, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(dr)
		return nil
	default:
		return fmt.Errorf("invalid duration: %v", v)
	}
}
