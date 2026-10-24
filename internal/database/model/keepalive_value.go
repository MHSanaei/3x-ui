package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const maxKeepAliveValue = 65535

type KeepAliveValue string

func (v KeepAliveValue) String() string { return string(v) }

func (v KeepAliveValue) Seconds() int {
	if n, err := strconv.Atoi(string(v)); err == nil {
		return n
	}
	return 0
}

func (v KeepAliveValue) MinSeconds() int {
	parts := strings.SplitN(string(v), "-", 2)
	n, _ := strconv.Atoi(parts[0])
	return n
}

func (v KeepAliveValue) Validate(allowRange bool) error {
	text := string(v)
	parts := strings.Split(text, "-")
	if len(parts) == 1 {
		n, err := strconv.Atoi(text)
		if err != nil || strconv.Itoa(n) != text || n < 0 || n > maxKeepAliveValue {
			return fmt.Errorf("keepAlive must be an integer from 0 to %d", maxKeepAliveValue)
		}
		return nil
	}
	if len(parts) != 2 || !allowRange {
		return errors.New("keepAlive ranges are supported only for AmneziaWG clients")
	}
	min, errMin := strconv.Atoi(parts[0])
	max, errMax := strconv.Atoi(parts[1])
	if errMin != nil || errMax != nil || strconv.Itoa(min) != parts[0] || strconv.Itoa(max) != parts[1] || min < 0 || max > maxKeepAliveValue || min > max {
		return fmt.Errorf("keepAlive range must be ordered integers from 0 to %d", maxKeepAliveValue)
	}
	return nil
}

func (v KeepAliveValue) Value() (driver.Value, error) {
	return string(v), nil
}

func (v *KeepAliveValue) Scan(src any) error {
	switch value := src.(type) {
	case nil:
		*v = ""
	case string:
		*v = KeepAliveValue(value)
	case []byte:
		*v = KeepAliveValue(string(value))
	case int64:
		if value == 0 {
			*v = ""
		} else {
			*v = KeepAliveValue(strconv.FormatInt(value, 10))
		}
	default:
		return fmt.Errorf("cannot scan %T into KeepAliveValue", src)
	}
	return nil
}

func (v KeepAliveValue) MarshalJSON() ([]byte, error) {
	if v == "" {
		return []byte("0"), nil
	}
	if _, err := strconv.ParseInt(string(v), 10, 64); err == nil {
		return []byte(string(v)), nil
	}
	if err := v.Validate(true); err != nil || !strings.Contains(string(v), "-") {
		return nil, fmt.Errorf("invalid stored keepAlive %q", v)
	}
	return json.Marshal(string(v))
}

func (v *KeepAliveValue) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*v = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		candidate := KeepAliveValue(text)
		if err := candidate.Validate(true); err != nil {
			return err
		}
		*v = candidate
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return errors.New("keepAlive must be a number or an AmneziaWG range string")
	}
	n, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil {
		return errors.New("keepAlive must be an integer")
	}
	*v = KeepAliveValue(strconv.FormatInt(n, 10))
	return nil
}
